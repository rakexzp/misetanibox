package runtimeassets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInterruptedStockRecoveryAndSmartIsolation(t *testing.T) {
	for _, phase := range []string{"intent", "renamed", "installed", "committed"} {
		t.Run(phase, func(t *testing.T) {
			d := t.TempDir()
			target := filepath.Join(d, "clash.stock.exe")
			active := filepath.Join(d, "clash.exe")
			model := filepath.Join(d, "Model.bin")
			seed := filepath.Join(d, "seed")
			put := func(p, s string) {
				t.Helper()
				if e := os.WriteFile(p, []byte(s), 0755); e != nil {
					t.Fatal(e)
				}
			}
			digest := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
			put(active, "smart")
			put(model, "model")
			put(seed, "v1.19.31")
			state := &AssetState{CopiedAssets: make(map[string]struct {
				SHA256 string `json:"sha256"`
				Source string `json:"source"`
			})}
			hash := digest("v1.19.27")
			if phase == "committed" {
				hash = digest("v1.19.31")
			}
			state.CopiedAssets["clash.stock.exe"] = struct {
				SHA256 string `json:"sha256"`
				Source string `json:"source"`
			}{hash, "seed"}
			if phase == "intent" {
				put(target, "v1.19.27")
			} else {
				put(target+".seed-backup", "v1.19.27")
			}
			if phase == "installed" || phase == "committed" {
				put(target, "v1.19.31")
			}
			j, _ := json.Marshal(stockJournal{OldHash: digest("v1.19.27"), NewHash: digest("v1.19.31"), HadOld: true})
			put(target+".seed-transaction.json", string(j))
			opts := StockSyncOptions{SeedPath: seed, TargetPath: target, State: state, Meta: SeedAssetMeta{Version: "v1.19.31", SHA256: digest("v1.19.31"), Size: 8}, Probe: func(_ context.Context, p string) (string, error) { b, e := os.ReadFile(p); return string(b), e }, Validate: func(string) error { return nil }, SaveState: func(*AssetState) error { return nil }}
			result, e := ReconcileStock(context.Background(), opts)
			if e != nil {
				t.Fatal(e)
			}
			if phase != "committed" && !result.Updated {
				t.Fatalf("%+v", result)
			}
			for p, want := range map[string]string{target: "v1.19.31", active: "smart", model: "model"} {
				b, e := os.ReadFile(p)
				if e != nil || string(b) != want {
					t.Fatalf("%s: %q %v", p, b, e)
				}
			}
			if _, e := os.Stat(target + ".seed-transaction.json"); !os.IsNotExist(e) {
				t.Fatal("journal remains", e)
			}
		})
	}
}
