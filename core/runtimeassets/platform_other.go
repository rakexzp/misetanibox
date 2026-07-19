//go:build !windows

package runtimeassets

import (
	"fmt"
	"os"
	"os/exec"
)

const coreBinName = "clash"

// systemCorePaths — где искать ядро, установленное системным пакетным менеджером.
// Нужно для дистрибутивных сборок (AUR и т.п.), где mihomo ставится зависимостью,
// а своё ядро в пакет не кладётся.
var systemCorePaths = []string{
	"/usr/bin/mihomo",
	"/usr/local/bin/mihomo",
	"/usr/bin/clash-meta",
	"/usr/bin/clash",
}

// findSystemCore возвращает путь к системному ядру mihomo или "" если его нет.
func findSystemCore() string {
	for _, p := range systemCorePaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			if validateCoreBinary(p) == nil {
				return p
			}
		}
	}
	for _, name := range []string{"mihomo", "clash-meta"} {
		if p, err := exec.LookPath(name); err == nil {
			if validateCoreBinary(p) == nil {
				return p
			}
		}
	}
	return ""
}
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
