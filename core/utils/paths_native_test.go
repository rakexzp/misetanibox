package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeDataDirIgnoresLegacyOverrides(t *testing.T) {
	before := os.Args
	t.Cleanup(func() { os.Args = before })
	os.Args = []string{"backend", "--data-dir", t.TempDir(), "--native-lite"}
	t.Setenv("GOCLASHZ_DATA_DIR", t.TempDir())
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if got := resolveStableDataDir(t.TempDir()); got != filepath.Join(cache, "Misetanibox.Lite") {
		t.Fatalf("native directory: %q", got)
	}
	if got := resolveLegacyAppDataDir(); got != "" {
		t.Fatalf("native exposes legacy migration source: %q", got)
	}
}

func TestLegacyDataDirOverrideUnchanged(t *testing.T) {
	before := os.Args
	t.Cleanup(func() { os.Args = before })
	dir := t.TempDir()
	os.Args = []string{"desktop", "--data-dir", dir}
	t.Setenv("GOCLASHZ_DATA_DIR", t.TempDir())
	if got := resolveStableDataDir(t.TempDir()); got != dir {
		t.Fatalf("legacy override: %q", got)
	}
	if got := resolveLegacyAppDataDir(); got == "" {
		t.Fatal("legacy migration source removed for desktop")
	}
}
