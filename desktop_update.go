package main

import (
	"errors"
	"fmt"
	"os"
)

// applyPendingJar 接收 ani-rss DesktopUpdate 交接的已校验 JAR。
// 调用时 Java 已退出，Windows 上也可以安全替换已释放的文件。
func (d *Desktop) applyPendingJar() (string, error) {
	pending := d.jar() + ".pending"
	if _, err := os.Stat(pending); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	version, err := jarVersion(pending)
	if err != nil {
		return "", fmt.Errorf("读取待更新 JAR 失败：%w", err)
	}
	current, _ := jarVersion(d.jar())
	if !newerVersion(version, current) {
		return "", os.Remove(pending)
	}
	backup, err := d.replaceJar(pending)
	if err != nil {
		return "", err
	}
	d.setVersion(version)
	d.appendLog([]byte(fmt.Sprintf("\n已接收 ani-rss 自更新，正在启动 v%s。\n", version)))
	return backup, nil
}

// restoreJar 恢复替换前的版本并刷新当前版本信息。
func (d *Desktop) restoreJar(backup string) error {
	if err := os.Rename(backup, d.jar()); err != nil {
		return fmt.Errorf("恢复旧 JAR 失败：%w", err)
	}
	d.setVersionFromJar()
	return nil
}
