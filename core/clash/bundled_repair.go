package clash

import (
	"context"
	"fmt"
	"runtime"

	"goclashz/core/runtimeassets"
	"goclashz/core/utils"
)

// RepairRuntimeAssets preserves the Wails status contract while routing explicit
// Windows stock restoration through the same serialized transaction as startup.
func RepairRuntimeAssets(ctx context.Context) (runtimeassets.RuntimeAssetStatus, error) {
	if runtime.GOOS != "windows" {
		return runtimeassets.EnsureReady(ctx, runtimeassets.RequireAll, runtimeassets.RepairForce)
	}
	coreBinaryMu.Lock()
	defer coreBinaryMu.Unlock()
	runtimeassets.EnableBundledCoreMaintenance()
	_, err := repairBundledCoreLocked(ctx, IsSmartCoreActive(), runtimeassets.RepairBundledStock)
	if err != nil {
		status := runtimeassets.GetStatus(ctx)
		health := status.Assets[runtimeassets.AssetCore]
		health.Ready = false
		health.Error = err.Error()
		health.ErrorCode = runtimeassets.ErrExecFailed
		status.Assets[runtimeassets.AssetCore] = health
		status.CoreReady = false
		status.Ready = false
		return status, err
	}
	other := runtimeassets.RequireAll
	other.NeedCore = false
	return runtimeassets.EnsureReady(ctx, other, runtimeassets.RepairForce)
}

func repairBundledCoreLocked(ctx context.Context, smart bool, repair func(context.Context, string, func(string) error) (runtimeassets.StockSyncResult, error)) (runtimeassets.StockSyncResult, error) {
	if smart {
		return runtimeassets.StockSyncResult{}, fmt.Errorf("Smart сохранён: восстановление активного Smart из stock-комплекта не поддерживается; явно переключитесь на stock или переустановите Smart")
	}
	result, err := repair(ctx, coreExePath(), func(path string) error {
		if err := utils.ValidateWindowsPE(path, 5*1024*1024, 300*1024*1024); err != nil {
			return err
		}
		return validateReplacementTun(ctx, path)
	})
	if result.Updated {
		ClearLocalCoreVersionCache()
	}
	if err == nil && !result.Updated {
		err = fmt.Errorf("восстановление ядра не выполнено: %s", result.Message)
	}
	return result, err
}
