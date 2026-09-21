package runtimeassets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyStateAndExplicitStockRepair(t *testing.T) {
	for _, tc := range []struct {
		name, old, source    string
		force, badHash, want bool
	}{
		{"legacy seed", "v1.19.27", "seed", false, false, true},
		{"downloaded remains", "v1.19.27", "download", false, false, false},
		{"custom remains", "custom", "", false, false, false},
		{"explicit custom repair", "custom", "", true, false, true},
		{"explicit equal repair", "v1.19.31", "seed", true, false, true},
		{"explicit still validates hash", "custom", "", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			target, seed := filepath.Join(d, "clash.exe"), filepath.Join(d, "seed")
			for p, s := range map[string]string{target: tc.old, seed: "v1.19.31"} {
				if e := os.WriteFile(p, []byte(s), 0755); e != nil {
					t.Fatal(e)
				}
			}
			digest := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
			// Exact JSON shape emitted by the original RepairFromSeed implementation.
			var state AssetState
			if e := json.Unmarshal([]byte(fmt.Sprintf(`{"seedManifestSha256":"old-manifest","copiedAssets":{"clash.exe":{"sha256":%q,"source":%q}}}`, digest(tc.old), tc.source)), &state); e != nil {
				t.Fatal(e)
			}
			meta := SeedAssetMeta{Version: "v1.19.31", Size: 8, SHA256: digest("v1.19.31")}
			if tc.badHash {
				meta.SHA256 = digest("bad")
			}
			result, e := ReconcileStock(context.Background(), StockSyncOptions{SeedPath: seed, TargetPath: target, Meta: meta, State: &state, ExplicitRepair: tc.force, Probe: func(_ context.Context, p string) (string, error) { b, e := os.ReadFile(p); return string(b), e }, Validate: func(string) error { return nil }, SaveState: func(*AssetState) error { return nil }})
			if tc.badHash && e == nil {
				t.Fatal("bad hash accepted")
			}
			if !tc.badHash && e != nil {
				t.Fatal(e)
			}
			if result.Updated != tc.want {
				t.Fatalf("%+v", result)
			}
			b, e := os.ReadFile(target)
			want := tc.old
			if tc.want {
				want = "v1.19.31"
			}
			if e != nil || string(b) != want {
				t.Fatalf("runtime %q %v", b, e)
			}
		})
	}
}
