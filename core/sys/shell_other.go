//go:build !windows

package sys

import "os/exec"

// ShellOpen открывает путь/URL в системном обработчике (Linux: xdg-open).
func ShellOpen(path string) error {
	return exec.Command("xdg-open", path).Start()
}
