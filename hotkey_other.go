//go:build !windows

package main

import "context"

func (a *App) startHotkeyWorker(ctx context.Context) {}

func (a *App) stopHotkey() {}
