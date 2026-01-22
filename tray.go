package main

import (
	"runtime"
	"time"

	"github.com/getlantern/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startTray() {
	a.mu.Lock()
	if a.trayStarted {
		a.mu.Unlock()
		return
	}
	a.trayStarted = true
	a.mu.Unlock()
	go a.trayLoop()
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

func (a *App) trayLoop() {
	for {
		runtime.LockOSThread()
		systray.Run(a.onTrayReady, a.onTrayExit)
		runtime.UnlockOSThread()
		if a.shouldStopTray() {
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIconICO)
	systray.SetTitle("禅道监控")
	systray.SetTooltip("禅道监控")

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
		wailsRuntime.WindowShow(a.ctx)
		wailsRuntime.WindowCenter(a.ctx)
		a.setWindowHidden(false)
		return
	}
	wailsRuntime.WindowHide(a.ctx)
	a.setWindowHidden(true)
}

func (a *App) quitFromTray() {
	a.mu.Lock()
	a.quitting = true
	a.mu.Unlock()
	if a.ctx != nil {
		wailsRuntime.Quit(a.ctx)
	}
}

func (a *App) setWindowHidden(hidden bool) {
	a.mu.Lock()
	a.windowHidden = hidden
	a.mu.Unlock()
}

func (a *App) shouldStopTray() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.quitting || !a.trayStarted
}
