//go:build windows

package runtimeassets

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Real PE processes exercise Windows execution, cache identity and replacement;
// all files and ownership state belong to this test, never the installed proxy.
func TestWindowsVersionProbeAndStockTransition(t *testing.T) {
	dir := t.TempDir()
	build := func(name, version, fail string) string {
		t.Helper()
		path := filepath.Join(dir, name+".exe")
		cmd := exec.Command("go", "build", "-ldflags", "-X main.version="+version+" -X main.fail="+fail, "-o", path, "./testdata/versionprobe")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build fixture: %v: %s", err, out)
		}
		return path
	}
	target := build("clash", "v1.19.27", "false")
	seed := build("seed", "v1.19.31", "false")
	ctx := context.Background()
	for path, want := range map[string]string{target: "v1.19.27", seed: "v1.19.31"} {
		if got, err := ProbeCoreVersion(ctx, path); err != nil || got != want {
			t.Fatalf("probe %s: %q %v", path, got, err)
		}
	}
	oldHash, err := calculateSHA256(target)
	if err != nil {
		t.Fatal(err)
	}
	newHash, err := calculateSHA256(seed)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(seed)
	if err != nil {
		t.Fatal(err)
	}
	state := &AssetState{CopiedAssets: make(map[string]struct {
		SHA256 string `json:"sha256"`
		Source string `json:"source"`
	})}
	state.CopiedAssets["clash.exe"] = struct {
		SHA256 string `json:"sha256"`
		Source string `json:"source"`
	}{oldHash, "seed"}
	saved := false
	result, err := ReconcileStock(ctx, StockSyncOptions{
		SeedPath: seed, TargetPath: target, State: state,
		Meta: SeedAssetMeta{Version: "v1.19.31", SHA256: newHash, Size: st.Size()},
		// Tiny fixtures are smaller than production's 5 MB lower bound.
		Validate:  func(string) error { return nil },
		SaveState: func(*AssetState) error { saved = true; return nil },
	})
	if err != nil || !result.Updated || !saved {
		t.Fatalf("transition: %+v %v saved=%v", result, err, saved)
	}
	if got, err := ProbeCoreVersion(ctx, target); err != nil || got != "v1.19.31" {
		t.Fatalf("cached replacement: %q %v", got, err)
	}
	bad := build("failed", "v1.19.33", "true")
	if _, err := ProbeCoreVersion(ctx, bad); err == nil {
		t.Fatal("nonzero exit accepted")
	}
	unknown := build("unknown", "unknown", "false")
	if _, err := ProbeCoreVersion(ctx, unknown); err == nil {
		t.Fatal("unknown version accepted")
	}
}
