package main

import (
	"path/filepath"
	"strings"
)

const defaultURL = "https://zentao.sskuaixiu.com/my-work-bug.html?tid=r6xl1evk"

func defaultConfig() Config {
	return Config{
		URL:                 defaultURL,
		IntervalMinutes:     15,
		EnableNotifications: true,
		EnableSound:         true,
	}
}

func sanitizeConfig(cfg Config) Config {
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.Cookie = strings.TrimSpace(cfg.Cookie)
	if cfg.URL == "" {
		cfg.URL = defaultURL
	}
	if cfg.IntervalMinutes < 1 {
		cfg.IntervalMinutes = 1
	}
	if cfg.IntervalMinutes > 60 {
		cfg.IntervalMinutes = 60
	}
	return cfg
}

func (a *App) loadConfig() error {
	path := filepath.Join(a.ensureDataDir(), configFileName)
	cfg := defaultConfig()
	if err := readJSON(path, &cfg); err != nil {
		a.config = cfg
		return nil
	}
	a.config = sanitizeConfig(cfg)
	return nil
}

func (a *App) saveConfig(cfg Config) error {
	path := filepath.Join(a.ensureDataDir(), configFileName)
	return writeJSON(path, cfg)
}

func (a *App) loadState() error {
	path := filepath.Join(a.ensureDataDir(), stateFileName)
	var state State
	if err := readJSON(path, &state); err != nil {
		return nil
	}
	a.stats = state.LastStats
	return nil
}

func (a *App) saveState(stats Stats) error {
	path := filepath.Join(a.ensureDataDir(), stateFileName)
	state := State{LastStats: stats}
	return writeJSON(path, state)
}

func (a *App) loadChangeLog() error {
	path := filepath.Join(a.ensureDataDir(), changeLogFileName)
	var entries []ChangeLogEntry
	if err := readJSON(path, &entries); err != nil {
		return nil
	}
	a.changeLog = entries
	return nil
}

func (a *App) saveChangeLog(entries []ChangeLogEntry) error {
	path := filepath.Join(a.ensureDataDir(), changeLogFileName)
	if entries == nil {
		entries = []ChangeLogEntry{}
	}
	return writeJSON(path, entries)
}
