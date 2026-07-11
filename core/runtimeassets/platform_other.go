//go:build !windows

package runtimeassets

import (
	"fmt"
	"os"
)

const coreBinName = "clash"
const wintunNeeded = false // на Linux TUN идёт через ядро ОС, wintun не нужен

// validateCoreBinary проверяет, что файл ядра — ELF-исполняемый Linux.
func validateCoreBinary(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		return err
	}
	if magic[0] != 0x7f || magic[1] != 'E' || magic[2] != 'L' || magic[3] != 'F' {
		return fmt.Errorf("файл ядра не является ELF-исполняемым Linux")
	}
	return nil
}
