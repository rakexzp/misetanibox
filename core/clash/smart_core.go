package clash

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goclashz/core/utils"
)

const (
	smartCoreSettingKey = "smart_core"

	smartMirrorZip   = "https://files.geodema.network/misetani/mihomo-smart-windows.zip"
	smartMirrorModel = "https://files.geodema.network/misetani/Model.bin"

	smartGithubZip   = "https://github.com/vernesong/mihomo/releases/download/Prerelease-Alpha/mihomo-windows-amd64-compatible-alpha-smart-3e709f6.zip"
	smartModelGithub = "https://github.com/vernesong/mihomo/releases/download/LightGBM-Model/Model.bin"
)

func IsSmartCoreActive() bool {
	v, err := utils.LoadSetting(smartCoreSettingKey, false)
	if err != nil || v == nil {
		return false
	}
	return *v
}

func setSmartCoreActive(active bool) error {
	return utils.SaveSetting(smartCoreSettingKey, &active)
}

func stockBackupPath() string { return filepath.Join(utils.GetCoreBinDir(), "clash.stock.exe") }
func coreExePath() string     { return filepath.Join(utils.GetCoreBinDir(), coreExeName()) }
func smartModelPath() string  { return filepath.Join(utils.GetCoreBinDir(), "Model.bin") }

func InstallSmartCore(ctx context.Context) error {
	client := &http.Client{Timeout: 120 * time.Second}

	dir := utils.GetCoreBinDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	zipPath := filepath.Join(dir, "smart-core.zip")
	if err := downloadToFirst(ctx, client, []string{smartMirrorZip, smartGithubZip}, zipPath); err != nil {
		return fmt.Errorf("не удалось скачать смарт-ядро: %w", err)
	}
	defer os.Remove(zipPath)

	newExe := filepath.Join(dir, "clash.smart.new.exe")
	if err := extractExeFromZip(zipPath, newExe); err != nil {
		return fmt.Errorf("не удалось распаковать смарт-ядро: %w", err)
	}
	defer os.Remove(newExe)

	modelTmp := smartModelPath() + ".new"
	if err := downloadToFirst(ctx, client, []string{smartMirrorModel, smartModelGithub}, modelTmp); err == nil {
		_ = os.Rename(modelTmp, smartModelPath())
	} else {
		_ = os.Remove(modelTmp)
	}

	if _, err := os.Stat(stockBackupPath()); os.IsNotExist(err) {
		if err := copyFile(coreExePath(), stockBackupPath()); err != nil {
			return fmt.Errorf("не удалось сохранить резервную копию стокового ядра: %w", err)
		}
	}

	if err := ReplaceFileWithBackup(newExe, coreExePath()); err != nil {
		return fmt.Errorf("не удалось заменить ядро: %w", err)
	}

	return setSmartCoreActive(true)
}

func RevertToStockCore() error {
	backup := stockBackupPath()
	if _, err := os.Stat(backup); err != nil {

		return setSmartCoreActive(false)
	}

	tmp := coreExePath() + ".stock.tmp"
	if err := copyFile(backup, tmp); err != nil {
		return err
	}
	if err := ReplaceFileWithBackup(tmp, coreExePath()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(smartModelPath())
	return setSmartCoreActive(false)
}

func downloadToFirst(ctx context.Context, client *http.Client, urls []string, dest string) error {
	var lastErr error
	for _, u := range urls {
		if err := downloadTo(ctx, client, u, dest); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("нет источников")
	}
	return lastErr
}

func downloadTo(ctx context.Context, client *http.Client, url, dest string) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "misetanibox")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func extractExeFromZip(zipPath, destExe string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".exe") {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			out, err := os.Create(destExe)
			if err != nil {
				return err
			}
			defer out.Close()
			if _, err := io.Copy(out, rc); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("exe не найден в архиве")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
