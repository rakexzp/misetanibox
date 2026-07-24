//go:build !windows

package runtimeassets

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
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

// validateCoreBinary проверяет, что файл ядра — исполняемый для текущей ОС:
// ELF в Linux, Mach-O в macOS (иначе штатное ядро под mac браковалось как «не исполняемый»).
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
	if runtime.GOOS == "darwin" {
		if isMachO(magic) {
			return nil
		}
		return fmt.Errorf("файл ядра не является исполняемым macOS (Mach-O)")
	}
	if magic[0] != 0x7f || magic[1] != 'E' || magic[2] != 'L' || magic[3] != 'F' {
		return fmt.Errorf("файл ядра не является ELF-исполняемым Linux")
	}
	return nil
}

// isMachO распознаёт Mach-O (32/64-бит, обе эндианности) и universal binary (fat).
func isMachO(m []byte) bool {
	be := uint32(m[0])<<24 | uint32(m[1])<<16 | uint32(m[2])<<8 | uint32(m[3])
	le := uint32(m[3])<<24 | uint32(m[2])<<16 | uint32(m[1])<<8 | uint32(m[0])
	for _, magic := range []uint32{
		0xfeedface, // Mach-O 32
		0xfeedfacf, // Mach-O 64
		0xcafebabe, // universal (fat)
		0xcafebabf, // universal 64
	} {
		if be == magic || le == magic {
			return true
		}
	}
	return false
}
