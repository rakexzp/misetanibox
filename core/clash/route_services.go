package clash

import (
	"strings"

	"goclashz/core/utils"
)

// Конфигуратор маршрутов по сервисам: дружелюбная надстройка над правилами.
// Юзер в UI раскидывает популярные сервисы (YouTube, TikTok, …) по трём целям,
// а здесь это превращается в GEOSITE-правила и инжектится в начало списка правил.

const routeConfigKey = "route_services"

// Цели маршрута сервиса.
const (
	RouteProxy  = "proxy"  // через VPN (группа PROXY)
	RouteDirect = "direct" // напрямую, мимо VPN
	RouteReject = "reject" // заблокировать
)

// ServiceDef — один сервис в каталоге конфигуратора.
type ServiceDef struct {
	Key      string `json:"key"`      // стабильный ключ
	Label    string `json:"label"`    // подпись в UI
	Icon     string `json:"icon"`     // эмодзи для плашки
	Geosite  string `json:"geosite"`  // категория geosite ядра
	Category string `json:"category"` // группа в UI: popular | media | social | ru | block
}

// ServiceCatalog — все geosite-категории проверены на реальном mihomo.
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

func geositeForKey(key string) string {
	for _, s := range ServiceCatalog {
		if s.Key == key {
			return s.Geosite
		}
	}
	return ""
}

// RouteConfig — сохранённая раскладка сервисов по целям.
type RouteConfig struct {
	Enabled  bool              `json:"enabled"`  // применять ли конфигуратор
	Services map[string]string `json:"services"` // ключ сервиса → цель (proxy/direct/reject)
}

// DefaultRouteConfig — ответ «Нет» в мастере: глобально + РФ напрямую + реклама в блок.
func DefaultRouteConfig() RouteConfig {
	return RouteConfig{
		Enabled: true,
		Services: map[string]string{
			"ru":  RouteDirect,
			"ads": RouteReject,
		},
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

// targetToPolicy переводит цель в имя политики mihomo.
func targetToPolicy(target string) string {
	switch target {
	case RouteDirect:
		return "DIRECT"
	case RouteReject:
		return "REJECT"
	default:
		return "PROXY"
	}
}

// buildServiceRoutes формирует GEOSITE-правила из раскладки.
// Порядок: сначала блокировки, затем прямые, затем через VPN — но т.к. категории
// не пересекаются доменами, порядок между ними некритичен; важно, что весь блок
// идёт ПЕРЕД пользовательскими правилами и MATCH.
func buildServiceRoutes(cfg RouteConfig) []string {
	if !cfg.Enabled || len(cfg.Services) == 0 {
		return nil
	}
	order := []string{RouteReject, RouteDirect, RouteProxy}
	var rules []string
	for _, want := range order {
		for _, s := range ServiceCatalog {
			target, ok := cfg.Services[s.Key]
			if !ok || target != want {
				continue
			}
			geo := s.Geosite
			if geo == "" {
				continue
			}
			rules = append(rules, "GEOSITE,"+geo+","+targetToPolicy(target))
		}
	}
	// РФ по IP тоже направляем как сайты РФ (если РФ выбран напрямую/через прокси)
	if t, ok := cfg.Services["ru"]; ok {
		rules = append(rules, "GEOIP,RU,"+targetToPolicy(t)+",no-resolve")
	}
	return rules
}

// injectServiceRoutes добавляет правила конфигуратора В НАЧАЛО списка правил,
// чтобы они имели приоритет над остальными (но после явных пользовательских, см. вызов).
func injectServiceRoutes(rules []string) []string {
	routes := buildServiceRoutes(GetRouteConfig())
	if len(routes) == 0 {
		return rules
	}
	// не дублируем, если те же правила уже есть
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
