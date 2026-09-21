package runtimeassets

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"goclashz/core/utils"
)

type StockSyncResult struct {
	Updated bool
	Message string
}

// StockSyncOptions separates filesystem fixtures from the installed runtime.
// Callers must serialize core replacements and supply a trusted seed manifest.
type StockSyncOptions struct {
	SeedPath, TargetPath string
	Meta                 SeedAssetMeta
	State                *AssetState
	Probe                func(context.Context, string) (string, error)
	Validate             func(string) error
	SaveState            func(*AssetState) error
	Rename               func(string, string) error
	// ExplicitRepair is only for a user-requested stock restoration. It permits
	// replacing custom/corrupt/newer stock, never bypassing seed verification.
	ExplicitRepair bool
}

type stockJournal struct {
	OldHash string `json:"oldHash"`
	NewHash string `json:"newHash"`
	HadOld  bool   `json:"hadOld"`
}

var stableStockVersion = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func compareStockVersion(a, b string) (int, error) {
	aa, bb := stableStockVersion.FindStringSubmatch(a), stableStockVersion.FindStringSubmatch(b)
	if aa == nil || bb == nil {
		return 0, fmt.Errorf("неизвестная или нестабильная версия ядра")
	}
	for i := 1; i <= 3; i++ {
		x, e := strconv.ParseUint(aa[i], 10, 64)
		if e != nil {
			return 0, e
		}
		y, e := strconv.ParseUint(bb[i], 10, 64)
		if e != nil {
			return 0, e
		}
		if x < y {
			return -1, nil
		}
		if x > y {
			return 1, nil
		}
	}
	return 0, nil
}

func verifyStockFile(path string, meta SeedAssetMeta, validate func(string) error) error {
	h, e := hex.DecodeString(meta.SHA256)
	if e != nil || len(h) != 32 || meta.Size <= 0 {
		return fmt.Errorf("неполный seed manifest")
	}
	st, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !st.Mode().IsRegular() || st.Size() != meta.Size {
		return fmt.Errorf("неверный размер или тип seed: %s", path)
	}
	hash, e := calculateSHA256(path)
	if e != nil {
		return e
	}
	if !strings.EqualFold(hash, meta.SHA256) {
		return fmt.Errorf("SHA256 seed не совпадает: %s", path)
	}
	return validate(path)
}

// ReconcileStock never treats a missing state entry or a version string as proof
// of ownership. A failed transaction retains both its journal and old ownership.
func ReconcileStock(ctx context.Context, o StockSyncOptions) (StockSyncResult, error) {
	result := StockSyncResult{}
	if o.Probe == nil {
		o.Probe = ProbeCoreVersion
	}
	if o.Validate == nil {
		o.Validate = validateCoreBinary
	}
	if o.Rename == nil {
		o.Rename = os.Rename
	}
	if o.State == nil || o.SaveState == nil {
		return result, fmt.Errorf("asset-state недоступен")
	}
	name := filepath.Base(o.TargetPath)
	journal, backup := o.TargetPath+".seed-transaction.json", o.TargetPath+".seed-backup"
	recoverOld := func(j stockJournal) error {
		current, e := calculateSHA256(o.TargetPath)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if j.HadOld {
			if current == j.OldHash {
				return nil
			}
			if e == nil && current != j.NewHash {
				return fmt.Errorf("runtime изменён вне транзакции; восстановление отложено")
			}
			h, e := calculateSHA256(backup)
			if e != nil || h != j.OldHash {
				return fmt.Errorf("резервная копия транзакции недоступна")
			}
			if current != "" {
				if e := os.Remove(o.TargetPath); e != nil {
					return e
				}
			}
			return o.Rename(backup, o.TargetPath)
		}
		if os.IsNotExist(e) {
			return nil
		}
		if current != j.NewHash {
			return fmt.Errorf("неизвестный runtime после прерывания")
		}
		return os.Remove(o.TargetPath)
	}
	if data, e := os.ReadFile(journal); e == nil {
		var j stockJournal
		if e = json.Unmarshal(data, &j); e != nil {
			return result, fmt.Errorf("повреждён журнал: %w", e)
		}
		h, _ := calculateSHA256(o.TargetPath)
		if h != j.NewHash || o.State.CopiedAssets[name].SHA256 != j.NewHash {
			if e := recoverOld(j); e != nil {
				return result, fmt.Errorf("восстановление отложено: %w", e)
			}
		}
		if e := os.Remove(backup); e != nil && !os.IsNotExist(e) {
			return result, e
		}
		if e := os.Remove(journal); e != nil {
			return result, e
		}
	} else if !os.IsNotExist(e) {
		return result, e
	}
	if _, e := os.Lstat(backup); e == nil {
		return result, fmt.Errorf("найдена резервная копия без журнала; нужна ручная проверка")
	} else if !os.IsNotExist(e) {
		return result, e
	}
	if err := verifyStockFile(o.SeedPath, o.Meta, o.Validate); err != nil {
		return result, err
	}
	seedVersion, err := o.Probe(ctx, o.SeedPath)
	if err != nil {
		return result, err
	}
	if cmp, e := compareStockVersion(seedVersion, o.Meta.Version); e != nil || cmp != 0 {
		return result, fmt.Errorf("версия seed не совпадает с manifest")
	}
	oldHash := ""
	hadOld := false
	if st, e := os.Lstat(o.TargetPath); e == nil {
		hadOld = true
		if !st.Mode().IsRegular() {
			return result, fmt.Errorf("runtime не является обычным файлом")
		}
		oldHash, err = calculateSHA256(o.TargetPath)
		if err != nil {
			return result, err
		}
		if !o.ExplicitRepair {
			ownership, ok := o.State.CopiedAssets[name]
			if !ok || ownership.Source != "seed" || ownership.SHA256 != oldHash {
				result.Message = "Неизвестное или пользовательское ядро сохранено"
				return result, nil
			}
			version, e := o.Probe(ctx, o.TargetPath)
			if e != nil {
				result.Message = "Версия ядра неизвестна; замена пропущена"
				return result, nil
			}
			cmp, e := compareStockVersion(version, seedVersion)
			if e != nil {
				result.Message = "Нестабильное ядро сохранено"
				return result, nil
			}
			if cmp >= 0 {
				result.Message = "Ядро не старее встроенного"
				return result, nil
			}
		}
	} else if !os.IsNotExist(e) {
		return result, e
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := os.MkdirAll(filepath.Dir(o.TargetPath), 0755); err != nil {
		return result, err
	}
	staged, err := os.CreateTemp(filepath.Dir(o.TargetPath), ".seed-stage-*.exe")
	if err != nil {
		return result, err
	}
	stage := staged.Name()
	if err = staged.Close(); err != nil {
		return result, err
	}
	defer os.Remove(stage)
	if err = utils.CopyFile(o.SeedPath, stage); err != nil {
		return result, err
	}
	if err = os.Chmod(stage, 0755); err != nil {
		return result, err
	}
	if err = verifyStockFile(stage, o.Meta, o.Validate); err != nil {
		return result, err
	}
	if version, e := o.Probe(ctx, stage); e != nil || version != seedVersion {
		return result, fmt.Errorf("проверка staged ядра не пройдена")
	}
	if hadOld {
		if hash, err := calculateSHA256(o.TargetPath); err != nil || hash != oldHash {
			return result, fmt.Errorf("runtime изменился во время подготовки; замена отменена")
		}
	} else if _, err := os.Lstat(o.TargetPath); !os.IsNotExist(err) {
		return result, fmt.Errorf("runtime появился во время подготовки; замена отменена")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	// Persist intent before touching the runtime, so a crash can be rolled back.
	j := stockJournal{OldHash: oldHash, NewHash: strings.ToLower(o.Meta.SHA256), HadOld: hadOld}
	data, _ := json.Marshal(j)
	if err = utils.WriteFileAtomic(journal, data, 0600); err != nil {
		return result, err
	}
	rollback := func(cause error) (StockSyncResult, error) {
		if e := recoverOld(j); e != nil {
			return result, fmt.Errorf("замена отложена: %v; rollback: %w", cause, e)
		}
		if e := os.Remove(journal); e != nil {
			return result, fmt.Errorf("%v; журнал: %w", cause, e)
		}
		return result, fmt.Errorf("замена отложена, прежнее ядро сохранено: %w", cause)
	}
	if hadOld {
		if err = o.Rename(o.TargetPath, backup); err != nil {
			return rollback(err)
		}
	}
	if err = o.Rename(stage, o.TargetPath); err != nil {
		return rollback(err)
	}
	if err = verifyStockFile(o.TargetPath, o.Meta, o.Validate); err != nil {
		return rollback(err)
	}
	if version, e := o.Probe(ctx, o.TargetPath); e != nil || version != seedVersion {
		return rollback(fmt.Errorf("проверка установленного ядра не пройдена"))
	}
	copyData, _ := json.Marshal(o.State)
	var next AssetState
	if err = json.Unmarshal(copyData, &next); err != nil {
		return rollback(err)
	}
	if next.CopiedAssets == nil {
		next.CopiedAssets = make(map[string]struct {
			SHA256 string `json:"sha256"`
			Source string `json:"source"`
		})
	}
	next.CopiedAssets[name] = struct {
		SHA256 string `json:"sha256"`
		Source string `json:"source"`
	}{j.NewHash, "seed"}
	if err = o.SaveState(&next); err != nil {
		return rollback(err)
	}
	*o.State = next
	result.Updated = true
	result.Message = "Встроенное stock-ядро обновлено до " + seedVersion
	// Keep a committed journal if cleanup fails; recovery checks committed ownership.
	if err = os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return result, err
	}
	if err = os.Remove(journal); err != nil {
		return result, err
	}
	return result, nil
}

func ReconcileBundledStock(ctx context.Context, target string, validate func(string) error) (StockSyncResult, error) {
	return reconcileBundledStock(ctx, target, validate, false)
}

// RepairBundledStock is the explicit user action, not an automatic repair policy.
func RepairBundledStock(ctx context.Context, target string, validate func(string) error) (StockSyncResult, error) {
	return reconcileBundledStock(ctx, target, validate, true)
}

func reconcileBundledStock(ctx context.Context, target string, validate func(string) error, explicit bool) (StockSyncResult, error) {
	catalog := LoadSeedCatalog()
	meta, ok := catalog.Items[coreBinName]
	if !catalog.ManifestOK || !ok {
		return StockSyncResult{}, fmt.Errorf("manifest встроенного ядра недоступен")
	}
	state := LoadAssetState()
	// Smart installation copies the former active stock without changing its
	// ownership record. Recognize that exact hash, never a matching version alone.
	if filepath.Base(target) == "clash.stock.exe" {
		if _, exists := state.CopiedAssets["clash.stock.exe"]; !exists {
			if previous, ok := state.CopiedAssets[coreBinName]; ok && previous.Source == "seed" {
				if hash, err := calculateSHA256(target); err == nil && hash == previous.SHA256 {
					state.CopiedAssets["clash.stock.exe"] = previous
				}
			}
		}
	}
	return ReconcileStock(ctx, StockSyncOptions{SeedPath: filepath.Join(utils.GetSeedCoreBinDir(), coreBinName), TargetPath: target, Meta: meta, State: state, Validate: validate, SaveState: saveAssetState, ExplicitRepair: explicit})
}
