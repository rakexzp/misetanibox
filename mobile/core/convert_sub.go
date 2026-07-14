package mobilecore

import (
	"fmt"
	"strings"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/common/convert"
	"github.com/metacubex/mihomo/common/yaml"
)

// ConvertSubscription normalizes subscription payloads for mihomo and drops
// entries that fail adapter.ParseProxy (one bad key used to wipe the whole
// provider → only COMPATIBLE left in UI).
//
// namePrefix is prepended to each proxy name (e.g. "[S1·file] ") so we do not
// rely on proxy-provider override (which also aborts on first error).
//
// Supports: vless, vmess, ss, ssr, trojan, hysteria, hysteria2/hy2, tuic,
// wireguard, anytls, mieru, and Clash YAML with proxies:.
func ConvertSubscription(raw string, namePrefix string) string {
	body, _ := convertSubscription(raw, namePrefix)
	return body
}

// ConvertSubscriptionCount returns how many valid proxies were produced.
func ConvertSubscriptionCount(raw string, namePrefix string) int {
	_, n := convertSubscription(raw, namePrefix)
	return n
}

func convertSubscription(raw string, namePrefix string) (string, int) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
	if raw == "" {
		return "", 0
	}
	buf := []byte(raw)

	var proxies []map[string]any

	// Already Clash YAML with proxies: — extract list.
	var probe map[string]any
	if err := yaml.Unmarshal(buf, &probe); err == nil && probe != nil {
		if rawList, ok := probe["proxies"]; ok {
			if list, ok := rawList.([]any); ok && len(list) > 0 {
				proxies = make([]map[string]any, 0, len(list))
				for _, item := range list {
					if m, ok := item.(map[string]any); ok {
						proxies = append(proxies, m)
					} else if m, ok := item.(map[string]interface{}); ok {
						proxies = append(proxies, m)
					}
				}
			}
		}
	}

	// Share-link list / base64 (vless, ss, hy2, …)
	if len(proxies) == 0 {
		converted, err := convert.ConvertsV2Ray(buf)
		if err != nil || len(converted) == 0 {
			return "", 0
		}
		proxies = converted
	}

	valid := make([]map[string]any, 0, len(proxies))
	names := make(map[string]struct{}, len(proxies))
	for _, mapping := range proxies {
		if mapping == nil {
			continue
		}
		// copy so we do not mutate shared maps
		m := make(map[string]any, len(mapping)+2)
		for k, v := range mapping {
			m[k] = v
		}

		name, _ := m["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			name = fmt.Sprintf("node-%d", len(valid)+1)
		}
		if namePrefix != "" && !strings.HasPrefix(name, namePrefix) {
			name = namePrefix + name
		}
		// unique names (select groups break on duplicates)
		base := name
		for i := 2; ; i++ {
			if _, ok := names[name]; !ok {
				break
			}
			name = fmt.Sprintf("%s-%d", base, i)
		}
		m["name"] = name

		// Soft defaults that help UDP protocols on Android TUN
		if t, _ := m["type"].(string); t != "" {
			switch strings.ToLower(t) {
			case "hysteria", "hysteria2", "tuic", "wireguard", "ss", "ssr", "vless", "vmess", "trojan":
				if _, ok := m["udp"]; !ok {
					m["udp"] = true
				}
			}
		}

		if _, err := adapter.ParseProxy(m); err != nil {
			// Skip broken key — never abort the whole subscription.
			continue
		}
		names[name] = struct{}{}
		valid = append(valid, m)
	}

	if len(valid) == 0 {
		return "", 0
	}

	out, err := yaml.Marshal(map[string]any{"proxies": valid})
	if err != nil {
		return "", 0
	}
	return string(out), len(valid)
}
