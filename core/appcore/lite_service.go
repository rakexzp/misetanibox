package appcore

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"goclashz/core/clash"
)

// LiteService serializes mutations made by the native client. Construct it only
// after the backend has acquired the data-directory lock and loaded the index.
// It is not a second controller and must not be used alongside Wails mutations.
type LiteService struct {
	controller *Controller
	mu         sync.Mutex
}

func NewLiteService(controller *Controller) *LiteService {
	return &LiteService{controller: controller}
}

type LiteProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Updated  int64  `json:"updated"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
	Total    int64  `json:"total"`
	Expire   int64  `json:"expire"`
}

// Profiles deliberately excludes URLs, fallback URLs, headers and renewal URLs.
func (s *LiteService) Profiles() []LiteProfile {
	out := []LiteProfile{}
	for _, p := range clash.ListSubIndex() {
		out = append(out, LiteProfile{ID: p.ID, Name: p.Name, Type: p.Type, Updated: p.Updated, Upload: p.Upload, Download: p.Download, Total: p.Total, Expire: p.Expire})
	}
	return out
}

func (c *Controller) ProbeSubscription(ctx context.Context, url string, headers map[string]string) clash.SubProbe {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return clash.ProbeSubURL(ctx, url, c.Behavior.Get().SubUA, headers)
}

// AddSubscriptionViaDNS preserves the existing Wails return contract: URL for a
// remote TXT result, profile ID for inline YAML. IPC must not return that URL.
func (c *Controller) AddSubscriptionViaDNS(ctx context.Context, domain, name string) (string, error) {
	content, err := clash.ResolveConfigViaDNS(ctx, domain)
	if err != nil {
		return "", err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("TXT-запись пуста")
	}
	if strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://") {
		if err := c.UpdateSub(ctx, name, content, nil); err != nil {
			return "", err
		}
		return content, nil
	}
	raw := []byte(content)
	if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(content, "base64:"))); err == nil && len(decoded) > 0 {
		raw = decoded
	}
	file, err := os.CreateTemp("", "mise-dns-*.yaml")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		name = "DNS-конфиг"
	}
	return c.DoLocalImport(file.Name(), name)
}

func (s *LiteService) AddURL(ctx context.Context, name, url string, convert bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.controller.UpdateSub(ctx, name, url, &clash.SubFetchOptions{Convert: convert})
}

func (s *LiteService) AddDNS(ctx context.Context, domain, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.controller.AddSubscriptionViaDNS(ctx, domain, name)
	return err
}

func (s *LiteService) AddLocal(path, name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.controller.DoLocalImport(path, name)
}

func (s *LiteService) Refresh(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.controller.UpdateSingleSub(ctx, id)
}

func (s *LiteService) SelectProfile(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.controller.LiteProfileServers(id); err != nil {
		return err
	}
	return s.controller.SelectLocalConfig(ctx, id)
}

func (s *LiteService) Servers() (LiteServers, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.controller.LiteProfileServers(s.controller.Behavior.Get().ActiveConfig)
}

// profileID is required so a delayed click from the previous profile cannot
// change the current profile's selection.
func (s *LiteService) SelectServer(ctx context.Context, profileID, groupName, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	behavior := s.controller.Behavior.Get()
	if profileID != behavior.ActiveConfig {
		return errors.New("profile_changed")
	}
	servers, err := s.controller.LiteProfileServers(profileID)
	if err != nil {
		return err
	}
	found := false
	for _, group := range servers.Groups {
		if group.Name != groupName || !strings.EqualFold(group.Type, "select") {
			continue
		}
		for _, member := range group.Members {
			if member == name {
				found = true
			}
		}
	}
	if !found {
		return errors.New("selection_not_in_profile")
	}
	if s.controller.IsUserRunning() {
		return s.controller.SelectProxyWithModeSync(ctx, profileID, behavior.ActiveMode, groupName, name)
	}
	s.controller.SelectOfflineProxyWithModeSync(profileID, behavior.ActiveMode, groupName, name)
	return nil
}
