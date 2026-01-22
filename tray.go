package main

import (
	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startTray() {
	a.mu.Lock()
	if a.trayStarted {
		a.mu.Unlock()
		return
	}
	a.trayStarted = true
	a.mu.Unlock()
	go systray.Run(a.onTrayReady, a.onTrayExit)
}

func (a *App) stopTray() {
	a.mu.Lock()
	started := a.trayStarted
	a.trayStarted = false
	a.mu.Unlock()
	if started {
		systray.Quit()
	}
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("ZenTao Bug Monitor")
	systray.SetTooltip("ZenTao Bug Monitor")

	toggleItem := systray.AddMenuItem("显示/隐藏", "切换主窗口")
	syncItem := systray.AddMenuItem("立即同步", "抓取最新缺陷")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("退出", "退出应用")

	go func() {
		for {
			select {
			case <-toggleItem.ClickedCh:
				a.toggleWindow()
			case <-syncItem.ClickedCh:
				_ = a.FetchNow()
			case <-quitItem.ClickedCh:
				a.quitFromTray()
				return
			}
		}
	}()
}

func (a *App) onTrayExit() {}

func (a *App) toggleWindow() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	hidden := a.windowHidden
	a.mu.Unlock()
	if hidden {
		runtime.WindowShow(a.ctx)
		runtime.WindowCenter(a.ctx)
		a.setWindowHidden(false)
		return
	}
	runtime.WindowHide(a.ctx)
	a.setWindowHidden(true)
}

func (a *App) quitFromTray() {
	a.mu.Lock()
	a.quitting = true
	a.mu.Unlock()
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

func (a *App) setWindowHidden(hidden bool) {
	a.mu.Lock()
	a.windowHidden = hidden
	a.mu.Unlock()
}
