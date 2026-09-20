package appcore

import (
	"context"
	"fmt"
	"goclashz/core/clash"
	"goclashz/core/utils"
	"sync"
)

var tunSaveMu sync.Mutex

func (c *Controller) SaveTunConfig(ctx context.Context, cfg *clash.TunConfig) error {
	tunSaveMu.Lock()
	defer tunSaveMu.Unlock()
	if err := clash.ValidateTunConfig(cfg); err != nil {
		return err
	}
	previous, err := clash.GetTunConfig()
	if err != nil {
		return err
	}
	wasRunning := clash.IsRunning()
	return applyTunTransaction(previous, cfg, func(value *clash.TunConfig) error {
		// Rollback must restore even a saved MIPS selection after a downgrade.
		return utils.SaveSetting("tun", value)
	}, func() error {
		if wasRunning {
			return c.RestartCore(ctx)
		}
		return nil
	})
}

func applyTunTransaction(previous, next *clash.TunConfig, save func(*clash.TunConfig) error, restart func() error) error {
	if err := save(next); err != nil {
		return err
	}
	if err := restart(); err != nil {
		if restoreErr := save(previous); restoreErr != nil {
			return fmt.Errorf("применение TUN: %w; восстановление настроек: %v", err, restoreErr)
		}
		if restoreErr := restart(); restoreErr != nil {
			return fmt.Errorf("применение TUN: %w; восстановление runtime: %v", err, restoreErr)
		}
		return fmt.Errorf("настройки TUN отменены, прежний runtime восстановлен: %w", err)
	}
	return nil
}

func (c *Controller) SaveDNSConfig(ctx context.Context, cfg *clash.DNSConfig) error {
	if err := clash.UpdateDNSConfig(cfg); err != nil {
		return err
	}
	if clash.IsRunning() {
		return c.RestartCore(ctx)
	}
	return nil
}

func (c *Controller) SaveNetworkConfig(ctx context.Context, cfg *clash.NetworkConfig) error {
	if err := clash.UpdateNetworkConfig(cfg); err != nil {
		return err
	}
	if clash.IsRunning() {
		return c.RestartCore(ctx)
	}
	return nil
}
func (c *Controller) ResetTunConfig(ctx context.Context) error {
	defaultCfg := clash.GetDefaultTunConfig()
	return c.SaveTunConfig(ctx, &defaultCfg)
}

func (c *Controller) ResetDNSConfig(ctx context.Context) error {
	defaultCfg := clash.GetDefaultDNSConfig()
	return c.SaveDNSConfig(ctx, &defaultCfg)
}

func (c *Controller) ResetNetworkConfig(ctx context.Context) error {
	defaultCfg := clash.GetDefaultNetworkConfig()
	return c.SaveNetworkConfig(ctx, &defaultCfg)
}
