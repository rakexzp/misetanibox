package mobilecore

import (
	"fmt"
	"net/netip"
	"syscall"

	"github.com/metacubex/mihomo/component/dialer"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/hub"
	"github.com/metacubex/mihomo/hub/executor"
	"github.com/metacubex/mihomo/listener"
	LC "github.com/metacubex/mihomo/listener/config"
)

// SocketProtector is implemented in Kotlin (VpnService.protect).
type SocketProtector interface {
	Protect(fd int) bool
}

// SetProtect marks every outbound core socket to bypass the VPN (no routing loop).
func SetProtect(p SocketProtector) {
	if p == nil {
		dialer.DefaultSocketHook = nil
		return
	}
	dialer.DefaultSocketHook = func(network, address string, conn syscall.RawConn) error {
		return conn.Control(func(fd uintptr) {
			ok := p.Protect(int(fd))
			if !ok {
				// Best-effort: still continue; some FDs are not protectable.
			}
		})
	}
}

// Must match MihomoVpnService.Builder address.
var androidTunAddr = netip.MustParsePrefix("172.19.0.1/30")

// vpnSessionActive is set while Start() owns a real TUN session.
var vpnSessionActive atomicBool

// Start applies YAML and attaches TUN to the Android VpnService fd.
// Returns "" on success, otherwise an error message.
func Start(homeDir, configYAML string, fd int) string {
	if fd <= 0 {
		return "invalid tun fd"
	}
	// End any offline ping session first.
	if testCoreActive.Load() {
		executor.Shutdown()
		testCoreActive.Store(false)
	}

	C.SetHomeDir(homeDir)

	cfg, err := executor.ParseWithBytes([]byte(configYAML))
	if err != nil {
		return "parse config: " + err.Error()
	}

	// On Android + VpnService FD:
	// - mixed/system stack: TCP often broken (user-space only gets UDP)
	// - gvisor: both TCP and UDP in userspace — required for real internet
	// Binary MUST be built with: -tags with_gvisor,cmfa
	if !gvisorIncluded {
		return "this build has no gVisor; rebuild with -tags with_gvisor,cmfa"
	}

	cfg.General.Tun = LC.Tun{
		Enable:              true,
		Stack:               C.TunGvisor,
		DNSHijack:           []string{"any:53", "tcp://any:53"},
		AutoRoute:           false,
		AutoDetectInterface: false,
		MTU:                 1500,
		Inet4Address:        []netip.Prefix{androidTunAddr},
		FileDescriptor:      fd,
		StrictRoute:         false,
		// EndpointIndependentNat helps some UDP apps under gvisor
		EndpointIndependentNat: true,
	}

	if cfg.DNS != nil {
		cfg.DNS.Enable = true
		cfg.DNS.IPv6 = false
	}

	// Prefer IPv4 only — matches VpnService without IPv6 routes.
	cfg.General.IPv6 = false

	hub.ApplyConfig(cfg)

	tunConf := listener.GetTunConf()
	if !tunConf.Enable {
		return fmt.Sprintf(
			"TUN failed (fd=%d stack=%s gvisor=%v)",
			fd, C.TunGvisor.String(), gvisorIncluded,
		)
	}
	vpnSessionActive.set(true)
	return ""
}

// Stop shuts the core down (VPN or test).
func Stop() {
	vpnSessionActive.set(false)
	testCoreActive.Store(false)
	executor.Shutdown()
}

// IsVpnSession is true while Start() TUN session is active.
func IsVpnSession() bool {
	return vpnSessionActive.get()
}

// TunStatus is a debug snapshot for the UI/plugin.
func TunStatus() string {
	t := listener.GetTunConf()
	return fmt.Sprintf("enable=%v fd=%d stack=%s addr=%v gvisor=%v",
		t.Enable, t.FileDescriptor, t.Stack.String(), t.Inet4Address, gvisorIncluded)
}
