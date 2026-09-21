package runtimeassets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileStockTransaction(t *testing.T) {
	for _, scenario := range []string{"old", "equal", "new", "unknown", "unowned", "hash", "size", "version", "state", "locked", "installed"} {
		t.Run(scenario, func(t *testing.T) {
			d := t.TempDir()
			seed := filepath.Join(d, "seed")
			target := filepath.Join(d, "clash.exe")
			old := "v1.19.27"
			if scenario == "equal" {
				old = "v1.19.31"
			}
			if scenario == "new" {
				old = "v1.19.32"
			}
			if scenario == "unknown" {
				old = "alpha"
			}
			os.WriteFile(seed, []byte("v1.19.31"), 0755)
			os.WriteFile(target, []byte(old), 0755)
			digest := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
			state := &AssetState{CopiedAssets: make(map[string]struct {
				SHA256 string `json:"sha256"`
				Source string `json:"source"`
			})}
			state.CopiedAssets["clash.exe"] = struct {
				SHA256 string `json:"sha256"`
				Source string `json:"source"`
			}{digest(old), "seed"}
			if scenario == "unowned" {
				delete(state.CopiedAssets, "clash.exe")
			}
			meta := SeedAssetMeta{Version: "v1.19.31", Size: 8, SHA256: digest("v1.19.31")}
			if scenario == "hash" {
				meta.SHA256 = digest("bad")
			}
			if scenario == "size" {
				meta.Size++
			}
			if scenario == "version" {
				meta.Version = "v1.19.32"
			}
			saves := 0
			opts := StockSyncOptions{SeedPath: seed, TargetPath: target, Meta: meta, State: state, Probe: func(_ context.Context, p string) (string, error) {
				b, e := os.ReadFile(p)
				if scenario == "installed" && p == target && string(b) == "v1.19.31" {
					return "", errors.New("probe failed")
				}
				return string(b), e
			}, Validate: func(string) error { return nil }, SaveState: func(*AssetState) error {
				saves++
				if scenario == "state" {
					return errors.New("state failed")
				}
				return nil
			}}
			if scenario == "locked" {
				opts.Rename = func(a, b string) error {
					if a == target {
						return errors.New("locked")
					}
					return os.Rename(a, b)
				}
			}
			result, err := ReconcileStock(context.Background(), opts)
			wantChange := scenario == "old"
			if (scenario == "hash" || scenario == "size" || scenario == "version" || scenario == "state" || scenario == "locked" || scenario == "installed") && err == nil {
				t.Fatal("expected failure")
			}
			if result.Updated != wantChange {
				t.Fatalf("result %+v err %v", result, err)
			}
			b, _ := os.ReadFile(target)
			want := old
			if wantChange {
				want = "v1.19.31"
			}
			if string(b) != want {
				t.Fatalf("runtime lost: %q want %q", b, want)
			}
			if !wantChange && scenario != "unowned" && state.CopiedAssets["clash.exe"].SHA256 != digest(old) {
				t.Fatal("ownership changed on failure")
			}
			if wantChange {
				if saves != 1 {
					t.Fatal(saves)
				}
				again, e := ReconcileStock(context.Background(), opts)
				if e != nil || again.Updated {
					t.Fatalf("not idempotent %+v %v", again, e)
				}
			}
		})
	}
}
