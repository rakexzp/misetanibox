package clash

import (
	"encoding/json"
	"fmt"
	"goclashz/core/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var rulesMutex sync.RWMutex

var validRuleTypes = map[string]bool{
	"DOMAIN":         true,
	"DOMAIN-SUFFIX":  true,
	"DOMAIN-KEYWORD": true,
	"DOMAIN-REGEX":   true,
	"GEOSITE":        true,
	"GEOIP":          true,
	"IP-CIDR":        true,
	"IP-CIDR6":       true,
	"IP-SUFFIX":      true,
	"IP-ASN":         true,
	"SRC-IP-CIDR":    true,
	"SRC-PORT":       true,
	"DST-PORT":       true,
	"IN-PORT":        true,
	"IN-TYPE":        true,
	"IN-USER":        true,
	"PROCESS-NAME":   true,
	"PROCESS-PATH":   true,
	"UID":            true,
	"NETWORK":        true,
	"DSCP":           true,
	"RULE-SET":       true,
	"AND":            true,
	"OR":             true,
	"NOT":            true,
	"SUB-RULE":       true,
	"MATCH":          true,
}

type CustomRuleSet struct {
	CustomRules []string `json:"customRules"`
}

func GetCustomRules(id string) ([]string, error) {

	safeId := filepath.Base(filepath.Clean(id))
	if safeId == "." || safeId == "/" || safeId == "\\" {
		return nil, fmt.Errorf("недопустимый ID файла, доступ запрещён")
	}

	rulesMutex.RLock()
	defer rulesMutex.RUnlock()

	path := filepath.Join(utils.GetSubscriptionsDir(), safeId+"_rules.json")
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return []string{}, nil
	}

	var set CustomRuleSet

	if err := json.Unmarshal(data, &set); err != nil {
		return nil, err
	}
	return set.CustomRules, nil
}

func SaveCustomRules(id string, rules []string) error {
	rulesMutex.Lock()
	defer rulesMutex.Unlock()

	if rules == nil {
		rules = []string{}
	}

	var sanitizedRules []string
	for _, r := range rules {
		cleanRule := strings.TrimSpace(r)
		if cleanRule == "" {
			continue
		}

		parts := strings.Split(cleanRule, ",")

		var cleanedParts []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed == "" {
				return fmt.Errorf("формат отклонён: пустые значения между запятыми не допускаются")
			}
			cleanedParts = append(cleanedParts, trimmed)
		}

		if len(cleanedParts) < 2 {
			return fmt.Errorf("формат отклонён: нужен разделитель-запятая и минимум два сегмента")
		}

		ruleType := strings.ToUpper(cleanedParts[0])
		if !validRuleTypes[ruleType] {
			return fmt.Errorf("формат отклонён: [%s] не является допустимым типом правила Clash. Поддерживаются типы вроде DOMAIN, IP-CIDR, MATCH", cleanedParts[0])
		}

		if ruleType == "MATCH" {
			if len(cleanedParts) < 2 {
				return fmt.Errorf("формат отклонён: правило MATCH требует минимум 2 сегмента (например MATCH,DIRECT)")
			}
		} else {

			if len(cleanedParts) < 3 {
				return fmt.Errorf("формат отклонён: правило [%s] требует минимум 3 сегмента (тип,цель,политика), например %s,example.com,DIRECT", ruleType, ruleType)
			}
		}

		cleanedParts[0] = ruleType
		sanitizedRules = append(sanitizedRules, strings.Join(cleanedParts, ","))
	}

	safeId := filepath.Base(filepath.Clean(id))
	if safeId == "." || safeId == "/" || safeId == "\\" {
		return fmt.Errorf("недопустимый ID файла, доступ запрещён")
	}

	path := filepath.Join(utils.GetSubscriptionsDir(), safeId+"_rules.json")
	tmpPath := path + ".tmp"

	set := CustomRuleSet{CustomRules: sanitizedRules}
	data, _ := json.Marshal(set)

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func GetOriginalRules(id string) ([]string, error) {

	safeId := filepath.Base(filepath.Clean(id))
	if safeId == "." || safeId == "/" || safeId == "\\" {
		return nil, fmt.Errorf("недопустимый ID файла, доступ запрещён")
	}

	yamlPath := filepath.Join(utils.GetSubscriptionsDir(), safeId+".yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}

	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	var rules []string
	if rawRules, ok := root["rules"].([]interface{}); ok {
		for _, r := range rawRules {
			if strRule, ok := r.(string); ok {
				rules = append(rules, strRule)
			}
		}
	}
	return rules, nil
}

func SyncRulesFromYaml(id string) error {
	originalRules, err := GetOriginalRules(id)
	if err != nil {
		return err
	}

	if len(originalRules) == 0 {
		originalRules = []string{"MATCH,DIRECT"}
	}
	return SaveCustomRules(id, originalRules)
}
