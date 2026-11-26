package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const minJava = 25

// Settings 保存桌面端下次启动 ani-rss 时使用的参数。
type Settings struct {
	DataDir string   `json:"dataDir"`
	Port    int      `json:"port"`
	Address string   `json:"address"`
	MinHeap int      `json:"minHeap"`
	MaxHeap int      `json:"maxHeap"`
	JVMArgs []string `json:"jvmArgs"`
	Swagger bool     `json:"swagger"`
	MCP     bool     `json:"mcp"`
	JDKID   string   `json:"jdkId"`
}

type storage struct {
	Settings    Settings `json:"settings"`
	JDKs        []JDK    `json:"jdks"`
	IgnoredJDKs []string `json:"ignoredJdks,omitempty"`
}

type Status struct {
	State      string `json:"state"`
	Message    string `json:"message"`
	URL        string `json:"url"`
	Version    string `json:"version"`
	RunningJDK string `json:"runningJdk"`
	RunningDir string `json:"runningDir"`
	Swagger    bool   `json:"swagger"`
	MCP        bool   `json:"mcp"`
	Operation  string `json:"operation"`
	Received   int64  `json:"received"`
	Total      int64  `json:"total"`
}

// Desktop 是前端调用的本机管理服务。op 串行化配置和进程操作，mu 保护界面快照。
type Desktop struct {
	root    string
	ctx     context.Context
	cancel  context.CancelFunc
	op      sync.Mutex
	mu      sync.Mutex
	data    storage
	status  Status
	proc    *javaRun
	logText []byte
}

var errBusy = errors.New("有其他管理操作正在进行")

// newDesktop 从当前用户主目录加载设置，并恢复上次遗留的 JAR 备份。
func newDesktop() (*Desktop, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(home, "ani-rss-desktop")
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	d := &Desktop{root: root, ctx: ctx, cancel: cancel}
	d.status.State = "stopped"
	d.data = storage{Settings: Settings{
		DataDir: filepath.Join(home, "ani-rss"), Port: 7789, Address: "0.0.0.0",
		MinHeap: 64, MaxHeap: 512, JVMArgs: []string{},
	}, JDKs: []JDK{}}

	raw, err := os.ReadFile(filepath.Join(root, "settings.json"))
	switch {
	case err == nil:
		err = json.Unmarshal(raw, &d.data)
	case errors.Is(err, os.ErrNotExist):
		err = d.save(d.data)
	}
	if err != nil {
		cancel()
		return nil, fmt.Errorf("读取桌面端配置失败：%w", err)
	}
	if _, err := os.Stat(d.jar()); errors.Is(err, os.ErrNotExist) {
		backup := d.jar() + ".bak"
		if _, err := os.Stat(backup); err == nil {
			if err := os.Rename(backup, d.jar()); err != nil {
				cancel()
				return nil, fmt.Errorf("恢复 JAR 失败：%w", err)
			}
		}
	}
	d.status.Version, _ = jarVersion(d.jar())
	// 自动发现失败时保留已有配置，仍可在 JDK 管理中手动添加。
	_ = d.discoverJDKs()
	return d, nil
}

// action 拒绝重叠的修改操作，并向前端提供进度状态。
func (d *Desktop) action(name string, run func() error) error {
	if !d.op.TryLock() {
		return errBusy
	}
	defer d.op.Unlock()
	if err := d.ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	d.status.Operation = name
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.status.Operation, d.status.Received, d.status.Total = "", 0, 0
		d.mu.Unlock()
	}()
	return run()
}

// snapshot 返回私有配置副本，调用者可以独立修改其中的列表。
func (d *Desktop) snapshot() storage {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := d.data
	result.JDKs = slices.Clone(result.JDKs)
	result.IgnoredJDKs = slices.Clone(result.IgnoredJDKs)
	result.Settings.JVMArgs = slices.Clone(result.Settings.JVMArgs)
	return result
}

// save 在同目录写入临时文件，成功替换后再发布内存状态。
func (d *Desktop) save(value storage) error {
	if err := writeJSON(filepath.Join(d.root, "settings.json"), value); err != nil {
		return err
	}
	d.mu.Lock()
	d.data = value
	d.mu.Unlock()
	return nil
}

// writeJSON 在同目录准备内容，再原子替换目标，避免留下半写入的 JSON。
func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(append(raw, '\n'))
	if err == nil {
		err = file.Sync()
	}
	if err := errors.Join(err, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// GetStatus 返回当前进程的状态和后台操作进度。
func (d *Desktop) GetStatus() Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.status
}

// GetSettings 返回已保存的下次启动配置。
func (d *Desktop) GetSettings() Settings { return d.snapshot().Settings }

// SaveSettings 校验并保存配置，可在服务运行时立即重启。
func (d *Desktop) SaveSettings(settings Settings, restart bool) error {
	return d.action("保存设置", func() error {
		settings.DataDir = strings.TrimSpace(settings.DataDir)
		settings.Address = strings.TrimSpace(settings.Address)
		if err := settings.validate(); err != nil {
			return err
		}
		data := d.snapshot()
		// JDK 通过列表单独保存，避免被打开较早的表单覆盖。
		settings.JDKID = data.Settings.JDKID
		data.Settings = settings
		if err := d.save(data); err != nil {
			return err
		}
		if restart && d.active() {
			return d.restart()
		}
		return nil
	})
}

// validate 校验路径与 Java 启动参数所需的字段。
func (s Settings) validate() error {
	switch {
	case !filepath.IsAbs(s.DataDir):
		return errors.New("请选择绝对路径作为配置与数据目录")
	case s.Port < 1 || s.Port > 65535:
		return errors.New("端口须在 1 到 65535 之间")
	case net.ParseIP(s.Address) == nil:
		return errors.New("请输入有效的 IPv4 或 IPv6 地址")
	case s.MinHeap < 1 || s.MaxHeap < s.MinHeap:
		return errors.New("堆内存须为正数，最大值须大于或等于初始值")
	}
	for _, arg := range s.JVMArgs {
		if !strings.HasPrefix(arg, "-") || slices.Contains([]string{"-jar", "-m", "--module", "--", "-version", "--version", "-help", "--help"}, arg) {
			return fmt.Errorf("无效的 JVM 参数：%s", arg)
		}
	}
	return nil
}

// serviceURL 将未指定绑定地址转换为桌面端可访问的回环地址。
func (s Settings) serviceURL() string {
	address := s.Address
	if ip := net.ParseIP(address); ip != nil && ip.IsUnspecified() {
		address = "::1"
		if ip.To4() != nil {
			address = "127.0.0.1"
		}
	}
	return "http://" + net.JoinHostPort(address, strconv.Itoa(s.Port))
}
