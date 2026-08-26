package appcore

import (
	"context"
	"fmt"
	"goclashz/core/clash"
)

// reloadActiveConfigLive пересобирает runtime-конфиг активного профиля и перечитывает его
// в работающем ядре БЕЗ полного рестарта (PUT /configs?force=true) — обновление подписки
// с включённым VPN больше не рвёт туннель. При любой ошибке — безопасный откат на рестарт.
func (c *Controller) reloadActiveConfigLive(ctx context.Context) error {
	desired := c.Desired.Get()
	behavior := c.Behavior.Get()
	if err := clash.BuildRuntimeConfig(desired.ActiveConfig, desired.Mode, behavior.LogLevel, desired.Tun); err != nil {
		return c.RestartCoreWithReason(ctx, "subscription-update")
	}
	if err := clash.ReloadConfig(); err != nil {
		return c.RestartCoreWithReason(ctx, "subscription-update")
	}
	c.SyncProxyStateOnce()
	c.SyncState()
	return nil
}

func (c *Controller) UpdateSub(ctx context.Context, name, url string, opts *clash.SubFetchOptions) error {
	ua := c.Behavior.Get().SubUA
	id, err := clash.DownloadSub(ctx, name, url, "", ua, opts)
	if err != nil {
		return err
	}

	state := c.GetAppState()
	if state.ActiveConfig == id && state.IsRunning {
		return c.reloadActiveConfigLive(ctx)
	}
	return nil
}

func (c *Controller) UpdateSingleSub(ctx context.Context, id string) error {
	item, ok := clash.FindSubIndexByID(id)
	if !ok {
		return fmt.Errorf("subscription not found")
	}
	if item.URL == "" {
		return fmt.Errorf("subscription not found")
	}

	ua := c.Behavior.Get().SubUA
	_, err := clash.DownloadSub(ctx, item.Name, item.URL, id, ua, &clash.SubFetchOptions{
		FallbackURLs: item.FallbackURLs,
		Headers:      item.Headers,
	})
	if err == nil {
		state := c.GetAppState()
		if state.ActiveConfig == id && state.IsRunning {
			return c.reloadActiveConfigLive(ctx)
		}
	}
	return err
}

func (c *Controller) UpdateAllSubsAsync(ctx context.Context) {
	c.Tasks.Run(ctx, "subs-update", true, func(ctx context.Context) error {
		items := clash.ListSubIndex()

		ua := c.Behavior.Get().SubUA
		needsRestart := false
		state := c.GetAppState()

		for _, item := range items {
			if item.URL != "" && item.Type == "remote" {
				id, err := clash.DownloadSub(ctx, item.Name, item.URL, item.ID, ua, &clash.SubFetchOptions{
					FallbackURLs: item.FallbackURLs,
					Headers:      item.Headers,
				})
				if err == nil && id == state.ActiveConfig {
					needsRestart = true
				}
			}
		}

		if needsRestart && state.IsRunning {
			return c.reloadActiveConfigLive(ctx)
		}
		return nil
	})
}

func (c *Controller) DeleteConfig(id string) error {
	if err := clash.DeleteConfig(id); err != nil {
		return err
	}
	state := c.GetAppState()
	if state.ActiveConfig == id {
		c.Behavior.SetActiveConfig("")
		if state.IsRunning {
			c.StopCoreProcess()
		}
	}
	c.SyncState()
	return nil
}

func (c *Controller) SelectLocalConfig(ctx context.Context, id string) error {
	state := c.GetAppState()
	if state.ActiveConfig == id {
		return nil
	}

	if err := c.Behavior.SetActiveConfig(id); err != nil {
		return err
	}

	desired := c.Desired.Get()
	c.fillDesiredTarget(&desired)
	c.Desired.SetAndSave(desired)

	if state.IsRunning {
		return c.RestartCoreWithReason(ctx, "config-switch")
	}

	c.SyncState()
	return nil
}

func (c *Controller) RenameConfig(id, newName string) error {
	if err := clash.RenameConfig(id, newName); err != nil {
		return err
	}
	c.SyncState()
	return nil
}

func (c *Controller) DoLocalImport(srcPath, name string) (string, error) {
	return clash.ImportLocalConfig(srcPath, name)
}
