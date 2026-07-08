//go:build !windows

package sys

import "os/exec"

func ShellOpen(path string) error {
	return exec.Command("xdg-open", path).Start()
}
