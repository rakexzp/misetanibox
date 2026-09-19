package appcore

import (
	"strings"

	"goclashz/core/clash"
	"gopkg.in/yaml.v3"
)

// LiteServers contains names/topology only, never proxy credentials or provider URLs.
// Provider members are resolved by mihomo at runtime; an empty static member list
// does not make a provider-backed group invalid.
type LiteServers struct {
	ProfileID string      `json:"profileId"`
	Selector  string      `json:"selector"`
	Groups    []LiteGroup `json:"groups"`
	Nodes     []LiteNode  `json:"nodes"`
}

type LiteGroup struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Members   []string `json:"members"`
	Providers []string `json:"providers"`
	Selected  string   `json:"selected"`
}

type LiteNode struct {
	Name string `json:"name" yaml:"name"`
	Type string `json:"type" yaml:"type"`
}

func parseLiteServers(profileID string, data []byte, selections map[string]string) (LiteServers, error) {
	result := LiteServers{ProfileID: profileID, Groups: []LiteGroup{}, Nodes: []LiteNode{}}
	var cfg struct {
		Rules   []string   `yaml:"rules"`
		Proxies []LiteNode `yaml:"proxies"`
		Groups  []struct {
			Name    string   `yaml:"name"`
			Type    string   `yaml:"type"`
			Proxies []string `yaml:"proxies"`
			Use     []string `yaml:"use"`
		} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return result, err
	}
	for i := len(cfg.Rules) - 1; i >= 0; i-- {
		parts := strings.Split(cfg.Rules[i], ",")
		if len(parts) >= 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "MATCH") {
			result.Selector = strings.TrimSpace(parts[1])
			break
		}
	}
	result.Nodes = append(result.Nodes, cfg.Proxies...)
	for _, group := range cfg.Groups {
		g := LiteGroup{Name: group.Name, Type: group.Type, Members: append([]string{}, group.Proxies...), Providers: append([]string{}, group.Use...)}
		for _, member := range g.Members {
			if member == selections[g.Name] {
				g.Selected = member
				break
			}
		}
		if g.Selected == "" && len(g.Members) > 0 {
			g.Selected = g.Members[0]
		}
		result.Groups = append(result.Groups, g)
	}
	return result, nil
}

// LiteProfileServers reads the selected profile, not a possibly stale runtime YAML.
func (c *Controller) LiteProfileServers(profileID string) (LiteServers, error) {
	if profileID == "" {
		return LiteServers{}, ErrNoActiveConfig
	}
	if _, ok := clash.FindSubIndexByID(profileID); !ok {
		return LiteServers{}, ErrNoActiveConfig
	}
	config, err := clash.ReadConfigText(profileID)
	if err != nil {
		return LiteServers{}, err
	}
	return parseLiteServers(profileID, []byte(config.Content), c.Offline.Snapshot(profileID))
}

func (c *Controller) GetMainSelector() string {
	servers, err := c.LiteProfileServers(c.Behavior.Get().ActiveConfig)
	if err != nil {
		return ""
	}
	return servers.Selector
}
