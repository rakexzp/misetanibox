package clash

import (
	"encoding/json"
	"os"
	"path/filepath"

	"goclashz/core/logger"
	"goclashz/core/utils"

	"gopkg.in/yaml.v3"
)

func EnsureRuleStorageMigratedLocked(id string) error {
	safeId, err := utils.SanitizeFilename(id)
	if err != nil {
		return err
	}
	rulesPath := filepath.Join(SubscriptionsDir(), safeId+"_rules.json")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		return nil
	}

	IndexLock.RLock()
	var itemType string
	for _, item := range SubIndex {
		if item.ID == id {
			itemType = item.Type
			break
		}
	}
	IndexLock.RUnlock()

	if itemType == "local" || itemType == "" {
		return migrateLocalRules(id, rulesPath)
	}

	return migrateRemoteRules(id, rulesPath)
}

func MigrateRuleStorageV2() error {
	IndexLock.RLock()
	ids := make([]string, 0, len(SubIndex))
	for _, item := range SubIndex {
		ids = append(ids, item.ID)
	}
	IndexLock.RUnlock()

	var firstErr error
	for _, id := range ids {
		err := WithRuleStorageLock(func() error {
			return EnsureRuleStorageMigratedLocked(id)
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		logger.Errorf("не удалось выполнить миграцию хранилища правил: %v", firstErr)
	}
	return firstErr
}

func migrateLocalRules(id, legacyRulesPath string) error {
	workingPath, _ := WorkingConfigPath(id)
	originPath, _ := OriginConfigPath(id)

	root, err := ReadYamlRoot(workingPath)
	if err != nil {
		return err
	}

	oldRules, err := readLegacyRulesJson(legacyRulesPath)
	if err != nil {
		return err
	}

	if oldRules != nil {
		root["rules"] = oldRules

		out, err := yaml.Marshal(root)
		if err != nil {
			return err
		}

		if err := utils.WriteFileAtomic(workingPath, out, 0644); err != nil {
			return err
		}
	}

	if _, err := os.Stat(originPath); os.IsNotExist(err) {
		data, _ := os.ReadFile(workingPath)
		utils.WriteFileAtomic(originPath, data, 0644)
	}

	return deleteLegacyRulesJson(legacyRulesPath)
}

func migrateRemoteRules(id, legacyRulesPath string) error {
	workingPath, _ := WorkingConfigPath(id)
	originPath, _ := OriginConfigPath(id)

	if _, err := os.Stat(originPath); os.IsNotExist(err) {
		data, _ := os.ReadFile(workingPath)
		utils.WriteFileAtomic(originPath, data, 0644)
	}

	originRoot, err := ReadYamlRoot(originPath)
	if err != nil {
		return err
	}

	originRules := ExtractRulesFromRootPublic(originRoot)

	oldRules, err := readLegacyRulesJson(legacyRulesPath)
	if err != nil {
		return err
	}

	if oldRules == nil {
		EnsureEmptyOverlay(id)
		return deleteLegacyRulesJson(legacyRulesPath)
	}

	originSet := map[string]bool{}
	for _, r := range originRules {
		originSet[NormalizeRule(r)] = true
	}

	oldSet := map[string]bool{}
	for _, r := range oldRules {
		oldSet[NormalizeRule(r)] = true
	}

	overlay := RuleOverlay{
		Add:    []string{},
		Delete: []string{},
	}

	for _, r := range oldRules {
		if !originSet[NormalizeRule(r)] {
			overlay.Add = append(overlay.Add, r)
		}
	}

	for _, r := range originRules {
		if !oldSet[NormalizeRule(r)] {
			overlay.Delete = append(overlay.Delete, r)
		}
	}

	if err := SaveRuleOverlay(id, overlay); err != nil {
		return err
	}

	return deleteLegacyRulesJson(legacyRulesPath)
}

func readLegacyRulesJson(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var set CustomRuleSet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, nil
	}
	return set.CustomRules, nil
}

func deleteLegacyRulesJson(path string) error {
	return os.Remove(path)
}
