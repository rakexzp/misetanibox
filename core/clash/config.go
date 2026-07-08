package clash

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"goclashz/core/utils"

	"gopkg.in/yaml.v3"
)

var configMu sync.Mutex

func GetConfigPath() string {
	return utils.GetRuntimeConfigPath()
}

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

type OfflineGroup struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Proxies []string `json:"proxies"`
}

type ProxyGroupSchema struct {
	Name string   `json:"name"`
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

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

	var conf struct {
		Mode        string                   `yaml:"mode"`
		Proxies     []map[string]interface{} `yaml:"proxies"`
		ProxyGroups []map[string]interface{} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(data, &conf); err != nil {
		return nil, err
	}

	proxiesMap := make(map[string]interface{})

	for _, p := range conf.Proxies {
		name, _ := p["name"].(string)
		pType, _ := p["type"].(string)
		proxiesMap[name] = map[string]interface{}{
			"name": name,
			"type": pType,
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

		proxiesMap[name] = map[string]interface{}{
			"name": name,
			"type": gType,
			"now":  "",
			"all":  all,
		}
	}

	return map[string]interface{}{
		"mode":       conf.Mode,
		"groups":     proxiesMap,
		"groupOrder": ExtractGroupOrder(data),
	}, nil
}

type TunConfig struct {
	Enable              bool     `yaml:"enable" json:"-"`
	Stack               string   `yaml:"stack" json:"stack"`
	Device              string   `yaml:"device,omitempty" json:"device"`
	AutoRoute           bool     `yaml:"auto-route" json:"autoRoute"`
	AutoDetectInterface bool     `yaml:"auto-detect-interface" json:"autoDetect"`
	DNSHijack           []string `yaml:"dns-hijack" json:"dnsHijack"`
	StrictRoute         bool     `yaml:"strict-route" json:"strictRoute"`
	MTU                 int      `yaml:"mtu" json:"mtu"`
}

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

type FallbackFilterConfig struct {
	GeoIP     bool     `yaml:"geoip" json:"geoip"`
	GeoIPCode string   `yaml:"geoip-code" json:"geoipCode"`
	IPCIDR    []string `yaml:"ipcidr" json:"ipcidr"`
	Domain    []string `yaml:"domain,omitempty" json:"domain"`
}

type DNSConfig struct {
	Enable                bool                 `yaml:"enable" json:"enable"`
	Listen                string               `yaml:"listen,omitempty" json:"listen"`
	IPv6                  bool                 `yaml:"ipv6" json:"ipv6"`
	PreferH3              bool                 `yaml:"prefer-h3,omitempty" json:"preferH3"`
	EnhancedMode          string               `yaml:"enhanced-mode" json:"enhancedMode"`
	RespectRules          bool                 `yaml:"respect-rules,omitempty" json:"respectRules"`
	FakeIPRange           string               `yaml:"fake-ip-range,omitempty" json:"fakeIpRange"`
	FakeIPFilter          []string             `yaml:"fake-ip-filter,omitempty" json:"fakeIpFilter"`
	UseSystemHosts        bool                 `yaml:"use-system-hosts,omitempty" json:"useSystemHosts"`
	UseHosts              bool                 `yaml:"use-hosts,omitempty" json:"useHosts"`
	DefaultNameserver     []string             `yaml:"default-nameserver,omitempty" json:"defaultNameserver"`
	Nameserver            []string             `yaml:"nameserver" json:"nameserver"`
	Fallback              []string             `yaml:"fallback,omitempty" json:"fallback"`
	DirectNameserver      []string             `yaml:"direct-nameserver,omitempty" json:"directNameserver"`
	ProxyServerNameserver []string             `yaml:"proxy-server-nameserver,omitempty" json:"proxyServerNameserver"`
	NameserverPolicy      map[string]string    `yaml:"nameserver-policy,omitempty" json:"nameserverPolicy"`
	FallbackFilter        FallbackFilterConfig `yaml:"fallback-filter" json:"fallbackFilter"`
}

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
		ExternalController:   "127.0.0.1:9090",
		AllowLan:             false,
		Hosts:                "",
	}
}

func GetNetworkConfig() (*NetworkConfig, error) {
	defaultNet := GetDefaultNetworkConfig()
	return utils.LoadSetting("network", defaultNet)
}

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

const SmartGroupName = "⚡ Смарт"

func injectSmartGroup(root map[string]interface{}) {
	if !IsSmartCoreActive() {
		return
	}

	_, hasProxies := root["proxies"].([]interface{})
	_, hasProviders := root["proxy-providers"].(map[string]interface{})
	if !hasProxies && !hasProviders {
		return
	}

	groups, _ := root["proxy-groups"].([]interface{})

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
		"uselightgbm": true,
		"collectdata": true,
		"strategy":    "sticky-sessions",
	}

	root["proxy-groups"] = append([]interface{}{smart}, groups...)
}

const smartRouteSettingKey = "smart_route"

func IsSmartRouteActive() bool {
	v, err := utils.LoadSetting(smartRouteSettingKey, false)
	if err != nil || v == nil {
		return false
	}
	return *v
}

func SetSmartRouteActive(active bool) error {
	return utils.SaveSetting(smartRouteSettingKey, &active)
}

var nonSmartTargets = map[string]bool{
	"DIRECT": true, "REJECT": true, "REJECT-DROP": true, "PASS": true, "COMPATIBLE": true,
}

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
	configMu.Lock()
	defer configMu.Unlock()

	configPath := GetConfigPath()

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

	if mode != "" {
		root["mode"] = mode
	}

	if id != "" {
		if ar, _ := LoadAppRouting(id); ar.Mode == AppRouteBlacklist || ar.Mode == AppRouteWhitelist {
			if len(ar.Apps) > 0 {
				root["mode"] = "rule"
			}
		}
	}

	if userNet != nil {
		root["ipv6"] = userNet.IPv6
		root["unified-delay"] = userNet.UnifiedDelay
		root["tcp-concurrent"] = userNet.TCPConcurrent
		root["tcp-keep-alive"] = userNet.TCPKeepAlive
		root["tcp-keep-alive-interval"] = userNet.TCPKeepAliveInterval

		if userNet.Hosts != "" {
			var hostsMap map[string]interface{}
			if err := yaml.Unmarshal([]byte(userNet.Hosts), &hostsMap); err == nil {
				root["hosts"] = hostsMap
			}
		}
	}

	if userTun != nil {
		tunRuntime := *userTun
		tunRuntime.Enable = tunEnabled
		root["tun"] = tunRuntime
	}

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

	if userNet != nil && userNet.TestURL != "" {
		if groups, ok := root["proxy-groups"].([]interface{}); ok {
			for _, g := range groups {
				if group, ok := g.(map[string]interface{}); ok {
					gType, _ := group["type"].(string)

					if gType == "url-test" || gType == "fallback" || gType == "load-balance" {
						group["url"] = userNet.TestURL
					}
				}
			}
		}
	}

	injectSmartGroup(root)

	if IsSmartCoreActive() && IsSmartRouteActive() {
		rewriteRulesToSmart(root)
	}

	if userNet != nil && userNet.MixedPort != 0 {
		root["mixed-port"] = userNet.MixedPort
	} else if userNet != nil && userNet.Port != 0 {

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

	controller := "127.0.0.1:9090"
	if userNet != nil && strings.TrimSpace(userNet.ExternalController) != "" {
		controller = NormalizeControllerHostPort(userNet.ExternalController)
	}
	root["external-controller"] = controller
	root["secret"] = ""

	UpdateAPIBaseURL(controller)

	root["rules"] = runtimeRules

	if logLevel != "" {
		root["log-level"] = logLevel
	} else {
		root["log-level"] = "info"
	}

	if err := ValidateClashReferences(root); err != nil {
		return fmt.Errorf("проверка целостности ссылок конфигурации не пройдена: %w", err)
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return err
	}

	return utils.WriteFileAtomic(configPath, out, 0644)
}

func ExtractGroupOrder(yamlData []byte) []string {
	var order []string
	var node yaml.Node
	if err := yaml.Unmarshal(yamlData, &node); err == nil && len(node.Content) > 0 {

		for i := 0; i < len(node.Content[0].Content); i += 2 {
			keyNode := node.Content[0].Content[i]
			if keyNode.Value == "proxy-groups" {
				valueNode := node.Content[0].Content[i+1]
				for _, groupNode := range valueNode.Content {

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
