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
	coreVersionCacheTime time.Time
	coreVersionCacheMu   sync.Mutex
)

func CheckCore(ctx context.Context) AssetHealth {
	path := filepath.Join(utils.GetCoreBinDir(), "clash.exe")
	return checkCoreByPath(ctx, path)
}

func checkCoreByPath(ctx context.Context, path string) AssetHealth {
	h := baseHealth(AssetCore, "Ядро Mihomo", path, true)

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

	if err := utils.ValidateWindowsPE(path, 5*1024*1024, 300*1024*1024); err != nil {
		h.ErrorCode = ErrInvalidPE
		h.Error = err.Error()
		h.Hint = "Текущий clash.exe повреждён или не является исполняемым файлом Windows amd64."
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

func readCoreVersion(path string) (string, error) {
	coreVersionCacheMu.Lock()
	defer coreVersionCacheMu.Unlock()

	if coreVersionCache != "" && time.Since(coreVersionCacheTime) < time.Minute {
		return coreVersionCache, nil
	}

	cmdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, path, "-v")
	cmd.Dir = filepath.Dir(path)
	utils.HideCommandWindow(cmd, 0)

	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))

	if err != nil {
		if s != "" {
			return s, nil
		}
		return "", fmt.Errorf("не удалось выполнить clash.exe -v: %w", err)
	}

	if s == "" {
		return "установлено, версия неизвестна", nil
	}

	ver := s
	if m := coreVersionRe.FindString(s); m != "" {
		if strings.HasPrefix(m, "v") {
			ver = m
		} else {
			ver = "v" + m
		}
	}

	coreVersionCache = ver
	coreVersionCacheTime = time.Now()

	return ver, nil
}

func CheckWintun() AssetHealth {
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
