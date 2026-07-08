package runtimeassets

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goclashz/core/utils"
)

func MigrateLegacyAssets() {
	targetDir := utils.GetCoreBinDir()
	_ = os.MkdirAll(targetDir, 0755)

	appDir := utils.GetAppDir()

	legacyDirs := []string{
		filepath.Join(appDir, "core", "bin"),
		utils.GetLegacyDataCoreBinDir(),
		utils.GetDataDir(),
	}

	assets := []string{
		"clash.exe", "wintun.dll",
		"geoip.metadb", "geosite.dat", "country.mmdb", "asn.dat",
	}

	backupDir := filepath.Join(utils.GetDataDir(), "backups", "legacy-migrate", time.Now().Format("20060102150405"))

	for _, name := range assets {
		target := filepath.Join(targetDir, name)

		targetExists := false
		var targetHash string
		if st, err := os.Stat(target); err == nil && !st.IsDir() {
			targetExists = true
			targetHash, _ = calculateSHA256(target)
		}

		isCore := name == "clash.exe" || name == "wintun.dll"

		for _, legacyDir := range legacyDirs {
			oldPath := filepath.Join(legacyDir, name)
			if _, err := os.Stat(oldPath); err != nil {
				continue
			}

			if targetExists {
				if isCore {

					oldHash, _ := calculateSHA256(oldPath)
					if oldHash != "" && oldHash == targetHash {
						_ = os.Remove(oldPath)
					} else {
						_ = os.MkdirAll(backupDir, 0755)
						backupPath := filepath.Join(backupDir, name)
						_ = os.Rename(oldPath, backupPath)
						if _, err := os.Stat(backupPath); err != nil {
							if data, err := os.ReadFile(oldPath); err == nil {
								_ = os.WriteFile(backupPath, data, 0755)
								_ = os.Remove(oldPath)
							}
						}
					}
				} else {

					_ = os.Remove(oldPath)
				}
				continue
			}

			if err := os.Rename(oldPath, target); err == nil {
				targetExists = true
				break
			}

			if data, err := os.ReadFile(oldPath); err == nil {
				if err := os.WriteFile(target, data, 0755); err == nil {
					_ = os.Remove(oldPath)
					targetExists = true
					break
				}
			}
		}
	}
}

func CleanupLegacyAssets() {
	appDir := utils.GetAppDir()
	dataDir := utils.GetDataDir()

	isDev := false
	if _, err := os.Stat(filepath.Join(appDir, "core", "appcore", "controller.go")); err == nil {
		isDev = true
	}

	legacyAssets := []string{
		"clash.exe", "wintun.dll",
		"geoip.metadb", "geosite.dat", "country.mmdb", "asn.dat",
		"clash.exe.old", "wintun.dll.old",
		"geoip.metadb.old", "geosite.dat.old", "country.mmdb.old", "asn.dat.old",
	}

	backupDir := filepath.Join(dataDir, "backups", "legacy-core", time.Now().Format("20060102150405"))

	cleanOrBackupFile := func(path string, activePath string) {
		if _, err := os.Stat(path); err != nil {
			return
		}

		same := false
		h1, err1 := calculateSHA256(path)
		h2, err2 := calculateSHA256(activePath)
		if err1 == nil && err2 == nil && h1 == h2 {
			same = true
		}

		if same {
			if err := os.Remove(path); err == nil {
				log.Printf("[runtimeassets] удалён устаревший остаточный файл с тем же именем: %s", path)
				return
			}
		}

		_ = os.MkdirAll(backupDir, 0755)
		backupPath := filepath.Join(backupDir, filepath.Base(path))
		if err := os.Rename(path, backupPath); err == nil {
			log.Printf("[runtimeassets] конфликтующий старый ресурс перемещён в резервную копию: %s -> %s", path, backupPath)
		} else {
			_ = os.Remove(path)
		}
	}

	appCoreBin := filepath.Join(appDir, "core", "bin")
	for _, name := range legacyAssets {
		cleanOrBackupFile(filepath.Join(appCoreBin, name), filepath.Join(utils.GetCoreBinDir(), name))
	}
	_ = os.Remove(appCoreBin)

	if !isDev {
		appCore := filepath.Join(appDir, "core")
		_ = os.Remove(appCore)
	}

	for _, name := range legacyAssets {
		cleanOrBackupFile(filepath.Join(dataDir, name), filepath.Join(utils.GetCoreBinDir(), name))
	}

	if !strings.EqualFold(dataDir, filepath.Join(appDir, "data")) {
		legacyDataCoreBin := filepath.Join(appDir, "data", "core", "bin")
		for _, name := range legacyAssets {
			cleanOrBackupFile(filepath.Join(legacyDataCoreBin, name), filepath.Join(utils.GetCoreBinDir(), name))
		}
		_ = os.Remove(legacyDataCoreBin)
		_ = os.Remove(filepath.Join(appDir, "data", "core"))
		_ = os.Remove(filepath.Join(appDir, "data"))
	}
}
