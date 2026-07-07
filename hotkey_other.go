//go:build !windows

package main

import "context"

// startHotkeyWorker — глобальный хоткей на Linux пока не реализован (no-op)
func (a *App) startHotkeyWorker(ctx context.Context) {}

// stopHotkey — no-op на Linux
func (a *App) stopHotkey() {}
