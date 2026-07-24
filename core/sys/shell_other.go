//go:build !windows

package sys

import (
	"os/exec"
	"runtime"
)

// ShellOpen открывает файл/ссылку системным обработчиком.
// В macOS это `open`, в Linux — `xdg-open`.
func ShellOpen(path string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, path).Start()
}
