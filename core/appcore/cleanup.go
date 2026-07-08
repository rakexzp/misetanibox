package appcore

import (
	"goclashz/core/logger"
	"goclashz/core/utils"
	"goclashz/core/version"
	"os"
	"path/filepath"
)

func CleanLegacyFiles(currentAppVersion string) {
	binDir := utils.GetCoreBinDir()
	_ = os.Remove(filepath.Join(binDir, "active_config.txt"))
	_ = os.Remove(filepath.Join(binDir, "active_mode.txt"))

	_ = os.Remove(filepath.Join(binDir, "mihomo-windows-amd64.exe.old"))
	_ = os.Remove(filepath.Join(binDir, "clash.exe.old"))

	updateTmp := filepath.Join(utils.GetDataDir(), "GoclashZ_update.exe.tmp")
	updateExe := filepath.Join(utils.GetDataDir(), "GoclashZ_update.exe")
	updateVer := filepath.Join(utils.GetDataDir(), "GoclashZ_update.version")
	_ = os.Remove(updateTmp)

	if cachedVer, err := os.ReadFile(updateVer); err == nil {
		if version.NormalizeVersion(string(cachedVer)) == version.NormalizeVersion(currentAppVersion) {
			_ = os.Remove(updateExe)
			_ = os.Remove(updateVer)
		}
	}

	if tmpFiles, err := filepath.Glob(filepath.Join(binDir, "*.tmp")); err == nil {
		for _, f := range tmpFiles {
			_ = os.Remove(f)
		}
	}
	if zipFiles, err := filepath.Glob(filepath.Join(binDir, "*.zip")); err == nil {
		for _, f := range zipFiles {
			_ = os.Remove(f)
		}
	}
	if metaFiles, err := filepath.Glob(filepath.Join(binDir, "*.meta.json")); err == nil {
		for _, f := range metaFiles {
			_ = os.Remove(f)
		}
	}
}

func moveOrCopyFile(oldPath, newPath string) error {
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return err
	}

	if err := os.Rename(oldPath, newPath); err == nil {
		return nil
	}

	data, err := os.ReadFile(oldPath)
	if err != nil {
		return err
	}

	if err := utils.WriteFileAtomic(newPath, data, 0644); err != nil {
		return err
	}

	return os.Remove(oldPath)
}

func MigrateLegacyRootSettings() {
	mappings := map[string]string{
		"behavior.json": "user_behavior.json",
		"dns.json":      "user_dns.json",
		"network.json":  "user_network.json",
		"tun.json":      "user_tun.json",
	}

	settingsDir := utils.GetSettingsDir()
	_ = os.MkdirAll(settingsDir, 0755)

	for oldName, newName := range mappings {
		oldPath := filepath.Join(utils.GetDataDir(), oldName)
		newPath := filepath.Join(settingsDir, newName)

		if _, err := os.Stat(oldPath); err != nil {
			continue
		}

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			if err := moveOrCopyFile(oldPath, newPath); err != nil {
				logger.Warnf("Не удалось перенести старые настройки %s -> %s: %v", oldPath, newPath, err)
				continue
			}
		} else {

			_ = os.Remove(oldPath)
		}
	}
}
