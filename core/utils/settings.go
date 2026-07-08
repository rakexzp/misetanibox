package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var settingsMu sync.Mutex

func LoadSetting[T any](fileName string, defaultData T) (*T, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	dir := GetSettingsDir()
	userPath := filepath.Join(dir, "user_"+fileName+".json")
	defaultPath := filepath.Join(dir, "default_"+fileName+".json")

	result := defaultData

	if data, err := os.ReadFile(userPath); err == nil {
		if json.Unmarshal(data, &result) == nil {

			patchedBytes, _ := json.MarshalIndent(result, "", "  ")
			_ = WriteFileAtomic(userPath, patchedBytes, 0644)
			return &result, nil
		}
	}

	if data, err := os.ReadFile(defaultPath); err == nil {
		if json.Unmarshal(data, &result) == nil {

			patchedBytes, _ := json.MarshalIndent(result, "", "  ")
			_ = WriteFileAtomic(defaultPath, patchedBytes, 0644)
			return &result, nil
		}
	}

	defaultBytes, _ := json.MarshalIndent(defaultData, "", "  ")
	_ = WriteFileAtomic(defaultPath, defaultBytes, 0644)

	return &defaultData, nil
}

func SaveSetting[T any](fileName string, data *T) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()

	if data == nil {
		return fmt.Errorf("setting %q cannot be nil", fileName)
	}

	safeName, err := SanitizeFilename(fileName)
	if err != nil {
		return err
	}
	if safeName != fileName {
		return fmt.Errorf("недопустимое имя файла настроек: %q", fileName)
	}

	userPath := filepath.Join(GetSettingsDir(), "user_"+safeName+".json")

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return WriteFileAtomic(userPath, bytes, 0644)
}

func ResetSetting(fileName string) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	os.Remove(filepath.Join(GetSettingsDir(), "user_"+fileName+".json"))
}
