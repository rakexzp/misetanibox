package appcore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"goclashz/core/clash"
)

type ComponentUpdateOptions struct {
	Name         string
	StopCore     bool
	RestartCore  bool
	Prepare      func(ctx context.Context, onProgress func(bytesDone, totalBytes, speedBps, etaSec int64)) (map[string]string, error)
	Commit       func(ctx context.Context, prepared map[string]string) (map[string]string, error)
	AfterSuccess func(map[string]string)
}

func (c *Controller) runComponentUpdateTransaction(
	ctx context.Context,
	taskName string,
	opt ComponentUpdateOptions,
) {
	c.Tasks.Run(ctx, taskName, true, func(ctx context.Context) error {
		c.events.Emit(taskName + "-start")
		state, _ := c.UpdateTasks.Get(taskName)
		state.Key = taskName
		state.Title = opt.Name
		state.Status = "running"
		state.Stage = "preparing"
		if state.StartedAt == 0 {
			state.StartedAt = time.Now().Unix()
		}
		state.Error = ""
		c.UpdateTasks.Set(taskName, state)

		c.componentUpdateMu.Lock()
		defer c.componentUpdateMu.Unlock()

		startedAt := time.Now().Unix()
		prepared, err := opt.Prepare(ctx, func(bytesDone, totalBytes, speedBps, etaSec int64) {
			state, _ := c.UpdateTasks.Get(taskName)
			state.Status = "running"
			state.Stage = "downloading"
			state.BytesDone = bytesDone
			state.BytesTotal = totalBytes
			state.SpeedBps = speedBps
			state.ETASeconds = etaSec
			state.StartedAt = startedAt
			c.UpdateTasks.Set(taskName, state)
		})
		if err != nil {
			c.SyncState()

			state, ok := c.UpdateTasks.Get(taskName)
			if !ok {
				state = UpdateTaskState{
					Key:   taskName,
					Title: opt.Name,
				}
			}

			isCanceled := errors.Is(err, context.Canceled) ||
				strings.Contains(strings.ToLower(err.Error()), "canceled") ||
				strings.Contains(strings.ToLower(err.Error()), "cancelled")

			if isCanceled {
				state.Status = "cancelled"
				state.Error = "Приостановлено"
			} else {
				state.Status = "error"
				state.Error = fmt.Sprintf("Ошибка подготовки: %v", err)
			}
			state.FinishedAt = time.Now().Unix()
			c.UpdateTasks.Set(taskName, state)

			c.events.Emit(taskName+"-error", err.Error())
			return fmt.Errorf("%s: ошибка подготовки: %w", opt.Name, err)
		}

		state, _ = c.UpdateTasks.Get(taskName)
		state.Status = "running"
		state.Stage = "committing"
		c.UpdateTasks.Set(taskName, state)

		c.coreLifecycleMu.Lock()

		wasRunning := clash.IsRunning()

		c.mu.RLock()
		wantSysProxy := c.sysProxyActive
		wantTun := c.tunActive
		c.mu.RUnlock()

		shouldRestart := wasRunning || wantSysProxy || wantTun

		if opt.StopCore && wasRunning {
			c.stopCoreProcessLocked()
			c.coreLifecycleMu.Unlock()

			c.SyncState()
		} else {
			c.coreLifecycleMu.Unlock()
		}

		result, err := opt.Commit(ctx, prepared)
		if err != nil {

			if shouldRestart {
				_ = c.EnsureCoreRunning(ctx)
			}

			c.SyncState()
			c.UpdateTasks.Set(taskName, UpdateTaskState{
				Key:        taskName,
				Title:      opt.Name,
				Status:     "error",
				Error:      fmt.Sprintf("Ошибка применения: %v", err),
				FinishedAt: time.Now().Unix(),
			})
			c.events.Emit(taskName+"-error", err.Error())
			return fmt.Errorf("%s: сбой: %w", opt.Name, err)
		}

		if opt.AfterSuccess != nil {
			opt.AfterSuccess(result)
		}

		if opt.RestartCore && shouldRestart {
			err = c.EnsureCoreRunning(ctx)

			if err != nil {
				c.SyncState()
				c.UpdateTasks.Set(taskName, UpdateTaskState{
					Key:        taskName,
					Title:      opt.Name,
					Status:     "error",
					Error:      fmt.Sprintf("Не удалось восстановить запуск ядра: %v", err),
					FinishedAt: time.Now().Unix(),
				})
				c.events.Emit(taskName+"-error", err.Error())
				return fmt.Errorf("%s: успешно, но не удалось снова запустить ядро: %w", opt.Name, err)
			}
		}

		c.UpdateTasks.Set(taskName, UpdateTaskState{
			Key:        taskName,
			Title:      opt.Name,
			Status:     "success",
			Progress:   100,
			FinishedAt: time.Now().Unix(),
		})
		c.events.Emit(taskName + "-success")

		c.SyncState()
		return nil
	})
}
