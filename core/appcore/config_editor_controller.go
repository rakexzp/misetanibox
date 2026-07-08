package appcore

import (
	"context"
	"fmt"
	"goclashz/core/clash"
)

func (c *Controller) SaveConfigText(ctx context.Context, id string, content string) error {

	if err := clash.SaveConfigText(id, content); err != nil {
		return err
	}

	state := c.GetAppState()

	c.events.Emit("config-file-saved", map[string]any{
		"id": id,
	})

	if state.ActiveConfig == id {
		c.events.Emit("delay-cache-clear", "config-save")

		if state.IsRunning || state.SystemProxy || state.Tun {
			if err := c.RestartCoreWithReason(ctx, "config-save"); err != nil {
				return fmt.Errorf("Сохранено успешно, но не удалось перезагрузить рабочую конфигурацию: %w", err)
			}
		}

		c.events.Emit("config-changed", id)
		c.SyncState()
	}

	return nil
}
