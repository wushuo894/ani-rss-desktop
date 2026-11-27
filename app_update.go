package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/plugins/updater"
)

// serviceResume 保存退出前实际生效的启动参数，避免应用尚未生效的设置。
type serviceResume struct {
	Settings Settings `json:"settings"`
	JDK      JDK      `json:"jdk"`
}

// CheckDesktopUpdate 打开桌面端更新窗口，检查、安装和重启由 MyGo 负责。
func (d *Desktop) CheckDesktopUpdate() { updater.CheckForUpdates() }

// shutdownForUpdate 等待管理操作结束，在安装新版后保存服务状态并停止 Java。
func (d *Desktop) shutdownForUpdate(installed bool) {
	d.cancel()
	d.op.Lock()
	defer d.op.Unlock()
	if installed {
		d.mu.Lock()
		var resume *serviceResume
		if d.proc != nil {
			resume = &serviceResume{Settings: d.proc.settings, JDK: d.proc.jdk}
		}
		d.mu.Unlock()
		// 只在真正退出时取快照，用户安装更新后仍可继续启动或停止服务。
		if resume != nil {
			if err := writeJSON(filepath.Join(d.root, "update-resume.json"), resume); err != nil {
				log.Printf("保存桌面端更新恢复状态失败：%v", err)
			}
		}
	}
	_ = d.stop()
}

// resumeAfterUpdate 消费一次恢复记录；原先停止的服务不会因桌面端更新而启动。
func (d *Desktop) resumeAfterUpdate() (bool, error) {
	path := filepath.Join(d.root, "update-resume.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("读取更新恢复状态失败：%w", err)
	}
	// 启动前移除记录，防止启动失败后每次打开桌面端都重复启动。
	if err := os.Remove(path); err != nil {
		return true, fmt.Errorf("清理更新恢复状态失败：%w", err)
	}
	var resume serviceResume
	if err := json.Unmarshal(raw, &resume); err != nil {
		return true, fmt.Errorf("解析更新恢复状态失败：%w", err)
	}
	err = d.action("恢复更新前的服务", func() error {
		return d.startWith(resume.Settings, resume.JDK)
	})
	return true, err
}
