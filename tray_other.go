//go:build !windows

package main

import (
	"context"

	"goclashz/core/appcore"
)

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

type TrayRuntime struct {
	app *App
}

func NewTrayRuntime(app *App) *TrayRuntime {
	return &TrayRuntime{app: app}
}

func (t *TrayRuntime) Start(ctx context.Context) {}

func (t *TrayRuntime) Stop() {}

func (t *TrayRuntime) UpdateState(state appcore.AppState) {}

func (t *TrayRuntime) PostCommand(cmd TrayCommand) {}
