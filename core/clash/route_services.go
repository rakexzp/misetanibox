package clash

import (
	"strings"

	"goclashz/core/utils"
)

// Конфигуратор селекторов по сервисам. Юзер выбирает популярные сервисы плашками, и
// для каждого создаётся СВОЯ группа-селектор (type: select, include-all) — в «Прокси-узлах»
// появляется отдельная вкладка сервиса, где можно выбрать сервер именно под него.
// Плюс два спец-режима на плашке: «напрямую» и «блок».

const routeConfigKey = "route_services"

// Режимы плашки.
const (
	RouteSelector = "selector" // своя группа-селектор (выбор сервера под сервис)
	RouteDirect   = "direct"   // напрямую, мимо VPN
	RouteReject   = "reject"   // заблокировать
	RouteProxy    = "proxy"    // через основной селектор (дефолт, не сохраняется)
)

// ServiceDef — один сервис в каталоге конфигуратора.
type ServiceDef struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Icon     string `json:"icon"`
	Geosite  string `json:"geosite"`
	Category string `json:"category"`
}

// ServiceCatalog — geosite-категории проверены на реальном mihomo.
var ServiceCatalog = []ServiceDef{
	{"youtube", "YouTube", "▶️", "youtube", "media"},
	{"instagram", "Instagram", "📸", "instagram", "social"},
	{"tiktok", "TikTok", "🎵", "tiktok", "social"},
	{"discord", "Discord", "🎮", "discord", "social"},
	{"telegram", "Telegram", "✈️", "telegram", "social"},
	{"twitter", "X / Twitter", "𝕏", "twitter", "social"},
	{"facebook", "Facebook", "👥", "facebook", "social"},
	{"netflix", "Netflix", "🎬", "netflix", "media"},
	{"spotify", "Spotify", "🎧", "spotify", "media"},
	{"google", "Google", "🔍", "google", "popular"},
	{"openai", "ChatGPT / OpenAI", "🤖", "openai", "popular"},
	{"games", "Игры", "🕹️", "category-games", "popular"},
	{"ru", "Российские сайты", "🇷🇺", "category-ru", "ru"},
	{"ads", "Реклама и трекеры", "🚫", "category-ads-all", "block"},
	{"porn", "Взрослый контент", "🔞", "category-porn", "block"},
}

func serviceByKey(key string) (ServiceDef, bool) {
	for _, s := range ServiceCatalog {
		if s.Key == key {
			return s, true
		}
	}
	return ServiceDef{}, false
}

// serviceGroupName — имя группы-селектора сервиса (видно в «Прокси-узлах»).
func serviceGroupName(s ServiceDef) string {
	return s.Icon + " " + s.Label
}

// RouteConfig — сохранённая раскладка.
type RouteConfig struct {
	Enabled  bool              `json:"enabled"`
	Services map[string]string `json:"services"` // ключ сервиса → selector/direct/reject
}

// DefaultRouteConfig — ответ «Нет» в мастере: глобально + РФ напрямую + реклама в блок,
// без отдельных селекторов.
func DefaultRouteConfig() RouteConfig {
	return RouteConfig{
		Enabled:  true,
		Services: map[string]string{"ru": RouteDirect, "ads": RouteReject},
	}
}

func GetRouteConfig() RouteConfig {
	v, err := utils.LoadSetting(routeConfigKey, RouteConfig{})
	if err != nil || v == nil || v.Services == nil {
		return RouteConfig{Services: map[string]string{}}
	}
	return *v
}

func SaveRouteConfig(cfg RouteConfig) error {
	if cfg.Services == nil {
		cfg.Services = map[string]string{}
	}
	return utils.SaveSetting(routeConfigKey, &cfg)
}

// injectServiceGroups добавляет группы-селекторы для сервисов с режимом «selector».
// Вызывается рядом с injectChains — до валидатора, чтобы правила могли на них ссылаться.
func injectServiceGroups(root map[string]interface{}) {
	cfg := GetRouteConfig()
	if !cfg.Enabled {
		return
	}
	groups, _ := root["proxy-groups"].([]interface{})
	existing := map[string]bool{}
	for _, g := range groups {
		if gm, ok := g.(map[string]interface{}); ok {
			if n, _ := gm["name"].(string); n != "" {
				existing[n] = true
			}
		}
	}
	var added []interface{}
	for _, s := range ServiceCatalog {
		if cfg.Services[s.Key] != RouteSelector {
			continue
		}
		name := serviceGroupName(s)
		if existing[name] {
			continue
		}
		added = append(added, map[string]interface{}{
			"name":        name,
			"type":        "select",
			"include-all": true,
		})
	}
	if len(added) > 0 {
		// селекторы сервисов — после основных групп
		root["proxy-groups"] = append(groups, added...)
	}
}

// buildServiceRoutes — правила: сервис → своя группа / DIRECT / REJECT.
func buildServiceRoutes(cfg RouteConfig) []string {
	if !cfg.Enabled || len(cfg.Services) == 0 {
		return nil
	}
	var rules []string
	// сначала блок/напрямую, потом селекторы — но категории не пересекаются, порядок некритичен
	order := []string{RouteReject, RouteDirect, RouteSelector}
	for _, want := range order {
		for _, s := range ServiceCatalog {
			if cfg.Services[s.Key] != want || s.Geosite == "" {
				continue
			}
			var policy string
			switch want {
			case RouteReject:
				policy = "REJECT"
			case RouteDirect:
				policy = "DIRECT"
			default:
				policy = serviceGroupName(s)
			}
			rules = append(rules, "GEOSITE,"+s.Geosite+","+policy)
		}
	}
	// РФ по IP — тем же маршрутом, что и сайты РФ
	if t, ok := cfg.Services["ru"]; ok {
		var policy string
		switch t {
		case RouteReject:
			policy = "REJECT"
		case RouteDirect:
			policy = "DIRECT"
		case RouteSelector:
			if s, ok := serviceByKey("ru"); ok {
				policy = serviceGroupName(s)
			}
		}
		if policy != "" {
			rules = append(rules, "GEOIP,RU,"+policy+",no-resolve")
		}
	}
	return rules
}

// injectServiceRoutes добавляет правила конфигуратора в начало списка правил.
func injectServiceRoutes(rules []string) []string {
	routes := buildServiceRoutes(GetRouteConfig())
	if len(routes) == 0 {
		return rules
	}
	existing := map[string]bool{}
	for _, r := range rules {
		existing[strings.TrimSpace(r)] = true
	}
	var fresh []string
	for _, r := range routes {
		if !existing[r] {
			fresh = append(fresh, r)
		}
	}
	return append(fresh, rules...)
}
