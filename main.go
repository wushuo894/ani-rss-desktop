package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
)

//go:embed resources/tray.png
var trayIcon []byte

// main 绑定前端服务，并接入单实例、窗口、托盘和登录自启。
func main() {
	if os.Getenv("MYGO_GENERATE") != "" {
		mygo.Bind(&Desktop{})
		if err := mygo.App.Run(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	desktop, err := newDesktop()
	if err != nil {
		mygo.App.WhenReady(func() {
			showError("无法读取桌面端配置", err)
			mygo.App.Quit()
		})
		if err := mygo.App.Run(); err != nil {
			log.Fatal(err)
		}
		return
	}
	mygo.Bind(desktop)
	// 桌面端更新使用独立发行源和框架弹窗，与 ANI-RSS JAR 更新分别处理。
	mygo.Use(updater.New(updater.Options{Language: "zh-Hans"}))
	var updateInstalled atomic.Bool
	updater.OnInstalled(func() { updateInstalled.Store(true) })

	var window *mygo.Window
	var tray *trayPanel
	var closing atomic.Bool
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) { closing.Store(true) })
	mygo.App.OnQuitCanceled(func() { closing.Store(false) })
	// OnQuit 在框架启动新版之前执行；不取消退出，避免丢失 Relaunch 请求。
	mygo.App.OnQuit(func() {
		desktop.shutdownForUpdate(updateInstalled.Load())
	})
	mygo.App.OnSecondInstance(func(_ []string, _ string) { showWindow(window) })
	mygo.App.OnActivate(func(visible bool) {
		if !visible && window != nil && !closing.Load() && (tray == nil || tray.panel == nil) {
			showWindow(window)
		}
	})
	mygo.App.WhenReady(func() {
		atLogin := mygo.App.WasOpenedAtLogin()
		// 默认使用系统完整窗口装饰，只有 macOS 启用沉浸式标题栏。
		opts := mygo.WindowOptions{
			Title: "ani-rss-desktop", URL: "/", Width: 1080, Height: 760,
			MinWidth: 680, MinHeight: 520, Hidden: atLogin,
			BackgroundColor: "light-dark(#faf7f2, #191411)",
		}
		if runtime.GOOS == "darwin" {
			// macOS 页面延伸到标题栏下方，并保留系统原生窗口按钮。
			opts.TitleBarStyle = mygo.TitleBarHidden
			opts.TitleBarHeight = 44
		}
		window = mygo.NewWindow(opts)
		desktop.mu.Lock()
		desktop.mainWindow = window
		desktop.mu.Unlock()
		tray, err = installTray(desktop)
		if err == nil {
			window.OnClose(func(event *mygo.CloseEvent) {
				if !closing.Load() {
					event.PreventDefault()
					window.Hide()
				}
			})
			go watchTray(desktop, tray.tray)
		} else if atLogin {
			showWindow(window)
		}
		go func() {
			resumed, err := desktop.resumeAfterUpdate()
			if !resumed && err == nil && atLogin {
				err = desktop.Start()
			}
			if err != nil {
				showWindow(window)
				showError("ani-rss 启动失败", err)
			}
		}()
	})
	err = mygo.App.Run()
	desktop.shutdown()
	if err != nil {
		log.Fatal(err)
	}
}

// showWindow 将隐藏窗口恢复到前台。
func showWindow(window *mygo.Window) {
	if window != nil {
		window.Show()
		window.Focus()
	}
}

// showError 在系统对话框显示后台操作错误。
func showError(title string, err error) {
	_, _ = mygo.Dialog.Message(mygo.MessageOptions{Title: title, Message: err.Error()})
}

// PickPath 打开数据目录、JDK 目录或 java 可执行文件选择器。
func (d *Desktop) PickPath(kind string) (string, error) {
	opts := mygo.OpenDialogOptions{Directory: true, TreatPackagesAsDirectories: true}
	switch kind {
	case "data":
		opts.Title = "选择 ani-rss 配置与数据目录"
		opts.DefaultPath = d.GetSettings().DataDir
		opts.CreateDirectories = true
	case "jdk":
		opts.Title = "选择 JDK 目录"
	case "java":
		opts.Title = "选择 java 可执行文件"
		opts.Directory = false
	default:
		return "", fmt.Errorf("未知的路径类型：%s", kind)
	}
	entries, err := mygo.Dialog.Open(opts)
	if err != nil || len(entries) == 0 {
		return "", err
	}
	return entries[0], nil
}

// OpenWeb 使用实际生效的地址在外部浏览器打开 Web UI 或 Swagger。
func (d *Desktop) OpenWeb(page string) error {
	base, err := d.runningURL()
	if err != nil {
		return err
	}
	switch page {
	case "ui":
		return mygo.Shell.OpenExternal(base)
	case "swagger":
		if !d.GetStatus().Swagger {
			return fmt.Errorf("请启用 Swagger 并重启服务")
		}
		return mygo.Shell.OpenExternal(base + "/swagger-ui/index.html")
	default:
		return fmt.Errorf("未知的页面：%s", page)
	}
}

// OpenLogDirectory 打开最近生效的数据目录下的日志文件夹。
func (d *Desktop) OpenLogDirectory() error {
	path := d.GetStatus().RunningDir
	if path == "" {
		path = d.GetSettings().DataDir
	}
	return mygo.Shell.OpenPath(filepath.Join(path, "logs"))
}

// CopyText 通过系统剪贴板复制日志或接入地址。
func (d *Desktop) CopyText(text string) { mygo.Clipboard.WriteText(text) }

// GetAutoStart 读取操作系统当前的用户登录项状态。
func (d *Desktop) GetAutoStart() bool { return mygo.App.OpenAtLogin() }

// SetAutoStart 将本应用注册为用户登录项或取消注册。
func (d *Desktop) SetAutoStart(enabled bool) error { return mygo.App.SetOpenAtLogin(enabled) }
