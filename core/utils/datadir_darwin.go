//go:build darwin

package utils

import (
	"os"
	"path/filepath"
)

// data на macOS — Application Support (Caches система может чистить)
func platformDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "Misetanibox")
}
