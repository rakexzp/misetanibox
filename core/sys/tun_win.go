package sys

import (
	"io"
	"os"
	"path/filepath"

	"goclashz/core/utils"
)

func GetWintunPath() string {
	return filepath.Join(utils.GetCoreBinDir(), "wintun.dll")
}

func IsWintunInstalled() bool {
	path := GetWintunPath()
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}

	if info.Size() < 32*1024 || info.Size() > 5*1024*1024 {
		return false
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	header := make([]byte, 2)
	if _, err := io.ReadFull(f, header); err != nil {
		return false
	}
	return header[0] == 'M' && header[1] == 'Z'
}
