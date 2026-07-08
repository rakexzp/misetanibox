package clash

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"goclashz/core/utils"

	"gopkg.in/yaml.v3"
)

var ruleStorageMu sync.Mutex

func WithRuleStorageLock(fn func() error) error {
	ruleStorageMu.Lock()
	defer ruleStorageMu.Unlock()
	return fn()
}

type RuleOverlay struct {
	Add    []string `json:"add"`
	Delete []string `json:"delete"`
}

type RulePageData struct {
	ConfigType        string   `json:"configType"`
	SubscriptionRules []string `json:"subscriptionRules"`
	LocalRules        []string `json:"localRules"`
	AddRules          []string `json:"addRules"`
	DeleteRules       []string `json:"deleteRules"`
	EffectiveRules    []string `json:"effectiveRules"`
}

func SubscriptionsDir() string {
	return utils.GetSubscriptionsDir()
}

func OriginDir() string {
	return filepath.Join(utils.GetSubscriptionsDir(), "origin")
}

func WorkingConfigPath(id string) (string, error) {
	safeId, err := utils.SanitizeFilename(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(SubscriptionsDir(), safeId+".yaml"), nil
}

func OriginConfigPath(id string) (string, error) {
	safeId, err := utils.SanitizeFilename(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(OriginDir(), safeId+".yaml"), nil
}

func RuleOverlayPath(id string) (string, error) {
	safeId, err := utils.SanitizeFilename(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(SubscriptionsDir(), safeId+"_overlay.json"), nil
}

func LoadRuleOverlay(id string) (RuleOverlay, error) {
	path, err := RuleOverlayPath(id)
	if err != nil {
		return RuleOverlay{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {

		return RuleOverlay{Add: []string{}, Delete: []string{}}, nil
	}

	var overlay RuleOverlay
	if err := json.Unmarshal(data, &overlay); err != nil {

		_ = utils.WriteFileAtomic(path+".corrupt.bak", data, 0644)
		return RuleOverlay{}, fmt.Errorf("overlay-конфигурация правил повреждена, резервная копия сохранена как %s.corrupt.bak: %w", path, err)
	}

	if overlay.Add == nil {
		overlay.Add = []string{}
	}
	if overlay.Delete == nil {
		overlay.Delete = []string{}
	}

	return overlay, nil
}

func SaveRuleOverlay(id string, overlay RuleOverlay) error {
	path, err := RuleOverlayPath(id)
	if err != nil {
		return err
	}

	data, err := json.Marshal(overlay)
	if err != nil {
		return err
	}

	return utils.WriteFileAtomic(path, data, 0644)
}

func EnsureEmptyOverlay(id string) error {
	path, err := RuleOverlayPath(id)
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return nil
	}

	return SaveRuleOverlay(id, RuleOverlay{Add: []string{}, Delete: []string{}})
}

func ExtractRulesFromRootPublic(root map[string]interface{}) []string {
	var rules []string
	if rawRules, ok := root["rules"].([]interface{}); ok {
		for _, r := range rawRules {
			if strRule, ok := r.(string); ok {
				rules = append(rules, strRule)
			}
		}
	}
	return rules
}

func ReadYamlRoot(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	return root, nil
}

func ReadWorkingRootWithRecovery(id string) (map[string]interface{}, error) {
	workingPath, err := WorkingConfigPath(id)
	if err != nil {
		return nil, err
	}

	root, err := ReadYamlRoot(workingPath)
	if err == nil {
		return root, nil
	}

	originPath, originErr := OriginConfigPath(id)
	if originErr != nil {
		return nil, err
	}

	originRoot, originReadErr := ReadYamlRoot(originPath)
	if originReadErr != nil {
		return nil, fmt.Errorf("рабочая конфигурация повреждена, и восстановить её из origin не удалось: working=%v origin=%v", err, originReadErr)
	}

	originData, readErr := os.ReadFile(originPath)
	if readErr != nil {
		return nil, readErr
	}

	if writeErr := utils.WriteFileAtomic(workingPath, originData, 0644); writeErr != nil {
		return nil, fmt.Errorf("не удалось восстановить working из origin: %w", writeErr)
	}

	return originRoot, nil
}

func readYamlRootFromOrigin(id string) (map[string]interface{}, error) {
	path, err := OriginConfigPath(id)
	if err != nil {
		return nil, err
	}
	return ReadYamlRoot(path)
}

func NormalizeRule(rule string) string {
	parts := strings.Split(rule, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) > 0 {
		parts[0] = strings.ToUpper(parts[0])
	}
	return strings.Join(parts, ",")
}

func ApplyRuleOverlay(baseRules []string, overlay RuleOverlay) []string {
	deleteSet := map[string]bool{}
	for _, r := range overlay.Delete {
		deleteSet[NormalizeRule(r)] = true
	}

	finalRules := make([]string, 0, len(overlay.Add)+len(baseRules))
	finalRules = append(finalRules, overlay.Add...)

	for _, r := range baseRules {
		if !deleteSet[NormalizeRule(r)] {
			finalRules = append(finalRules, r)
		}
	}
	return finalRules
}

func ApplyDeleteOnly(baseRules []string, deleteRules []string) []string {
	deleteSet := map[string]bool{}
	for _, r := range deleteRules {
		deleteSet[NormalizeRule(r)] = true
	}

	kept := make([]string, 0, len(baseRules))
	for _, r := range baseRules {
		if !deleteSet[NormalizeRule(r)] {
			kept = append(kept, r)
		}
	}
	return kept
}

func BuildRuntimeRules(id string, workingRoot map[string]interface{}) ([]string, error) {
	item, _ := FindSubIndexByID(id)

	baseRules := ExtractRulesFromRootPublic(workingRoot)

	if item.Type != "remote" {
		ar, _ := LoadAppRouting(id)
		return applyAppRouting(workingRoot, baseRules, ar), nil
	}

	overlay, err := LoadRuleOverlay(id)
	if err != nil {
		return nil, err
	}

	rules := ApplyRuleOverlay(baseRules, overlay)

	ar, _ := LoadAppRouting(id)
	return applyAppRouting(workingRoot, rules, ar), nil
}
