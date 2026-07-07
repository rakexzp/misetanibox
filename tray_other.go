//go:build !windows

package main

import (
	"context"

	"goclashz/core/appcore"
)

// TrayCommand represents a command from the tray menu
type TrayCommand int

const (
	TrayCmdShowMainWindow TrayCommand = iota
	TrayCmdToggleSystemProxy
	TrayCmdToggleTun
	TrayCmdModeRule
	TrayCmdModeGlobal
	TrayCmdModeDirect
	TrayCmdRestartCore
	TrayCmdQuitApp
)

// TrayRuntime is a no-op tray stub: системного трея на Linux пока нет.
type TrayRuntime struct {
	app *App
}

// NewTrayRuntime creates a no-op tray runtime
func NewTrayRuntime(app *App) *TrayRuntime {
	return &TrayRuntime{app: app}
}

// Start is a no-op on Linux
func (t *TrayRuntime) Start(ctx context.Context) {}

// Stop is a no-op on Linux
func (t *TrayRuntime) Stop() {}

// UpdateState is a no-op on Linux
func (t *TrayRuntime) UpdateState(state appcore.AppState) {}

// PostCommand is a no-op on Linux
func (t *TrayRuntime) PostCommand(cmd TrayCommand) {}
