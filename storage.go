package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configFileName    = "config.json"
	stateFileName     = "state.json"
	changeLogFileName = "changelog.json"
)

func (a *App) ensureDataDir() string {
	if a.dataDir != "" {
		return a.dataDir
	}
	baseDir, err := os.UserConfigDir()
	if err != nil {
		baseDir = "."
	}
	path := filepath.Join(baseDir, "ZenTaoBugMonitor")
	_ = os.MkdirAll(path, 0o755)
	a.dataDir = path
	return path
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeJSON(path string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
