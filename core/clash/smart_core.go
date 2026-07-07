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
	smartCoreSettingKey = "smart_core" // хранит bool: активно ли смарт-ядро
	// Зеркало (HTTP, без TLS) — основной источник: в РФ github/objects.githubusercontent режется по TLS.
	smartMirrorZip   = "http://files.geodema.network/misetani/mihomo-smart-windows.zip"
	smartMirrorModel = "http://files.geodema.network/misetani/Model.bin"
	// GitHub — фолбёк (для не-РФ / если зеркало недоступно)
	smartGithubZip   = "https://github.com/vernesong/mihomo/releases/download/Prerelease-Alpha/mihomo-windows-amd64-compatible-alpha-smart-3e709f6.zip"
	smartModelGithub = "https://github.com/vernesong/mihomo/releases/download/LightGBM-Model/Model.bin"
)

// IsSmartCoreActive сообщает, установлено ли пользователем смарт-ядро (форк vernesong).
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
func coreExePath() string     { return filepath.Join(utils.GetCoreBinDir(), "clash.exe") }
func smartModelPath() string  { return filepath.Join(utils.GetCoreBinDir(), "Model.bin") }

// InstallSmartCore скачивает смарт-ядро vernesong (compatible) и модель, сохраняет резервную
// копию стокового ядра и подменяет clash.exe. Ядро ДОЛЖНО быть остановлено вызывающим (файл занят).
func InstallSmartCore(ctx context.Context) error {
	client := &http.Client{Timeout: 120 * time.Second}

	dir := utils.GetCoreBinDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 1. скачать zip (зеркало → github) и распаковать exe
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

	// 2. скачать модель (некритично: если не вышло — смарт работает по истории без ML)
	modelTmp := smartModelPath() + ".new"
	if err := downloadToFirst(ctx, client, []string{smartMirrorModel, smartModelGithub}, modelTmp); err == nil {
		_ = os.Rename(modelTmp, smartModelPath())
	} else {
		_ = os.Remove(modelTmp)
	}

	// 4. сохранить стоковое ядро для отката (только если ещё не сохранено)
	if _, err := os.Stat(stockBackupPath()); os.IsNotExist(err) {
		if err := copyFile(coreExePath(), stockBackupPath()); err != nil {
			return fmt.Errorf("не удалось сохранить резервную копию стокового ядра: %w", err)
		}
	}

	// 5. подменить clash.exe смарт-ядром
	if err := ReplaceFileWithBackup(newExe, coreExePath()); err != nil {
		return fmt.Errorf("не удалось заменить ядро: %w", err)
	}

	return setSmartCoreActive(true)
}

// RevertToStockCore возвращает стоковое ядро из резервной копии и выключает смарт-режим.
func RevertToStockCore() error {
	backup := stockBackupPath()
	if _, err := os.Stat(backup); err != nil {
		// нет бэкапа — просто сбросим флаг; ядро подтянется штатным обновлением
		return setSmartCoreActive(false)
	}
	// копируем стоковое ядро поверх (бэкап оставляем на месте)
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

// downloadToFirst пробует URL по очереди (зеркало, затем github) до первого успеха.
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
