package appcore

import (
	"context"
	"fmt"
	"goclashz/core/clash"
	"goclashz/core/downloader"
	"goclashz/core/logger"
	"goclashz/core/sys"
	"goclashz/core/utils"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

func (c *Controller) updateGeoDatabase(ctx context.Context, key string, onProgress func(bytesDone, totalBytes, speedBps, etaSec int64)) error {
	var url string
	behavior := c.Behavior.Get()

	switch key {
	case "geoip":
		url = behavior.GeoIpLink
	case "geosite":
		url = behavior.GeoSiteLink
	case "mmdb":
		url = behavior.MmdbLink
	case "asn":
		url = behavior.AsnLink
	default:
		return fmt.Errorf("unknown geo key: %s", key)
	}

	if url == "" {
		return fmt.Errorf("Ссылка для загрузки не настроена")
	}

	return clash.UpdateGeoDB(ctx, key, url, c.getDynamicStrategy, onProgress)
}

func (c *Controller) UpdateGeoDatabaseAsync(ctx context.Context, key string) {
	c.GeoUpdates.UpdateOneAsync(ctx, key)
}

func (c *Controller) UpdateAllGeoDatabasesAsync(ctx context.Context) {
	c.GeoUpdates.UpdateAllAsync(ctx)
}

func (c *Controller) IsCoreBinWritable() bool {
	testFile := filepath.Join(utils.GetCoreBinDir(), ".write_test")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		return false
	}
	os.Remove(testFile)
	return true
}

func (c *Controller) UpdateCoreComponentAsync(ctx context.Context) {

	if !c.IsCoreBinWritable() {
		helperStatus := sys.CheckHelperService()
		if !helperStatus.Reachable {
			errStr := "Для обновления ядра нужны права администратора (компонент защищён политикой безопасности Windows). Установите фоновую службу или перезапустите программу от имени администратора и повторите попытку."
			c.setLastError(errStr)
			c.UpdateTasks.Set("core-update", UpdateTaskState{
				Key:    "core-update",
				Title:  "Обновление ядра Mihomo",
				Status: "error",
				Error:  errStr,
			})
			return
		}
	}

	c.runComponentUpdateTransaction(ctx, "core-update", ComponentUpdateOptions{
		Name:        "Обновление ядра Mihomo",
		StopCore:    true,
		RestartCore: true,
		Prepare: func(ctx context.Context, onProgress func(int64, int64, int64, int64)) (map[string]string, error) {
			assetURL := ""

			c.mu.RLock()
			cachedURL := c.pendingCoreUpdateAssetURL
			c.mu.RUnlock()

			if cachedURL != "" {
				assetURL = cachedURL
			} else {
				_, discoveredURL, _, err := clash.CheckLatestCore(ctx, c.getDynamicStrategy)
				if err != nil {
					return nil, err
				}
				assetURL = discoveredURL
			}

			return clash.PrepareCoreUpdate(ctx, assetURL, c.getDynamicStrategy, onProgress)
		},
		Commit: func(ctx context.Context, prepared map[string]string) (map[string]string, error) {
			version, err := clash.CommitCoreUpdate(ctx, prepared)
			if err != nil {
				return nil, err
			}
			return map[string]string{
				"version": version,
			}, nil
		},
		AfterSuccess: func(result map[string]string) {
			if version := result["version"]; version != "" {
				c.events.Emit("core-version-updated", map[string]string{
					"version": version,
				})
			}

			c.events.Emit("delay-cache-clear", "core-update")

			c.mu.Lock()
			c.pendingCoreUpdateAssetURL = ""
			c.pendingCoreUpdateVersion = ""
			c.mu.Unlock()
		},
	})
}

func (c *Controller) CheckCoreUpdateAsync(ctx context.Context) {
	c.Tasks.Run(ctx, "core-update-check", true, func(ctx context.Context) error {
		timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		local := clash.GetLocalCoreVersion(timeoutCtx)

		remote, assetURL, releaseURL, err := clash.CheckLatestCore(timeoutCtx, c.getDynamicStrategy)
		if err != nil {
			return err
		}

		cmp, err := clash.CompareCoreVersion(remote, local)
		if err != nil {
			return err
		}

		if cmp <= 0 {
			c.mu.Lock()
			c.pendingCoreUpdateAssetURL = ""
			c.pendingCoreUpdateVersion = ""
			c.mu.Unlock()

			c.events.Emit("core-update-none", map[string]string{
				"local":  local,
				"remote": remote,
			})
			return nil
		}

		c.mu.Lock()
		c.pendingCoreUpdateAssetURL = assetURL
		c.pendingCoreUpdateVersion = remote
		c.mu.Unlock()

		c.events.Emit("core-update-available", map[string]string{
			"local":      local,
			"remote":     remote,
			"assetUrl":   assetURL,
			"releaseUrl": releaseURL,
		})

		return nil
	})
}

func (c *Controller) InstallTunDriverAsync(ctx context.Context) {
	c.runComponentUpdateTransaction(ctx, "driver-install", ComponentUpdateOptions{
		Name:        "Переустановка Wintun",
		StopCore:    true,
		RestartCore: true,
		Prepare: func(ctx context.Context, onProgress func(int64, int64, int64, int64)) (map[string]string, error) {
			return clash.PrepareWintunRuntime(ctx, c.getDynamicStrategy, onProgress)
		},
		Commit: func(ctx context.Context, prepared map[string]string) (map[string]string, error) {
			version, err := clash.CommitWintunRuntime(ctx, prepared)
			if err != nil {
				return nil, err
			}
			return map[string]string{
				"version": version,
			}, nil
		},
		AfterSuccess: func(result map[string]string) {
			if version := result["version"]; version != "" {
				c.events.Emit("wintun-version-updated", map[string]string{
					"version": version,
				})
			}
		},
	})
}

func (c *Controller) GetCoreVersion(ctx context.Context) string {
	return clash.GetLocalCoreVersion(ctx)
}

func (c *Controller) CheckAppUpdateAsync(ctx context.Context, currentVersion string, manual bool) {
	ok := c.Tasks.RunIfIdle(ctx, "app-update-flow", false, func(ctx context.Context) error {
		if manual {
			c.events.Emit("app-update-check-start")
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		info, err := downloader.CheckAppUpdate(timeoutCtx, currentVersion, c.getDynamicStrategy)
		if err != nil {
			if manual {
				c.events.Emit("app-update-error", "Не удалось проверить обновления программы: "+err.Error())
			}
			return nil
		}

		if info == nil || !info.HasUpdate {
			if manual {
				c.events.Emit("app-update-none", map[string]string{
					"message": "Установлена последняя версия.",
				})
			}
			return nil
		}

		// На Linux программа ставится пакетом (deb/AUR) или AppImage, а в обновлениях
		// лежит установщик под Windows — качать и запускать его тут нельзя.
		// Сообщаем о версии и отправляем на страницу загрузки.
		if goruntime.GOOS != "windows" {
			if manual {
				c.events.Emit("app-update-error", fmt.Sprintf(
					"Доступна версия %s. На Linux обновите пакет через менеджер пакетов или скачайте сборку: %s",
					info.Version, info.ReleaseURL,
				))
			}
			return nil
		}

		if info.DownloadURL == "" {
			if manual {
				c.events.Emit("app-update-error", fmt.Sprintf(
					"Найдена новая версия %s, но в Release нет подходящего установочного пакета программы",
					info.Version,
				))
			}
			return nil
		}

		c.mu.Lock()
		c.pendingAppUpdateInfo = info
		c.mu.Unlock()

		c.events.Emit("app-update-available", map[string]any{
			"version": info.Version,
			"manual":  manual,
		})

		return nil
	})

	if !ok && manual {
		c.events.Emit("app-update-busy")
	}
}

func (c *Controller) DownloadPendingAppUpdateAsync(ctx context.Context) {
	ok := c.Tasks.RunIfIdle(ctx, "app-update-flow", false, func(ctx context.Context) error {
		c.mu.RLock()
		info := c.pendingAppUpdateInfo
		c.mu.RUnlock()

		if info == nil || !info.HasUpdate || info.DownloadURL == "" {
			c.events.Emit("app-update-error", "Нет доступного обновления для загрузки, проверьте обновления заново.")
			return nil
		}

		return c.downloadAppUpdateWithInfo(ctx, info)
	})

	if !ok {
		c.events.Emit("app-update-busy")
	}
}

func (c *Controller) CheckAndDownloadAppUpdateAsync(ctx context.Context, currentVersion string) {

	c.CheckAppUpdateAsync(ctx, currentVersion, true)
}

func (c *Controller) AutoCheckAndDownloadAppUpdateAsync(ctx context.Context, currentVersion string) {

	c.CheckAppUpdateAsync(ctx, currentVersion, false)
}

func (c *Controller) downloadAppUpdateWithInfo(ctx context.Context, info *downloader.AppUpdateInfo) error {
	c.SetUpdateStatus(true, info.Version)
	c.setAppUpdateDownloading(true)
	defer c.setAppUpdateDownloading(false)

	c.events.Emit("app-update-start", map[string]string{
		"version": info.Version,
	})

	destDir := filepath.Join(utils.GetDataDir(), "updates")

	onProgress := func(bytesDone, totalBytes, speedBps, etaSec int64) {
		c.events.Emit("app-update-progress", map[string]interface{}{
			"bytesDone":  bytesDone,
			"totalBytes": totalBytes,
			"speedBps":   speedBps,
			"etaSec":     etaSec,
		})
	}

	path, err := downloader.DownloadAppUpdate(ctx, info, destDir, onProgress, c.getDynamicStrategy)
	if err != nil {
		logger.Errorf("Не удалось загрузить обновление программы: %v", err)
		c.events.Emit("app-update-error", userFacingAppUpdateDownloadError(err))
		return err
	}

	c.SetDownloadedAppUpdate(path, info.Version)
	c.events.Emit("app-update-downloaded", map[string]string{
		"version": info.Version,
		"path":    path,
	})
	return nil
}

func (c *Controller) getDynamicStrategy() downloader.DownloadStrategy {
	c.mu.RLock()
	preferProxy := c.sysProxyActive || c.tunActive
	c.mu.RUnlock()

	return downloader.DownloadStrategy{
		ProxyURL:    resolveLocalProxyURL(),
		PreferProxy: preferProxy,
	}
}

func userFacingAppUpdateDownloadError(err error) string {
	if err == nil {
		return "Не удалось загрузить обновление программы, повторите попытку позже."
	}

	msg := strings.ToLower(err.Error())

	if strings.Contains(msg, "wsarecv") ||
		strings.Contains(msg, "connection was aborted") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "все сетевые окружения") ||
		strings.Contains(msg, "github") {
		return "Не удалось загрузить обновление: текущая сеть не может подключиться к адресу загрузки. Проверьте сеть или переключитесь на рабочий узел и повторите попытку."
	}

	if strings.Contains(msg, "mz") ||
		strings.Contains(msg, "не является допустимым исполняемым файлом windows") ||
		strings.Contains(msg, "аномальный размер") {
		return "Загруженный установочный пакет повреждён, повторите загрузку позже."
	}

	return "Не удалось загрузить обновление программы, повторите попытку позже."
}
