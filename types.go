package main

import "time"

type Config struct {
	URL                 string `json:"url"`
	Cookie              string `json:"cookie"`
	IntervalMinutes     int    `json:"intervalMinutes"`
	EnableNotifications bool   `json:"enableNotifications"`
	EnableSound         bool   `json:"enableSound"`
}

type SeverityCounts struct {
	Critical int `json:"critical"`
	Severe   int `json:"severe"`
	Major    int `json:"major"`
	Minor    int `json:"minor"`
}

type Stats struct {
	Total       int            `json:"total"`
	Severity    SeverityCounts `json:"severity"`
	LastUpdated time.Time      `json:"lastUpdated"`
}

type ChangeLogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Total     int            `json:"total"`
	Delta     int            `json:"delta"`
	Severity  SeverityCounts `json:"severity"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Status    int       `json:"status"`
	Message   string    `json:"message"`
}

type State struct {
	LastStats Stats `json:"lastStats"`
}
