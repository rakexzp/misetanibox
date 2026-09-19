package appcore

import (
	"context"
	"errors"
	"sync"
	"testing"

	"goclashz/core/sys"
)

type proxyTestSink struct{}

func (proxyTestSink) Emit(string, ...any) {}

func TestNativeProxyCleanupNeverTouchesForeignSettings(t *testing.T) {
	// Linux tests must never reach the platform implementation, even for reads.
	c := NewController(Options{Events: proxyTestSink{}})
	c.systemProxy.read = func() (sys.SystemProxyState, error) {
		t.Error("foreign proxy read")
		return sys.SystemProxyState{Enabled: true, Server: "foreign:7890"}, nil
	}
	c.systemProxy.enable = func(string, int, string) error { t.Error("foreign proxy overwritten"); return nil }
	c.systemProxy.disable = func() error { t.Error("foreign proxy disabled"); return nil }
	ctx, cancel := context.WithCancel(context.Background())
	c.Bootstrap(ctx, BootstrapOptions{NativeLite: true})
	defer c.Supervisor.Stop()
	defer cancel()
	s := NewLiteService(c)
	if err := c.EnsureCoreRunning(ctx); !errors.Is(err, ErrNativeProxyOwnershipUnavailable) {
		t.Fatalf("direct core activation: %v", err)
	}
	if err := c.StartCoreOnly(ctx, ""); !errors.Is(err, ErrNativeProxyOwnershipUnavailable) {
		t.Fatalf("delay core activation: %v", err)
	}
	if err := s.Connect(ctx); !errors.Is(err, ErrNativeProxyOwnershipUnavailable) {
		t.Fatalf("connect: %v", err)
	}
	if _, err := s.Ping(ctx, "", ""); !errors.Is(err, ErrNativeProxyOwnershipUnavailable) {
		t.Fatalf("ping: %v", err)
	}
	if err := c.ensureSystemProxyEnabled(); !errors.Is(err, ErrNativeProxyOwnershipUnavailable) {
		t.Fatalf("enable: %v", err)
	}
	if err := c.ensureSystemProxyDisabled(); err != nil {
		t.Fatal(err)
	}
	if err := c.Supervisor.Reconcile(ctx, "idle"); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	d := c.Desired.Get()
	d.CoreRunning, d.SystemProxy = true, true
	if err := c.Desired.SetAndSave(d); err != nil {
		t.Fatal(err)
	}
	if err := c.Supervisor.Reconcile(ctx, "bypass-connect"); !errors.Is(err, ErrNativeProxyOwnershipUnavailable) {
		t.Fatalf("reconcile: %v", err)
	}
	cancel()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.Supervisor.ShutdownRuntime("cancel-cleanup") }()
	}
	wg.Wait()
	if err := s.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyProxyBoundaryStillDelegates(t *testing.T) {
	p := newSystemProxyRuntime()
	reads, enables, disables := 0, 0, 0
	p.read = func() (sys.SystemProxyState, error) {
		reads++
		return sys.SystemProxyState{Enabled: true, Server: "foreign:1234"}, nil
	}
	p.enable = func(host string, port int, bypass string) error {
		enables++
		if host != "127.0.0.1" || port != 1234 || bypass != "local" {
			t.Fatal("changed arguments")
		}
		return nil
	}
	p.disable = func() error { disables++; return nil }
	if state, err := p.Get(); err != nil || !state.Enabled {
		t.Fatalf("read: %v %v", state, err)
	}
	if err := p.Enable("127.0.0.1", 1234, "local"); err != nil {
		t.Fatal(err)
	}
	if err := p.Disable(); err != nil {
		t.Fatal(err)
	}
	if reads != 1 || enables != 1 || disables != 1 {
		t.Fatalf("calls: %d/%d/%d", reads, enables, disables)
	}
}
