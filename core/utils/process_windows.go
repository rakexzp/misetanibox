//go:build windows

package utils

import (
	"os/exec"
	"syscall"
)

func HideCommandWindow(cmd *exec.Cmd, extraFlags uint32) {
	if cmd == nil {
		return
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.HideWindow = true

	cmd.SysProcAttr.CreationFlags = 0x08000000 | extraFlags
}
