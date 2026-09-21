package clash

import (
	"context"
	"runtime"

	"goclashz/core/runtimeassets"
	"goclashz/core/utils"
)

// ReconcileBundledCore is opt-in for the Windows Wails startup, not native hosts.
func ReconcileBundledCore(ctx context.Context) (runtimeassets.StockSyncResult, error) {
	if runtime.GOOS != "windows" {
		return runtimeassets.StockSyncResult{}, nil
	}
	coreBinaryMu.Lock()
	defer coreBinaryMu.Unlock()
	runtimeassets.EnableBundledCoreMaintenance()
	target := coreExePath()
	smart := IsSmartCoreActive()
	if smart {
		target = stockBackupPath()
	}
	result, err := runtimeassets.ReconcileBundledStock(ctx, target, func(path string) error {
		if err := utils.ValidateWindowsPE(path, 5*1024*1024, 300*1024*1024); err != nil {
			return err
		}
		if !smart {
			return validateReplacementTun(ctx, path)
		}
		return nil
	})
	if result.Updated {
		ClearLocalCoreVersionCache()
	}
	if smart {
		result.Message = "Smart сохранён; stock backup: " + result.Message
	}
	return result, err
}
