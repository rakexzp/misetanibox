package appcore

import (
	"context"
	"goclashz/core/clash"
)

const LiteCoreVersion = "v1.19.31"

type LiteSnapshot struct {
	Profiles            []LiteProfile `json:"profiles"`
	ActiveProfile       string        `json:"activeProfile"`
	Desired             bool          `json:"desired"`
	Running             bool          `json:"running"`
	SystemProxy         bool          `json:"systemProxy"`
	TunAvailable        bool          `json:"tunAvailable"`
	TunReason           string        `json:"tunReason"`
	RequiredCoreVersion string        `json:"requiredCoreVersion"`
}

func (s *LiteService) Snapshot() LiteSnapshot {
	c := s.controller
	c.mu.RLock()
	running, proxy := c.userCoreRunning, c.sysProxyActive
	c.mu.RUnlock()
	return LiteSnapshot{Profiles: s.Profiles(), ActiveProfile: c.Behavior.Get().ActiveConfig, Desired: c.Desired.Get().SystemProxy, Running: running && clash.IsRunning(), SystemProxy: proxy, TunReason: "helper_security_not_ready", RequiredCoreVersion: LiteCoreVersion}
}
func (s *LiteService) Connect(ctx context.Context) error {
	return ErrNativeProxyOwnershipUnavailable
}
func (s *LiteService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.controller.Desired.Get()
	d.CoreRunning = false
	d.SystemProxy = false
	d.Tun = false
	saveErr := s.controller.Desired.SetAndSave(d)
	s.controller.Supervisor.ShutdownRuntime("native-stop")
	return saveErr
}
func (s *LiteService) Delete(ctx context.Context, id string) error {
	if s.controller.Behavior.Get().ActiveConfig == id {
		if err := s.Stop(ctx); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.controller.DeleteConfig(id)
}
func (s *LiteService) Ping(ctx context.Context, profileID, name string) (int, error) {
	// Delay tests also start a core using shared fixed ports. Keep them disabled
	// alongside connect until the native runtime has independent listeners.
	return 0, ErrNativeProxyOwnershipUnavailable
}
