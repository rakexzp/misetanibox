package clash

import "goclashz/core/utils"

// ProxyChain — пользовательская цепочка прокси (relay): трафик идёт узел[0] → узел[1] → … → выход.
type ProxyChain struct {
	Name  string   `json:"name"`
	Nodes []string `json:"nodes"`
}

const proxyChainsKey = "proxy_chains"

// ChainPrefix — префикс имени группы-цепочки в списке прокси.
const ChainPrefix = "🔗 "

func GetChains() []ProxyChain {
	v, err := utils.LoadSetting(proxyChainsKey, []ProxyChain{})
	if err != nil || v == nil {
		return []ProxyChain{}
	}
	return *v
}

func SaveChains(chains []ProxyChain) error {
	if chains == nil {
		chains = []ProxyChain{}
	}
	return utils.SaveSetting(proxyChainsKey, &chains)
}

// injectChains добавляет relay-группы цепочек в конфиг (как injectSmartGroup).
func injectChains(root map[string]interface{}) {
	chains := GetChains()
	if len(chains) == 0 {
		return
	}
	_, hasProxies := root["proxies"].([]interface{})
	_, hasProviders := root["proxy-providers"].(map[string]interface{})
	if !hasProxies && !hasProviders {
		return
	}
	groups, _ := root["proxy-groups"].([]interface{})

	var relays []interface{}
	for _, ch := range chains {
		if ch.Name == "" || len(ch.Nodes) < 2 {
			continue // цепочка из <2 узлов бессмысленна
		}
		proxies := make([]interface{}, 0, len(ch.Nodes))
		for _, n := range ch.Nodes {
			if n != "" {
				proxies = append(proxies, n)
			}
		}
		if len(proxies) < 2 {
			continue
		}
		relays = append(relays, map[string]interface{}{
			"name":    ChainPrefix + ch.Name,
			"type":    "relay",
			"proxies": proxies,
		})
	}
	if len(relays) == 0 {
		return
	}
	// цепочки — вперёд, чтобы были заметны в списке
	root["proxy-groups"] = append(relays, groups...)
}
