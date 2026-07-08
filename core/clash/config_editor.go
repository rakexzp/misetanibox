package clash

import (
	"fmt"
	"goclashz/core/utils"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ConfigTextResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Path    string `json:"path"`
}

func resolveConfigType(id string) string {
	if item, ok := FindSubIndexByID(id); ok {
		return item.Type
	}
	if id == MainConfigID || id == "" {
		return "runtime"
	}
	return "local"
}

func ReadConfigText(id string) (ConfigTextResult, error) {
	normalizedID, configPath, err := ProfilePathByIDStrict(id)
	if err != nil {
		return ConfigTextResult{}, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ConfigTextResult{}, fmt.Errorf("не удалось прочитать файл конфигурации: %w", err)
	}

	return ConfigTextResult{
		ID:      normalizedID,
		Name:    filepath.Base(configPath),
		Type:    resolveConfigType(id),
		Content: string(data),
		Path:    configPath,
	}, nil
}

func SaveConfigText(id string, content string) error {
	return WithRuleStorageLock(func() error {
		item, _ := FindSubIndexByID(id)

		var root map[string]interface{}
		if err := yaml.Unmarshal([]byte(content), &root); err != nil {
			return fmt.Errorf("ошибка синтаксиса YAML: %w", err)
		}

		if item.Type == "remote" {
			overlay, err := LoadRuleOverlay(id)
			if err != nil {
				return err
			}

			baseRules := ExtractRulesFromRootPublic(root)
			runtimeRules := ApplyRuleOverlay(baseRules, overlay)

			rootForValidation := make(map[string]interface{})
			for k, v := range root {
				rootForValidation[k] = v
			}
			rootForValidation["rules"] = runtimeRules

			if err := ValidateClashReferences(rootForValidation); err != nil {
				return fmt.Errorf("удалённая конфигурация: ошибка целостности runtime-ссылок: %w", err)
			}
		} else {
			if err := ValidateClashReferences(root); err != nil {
				return fmt.Errorf("ошибка целостности ссылок: %w", err)
			}
		}

		_, configPath, err := ProfilePathByIDStrict(id)
		if err != nil {
			return err
		}

		if err := utils.WriteFileAtomic(configPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("не удалось сохранить файл конфигурации: %w", err)
		}

		return nil
	})
}

func ValidateConfigText(content string) error {
	var root map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return fmt.Errorf("ошибка синтаксиса YAML: %w", err)
	}
	if err := ValidateClashReferences(root); err != nil {
		return fmt.Errorf("ошибка целостности ссылок: %w", err)
	}
	return nil
}

func GetConfigFilePath(id string) string {
	_, configPath, err := ProfilePathByIDStrict(id)
	if err != nil {
		return ""
	}
	return configPath
}

func IsConfigEditable(id string) bool {
	_, configPath, err := ProfilePathByIDStrict(id)
	if err != nil {
		return false
	}
	_, err = os.Stat(configPath)
	return err == nil
}

func GetEditableConfigs() []ConfigTextResult {
	var results []ConfigTextResult

	items := ListSubIndex()
	for _, item := range items {
		if IsConfigEditable(item.ID) {
			res, err := ReadConfigText(item.ID)
			if err == nil {
				res.Name = item.Name + ".yaml"
				results = append(results, res)
			}
		}
	}

	return results
}
