package appcore

import (
	"errors"
	"goclashz/core/sys"
	"sync/atomic"
)

// Native connect stays fail-closed until a Windows implementation can snapshot
// and conditionally restore all affected WinINet/RAS settings. The legacy guard
// only remembers an endpoint and disables it; it cannot preserve prior state.
var ErrNativeProxyOwnershipUnavailable = errors.New("system_proxy_ownership_not_ready")

type systemProxyRuntime struct {
	native  atomic.Bool
	read    func() (sys.SystemProxyState, error)
	enable  func(string, int, string) error
	disable func() error
}

func newSystemProxyRuntime() *systemProxyRuntime {
	return &systemProxyRuntime{read: sys.GetSystemProxyState, enable: sys.EnableSystemProxy, disable: sys.DisableSystemProxy}
}

func (p *systemProxyRuntime) Get() (sys.SystemProxyState, error) {
	if p.native.Load() {
		return sys.SystemProxyState{}, ErrNativeProxyOwnershipUnavailable
	}
	return p.read()
}

func (p *systemProxyRuntime) Enable(host string, port int, bypass string) error {
	if p.native.Load() {
		return ErrNativeProxyOwnershipUnavailable
	}
	return p.enable(host, port, bypass)
}

func (p *systemProxyRuntime) Disable() error {
	if p.native.Load() {
		// This runtime never acquires ownership, so cleanup must be a no-op.
		return nil
	}
	return p.disable()
}
