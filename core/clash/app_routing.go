package clash

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"goclashz/core/utils"
)

// Режимы маршрутизации по приложениям.
const (
	AppRouteOff       = "off"       // фича выключена, правила приложений не подмешиваются
	AppRouteBlacklist = "blacklist" // выбранные приложения идут напрямую (мимо VPN), остальное — через VPN
	AppRouteWhitelist = "whitelist" // только выбранные приложения через VPN, остальное — напрямую
)

// AppRouting — пер-конфиг настройка маршрутизации по приложениям.
type AppRouting struct {
	Mode string   `json:"mode"` // off | blacklist | whitelist
	Apps []string `json:"apps"` // basename экзешников в нижнем регистре, напр. "telegram.exe"
}

func normalizeAppRouting(ar *AppRouting) {
	if ar.Mode != AppRouteBlacklist && ar.Mode != AppRouteWhitelist {
		ar.Mode = AppRouteOff
	}
	if ar.Apps == nil {
		ar.Apps = []string{}
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(ar.Apps))
	for _, a := range ar.Apps {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		clean = append(clean, a)
	}
	ar.Apps = clean
}

func AppRoutingPath(id string) (string, error) {
	safeId, err := utils.SanitizeFilename(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(SubscriptionsDir(), safeId+"_apps.json"), nil
}

// LoadAppRouting читает настройку; при отсутствии/повреждении возвращает выключенный режим.
func LoadAppRouting(id string) (AppRouting, error) {
	path, err := AppRoutingPath(id)
	if err != nil {
		return AppRouting{Mode: AppRouteOff, Apps: []string{}}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return AppRouting{Mode: AppRouteOff, Apps: []string{}}, nil
	}
	var ar AppRouting
	if err := json.Unmarshal(data, &ar); err != nil {
		_ = utils.WriteFileAtomic(path+".corrupt.bak", data, 0644)
		return AppRouting{Mode: AppRouteOff, Apps: []string{}}, nil
	}
	normalizeAppRouting(&ar)
	return ar, nil
}

func SaveAppRouting(id string, ar AppRouting) error {
	normalizeAppRouting(&ar)
	path, err := AppRoutingPath(id)
	if err != nil {
		return err
	}
	data, err := json.Marshal(ar)
	if err != nil {
		return err
	}
	return utils.WriteFileAtomic(path, data, 0644)
}

// defaultProxyGroupName возвращает имя основной策略组 (первой proxy-group) как цель
// для whitelist-правил; если групп нет — резервно GLOBAL.
func defaultProxyGroupName(root map[string]interface{}) string {
	if groups, ok := root["proxy-groups"].([]interface{}); ok {
		for _, g := range groups {
			if group, ok := g.(map[string]interface{}); ok {
				if name, ok := group["name"].(string); ok && strings.TrimSpace(name) != "" {
					return name
				}
			}
		}
	}
	return "GLOBAL"
}

// applyAppRouting оборачивает уже собранные правила (base) правилами маршрутизации
// по приложениям в зависимости от режима.
//
//	blacklist: PROCESS-NAME,<app>,DIRECT (в начало) + base
//	whitelist: PROCESS-NAME,<app>,<group> (в начало) + MATCH,DIRECT (в конец), base игнорируется —
//	           это строгий режим «через VPN только выбранное».
func applyAppRouting(root map[string]interface{}, base []string, ar AppRouting) []string {
	normalizeAppRouting(&ar)
	if ar.Mode == AppRouteOff || len(ar.Apps) == 0 {
		return base
	}

	switch ar.Mode {
	case AppRouteBlacklist:
		prepend := make([]string, 0, len(ar.Apps))
		for _, app := range ar.Apps {
			prepend = append(prepend, fmt.Sprintf("PROCESS-NAME,%s,DIRECT", app))
		}
		return append(prepend, base...)

	case AppRouteWhitelist:
		group := defaultProxyGroupName(root)
		out := make([]string, 0, len(ar.Apps)+1)
		for _, app := range ar.Apps {
			out = append(out, fmt.Sprintf("PROCESS-NAME,%s,%s", app, group))
		}
		out = append(out, "MATCH,DIRECT")
		return out
	}

	return base
}
