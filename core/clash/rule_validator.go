package clash

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func ValidateClashReferencesBytes(data []byte) error {
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("ошибка формата YAML: %v", err)
	}
	return ValidateClashReferences(root)
}

func ValidateClashReferences(root map[string]interface{}) error {
	builtinPolicies := map[string]bool{
		"DIRECT":      true,
		"REJECT":      true,
		"REJECT-DROP": true,
		"PASS":        true,
		"GLOBAL":      true,
		"COMPAT":      true,
	}

	proxyNames := map[string]bool{}
	groupNames := map[string]bool{}
	proxyProviderNames := map[string]bool{}
	ruleProviderNames := map[string]bool{}

	if proxiesNode, ok := root["proxies"].([]interface{}); ok {
		for _, p := range proxiesNode {
			if proxy, isMap := p.(map[string]interface{}); isMap {
				if name, _ := proxy["name"].(string); name != "" {
					proxyNames[name] = true
				}
			}
		}
	}

	if groupsNode, ok := root["proxy-groups"].([]interface{}); ok {
		for _, g := range groupsNode {
			if group, isMap := g.(map[string]interface{}); isMap {
				if name, _ := group["name"].(string); name != "" {
					groupNames[name] = true
				}
			}
		}
	}

	if providersNode, ok := root["proxy-providers"].(map[string]interface{}); ok {
		for name := range providersNode {
			proxyProviderNames[name] = true
		}
	}

	if ruleProvidersNode, ok := root["rule-providers"].(map[string]interface{}); ok {
		for name := range ruleProvidersNode {
			ruleProviderNames[name] = true
		}
	}

	isValidPolicyTarget := func(name string) bool {
		return builtinPolicies[name] || proxyNames[name] || groupNames[name]
	}

	if groupsNode, ok := root["proxy-groups"].([]interface{}); ok {
		for _, g := range groupsNode {
			if group, isMap := g.(map[string]interface{}); isMap {
				groupName, _ := group["name"].(string)

				if useNode, ok := group["use"].([]interface{}); ok {
					for _, u := range useNode {
						if providerName, ok := u.(string); ok {
							if !proxyProviderNames[providerName] {
								return fmt.Errorf("группа [%s]: use ссылается на несуществующий provider: %s", groupName, providerName)
							}
						}
					}
				}

				if pList, ok := group["proxies"].([]interface{}); ok {
					for _, p := range pList {
						if proxyName, ok := p.(string); ok {
							if !isValidPolicyTarget(proxyName) {
								return fmt.Errorf("группа [%s] ссылается на несуществующий узел/группу: %s", groupName, proxyName)
							}
						}
					}
				}
			}
		}
	}

	if rulesNode, ok := root["rules"].([]interface{}); ok {
		for _, r := range rulesNode {
			if ruleStr, ok := r.(string); ok {
				parts := strings.Split(ruleStr, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}

				if len(parts) >= 2 {
					ruleType := strings.ToUpper(parts[0])

					if ruleType == "MATCH" {
						target := parts[1]
						if !isValidPolicyTarget(target) {
							return fmt.Errorf("правило [%s] ссылается на несуществующую группу/узел: %s", ruleStr, target)
						}
					} else if ruleType == "RULE-SET" {
						if len(parts) >= 3 {
							provider := parts[1]
							target := parts[2]
							if !ruleProviderNames[provider] {
								return fmt.Errorf("правило [%s] ссылается на несуществующий rule-provider: %s", ruleStr, provider)
							}
							if !isValidPolicyTarget(target) {
								return fmt.Errorf("правило [%s] ссылается на несуществующую группу/узел: %s", ruleStr, target)
							}
						}
					} else if ruleType == "AND" || ruleType == "OR" || ruleType == "NOT" || ruleType == "SUB-RULE" {

						continue
					} else if len(parts) >= 3 {
						target := parts[2]
						if !isValidPolicyTarget(target) {
							return fmt.Errorf("правило [%s] ссылается на несуществующую группу/узел: %s", ruleStr, target)
						}
					}
				}
			}
		}
	}

	return nil
}

func SanitizeRuleLine(rule string) (string, error) {
	rule = NormalizeRule(rule)
	if rule == "" {
		return "", fmt.Errorf("правило не может быть пустым")
	}

	rawParts := strings.Split(rule, ",")
	parts := make([]string, 0, len(rawParts))
	for _, p := range rawParts {
		p = strings.TrimSpace(p)
		if p == "" {
			return "", fmt.Errorf("неверный формат правила, есть пустой сегмент: %s", rule)
		}
		parts = append(parts, p)
	}

	ruleType := strings.ToUpper(parts[0])
	parts[0] = ruleType

	switch ruleType {
	case "MATCH":
		if len(parts) != 2 {
			return "", fmt.Errorf("правило MATCH должно иметь вид MATCH,политика")
		}
	case "AND", "OR", "NOT", "SUB-RULE":
		if len(parts) < 2 {
			return "", fmt.Errorf("%s: неполная структура правила", ruleType)
		}
	default:
		if len(parts) < 3 {
			return "", fmt.Errorf("%s: формат правила должен быть минимум тип,содержимое,политика", ruleType)
		}
	}

	return strings.Join(parts, ","), nil
}

func SanitizeRuleList(rules []string) ([]string, error) {
	var valid []string
	for _, r := range rules {
		cleaned, err := SanitizeRuleLine(r)
		if err != nil {
			return nil, err
		}
		valid = append(valid, cleaned)
	}
	return valid, nil
}
