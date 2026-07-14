package mobilecore

import (
	"sync/atomic"

	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/hub"
	"github.com/metacubex/mihomo/hub/executor"
	LC "github.com/metacubex/mihomo/listener/config"
)

// testCoreActive is true while StartTest holds the controller without TUN.
// Used so StopTest does not tear down a real VPN session started via Start().
var testCoreActive atomic.Bool

// StartTest loads proxies + external-controller only (no Android TUN).
// Used for delay/ping checks before the user enables VPN.
// Returns "" on success.
func StartTest(homeDir, configYAML string) string {
	if homeDir == "" {
		return "home empty"
	}
	// Never replace a live VPN tunnel.
	if IsVpnSession() {
		return "vpn already running"
	}

	// Clean previous test session if any.
	if testCoreActive.Load() {
		executor.Shutdown()
		testCoreActive.Store(false)
	}

	C.SetHomeDir(homeDir)

	cfg, err := executor.ParseWithBytes([]byte(configYAML))
	if err != nil {
		return "parse config: " + err.Error()
	}

	// Force no TUN — only API + proxy dial for /delay.
	cfg.General.Tun = LC.Tun{Enable: false}
	cfg.General.IPv6 = false
	// fake-ip without TUN → every /delay times out (dest becomes 198.18.x.x).
	if cfg.DNS != nil {
		cfg.DNS.Enable = true
		cfg.DNS.IPv6 = false
		// "redir-host" in YAML == DNSMapping; never fake-ip offline
		cfg.DNS.EnhancedMode = C.DNSMapping
		if len(cfg.DNS.NameServer) == 0 {
			// leave as configured in YAML
		}
	}

	hub.ApplyConfig(cfg)
	testCoreActive.Store(true)
	return ""
}

// StopTest shuts down a StartTest session. No-op if a real VPN is active.
func StopTest() {
	if IsVpnSession() {
		return
	}
	if testCoreActive.CompareAndSwap(true, false) {
		executor.Shutdown()
	}
}

// IsTestCore reports whether StartTest is currently active.
func IsTestCore() bool {
	return testCoreActive.Load() && !IsVpnSession()
}
