package appcore

import (
	"context"
	"fmt"

	"goclashz/core/backup"
	"goclashz/core/clash"
	"goclashz/core/utils"
)

func (c *Controller) ExportBackup(destPath string) error {
	return backup.Export(utils.GetDataDir(), destPath, c.version)
}

func (c *Controller) RestoreBackup(ctx context.Context, selected string, mode string) error {
	if selected == "" {
		return fmt.Errorf("Не выбран корректный файл резервной копии")
	}

	c.componentUpdateMu.Lock()
	defer c.componentUpdateMu.Unlock()

	c.coreLifecycleMu.Lock()
	wasRunning := clash.IsRunning()

	c.mu.RLock()
	wantSysProxy := c.sysProxyActive
	wantTun := c.tunActive
	c.mu.RUnlock()

	shouldRestart := wasRunning || wantSysProxy || wantTun

	if wasRunning {
		c.stopCoreProcessLocked()
		c.coreLifecycleMu.Unlock()
		c.SyncState()
	} else {
		c.coreLifecycleMu.Unlock()
	}

	if err := backup.RestoreTransactional(ctx, utils.GetDataDir(), selected, mode); err != nil {

		if shouldRestart {
			_ = c.EnsureCoreRunning(ctx)
		}
		c.SyncState()
		return err
	}

	_ = clash.LoadIndex()

	if err := clash.MigrateRuleStorageV2(); err != nil {
		c.SyncState()
		return fmt.Errorf("Резервная копия восстановлена, но не удалось перенести хранилище правил: %w", err)
	}

	if err := c.Behavior.Load(); err != nil {
		c.SyncState()
		return fmt.Errorf("Конфигурация восстановлена успешно, но не удалось её перезагрузить: %v", err)
	}

	if shouldRestart {
		err := c.EnsureCoreRunning(ctx)
		if err != nil {
			c.SyncState()
			return fmt.Errorf("Восстановление успешно, но не удалось запустить ядро: %v", err)
		}
	} else {

		state := c.GetAppState()
		if state.ActiveConfig != "" {

			_ = clash.BuildRuntimeConfig(
				c.Behavior.Get().ActiveConfig,
				c.Behavior.Get().ActiveMode,
				c.Behavior.Get().LogLevel,
				c.Desired.Get().Tun,
			)
		}
	}

	c.RefreshAutoDelayTest(AutoDelayRefreshOptions{
		Immediate: false,
		Reason:    "restore",
	})
	c.RefreshAppAutoUpdate()
	c.SyncState()

	return nil
}
