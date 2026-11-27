package main

import (
	"context"
	"runtime"
	"time"

	"github.com/egoist/mygo"
)

// trayPanel 的窗口生命周期只在 MyGo 主线程操作，关闭后释放网页与轮询任务。
type trayPanel struct {
	tray  *mygo.Tray
	panel *mygo.Window
}

// installTray 统一使用 Flyout 展示操作；Linux 仅保留 AppIndicator 所需的入口。
func installTray(d *Desktop) (*trayPanel, error) {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID: "main", Icon: trayIcon, IconIsTemplate: runtime.GOOS == "darwin",
		ToolTip: "ani-rss-desktop",
	})
	if err != nil {
		return nil, err
	}
	p := &trayPanel{tray: tray}
	if runtime.GOOS == "linux" {
		// AppIndicator 没有可靠的单击事件与图标位置，菜单只负责打开同一个面板。
		tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Label: "打开快捷面板", Click: func(*mygo.MenuItem, *mygo.Window) { p.toggle() }},
		}))
	} else {
		tray.OnClick(p.toggle)
		tray.OnRightClick(p.toggle)
	}
	return p, nil
}

// toggle 复用同一入口打开或关闭面板，使用独立页面避免加载整个主窗口。
func (p *trayPanel) toggle() {
	if p.panel != nil {
		p.panel.Close()
		return
	}
	panel := mygo.NewFlyout(mygo.FlyoutOptions{
		Tray: p.tray, Placement: mygo.PlacementBottom, Gap: 8,
		Width: 360, Height: 420, FitContent: true,
		MinWidth: 360, MaxWidth: 360, MaxHeight: 640,
		Focusable: true, Hidden: true, Shadow: true, Popover: true,
		// 原生窗口同步裁剪圆角；Linux 由透明页面中的 CSS 绘制同样的轮廓。
		CornerRadius: 16,
		URL:          "/tray.html",
	})
	p.panel = panel
	// 页面完成首帧后再显示；创建时已登记 panel，应用激活不会误弹主窗口。
	panel.OnReadyToShow(panel.Show)
	panel.OnClosed(func() { p.panel = nil })
}

// ShowMainWindow 从快捷面板回到主窗口，并收起调用它的面板。
func (d *Desktop) ShowMainWindow(ctx context.Context) {
	d.mu.Lock()
	window := d.mainWindow
	d.mu.Unlock()
	if caller := mygo.CallerWindow(ctx); caller != nil && caller != window {
		caller.Close()
	}
	showWindow(window)
}

// QuitDesktop 走原有退出流程，确保关闭面板时仍会停止托管的 Java 服务。
func (d *Desktop) QuitDesktop() { mygo.App.Quit() }

// watchTray 只更新当前应用的提示，不重载系统扩展或重建托盘。
func watchTray(d *Desktop, tray *mygo.Tray) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	labels := map[string]string{"running": "运行中", "starting": "启动中", "stopping": "停止中", "stopped": "已停止", "failed": "启动失败"}
	var previous string
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			state := d.GetStatus().State
			if state != previous {
				tray.SetToolTip("ani-rss · " + labels[state])
				previous = state
			}
		}
	}
}
