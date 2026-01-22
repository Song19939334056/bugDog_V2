package main

import "github.com/gen2brain/beeep"

func (a *App) sendNotification(title, message string) {
	_ = beeep.Notify(title, message, "")
}
