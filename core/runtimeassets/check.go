package runtimeassets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"goclashz/core/sys"
	"goclashz/core/utils"
)

var (
	coreVersionRe        = regexp.MustCompile(`v?\d+\.\d+\.\d+(?:[-+][^\s]+)?`)
	coreVersionCache     string
	coreVersionCacheKey  string
	coreVersionCacheTime time.Time
	coreVersionCacheMu   sync.Mutex
)

func CheckCore(ctx context.Context) AssetHealth {
	path := filepath.Join(utils.GetCoreBinDir(), coreBinName)
	return checkCoreByPath(ctx, path)
}

func checkCoreByPath(ctx context.Context, path string) AssetHealth {
	h := baseHealth(AssetCore, "Ядро Mihomo", path, true)
	if bundledCoreMaintenance.Load() {
		if _, err := os.Lstat(path + ".seed-transaction.json"); err == nil || !os.IsNotExist(err) {
			h.ErrorCode = ErrInvalidPE
			h.Error = "Замена ядра не завершена; автозапуск отложен до восстановления"
			return h
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		h.ErrorCode = ErrMissing
		h.Error = "файл ядра не найден"
		h.Hint = "Используйте «Восстановить встроенные компоненты» или «Обновить ядро»."
		return h
	}
	if info.IsDir() {
		h.Exists = true
		h.ErrorCode = ErrIsDir
		h.Error = "путь к ядру является каталогом, а не исполняемым файлом"
		return h
	}

	h.Exists = true
	h.Size = info.Size()
	h.ModTime = info.ModTime().Unix()

	if err := validateCoreBinary(path); err != nil {
		h.ErrorCode = ErrInvalidPE
		h.Error = err.Error()
		h.Hint = "Файл ядра повреждён или не является исполняемым файлом для текущей ОС."
		return h
	}

	h.Valid = true
	h.Ready = true

	if hash, err := calculateSHA256(path); err == nil {
		h.SHA256 = hash
	}

	version, execErr := readCoreVersion(path)
	if execErr != nil {
		h.Version = "установлено, версия неизвестна"
		h.Hint = "Определение версии не удалось или превышен таймаут, но формат файла ядра корректен; можно попробовать запустить напрямую."
		return h
	}

	h.Version = version
	h.VersionProbeOK = true
	return h
}

// ProbeCoreVersion requires a successful process exit and keys cached results by
// absolute path and content hash, including replacements with identical timestamps.
func ProbeCoreVersion(ctx context.Context, path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	hash, err := calculateSHA256(path)
	if err != nil {
		return "", err
	}
	key := path + "\x00" + hash
	coreVersionCacheMu.Lock()
	defer coreVersionCacheMu.Unlock()
	if coreVersionCacheKey == key && coreVersionCache != "" && time.Since(coreVersionCacheTime) < time.Minute {
		return coreVersionCache, nil
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, path, "-v")
	cmd.Dir = filepath.Dir(path)
	utils.HideCommandWindow(cmd, 0)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("не удалось выполнить ядро -v: %w", err)
	}
	m := coreVersionRe.FindString(strings.TrimSpace(string(out)))
	if m == "" {
		return "", fmt.Errorf("версия ядра неизвестна")
	}
	after, err := calculateSHA256(path)
	if err != nil || after != hash {
		return "", fmt.Errorf("ядро изменилось во время проверки версии")
	}
	ver := "v" + strings.TrimPrefix(m, "v")
	coreVersionCacheKey, coreVersionCache, coreVersionCacheTime = key, ver, time.Now()
	return ver, nil
}

func readCoreVersion(path string) (string, error) {
	return ProbeCoreVersion(context.Background(), path)
}

func CheckWintun() AssetHealth {
	if !wintunNeeded {
		// на Linux/macOS wintun не требуется — считаем готовым
		h := baseHealth(AssetWintun, "Wintun (не требуется на этой ОС)", "", true)
		h.Valid = true
		h.Ready = true
		return h
	}
	path := filepath.Join(utils.GetCoreBinDir(), "wintun.dll")
	return checkWintunByPath(path)
}

func checkWintunByPath(path string) AssetHealth {
	h := baseHealth(AssetWintun, "DLL драйвера Wintun", path, true)

	info, err := os.Stat(path)
	if err != nil {
		h.ErrorCode = ErrMissing
		h.Error = "wintun.dll не найден"
		h.Hint = "Используйте «Восстановить встроенные компоненты» или «Установить драйвер»."
		return h
	}
	if info.IsDir() {
		h.Exists = true
		h.ErrorCode = ErrIsDir
		h.Error = "путь wintun является каталогом, а не DLL-файлом"
		return h
	}

	h.Exists = true
	h.Size = info.Size()
	h.ModTime = info.ModTime().Unix()

	if err := utils.ValidateWindowsPE(path, 32*1024, 5*1024*1024); err != nil {
		h.ErrorCode = ErrInvalidPE
		h.Error = err.Error()
		h.Hint = "Текущий wintun.dll повреждён или не является допустимой DLL Windows."
		return h
	}

	h.Valid = true
	h.Ready = true

	v, err := sys.GetFileVersion(path)
	if err == nil && strings.TrimSpace(v) != "" {
		h.Version = v
	} else {
		h.Version = "установлено, версия неизвестна"
	}

	if hash, err := calculateSHA256(path); err == nil {
		h.SHA256 = hash
	}

	return h
}

func CheckDataFile(key AssetKey, label string, filename string) AssetHealth {
	path := filepath.Join(utils.GetCoreBinDir(), filename)
	return checkDataFileByPath(key, label, path)
}

func checkDataFileByPath(key AssetKey, label string, path string) AssetHealth {
	h := baseHealth(key, label, path, false)

	info, err := os.Stat(path)
	if err != nil {
		h.ErrorCode = ErrMissing
		h.Error = label + ": файл не найден"
		return h
	}
	if info.IsDir() {
		h.Exists = true
		h.ErrorCode = ErrIsDir
		h.Error = label + ": путь является каталогом"
		return h
	}

	h.Exists = true
	h.Size = info.Size()
	h.ModTime = info.ModTime().Unix()
	h.Valid = true
	h.Ready = true

	if hash, err := calculateSHA256(path); err == nil {
		h.SHA256 = hash
	}

	return h
}

func calculateSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
