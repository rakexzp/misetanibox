package clash

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"goclashz/core/utils"

	"gopkg.in/yaml.v3"
)

// 👈 新增：定义一个全局互斥锁，专门保护 config.yaml 的并发 RMW (读-改-写) 操作
var configMu sync.Mutex

// GetConfigPath 获取 config.yaml 的绝对路径（导出供 app.go 使用，确保路径一致）
func GetConfigPath() string {
	return utils.GetRuntimeConfigPath()
}

// ClashConfig 映射完整的 YAML 结构
type ClashConfig struct {
	Mode        string                   `yaml:"mode"`
	ProxyGroups []map[string]interface{} `yaml:"proxy-groups"`
}

type NetworkConfig struct {
	Port                 int    `yaml:"port" json:"port"`
	MixedPort            int    `yaml:"mixed-port" json:"mixedPort"`
	IPv6                 bool   `yaml:"ipv6" json:"ipv6"`
	UnifiedDelay         bool   `yaml:"unified-delay" json:"unifiedDelay"`
	TCPConcurrent        bool   `yaml:"tcp-concurrent" json:"tcpConcurrent"`
	TCPKeepAlive         bool   `yaml:"tcp-keep-alive" json:"tcpKeepAlive"`
	TCPKeepAliveInterval int    `yaml:"tcp-keep-alive-interval" json:"tcpKeepAliveInterval"`
	TestURL              string `yaml:"test-url" json:"testUrl"`
	ExternalController   string `yaml:"external-controller" json:"externalController"`
	AllowLan             bool   `yaml:"allow-lan" json:"allowLan"`
	Hosts                string `yaml:"-" json:"hosts"`
}

// OfflineGroup 专供前端在“未启动”状态下展示的节点组结构
type OfflineGroup struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Proxies []string `json:"proxies"` // 组内包含的所有节点名称
}

// ProxyGroupSchema 模拟内核 API 的 Group 结构
type ProxyGroupSchema struct {
	Name string   `json:"name"`
	Type string   `json:"type"`
	Now  string   `json:"now"` // 当前选中节点
	All  []string `json:"all"` // 包含的所有节点名
}

// GetOfflineData 核心方法：模拟内核 API 返回的格式，从本地文件中提取数据
func GetOfflineData(id string) (map[string]interface{}, error) {
	configMu.Lock()
	defer configMu.Unlock()

	_, configPath, err := ProfilePathByIDStrict(id)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// ⭐️ 核心修复 1：结构体中加入对 Proxies 原始节点的解析
	var conf struct {
		Mode        string                   `yaml:"mode"`
		Proxies     []map[string]interface{} `yaml:"proxies"` // <--- 新增这行，读取底层节点
		ProxyGroups []map[string]interface{} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(data, &conf); err != nil {
		return nil, err
	}

	// 2. 构造与内核 API 完全一致的 Map 结构
	proxiesMap := make(map[string]interface{})

	// ⭐️ 核心修复 2：遍历底层节点，将其注入到 Map 中，供前端提取协议名
	for _, p := range conf.Proxies {
		name, _ := p["name"].(string)
		pType, _ := p["type"].(string)
		proxiesMap[name] = map[string]interface{}{
			"name": name,
			"type": pType, // 这里就是真实的 vless, trojan, hysteria2 等
		}
	}

	for _, g := range conf.ProxyGroups {
		name, _ := g["name"].(string)
		gTypeRaw, _ := g["type"].(string)

		gType := gTypeRaw
		switch gTypeRaw {
		case "select":
			gType = "Selector"
		case "url-test":
			gType = "URLTest"
		case "fallback":
			gType = "Fallback"
		case "load-balance":
			gType = "LoadBalance"
		}

		var all []string
		if pList, ok := g["proxies"].([]interface{}); ok {
			for _, p := range pList {
				if s, ok := p.(string); ok {
					all = append(all, s)
				}
			}
		}

		// 模拟内核的单组数据结构
		proxiesMap[name] = map[string]interface{}{
			"name": name,
			"type": gType,
			"now":  "",
			"all":  all,
		}
	}

	return map[string]interface{}{
		"mode":       conf.Mode,
		"groups":     proxiesMap, // 现在包含了所有真实节点信息
		"groupOrder": ExtractGroupOrder(data),
	}, nil
}

// TunConfig 映射 yaml 中的 tun 配置块
type TunConfig struct {
	Enable              bool     `yaml:"enable" json:"-"` // Not persisted to user_tun.json anymore
	Stack               string   `yaml:"stack" json:"stack"`
	Device              string   `yaml:"device,omitempty" json:"device"`
	AutoRoute           bool     `yaml:"auto-route" json:"autoRoute"`
	AutoDetectInterface bool     `yaml:"auto-detect-interface" json:"autoDetect"`
	DNSHijack           []string `yaml:"dns-hijack" json:"dnsHijack"`
	StrictRoute         bool     `yaml:"strict-route" json:"strictRoute"`
	MTU                 int      `yaml:"mtu" json:"mtu"`
}

// ================= TUN 设置 =================
func GetDefaultTunConfig() TunConfig {
	return TunConfig{
		Enable:              false,
		Stack:               "gvisor",
		Device:              "GOCLASHZ",
		AutoRoute:           true,
		AutoDetectInterface: true,
		DNSHijack:           []string{"any:53"},
		StrictRoute:         false,
		MTU:                 1430,
	}
}

func GetTunConfig() (*TunConfig, error) {
	defaultTun := GetDefaultTunConfig()
	return utils.LoadSetting("tun", defaultTun)
}

func UpdateTunConfig(newTun *TunConfig) error {
	return utils.SaveSetting("tun", newTun)
}

// -------------------- DNS 配置相关 --------------------

// FallbackFilterConfig 映射 yaml 中 dns.fallback-filter 配置块
type FallbackFilterConfig struct {
	GeoIP     bool     `yaml:"geoip" json:"geoip"`
	GeoIPCode string   `yaml:"geoip-code" json:"geoipCode"`
	IPCIDR    []string `yaml:"ipcidr" json:"ipcidr"`
	Domain    []string `yaml:"domain,omitempty" json:"domain"` // 👈 新增：域名过滤
}

// DNSConfig 映射 yaml 中的 dns 配置块
type DNSConfig struct {
	Enable                bool                 `yaml:"enable" json:"enable"`
	Listen                string               `yaml:"listen,omitempty" json:"listen"` // 👈 新增：监听端口
	IPv6                  bool                 `yaml:"ipv6" json:"ipv6"`
	PreferH3              bool                 `yaml:"prefer-h3,omitempty" json:"preferH3"` // 👈 新增：偏好 HTTP/3
	EnhancedMode          string               `yaml:"enhanced-mode" json:"enhancedMode"`
	RespectRules          bool                 `yaml:"respect-rules,omitempty" json:"respectRules"` // 👈 新增：遵守规则
	FakeIPRange           string               `yaml:"fake-ip-range,omitempty" json:"fakeIpRange"`
	FakeIPFilter          []string             `yaml:"fake-ip-filter,omitempty" json:"fakeIpFilter"`
	UseSystemHosts        bool                 `yaml:"use-system-hosts,omitempty" json:"useSystemHosts"`
	UseHosts              bool                 `yaml:"use-hosts,omitempty" json:"useHosts"`
	DefaultNameserver     []string             `yaml:"default-nameserver,omitempty" json:"defaultNameserver"`
	Nameserver            []string             `yaml:"nameserver" json:"nameserver"`
	Fallback              []string             `yaml:"fallback,omitempty" json:"fallback"`
	DirectNameserver      []string             `yaml:"direct-nameserver,omitempty" json:"directNameserver"` // 👈 新增：直连 DNS
	ProxyServerNameserver []string             `yaml:"proxy-server-nameserver,omitempty" json:"proxyServerNameserver"`
	NameserverPolicy      map[string]string    `yaml:"nameserver-policy,omitempty" json:"nameserverPolicy"`
	FallbackFilter        FallbackFilterConfig `yaml:"fallback-filter" json:"fallbackFilter"`
}

// ================= DNS 设置 =================
func GetDefaultDNSConfig() DNSConfig {
	return DNSConfig{
		Enable:                true,
		Listen:                "0.0.0.0:1053",
		IPv6:                  false,
		PreferH3:              false,
		EnhancedMode:          "fake-ip",
		RespectRules:          false,
		FakeIPRange:           "198.18.0.1/16",
		FakeIPFilter:          []string{"*.lan", "*.localdomain", "*.example", "*.invalid", "*.localhost", "*.test", "lan", "localdomain", "localhost"},
		UseSystemHosts:        true,
		UseHosts:              false,
		DefaultNameserver:     []string{"223.5.5.5", "114.114.114.114"},
		Nameserver:            []string{"https://doh.pub/dns-query", "https://dns.alidns.com/dns-query"},
		Fallback:              []string{"https://doh.dns.sb/dns-query", "https://dns.cloudflare.com/dns-query"},
		DirectNameserver:      []string{"https://dns.alidns.com/dns-query", "https://doh.pub/dns-query"},
		ProxyServerNameserver: []string{"223.5.5.5"},
		NameserverPolicy:      map[string]string{"geosite:cn": "https://doh.pub/dns-query"},
		FallbackFilter: FallbackFilterConfig{
			GeoIP:     true,
			GeoIPCode: "CN",
			IPCIDR:    []string{"240.0.0.0/4", "0.0.0.0/32"},
			Domain:    []string{"+.google.com", "+.facebook.com", "+.twitter.com"},
		},
	}
}

func GetDNSConfig() (*DNSConfig, error) {
	defaultDNS := GetDefaultDNSConfig()
	return utils.LoadSetting("dns", defaultDNS)
}

func UpdateDNSConfig(newDNS *DNSConfig) error {
	return utils.SaveSetting("dns", newDNS)
}

// ================= 基础网络设置 =================
func GetDefaultNetworkConfig() NetworkConfig {
	return NetworkConfig{
		Port:                 0,
		MixedPort:            7890,
		IPv6:                 false,
		UnifiedDelay:         true,
		TCPConcurrent:        true,
		TCPKeepAlive:         true,
		TCPKeepAliveInterval: 30,
		TestURL:              "http://www.g.cn/generate_204",
		ExternalController:   "127.0.0.1:9090", // 🚀 默认值
		AllowLan:             false,
		Hosts:                "",
	}
}

func GetNetworkConfig() (*NetworkConfig, error) {
	defaultNet := GetDefaultNetworkConfig()
	return utils.LoadSetting("network", defaultNet)
}

// GetProxyPort 获取代理端口：优先 MixedPort，其次 Port，兜底 7890
func GetProxyPort() int {
	if netCfg, err := GetNetworkConfig(); err == nil && netCfg != nil {
		if netCfg.MixedPort != 0 {
			return netCfg.MixedPort
		}
		if netCfg.Port != 0 {
			return netCfg.Port
		}
	}
	return 7890
}

func UpdateNetworkConfig(newCfg *NetworkConfig) error {
	return utils.SaveSetting("network", newCfg)
}

// ==========================================
// --- 运行时参数注入器 ---
// ==========================================

// BuildRuntimeConfig 核心流水线：基础配置 + 用户设置 = 最终运行配置
// SmartGroupName — имя авто-группы Smart, добавляемой в каждый конфиг.
const SmartGroupName = "⚡ Смарт"

// injectSmartGroup добавляет в конфиг proxy-group типа smart (ядро vernesong с LightGBM).
// include-all:true цепляет ВСЕ узлы — и из списка proxies, и из proxy-providers (важно: многие
// подписки отдают узлы только через провайдеры). Инжектится ТОЛЬКО когда установлено смарт-ядро —
// иначе стоковое ядро MetaCubeX не знает type=smart и конфиг не распарсится.
func injectSmartGroup(root map[string]interface{}) {
	if !IsSmartCoreActive() {
		return
	}

	// есть ли вообще узлы (inline или через провайдеры)?
	_, hasProxies := root["proxies"].([]interface{})
	_, hasProviders := root["proxy-providers"].(map[string]interface{})
	if !hasProxies && !hasProviders {
		return
	}

	groups, _ := root["proxy-groups"].([]interface{})

	// если Smart-группа уже есть — не дублируем
	for _, g := range groups {
		if gm, ok := g.(map[string]interface{}); ok {
			if n, _ := gm["name"].(string); n == SmartGroupName {
				return
			}
		}
	}

	smart := map[string]interface{}{
		"name":        SmartGroupName,
		"type":        "smart",
		"include-all": true,
		"uselightgbm": true,               // ML-модель (Model.bin) — умный выбор по обученным весам
		"collectdata": true,               // копит историю задержек/успешности для обучения
		"strategy":    "sticky-sessions",  // трафик к одному адресу держится на одном узле —
		//                                    смарт не перекидывает сервера посреди игры
	}

	// ставим Smart-группу первой, чтобы была видна вверху выбора
	root["proxy-groups"] = append([]interface{}{smart}, groups...)
}

const smartRouteSettingKey = "smart_route" // рулит ли Смарт всем прокси-трафиком (режим по правилам)

// IsSmartRouteActive — включён ли режим «весь прокси-трафик через Смарт».
func IsSmartRouteActive() bool {
	v, err := utils.LoadSetting(smartRouteSettingKey, false)
	if err != nil || v == nil {
		return false
	}
	return *v
}

// SetSmartRouteActive сохраняет флаг.
func SetSmartRouteActive(active bool) error {
	return utils.SaveSetting(smartRouteSettingKey, &active)
}

// специальные цели правил, которые НЕ перенаправляем на Смарт (сохраняем сплит подписки).
var nonSmartTargets = map[string]bool{
	"DIRECT": true, "REJECT": true, "REJECT-DROP": true, "PASS": true, "COMPATIBLE": true,
}

// rewriteRulesToSmart перенаправляет все прокси-правила на Смарт-группу, оставляя DIRECT/REJECT.
// Так РФ-напрямую и блок рекламы остаются, а весь прокси-трафик идёт через смарт-выбор.
func rewriteRulesToSmart(root map[string]interface{}) {
	rulesRaw, ok := root["rules"].([]interface{})
	if !ok {
		return
	}
	for i, r := range rulesRaw {
		line, ok := r.(string)
		if !ok {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		// последний непустой сегмент — это цель (у MATCH их 2: MATCH,TARGET; у обычных 3+ и могут быть опции no-resolve)
		// определяем цель: для MATCH — parts[1]; иначе parts[2] (тип,значение,цель[,опции])
		var tIdx int
		if strings.EqualFold(strings.TrimSpace(parts[0]), "MATCH") {
			tIdx = 1
		} else if len(parts) >= 3 {
			tIdx = 2
		} else {
			continue
		}
		target := strings.TrimSpace(parts[tIdx])
		up := strings.ToUpper(target)
		if nonSmartTargets[up] || target == SmartGroupName {
			continue
		}
		parts[tIdx] = SmartGroupName
		rulesRaw[i] = strings.Join(parts, ",")
	}
	root["rules"] = rulesRaw
}

func BuildRuntimeConfig(id string, mode string, logLevel string, tunEnabled bool) error {
	configMu.Lock()         // ✅ 加锁
	defer configMu.Unlock() // ✅ 保证最终释放

	configPath := GetConfigPath() // 目标: DataDir/config.yaml

	// 1. 提取当前界面的全局设置 (避免被覆盖丢失)
	userDns, err := GetDNSConfig()
	if err != nil {
		return fmt.Errorf("не удалось прочитать настройки DNS: %w", err)
	}

	userTun, err := GetTunConfig()
	if err != nil {
		return fmt.Errorf("не удалось прочитать настройки TUN: %w", err)
	}

	userNet, err := GetNetworkConfig()
	if err != nil {
		return fmt.Errorf("не удалось прочитать сетевые настройки: %w", err)
	}

	// 2. 读取选中的订阅文件作为 "Base Config" (只读模板)
	var root map[string]interface{}
	var runtimeRules []string

	if id != "" && id != "config.yaml" {
		if err := WithRuleStorageLock(func() error {
			recoveredRoot, innerErr := ReadWorkingRootWithRecovery(id)
			if innerErr != nil {
				return fmt.Errorf("не удалось прочитать или восстановить конфигурацию: %w", innerErr)
			}
			root = recoveredRoot

			rules, err := BuildRuntimeRules(id, root)
			if err != nil {
				return err
			}
			runtimeRules = rules
			return nil
		}); err != nil {
			return err
		}
	} else {
		_, profilePath, err := ProfilePathByID(id)
		if err != nil {
			return err
		}

		baseData, err := os.ReadFile(profilePath)
		if err != nil {
			return fmt.Errorf("не удалось прочитать базовую конфигурацию: %w (путь: %s)", err, profilePath)
		}

		if err := yaml.Unmarshal(baseData, &root); err != nil {
			return fmt.Errorf("не удалось разобрать базовую конфигурацию: %v", err)
		}

		rules, err := BuildRuntimeRules(id, root)
		if err != nil {
			return fmt.Errorf("не удалось построить runtime-правила: %w", err)
		}
		runtimeRules = rules
	}

	// 3. 运行时参数强制注入 (Injector)
	// 注入模式 (rule / global / direct)
	if mode != "" {
		root["mode"] = mode
	}

	// Маршрутизация по приложениям работает через секцию rules, а global/direct её
	// игнорируют — поэтому при активной фиче принудительно включаем rule-режим.
	if id != "" {
		if ar, _ := LoadAppRouting(id); ar.Mode == AppRouteBlacklist || ar.Mode == AppRouteWhitelist {
			if len(ar.Apps) > 0 {
				root["mode"] = "rule"
			}
		}
	}

	// 注入基础网络设置
	if userNet != nil {
		root["ipv6"] = userNet.IPv6
		root["unified-delay"] = userNet.UnifiedDelay
		root["tcp-concurrent"] = userNet.TCPConcurrent
		root["tcp-keep-alive"] = userNet.TCPKeepAlive
		root["tcp-keep-alive-interval"] = userNet.TCPKeepAliveInterval

		// 👈 新增：注入自定义 Hosts 映射
		if userNet.Hosts != "" {
			var hostsMap map[string]interface{}
			if err := yaml.Unmarshal([]byte(userNet.Hosts), &hostsMap); err == nil {
				root["hosts"] = hostsMap
			}
		}
	}

	// 注入 TUN 配置
	if userTun != nil {
		tunRuntime := *userTun
		tunRuntime.Enable = tunEnabled
		root["tun"] = tunRuntime
	}

	// 注入 DNS 配置
	if userDns != nil && userDns.Enable {
		root["dns"] = map[string]interface{}{
			"enable":                  userDns.Enable,
			"listen":                  userDns.Listen,
			"ipv6":                    userDns.IPv6,
			"prefer-h3":               userDns.PreferH3,
			"enhanced-mode":           userDns.EnhancedMode,
			"respect-rules":           userDns.RespectRules,
			"fake-ip-range":           userDns.FakeIPRange,
			"fake-ip-filter":          userDns.FakeIPFilter,
			"use-system-hosts":        userDns.UseSystemHosts,
			"use-hosts":               userDns.UseHosts,
			"default-nameserver":      userDns.DefaultNameserver,
			"nameserver":              userDns.Nameserver,
			"fallback":                userDns.Fallback,
			"direct-nameserver":       userDns.DirectNameserver,
			"proxy-server-nameserver": userDns.ProxyServerNameserver,
			"nameserver-policy":       userDns.NameserverPolicy,
			"fallback-filter":         userDns.FallbackFilter,
		}
	}

	// 👇 核心注入：遍历所有策略组，将用户设置的测速网址应用到 url-test/fallback 组
	if userNet != nil && userNet.TestURL != "" {
		if groups, ok := root["proxy-groups"].([]interface{}); ok {
			for _, g := range groups {
				if group, ok := g.(map[string]interface{}); ok {
					gType, _ := group["type"].(string)
					// 仅对具有自动测速性质的组生效
					if gType == "url-test" || gType == "fallback" || gType == "load-balance" {
						group["url"] = userNet.TestURL
					}
				}
			}
		}
	}

	// Инжект Smart-группы (type=smart, ML-модель) — доступна для выбора и в Lite, и в Про.
	injectSmartGroup(root)

	// Если включено «Использовать Смарт» — весь прокси-трафик правил идёт через Смарт-группу.
	if IsSmartCoreActive() && IsSmartRouteActive() {
		rewriteRulesToSmart(root)
	}

	// 👇 新增：强制注入混合代理端口和外部控制 API
	// 确保在不启用 TUN 时，系统代理能够将流量送入内核
	if userNet != nil && userNet.MixedPort != 0 {
		root["mixed-port"] = userNet.MixedPort
	} else if userNet != nil && userNet.Port != 0 {
		// 如果只有 Port 没有 MixedPort，优先保证连通性
		root["port"] = userNet.Port
		delete(root, "mixed-port")
	} else {
		root["mixed-port"] = 7890
	}
	allowLan := false
	if userNet != nil {
		allowLan = userNet.AllowLan
	}
	root["allow-lan"] = allowLan

	// 👇 核心注入：控制端口动态化
	controller := "127.0.0.1:9090"
	if userNet != nil && strings.TrimSpace(userNet.ExternalController) != "" {
		controller = NormalizeControllerHostPort(userNet.ExternalController)
	}
	root["external-controller"] = controller
	root["secret"] = "" // 确保没有意外的密码阻挡前端 WebSocket

	// 同步更新 Go API Client 的基准地址
	UpdateAPIBaseURL(controller)

	// 👇 核心注入：规则接管：local 使用 working.rules；remote 使用 overlay.add + (working.rules - overlay.delete)
	root["rules"] = runtimeRules

	// 👇 核心新增：动态读取并注入我们设置的日志等级
	if logLevel != "" {
		root["log-level"] = logLevel
	} else {
		root["log-level"] = "info"
	}

	// 4. 执行最终严格的引用校验，确保没有悬空引用导致 mihomo 启动崩溃
	if err := ValidateClashReferences(root); err != nil {
		return fmt.Errorf("проверка целостности ссылок конфигурации не пройдена: %w", err)
	}

	// 5. 序列化并生成最终的 config.yaml
	out, err := yaml.Marshal(root)
	if err != nil {
		return err
	}

	return utils.WriteFileAtomic(configPath, out, 0644)
}

// ExtractGroupOrder 核心逻辑：从 YAML 数据中提取 proxy-groups 的原始定义顺序
func ExtractGroupOrder(yamlData []byte) []string {
	var order []string
	var node yaml.Node
	if err := yaml.Unmarshal(yamlData, &node); err == nil && len(node.Content) > 0 {
		// 遍历根节点，寻找 proxy-groups
		for i := 0; i < len(node.Content[0].Content); i += 2 {
			keyNode := node.Content[0].Content[i]
			if keyNode.Value == "proxy-groups" {
				valueNode := node.Content[0].Content[i+1]
				for _, groupNode := range valueNode.Content {
					// 提取每个 group 的 name
					for j := 0; j < len(groupNode.Content); j += 2 {
						if groupNode.Content[j].Value == "name" {
							order = append(order, groupNode.Content[j+1].Value)
							break
						}
					}
				}
				break
			}
		}
	}
	return order
}
