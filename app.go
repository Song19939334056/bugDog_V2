package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxLogEntries = 200

// App struct
type App struct {
	ctx         context.Context
	mu          sync.Mutex
	config      Config
	stats       Stats
	changeLog   []ChangeLogEntry
	logEntries  []LogEntry
	dataDir     string
	pollerStop  chan struct{}
	scrapeGate  chan struct{}
	quitting    bool
	windowHidden bool
	monitoringEnabled bool
	httpClient  *http.Client
	trayStarted bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		config:            defaultConfig(),
		scrapeGate:        make(chan struct{}, 1),
		monitoringEnabled: true,
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.httpClient = &http.Client{Timeout: 25 * time.Second}
	_ = a.loadConfig()
	_ = a.loadState()
	_ = a.loadChangeLog()
	a.startPolling()
	a.resizeToScreen()
	a.emitAll()
	a.startTray()
	a.setWindowHidden(false)
	go a.FetchNow()
}

func (a *App) shutdown(ctx context.Context) {
	a.stopPolling()
	a.stopTray()
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	quitting := a.quitting
	a.mu.Unlock()
	if quitting {
		return false
	}
	runtime.WindowHide(ctx)
	a.setWindowHidden(true)
	return true
}

func (a *App) GetConfig() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config
}

func (a *App) GetMonitoringStatus() bool {
	return a.isMonitoringEnabled()
}

func (a *App) StopMonitoring() {
	a.setMonitoringEnabled(false)
}

func (a *App) StartMonitoring() {
	a.setMonitoringEnabled(true)
}

func (a *App) SaveConfig(cfg Config) error {
	cfg = sanitizeConfig(cfg)
	a.mu.Lock()
	a.config = cfg
	a.mu.Unlock()
	if err := a.saveConfig(cfg); err != nil {
		return err
	}
	if a.isMonitoringEnabled() {
		a.startPolling()
	}
	a.emitConfig()
	if a.isMonitoringEnabled() {
		go a.FetchNow()
	}
	return nil
}

func (a *App) GetStats() Stats {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stats
}

func (a *App) GetChangeLog() []ChangeLogEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.changeLog) == 0 {
		return []ChangeLogEntry{}
	}
	return append([]ChangeLogEntry(nil), a.changeLog...)
}

func (a *App) GetLogs() []LogEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.logEntries) == 0 {
		return []LogEntry{}
	}
	return append([]LogEntry(nil), a.logEntries...)
}

func (a *App) FetchNow() error {
	select {
	case a.scrapeGate <- struct{}{}:
		defer func() { <-a.scrapeGate }()
	default:
		a.addLog("info", "Scrape skipped: previous sync still running", 0)
		return nil
	}

	cfg := a.GetConfig()
	if cfg.URL == "" {
		return errors.New("missing URL")
	}

	a.addLog("info", fmt.Sprintf("Scraping %s", cfg.URL), 0)
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	stats, status, err := a.scrape(ctx, cfg)
	if err != nil {
		a.addLog("error", fmt.Sprintf("Scrape failed: %v", err), status)
		return err
	}
	a.addLog("info", fmt.Sprintf("HTTP %d - parsed %d bugs", status, stats.Total), status)

	var previous Stats
	var notify bool
	var delta int
	a.mu.Lock()
	previous = a.stats
	a.stats = stats
	notify = previous.Total != 0 && previous.Total != stats.Total
	if notify {
		delta = stats.Total - previous.Total
	}
	a.mu.Unlock()

	_ = a.saveState(stats)
	a.emitStats()

	if notify {
		entry := ChangeLogEntry{
			Timestamp: stats.LastUpdated,
			Total:     stats.Total,
			Delta:     delta,
			Severity:  stats.Severity,
		}
		a.addChangeLog(entry)
		a.emitChangeLog()
		a.maybeNotifyChange(entry)
	}

	return nil
}

func (a *App) TestNotification() error {
	a.sendNotification("ZenTao Bug Monitor", "测试通知：系统通知与声音已触发。")
	a.playSound(true)
	a.addLog("info", "Test notification triggered", 0)
	a.emitLogs()
	return nil
}

func (a *App) ClearChangeLog() error {
	a.mu.Lock()
	a.changeLog = nil
	a.mu.Unlock()
	_ = a.saveChangeLog(nil)
	_ = os.Remove(filepath.Join(a.ensureDataDir(), changeLogFileName))
	a.emitChangeLog()
	return nil
}

func (a *App) ClearMonitoringData() error {
	a.mu.Lock()
	a.stats = Stats{}
	a.changeLog = nil
	a.logEntries = nil
	a.mu.Unlock()
	a.emitAll()
	_ = a.saveChangeLog(nil)
	_ = os.Remove(filepath.Join(a.ensureDataDir(), changeLogFileName))
	_ = os.Remove(filepath.Join(a.ensureDataDir(), stateFileName))
	return nil
}

func (a *App) startPolling() {
	a.stopPolling()
	if !a.isMonitoringEnabled() {
		return
	}
	cfg := a.GetConfig()
	intervalMinutes := cfg.IntervalMinutes
	interval := time.Duration(intervalMinutes) * time.Minute
	if interval <= 0 {
		intervalMinutes = 15
		interval = 15 * time.Minute
	}
	stop := make(chan struct{})
	a.mu.Lock()
	a.pollerStop = stop
	a.mu.Unlock()
	a.addLog("info", fmt.Sprintf("Polling started: every %d minutes", intervalMinutes), 0)
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				a.addLog("info", fmt.Sprintf("Auto sync triggered (every %d minutes)", intervalMinutes), 0)
				_ = a.FetchNow()
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (a *App) stopPolling() {
	a.mu.Lock()
	stop := a.pollerStop
	a.pollerStop = nil
	a.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

func (a *App) resizeToScreen() {
	screens, err := runtime.ScreenGetAll(a.ctx)
	if err != nil || len(screens) == 0 {
		return
	}
	screen := screens[0]
	for _, candidate := range screens {
		if candidate.IsPrimary {
			screen = candidate
			break
		}
	}
	width := screen.Size.Width / 2
	height := screen.Size.Height / 2
	if width < 960 {
		width = 960
	}
	if height < 720 {
		height = 720
	}
	runtime.WindowSetSize(a.ctx, width, height)
	runtime.WindowCenter(a.ctx)
}

func (a *App) addLog(level, message string, status int) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Status:    status,
		Message:   message,
	}
	a.mu.Lock()
	a.logEntries = append([]LogEntry{entry}, a.logEntries...)
	if len(a.logEntries) > maxLogEntries {
		a.logEntries = a.logEntries[:maxLogEntries]
	}
	a.mu.Unlock()
	a.emitLogs()
}

func (a *App) addChangeLog(entry ChangeLogEntry) {
	a.mu.Lock()
	a.changeLog = append([]ChangeLogEntry{entry}, a.changeLog...)
	if len(a.changeLog) > 200 {
		a.changeLog = a.changeLog[:200]
	}
	entries := append([]ChangeLogEntry(nil), a.changeLog...)
	a.mu.Unlock()
	_ = a.saveChangeLog(entries)
}

func (a *App) emitAll() {
	a.emitConfig()
	a.emitStats()
	a.emitChangeLog()
	a.emitLogs()
	a.emitMonitoring()
}

func (a *App) emitConfig() {
	runtime.EventsEmit(a.ctx, "config", a.GetConfig())
}

func (a *App) emitStats() {
	runtime.EventsEmit(a.ctx, "stats", a.GetStats())
}

func (a *App) emitChangeLog() {
	runtime.EventsEmit(a.ctx, "changelog", a.GetChangeLog())
}

func (a *App) emitLogs() {
	runtime.EventsEmit(a.ctx, "logs", a.GetLogs())
}

func (a *App) emitMonitoring() {
	runtime.EventsEmit(a.ctx, "monitoring", a.isMonitoringEnabled())
}

func (a *App) maybeNotifyChange(entry ChangeLogEntry) {
	cfg := a.GetConfig()
	if cfg.EnableNotifications {
		title := "ZenTao Bug Monitor"
		message := fmt.Sprintf("缺陷数量变化：%+d，当前总数 %d", entry.Delta, entry.Total)
		a.sendNotification(title, message)
	}
	if cfg.EnableSound {
		a.playSound(false)
	}
}

func (a *App) playSound(force bool) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "play-sound", force)
}

func (a *App) setMonitoringEnabled(enabled bool) {
	a.mu.Lock()
	if a.monitoringEnabled == enabled {
		a.mu.Unlock()
		return
	}
	a.monitoringEnabled = enabled
	a.mu.Unlock()

	if enabled {
		a.addLog("info", "Monitoring resumed", 0)
		a.startPolling()
	} else {
		a.stopPolling()
		a.addLog("info", "Monitoring paused", 0)
	}
	a.emitMonitoring()
}

func (a *App) isMonitoringEnabled() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.monitoringEnabled
}
