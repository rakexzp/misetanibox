package appcore

import (
	"goclashz/core/utils"
	"os"
	"path/filepath"
	"sync"
)

type AppBehavior struct {
	SilentStart        bool   `json:"silentStart"`
	CloseToTray        bool   `json:"closeToTray"`
	ColorDelay         bool   `json:"colorDelay"`
	DelayRetention     bool   `json:"delayRetention"`
	DelayRetentionTime string `json:"delayRetentionTime"`
	LogLevel           string `json:"logLevel"`
	AppLogLevel        string `json:"appLogLevel"`
	HideLogs           bool   `json:"hideLogs"`
	SubUA              string `json:"subUA"`

	ActiveConfig string `json:"activeConfig"`
	ActiveMode   string `json:"activeMode"`

	GeoIpLink   string `json:"geoIpLink"`
	GeoSiteLink string `json:"geoSiteLink"`
	MmdbLink    string `json:"mmdbLink"`
	AsnLink     string `json:"asnLink"`

	AutoUpdate      bool   `json:"autoUpdate"`
	UpdateMethod    string `json:"updateMethod"`
	UpdateInterval  int    `json:"updateInterval"`
	LastUpdateCheck int64  `json:"lastUpdateCheck"`

	AutoDelayTest         bool `json:"autoDelayTest"`
	AutoDelayTestInterval int  `json:"autoDelayTestInterval"`

	ProxyTrafficOnly bool `json:"proxyTrafficOnly"`

	StartupWithOS    bool `json:"startupWithOS"`
	RestoreOnStartup bool `json:"restoreOnStartup"`

	LongConnectionProtection bool `json:"longConnectionProtection"`
	DeferRestartWhenActive   bool `json:"deferRestartWhenActive"`
	LongConnectionMinSeconds int  `json:"longConnectionMinSeconds"`
}

type BehaviorStore struct {
	mu    sync.RWMutex
	cache AppBehavior
}

func NewBehaviorStore() *BehaviorStore {

	oldPath := filepath.Join(utils.GetDataDir(), "behavior.json")
	if _, err := os.Stat(oldPath); err == nil {
		newPath := filepath.Join(utils.GetSettingsDir(), "user_behavior.json")

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			_ = os.MkdirAll(utils.GetSettingsDir(), 0755)
			_ = os.Rename(oldPath, newPath)
		} else {

			_ = os.Remove(oldPath)
		}
	}

	store := &BehaviorStore{}
	_ = store.Load()
	return store
}

func (s *BehaviorStore) Get() AppBehavior {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache
}

func (s *BehaviorStore) SetAndSave(b AppBehavior) error {
	b = normalizeBehavior(b)

	s.mu.Lock()
	s.cache = b
	snapshot := s.cache
	s.mu.Unlock()

	return utils.SaveSetting("behavior", &snapshot)
}

func (s *BehaviorStore) SetActiveConfig(id string) error {
	s.mu.Lock()
	s.cache.ActiveConfig = id
	snapshot := s.cache
	s.mu.Unlock()

	return utils.SaveSetting("behavior", &snapshot)
}

func (s *BehaviorStore) Load() error {
	defaults := s.Default()

	cfg, err := utils.LoadSetting("behavior", defaults)
	if err != nil {
		s.mu.Lock()
		s.cache = defaults
		s.mu.Unlock()
		return err
	}

	s.mu.Lock()
	s.cache = normalizeBehavior(*cfg)
	s.mu.Unlock()
	return nil
}

func (s *BehaviorStore) Save() error {
	s.mu.RLock()
	cfg := s.cache
	s.mu.RUnlock()

	return utils.SaveSetting("behavior", &cfg)
}

func normalizeBehavior(b AppBehavior) AppBehavior {
	if b.LogLevel == "" {
		b.LogLevel = "error"
	}

	if b.AppLogLevel == "" {
		b.AppLogLevel = "info"
	}

	if b.DelayRetentionTime == "" {
		b.DelayRetentionTime = "long"
	}

	if b.ActiveMode == "" {
		b.ActiveMode = "rule"
	}

	if b.SubUA == "" {
		b.SubUA = "clash-verge"
	}

	if b.UpdateMethod == "" {
		b.UpdateMethod = "startup"
	}

	if b.UpdateInterval <= 0 {
		b.UpdateInterval = 1
	}

	if b.AutoDelayTest && b.AutoDelayTestInterval <= 0 {
		b.AutoDelayTestInterval = 60
	}

	if b.GeoIpLink == "" {
		b.GeoIpLink = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb"
	}
	if b.GeoSiteLink == "" {
		b.GeoSiteLink = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat"
	}
	if b.MmdbLink == "" {
		b.MmdbLink = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb"
	}
	if b.AsnLink == "" {
		b.AsnLink = "https://github.com/xishang0128/geoip/releases/download/latest/GeoLite2-ASN.mmdb"
	}

	if b.LongConnectionMinSeconds <= 0 {
		b.LongConnectionMinSeconds = 60
	}

	return b
}

func (s *BehaviorStore) Default() AppBehavior {
	return AppBehavior{
		SilentStart:        false,
		CloseToTray:        true,
		ColorDelay:         false,
		DelayRetention:     true,
		DelayRetentionTime: "long",
		LogLevel:           "error",
		AppLogLevel:        "info",
		HideLogs:           false,
		SubUA:              "clash-verge",
		ActiveMode:         "rule",

		AutoUpdate:     true,
		UpdateMethod:   "startup",
		UpdateInterval: 1,

		AutoDelayTest:         false,
		AutoDelayTestInterval: 60,

		ProxyTrafficOnly: false,

		GeoIpLink:   "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb",
		GeoSiteLink: "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat",
		MmdbLink:    "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb",
		AsnLink:     "https://github.com/xishang0128/geoip/releases/download/latest/GeoLite2-ASN.mmdb",

		StartupWithOS:    false,
		RestoreOnStartup: false,

		LongConnectionProtection: true,
		DeferRestartWhenActive:   true,
		LongConnectionMinSeconds: 60,
	}
}
