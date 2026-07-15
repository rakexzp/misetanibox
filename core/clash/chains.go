package clash

import (
	"strconv"
	"strings"

	"goclashz/core/utils"
)

// ProxyChain — пользовательская цепочка прокси: трафик идёт узел[0] → узел[1] → … → выход.
type ProxyChain struct {
	Name  string   `json:"name"`
	Nodes []string `json:"nodes"`
}

const proxyChainsKey = "proxy_chains"

// ChainPrefix — префикс имени узла-выхода цепочки (виден в списке, выбирается как обычный сервер).
const ChainPrefix = "🔗 "

// ChainHopPrefix — префикс внутренних промежуточных хопов цепочки (скрыты в UI фронтом).
const ChainHopPrefix = "⛓ "

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

// cloneProxyMap — поверхностная копия определения прокси (меняем только name/dialer-proxy сверху).
func cloneProxyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src)+2)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// injectChains строит цепочки через dialer-proxy (тип relay в mihomo удалён).
// Для цепочки [n0, n1, … nk]: вход n0 — оригинальный узел, каждый следующий — копия узла
// с полем dialer-proxy на предыдущий хоп; последний — копия-выход с именем «🔗 Name»
// (её и выбирает юзер). Выход добавляется в пользовательские select-группы, чтобы быть выбираемым.
func injectChains(root map[string]interface{}) {
	chains := GetChains()
	if len(chains) == 0 {
		return
	}
	proxies, ok := root["proxies"].([]interface{})
	if !ok || len(proxies) == 0 {
		return
	}

	// индекс определений узлов по имени
	byName := make(map[string]map[string]interface{}, len(proxies))
	for _, p := range proxies {
		if pm, ok := p.(map[string]interface{}); ok {
			if n, _ := pm["name"].(string); n != "" {
				byName[n] = pm
			}
		}
	}

	var added []interface{}
	var exits []string
	for _, ch := range chains {
		name := strings.TrimSpace(ch.Name)
		if name == "" {
			continue
		}
		nodes := make([]string, 0, len(ch.Nodes))
		for _, n := range ch.Nodes {
			if n != "" {
				nodes = append(nodes, n)
			}
		}
		if len(nodes) < 2 {
			continue
		}
		// все узлы должны быть определены в proxies (dialer-proxy требует реальный узел);
		// провайдерные/отсутствующие цепочки пропускаем, чтобы не уронить ядро
		valid := true
		for _, n := range nodes {
			if _, f := byName[n]; !f {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}

		prev := nodes[0] // вход — оригинальный узел (как dialer-proxy)
		for i := 1; i < len(nodes); i++ {
			def := cloneProxyMap(byName[nodes[i]])
			var cname string
			if i == len(nodes)-1 {
				cname = ChainPrefix + name // выход — видимый, выбираемый
			} else {
				cname = ChainHopPrefix + name + " " + strconv.Itoa(i) // промежуточный — скрыт
			}
			def["name"] = cname
			def["dialer-proxy"] = prev
			added = append(added, def)
			prev = cname
		}
		exits = append(exits, ChainPrefix+name)
	}
	if len(added) == 0 {
		return
	}
	root["proxies"] = append(proxies, added...)

	// добавить выходы цепочек в пользовательские select-группы выбора сервера,
	// чтобы они были выбираемы как обычные узлы (include-all-группы подхватят сами)
	if groups, ok := root["proxy-groups"].([]interface{}); ok {
		for _, g := range groups {
			gm, ok := g.(map[string]interface{})
			if !ok {
				continue
			}
			if t, _ := gm["type"].(string); t != "select" {
				continue
			}
			if ia, _ := gm["include-all"].(bool); ia {
				continue
			}
			if ia, _ := gm["include-all-proxies"].(bool); ia {
				continue
			}
			gname, _ := gm["name"].(string)
			if strings.HasPrefix(gname, ChainPrefix) || strings.HasPrefix(gname, ChainHopPrefix) {
				continue
			}
			gp, _ := gm["proxies"].([]interface{})
			// только группы, где уже есть хотя бы один реальный узел (группы выбора сервера)
			hasRealNode := false
			for _, m := range gp {
				if mn, _ := m.(string); byName[mn] != nil {
					hasRealNode = true
					break
				}
			}
			if !hasRealNode {
				continue
			}
			for _, e := range exits {
				gp = append(gp, e)
			}
			gm["proxies"] = gp
		}
	}
}
