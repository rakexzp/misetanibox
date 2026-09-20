package clash

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestTunProbeControlsAndIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process fixture; real Windows core needs Windows")
	}
	path := filepath.Join(t.TempDir(), "core")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	// The fixture consumes exactly the same isolated base64 config as the executable.
	write("case \"$3\" in * ) ;; esac\nprintf '%s' \"$3\" | base64 -d | grep -q invalid && { echo 'invalid tun stack'; exit 1; }; exit 0\n")
	if got := probeTunCapabilities(context.Background(), path, time.Second); got.Status != "supported" {
		t.Fatalf("supported: %+v", got)
	}
	write("printf '%s' \"$3\" | base64 -d | grep -Eq 'mips|invalid' && { echo 'invalid tun stack'; exit 1; }; exit 0\n")
	if got := probeTunCapabilities(context.Background(), path, time.Second); got.Status != "unsupported" {
		t.Fatalf("changed exe not invalidated: %+v", got)
	}
	write("exit 0\n")
	if got := probeTunCapabilities(context.Background(), path, time.Second); got.Status != "unknown" {
		t.Fatalf("invalid control accepted: %+v", got)
	}
	write("exec sleep 2\n")
	start := time.Now()
	if got := probeTunCapabilities(context.Background(), path, 30*time.Millisecond); got.Status != "unknown" {
		t.Fatalf("timeout: %+v", got)
	}
	if time.Since(start) > time.Second {
		t.Fatal("timeout not bounded")
	}
	if got := probeTunCapabilities(context.Background(), path+"missing", time.Second); got.Status != "unknown" {
		t.Fatalf("missing: %+v", got)
	}
}

func TestTunProbeOfficialCore(t *testing.T) {
	path := os.Getenv("MISETANIBOX_TEST_CORE")
	if path == "" {
		t.Skip("set MISETANIBOX_TEST_CORE to a verified isolated core binary")
	}
	if got := probeTunCapabilities(context.Background(), path, 3*time.Second); got.Status != "supported" {
		t.Fatalf("official core: %+v", got)
	}
}

func TestTunValidationPreservesDefaults(t *testing.T) {
	if GetDefaultTunConfig().Stack != "gvisor" {
		t.Fatal("default changed")
	}
	if ValidateTunConfig(nil) == nil {
		t.Fatal("nil accepted")
	}
	for _, stack := range []string{"gvisor", "mixed", "system", "invalid", ""} {
		cfg := GetDefaultTunConfig()
		cfg.Stack = stack
		err := ValidateTunConfig(&cfg)
		if (err != nil) != (stack == "invalid" || stack == "") {
			t.Fatalf("%q: %v", stack, err)
		}
	}
}

func TestTunRuntimeFallbackPreservesSelection(t *testing.T) {
	cfg := GetDefaultTunConfig()
	cfg.Stack = "mips"
	effective, warning := resolveTunRuntime(cfg, TunCapabilities{Status: "unsupported", Reason: "old Smart"})
	if effective.Stack != "gvisor" || cfg.Stack != "mips" || warning == "" {
		t.Fatalf("unsafe fallback: %+v %s", effective, warning)
	}
	effective, warning = resolveTunRuntime(cfg, TunCapabilities{Status: "supported"})
	if effective.Stack != "mips" || warning != "" {
		t.Fatalf("supported changed: %+v %s", effective, warning)
	}
	for _, stack := range []string{"gvisor", "system", "mixed"} {
		cfg.Stack = stack
		effective, warning = resolveTunRuntime(cfg, TunCapabilities{Status: "unknown"})
		if effective.Stack != stack || warning != "" {
			t.Fatalf("legacy changed: %+v", effective)
		}
	}
}
