package appcore

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"goclashz/core/clash"
	"goclashz/core/downloader"
	"goclashz/core/logger"
	"goclashz/core/runtimeassets"
	"goclashz/core/sys"
	"goclashz/core/tasks"
	"goclashz/core/utils"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

var helperReadyGroup singleflight.Group

type LogEntry = logger.LogEntry

type AutoDelayRefreshOptions struct {
	Immediate bool
	Reason    string
}

type BootstrapOptions struct {
	IsStartupLaunch bool
	Silent          bool
}

type Options struct {
	Events  EventSink
	Version string

	OnStateChange func()
}

type CoreRuntimeState struct {
	Running      bool
	PID          int
	Owner        string
	Purpose      string
	Tun          bool
	SystemProxy  bool
	ActiveConfig string
	Mode         string
	StartedAt    time.Time
}

type runtimeStateStore struct {
	mu    sync.RWMutex
	state CoreRuntimeState
}

func (s *runtimeStateStore) Get() CoreRuntimeState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *runtimeStateStore) Set(st CoreRuntimeState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = st
}

func (s *runtimeStateStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = CoreRuntimeState{}
}

func (c *Controller) runtimeSatisfiesCore(plan RuntimePlan) bool {
	if !clash.IsRunning() {
		return false
	}

	st := c.runtimeState.Get()
	if !st.Running {
		return false
	}
	return st.Purpose == "user" &&
		st.Tun == plan.NeedTun &&
		st.ActiveConfig == plan.ActiveConfig &&
		st.Mode == plan.Mode
}

type Controller struct {
	events        EventSink
	onStateChange func()
	Behavior      *BehaviorStore
	Offline       *OfflineNodeStore
	Tasks         *tasks.Manager
	version       string

	Supervisor *CoreSupervisor
	Desired    *DesiredStateStore

	mu                sync.RWMutex
	coreLifecycleMu   sync.Mutex
	componentUpdateMu sync.Mutex
	sysProxyActive    bool
	lastSysProxyApply time.Time
	tunActive         bool

	autoTestQuit chan struct{}
	autoTestMu   sync.Mutex

	appLogger   *LogDispatcher
	traffic     *TrafficStreamManager
	logs        *LogStreamManager
	Delay       *DelayTestManager
	proxyState  *ProxyStateMonitor
	connections *ConnectionMonitorManager
	ctx         context.Context

	updateReady   bool
	newAppVersion string

	downloadedUpdatePath    string
	downloadedUpdateVersion string
	pendingAppUpdateInfo    *downloader.AppUpdateInfo

	GeoUpdates  *GeoUpdateManager
	UpdateTasks *ComponentUpdateTaskStore

	pendingCoreUpdateAssetURL string
	pendingCoreUpdateVersion  string

	delayCoreMu          sync.Mutex
	delayCoreRefs        int
	delayCoreStarted     bool
	delayCoreStarting    bool
	delayCoreReady       chan struct{}
	delayCoreStartCancel context.CancelFunc
	delayCoreStartErr    error
	delayCoreStopTimer   *time.Timer

	userCoreRunning bool
	coreStartedAt   time.Time

	runtimeState runtimeStateStore

	networkMu            sync.RWMutex
	appUpdateDownloading bool
	autoDelayRunning     bool
}

func (c *Controller) setAppUpdateDownloading(active bool) {
	c.networkMu.Lock()
	defer c.networkMu.Unlock()
	c.appUpdateDownloading = active
}

func (c *Controller) isAppUpdateDownloading() bool {
	c.networkMu.RLock()
	defer c.networkMu.RUnlock()
	return c.appUpdateDownloading
}

func (c *Controller) setAutoDelayRunning(active bool) {
	c.networkMu.Lock()
	defer c.networkMu.Unlock()
	c.autoDelayRunning = active
}

func NewController(opts Options) *Controller {
	behavior := NewBehaviorStore()
	activeConfig := behavior.Get().ActiveConfig

	c := &Controller{
		events:        opts.Events,
		onStateChange: opts.OnStateChange,
		version:       opts.Version,
		Behavior:      behavior,
		Offline:       NewOfflineNodeStore(activeConfig),
		Tasks:         tasks.NewManager(opts.Events),
		Desired:       NewDesiredStateStore(),
	}

	if !behavior.Get().RestoreOnStartup {
		d := c.Desired.Get()
		if d.SystemProxy || d.Tun || d.CoreRunning {
			d.SystemProxy = false
			d.Tun = false
			d.CoreRunning = false
			_ = c.Desired.SetAndSave(d)
		}
	}
	c.appLogger = NewLogDispatcher(opts.Events, func() string {
		return c.Behavior.Get().AppLogLevel
	})
	c.Supervisor = NewCoreSupervisor(c, c.Desired)
	c.traffic = NewTrafficStreamManager(opts.Events, c.appLogger)
	c.logs = NewLogStreamManager(c.appLogger)
	c.Delay = NewDelayTestManager(opts.Events, c)
	c.proxyState = NewProxyStateMonitor(opts.Events)
	c.connections = NewConnectionMonitorManager(opts.Events)

	c.UpdateTasks = NewComponentUpdateTaskStore(opts.Events)
	c.GeoUpdates = NewGeoUpdateManager(opts.Events, c.updateGeoDatabase, c.UpdateTasks)

	return c
}

func (c *Controller) logInfo(format string, args ...any) {
	c.appLogger.AddApp(logger.LogEntry{
		Type:    "info",
		Payload: fmt.Sprintf(format, args...),
	})
}

func (c *Controller) logWarn(format string, args ...any) {
	c.appLogger.AddApp(logger.LogEntry{
		Type:    "warn",
		Payload: fmt.Sprintf(format, args...),
	})
}

func (c *Controller) logError(format string, args ...any) {
	c.appLogger.AddApp(logger.LogEntry{
		Type:    "error",
		Payload: fmt.Sprintf(format, args...),
	})
}

func (c *Controller) LogApp(level string, format string, args ...any) {
	if c == nil || c.appLogger == nil {
		return
	}

	c.appLogger.AddApp(logger.LogEntry{
		Type:    logger.NormalizeLevel(level),
		Payload: fmt.Sprintf(format, args...),
	})
}

func (c *Controller) CancelUpdateTask(key string) {
	if isGeoKey(key) {
		c.GeoUpdates.Cancel(key)
	} else {
		c.Tasks.Cancel(key)
	}
}

func (c *Controller) RemoveUpdateTask(key string) {
	c.CancelUpdateTask(key)
	c.ClearUpdateCache(key)
	c.UpdateTasks.Remove(key)
}

func (c *Controller) ClearUpdateCache(key string) {
	var destPath string
	switch key {
	case "core-update":
		destPath = filepath.Join(utils.GetCoreBinDir(), "clash.update.zip")
	case "driver-install":
		destPath = filepath.Join(utils.GetCoreBinDir(), "wintun.dll.zip")
	default:
		destPath, _ = clash.GeoDBPath(key)
	}

	if destPath != "" {
		_ = os.Remove(destPath + ".tmp")
		_ = os.Remove(destPath + ".tmp.meta.json")
	}
}

func (c *Controller) Bootstrap(ctx context.Context, opts BootstrapOptions) {
	c.ctx = ctx
	CleanLegacyFiles(c.version)

	clash.SetOnExitCallback(func(e clash.ExitEvent) {
		if !e.Intentional {
			c.mu.Lock()
			wasSysProxy := c.sysProxyActive
			c.sysProxyActive = false
			c.tunActive = false
			c.userCoreRunning = false
			c.coreStartedAt = time.Time{}
			c.mu.Unlock()

			c.runtimeState.Clear()

			if wasSysProxy {
				if err := sys.DisableSystemProxy(); err != nil {
					c.logWarn("Не удалось отключить системный прокси после аварийного завершения ядра: %v", err)
				}
			}

			c.events.Emit("clash-exited", e.Message)
			c.SyncState()
			c.Supervisor.ReconcileAsync("core-exited")
		}
	})

	c.syncStartupTaskStateSafe()

	c.Supervisor.Start(ctx)

	if opts.IsStartupLaunch && c.Behavior.Get().RestoreOnStartup {

		go func() {
			c.runStartupRestoreJob(ctx)
			c.RefreshAutoDelayTest(AutoDelayRefreshOptions{
				Immediate: true,
				Reason:    "startup",
			})
		}()
	} else {

		desired := c.Desired.Get()
		desired.SystemProxy = false
		desired.Tun = false
		desired.CoreRunning = false
		c.Desired.SetAndSave(desired)

		c.SyncState()

		c.RefreshAutoDelayTest(AutoDelayRefreshOptions{
			Immediate: true,
			Reason:    "startup",
		})
	}

	c.RefreshAppAutoUpdate()
}

func (c *Controller) runStartupRestoreJob(ctx context.Context) {
	backoff := []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second}

	for i, delay := range backoff {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		desired := c.Desired.Get()
		if !desired.CoreRunning && !desired.SystemProxy && !desired.Tun {
			return
		}

		restoreTimeout := 8 * time.Second
		if desired.Tun {
			restoreTimeout = 15 * time.Second
		}

		attemptCtx, cancel := context.WithTimeout(ctx, restoreTimeout)
		err := c.Supervisor.Reconcile(attemptCtx, "startup-restore")
		cancel()

		if err == nil {
			c.logInfo("Восстановление запуска успешно (attempt %d)", i+1)
			return
		}

		switch {
		case errors.Is(err, ErrNoActiveConfig):
			c.events.Emit("notify-error", "Восстановление запуска не удалось: конфигурация ещё не добавлена")
			c.abortStartupRestore("нет активной конфигурации")
			return
		case errors.Is(err, ErrHelperInstallRequired):
			c.events.Emit("notify-error", "Для восстановления запуска TUN требуется инициализация фоновой службы")
			c.abortStartupRestore("служба helper не установлена")
			return
		case errors.Is(err, ErrHelperRepairRequired):
			c.events.Emit("notify-error", "Не удалось запустить фоновую службу для TUN — включите VPN вручную (потребуются права администратора)")
			c.abortStartupRestore("служба helper недоступна")
			return
		case errors.Is(err, ErrWintunMissing):
			c.events.Emit("notify-error", "Восстановление запуска TUN не удалось: отсутствует драйвер Wintun")
			c.abortStartupRestore("нет драйвера Wintun")
			return
		default:
			c.logWarn("Восстановление запуска не удалось attempt=%d: %v", i+1, err)
		}
	}

	c.events.Emit("notify-error", "Не удалось восстановить VPN после запуска системы — включите вручную")
	c.abortStartupRestore("исчерпаны попытки")
}

// abortStartupRestore сбрасывает «желаемое» состояние после неудачного восстановления.
// Без этого desired.Tun/CoreRunning оставались true, и интерфейс показывал «VPN включён»,
// хотя туннель не поднят — пользователь думал, что защищён.
func (c *Controller) abortStartupRestore(reason string) {
	c.StopCoreService()

	desired := c.Desired.Get()
	desired.CoreRunning = false
	desired.SystemProxy = false
	desired.Tun = false
	c.Desired.SetAndSave(desired)

	c.runtimeState.Clear()
	c.mu.Lock()
	c.tunActive = false
	c.sysProxyActive = false
	c.userCoreRunning = false
	c.mu.Unlock()

	c.SyncState()
	c.logWarn("Восстановление запуска прервано (%s) — состояние сброшено, интерфейс покажет «выключено»", reason)
}

func (c *Controller) syncStartupTaskStateSafe() {
	info, err := sys.CheckStartupTask()
	if err != nil {
		c.events.Emit("startup-task-check-warning", err.Error())
		return
	}

	behavior := c.Behavior.Get()

	behavior.StartupWithOS = info.Exists && info.Enabled

	_ = c.Behavior.SetAndSave(behavior)
}

func (c *Controller) GetEvents() EventSink {
	return c.events
}

type AppState struct {
	IsRunning bool `json:"isRunning"`
	IsAdmin   bool `json:"isAdmin"`
	// Platform — ОС ("windows"/"linux"): интерфейс прячет неприменимые разделы
	Platform    string `json:"platform"`
	Mode        string `json:"mode"`
	Theme       string `json:"theme"`
	HideLogs    bool   `json:"hideLogs"`
	AppLogLevel string `json:"appLogLevel"`

	DesiredSystemProxy bool `json:"desiredSystemProxy"`
	DesiredTun         bool `json:"desiredTun"`

	ActualSystemProxy bool `json:"actualSystemProxy"`
	ActualTun         bool `json:"actualTun"`

	SystemProxy bool   `json:"systemProxy"`
	Tun         bool   `json:"tun"`
	Version     string `json:"version"`
	AppVersion  string `json:"appVersion"`

	ActiveConfig     string `json:"activeConfig"`
	ActiveConfigName string `json:"activeConfigName"`
	ActiveConfigType string `json:"activeConfigType"`

	DelayRetention     bool   `json:"delayRetention"`
	DelayRetentionTime string `json:"delayRetentionTime"`

	UpdateReady      bool   `json:"updateReady"`
	NewAppVersion    string `json:"newAppVersion"`
	UpdateDownloaded bool   `json:"updateDownloaded"`
	DownloadedPath   string `json:"downloadedPath"`
}

func (c *Controller) GetAppState() AppState {
	behavior := c.Behavior.Get()
	activeConfig := behavior.ActiveConfig
	desired := c.Desired.Get()

	c.mu.RLock()
	sysProxy := c.sysProxyActive
	tunActive := c.tunActive
	userRunning := c.userCoreRunning
	c.mu.RUnlock()

	logicalIsRunning := clash.IsRunning() && userRunning

	state := AppState{
		IsRunning:          logicalIsRunning,
		IsAdmin:            sys.CheckAdmin(),
		Platform:           goruntime.GOOS,
		Mode:               behavior.ActiveMode,
		DesiredSystemProxy: desired.SystemProxy,
		DesiredTun:         desired.Tun,
		ActualSystemProxy:  sysProxy,
		ActualTun:          tunActive,
		SystemProxy:        sysProxy,
		Tun:                tunActive,
		Theme:              utils.GetGlobalTheme(),
		Version:            clash.GetLocalCoreVersion(c.ctx),
		AppVersion:         c.version,
		ActiveConfig:       activeConfig,
		DelayRetention:     behavior.DelayRetention,
		DelayRetentionTime: behavior.DelayRetentionTime,
		HideLogs:           behavior.HideLogs,
		AppLogLevel:        behavior.AppLogLevel,
		UpdateReady:        c.updateReady,
		NewAppVersion:      c.newAppVersion,
		UpdateDownloaded:   c.downloadedUpdatePath != "",
		DownloadedPath:     c.downloadedUpdatePath,
	}

	if activeConfig != "" {
		state.ActiveConfigName = activeConfig
		if item, ok := clash.FindSubIndexByID(activeConfig); ok {
			state.ActiveConfigName = item.Name
			state.ActiveConfigType = item.Type
		}
	}

	return state
}

func (c *Controller) SyncState() {
	state := c.GetAppState()
	c.events.Emit("app-state-sync", state)

	if c.onStateChange != nil {
		c.onStateChange()
	}

	if c.ctx != nil {
		if state.IsRunning {
			behavior := c.Behavior.Get()
			c.traffic.Start(c.ctx, clash.APIURL("/traffic?interval=900"), behavior.ProxyTrafficOnly)
			c.proxyState.Start(c.ctx)
		} else {
			c.traffic.Stop()
			c.proxyState.Stop()
		}
	}
}

func (c *Controller) ensureCoreRunningWithDesiredState(ctx context.Context, desired DesiredState) error {

	if c.runtimeSatisfiesCore(RuntimePlan{
		NeedTun:      desired.Tun,
		ActiveConfig: desired.ActiveConfig,
		Mode:         desired.Mode,
	}) {
		return nil
	}

	assetReq := runtimeassets.RequireCoreOnly
	if desired.Tun {
		assetReq = runtimeassets.RequireTun
	}
	if assetStatus, assetErr := runtimeassets.EnsureReady(ctx, assetReq, runtimeassets.RepairInvalid); assetErr != nil {
		for _, asset := range assetStatus.Assets {
			if !asset.Ready && asset.Error != "" {
				c.logWarn("Рабочий компонент недоступен: %s path=%s error=%s", asset.Key, asset.Path, asset.Error)
			}
		}
		return fmt.Errorf("Рабочий компонент недоступен: %w", assetErr)
	}

	if desired.ActiveConfig == "" {
		return ErrNoActiveConfig
	}

	if desired.Tun {
		if err := c.EnsureHelperReady("tun-start"); err != nil {
			return err
		}

		if err := sys.EnsureTunPrivilege(); err != nil {
			return err
		}
	}

	behavior := c.Behavior.Get()

	err := clash.BuildRuntimeConfig(desired.ActiveConfig, desired.Mode, behavior.LogLevel, desired.Tun)
	if err != nil {
		return err
	}

	if err := EnsureRuntimeAssets(ctx, behavior); err != nil {
		c.logWarn("Не удалось подготовить рабочие ресурсы: %v", err)
	}

	if err := clash.Start(ctx, desired.Tun); err != nil {
		return err
	}

	apiReady := false
	var lastProbeErr error
	for i := 0; i < 100; i++ {
		select {
		case <-ctx.Done():
			clash.Stop()
			c.mu.Lock()
			c.userCoreRunning = false
			c.coreStartedAt = time.Time{}
			c.mu.Unlock()
			return ctx.Err()
		default:
		}

		if err := clash.PingAPIWithContext(ctx); err == nil {
			apiReady = true
			break
		} else {
			lastProbeErr = err
		}

		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			clash.Stop()
			c.mu.Lock()
			c.userCoreRunning = false
			c.coreStartedAt = time.Time{}
			c.mu.Unlock()
			return ctx.Err()
		case <-timer.C:
		}
	}

	if !apiReady {
		clash.Stop()
		c.mu.Lock()
		c.userCoreRunning = false
		c.coreStartedAt = time.Time{}
		c.mu.Unlock()
		return fmt.Errorf("Не удалось запустить ядро: %s", classifyProbeError(lastProbeErr))
	}

	c.mu.Lock()
	c.userCoreRunning = true
	c.tunActive = desired.Tun
	c.coreStartedAt = time.Now()
	c.mu.Unlock()

	owner := "direct"
	if desired.Tun {
		owner = "helper"
	}
	c.runtimeState.Set(CoreRuntimeState{
		Running:      true,
		Owner:        owner,
		Purpose:      "user",
		Tun:          desired.Tun,
		SystemProxy:  c.sysProxyActive,
		ActiveConfig: desired.ActiveConfig,
		Mode:         desired.Mode,
		StartedAt:    time.Now(),
	})

	c.SyncState()

	go func() {
		bgCtx, cancel := context.WithTimeout(c.ctx, 8*time.Second)
		defer cancel()
		c.applyStoredProxySelections(bgCtx, desired.ActiveConfig)
		c.SyncProxyStateOnce()
	}()

	return nil
}

func (c *Controller) stopCoreProcessLocked() {
	clash.Stop()
	c.mu.Lock()
	c.userCoreRunning = false
	c.coreStartedAt = time.Time{}
	c.mu.Unlock()
	c.runtimeState.Clear()
}

func classifyProbeError(err error) string {
	if err == nil {
		return "API не стал доступен за отведённое время"
	}

	msg := err.Error()

	if strings.Contains(msg, "connection refused") {
		return "Порт не прослушивается: ядро не привязало API-порт"
	}

	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") {
		return "Таймаут ответа ядра: возможно, слишком большая конфигурация или нехватка ресурсов системы"
	}

	if strings.Contains(msg, "failed: HTTP") || strings.Contains(msg, "HTTP ") {
		return "Порт занят другой программой: получен неожиданный ответ"
	}

	if strings.Contains(msg, "unexpected end") || strings.Contains(msg, "invalid character") ||
		strings.Contains(msg, "cannot decode") || strings.Contains(msg, "JSON") {
		return "Порт занят другой программой: ответ не в формате ядра"
	}

	if strings.Contains(msg, "connection reset") || strings.Contains(msg, "broken pipe") {
		return "Соединение сброшено: ядро, вероятно, аварийно завершилось"
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return "Ошибка сетевого соединения"
	}

	return "Сбой API-пробы, проверьте конфигурацию ядра"
}

func (c *Controller) EnsureCoreRunning(ctx context.Context) error {
	c.coreLifecycleMu.Lock()
	defer c.coreLifecycleMu.Unlock()

	desired := c.Desired.Get()
	if desired.ActiveConfig == "" {
		behavior := c.Behavior.Get()
		desired.ActiveConfig = behavior.ActiveConfig
		desired.Mode = behavior.ActiveMode
	}
	return c.ensureCoreRunningWithDesiredState(ctx, desired)
}

func (c *Controller) StopCoreService() {
	c.DisableAll()
}

func (c *Controller) reconcileCoreProcess(ctx context.Context, plan RuntimePlan) error {
	if c.runtimeSatisfiesCore(plan) {
		return nil
	}

	if plan.RestartRequired {
		if err := c.RestartCoreWithReason(ctx, plan.Reason); err != nil {
			return fmt.Errorf("restart core failed: %w", err)
		}
	} else {
		if err := c.EnsureCoreRunning(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) reconcileSystemProxy(plan RuntimePlan) error {
	if plan.NeedSysProxy {
		return c.ensureSystemProxyEnabled()
	}
	return c.ensureSystemProxyDisabled()
}

func (c *Controller) ensureSystemProxyEnabled() error {
	port := clash.GetProxyPort()
	target := fmt.Sprintf("127.0.0.1:%d", port)
	current, err := sys.GetSystemProxyState()

	// Уже наш прокси стоит? Проверяем устойчиво (Contains, а не байт-в-байт ==) —
	// per-connection WinINet API пишет ProxyServer не всегда точь-в-точь, и строгое ==
	// приводило к переустановке прокси каждый тик watchdog'а (5с), а каждая переустановка
	// шлёт INTERNET_OPTION_SETTINGS_CHANGED → приложения сбрасывают соединения («рвётся по дикому»).
	if err == nil && current.Enabled && strings.Contains(current.Server, target) {
		c.mu.Lock()
		c.sysProxyActive = true
		c.mu.Unlock()
		return nil
	}

	// Дебаунс: не переустанавливать прокси чаще раза в 30с, даже если проверка не сошлась —
	// защита от флапа, если реестр отдаёт нестандартный формат.
	c.mu.Lock()
	if c.sysProxyActive && time.Since(c.lastSysProxyApply) < 30*time.Second {
		c.mu.Unlock()
		return nil
	}
	c.lastSysProxyApply = time.Now()
	c.mu.Unlock()

	err = sys.EnableSystemProxy(
		"127.0.0.1",
		port,
		"localhost;127.*;10.*;172.16.*;172.17.*;172.18.*;172.19.*;172.20.*;172.21.*;172.22.*;172.23.*;172.24.*;172.25.*;172.26.*;172.27.*;172.28.*;172.29.*;172.30.*;172.31.*;192.168.*;<local>",
	)
	if err != nil {
		c.setLastError("Не удалось установить системный прокси Windows: " + err.Error())
		c.mu.Lock()
		c.sysProxyActive = false
		c.mu.Unlock()
		return err
	}
	c.mu.Lock()
	c.sysProxyActive = true
	c.mu.Unlock()
	return nil
}

func (c *Controller) ensureSystemProxyDisabled() error {
	current, err := sys.GetSystemProxyState()
	if err != nil {
		return fmt.Errorf("Не удалось прочитать состояние системного прокси Windows: %w", err)
	}

	if current.Enabled {
		if err := sys.DisableSystemProxy(); err != nil {
			return fmt.Errorf("Не удалось отключить системный прокси Windows: %w", err)
		}
	}

	c.mu.Lock()
	c.sysProxyActive = false
	c.mu.Unlock()
	return nil
}

func (c *Controller) setRuntimeStateFromPlan(plan RuntimePlan) {
	c.mu.Lock()
	c.tunActive = plan.NeedTun
	c.userCoreRunning = plan.NeedCore
	c.mu.Unlock()
}

func (c *Controller) setLastError(msg string) {
	c.events.Emit("notify-error", msg)
}

func (c *Controller) DisableAll() {
	c.coreLifecycleMu.Lock()
	defer c.coreLifecycleMu.Unlock()

	c.stopCoreProcessLocked()
	_ = sys.DisableSystemProxy()

	c.mu.Lock()
	c.sysProxyActive = false
	c.tunActive = false
	c.userCoreRunning = false
	c.coreStartedAt = time.Time{}
	c.mu.Unlock()

	shouldStopDelayCore := false
	var cancelStart context.CancelFunc
	var ready chan struct{}

	c.delayCoreMu.Lock()
	if c.delayCoreStarted || c.delayCoreStarting {
		shouldStopDelayCore = true
	}

	cancelStart = c.delayCoreStartCancel
	ready = c.delayCoreReady

	c.delayCoreRefs = 0
	c.delayCoreStarted = false
	c.delayCoreStarting = false
	c.delayCoreStartErr = fmt.Errorf("Тест задержки отменён")
	c.delayCoreStartCancel = nil
	c.delayCoreReady = nil

	if c.delayCoreStopTimer != nil {
		c.delayCoreStopTimer.Stop()
		c.delayCoreStopTimer = nil
	}
	c.delayCoreMu.Unlock()

	if cancelStart != nil {
		cancelStart()
	}

	if ready != nil {
		close(ready)
	}

	if shouldStopDelayCore {
		clash.Stop()
	}

	c.SyncState()
}

func (c *Controller) EnsureDelayCore(ctx context.Context) (cleanup func(), warmupRequired bool, err error) {
	c.delayCoreMu.Lock()

	if c.delayCoreStopTimer != nil {
		c.delayCoreStopTimer.Stop()
		c.delayCoreStopTimer = nil
	}

	if clash.IsRunning() {
		c.delayCoreRefs++
		c.delayCoreMu.Unlock()
		warmup := c.NeedsDelayWarmup()
		return c.releaseDelayCore, warmup, nil
	}

	if c.delayCoreStarting {
		ready := c.delayCoreReady
		c.delayCoreMu.Unlock()

		select {
		case <-ready:
		case <-ctx.Done():
			return nil, false, ctx.Err()
		}

		c.delayCoreMu.Lock()
		defer c.delayCoreMu.Unlock()

		if c.delayCoreStartErr != nil {
			return nil, false, c.delayCoreStartErr
		}

		if !clash.IsRunning() {
			return nil, false, fmt.Errorf("Не удалось запустить ядро для теста задержки")
		}

		c.delayCoreRefs++
		return c.releaseDelayCore, true, nil
	}

	profileID := c.Behavior.Get().ActiveConfig
	if profileID == "" {
		c.delayCoreMu.Unlock()
		return nil, false, fmt.Errorf("Сначала выберите и примените конфигурацию в управлении подписками")
	}

	startCtx, cancel := context.WithCancel(ctx)

	c.delayCoreStarting = true
	c.delayCoreReady = make(chan struct{})
	c.delayCoreStartCancel = cancel
	ready := c.delayCoreReady
	c.delayCoreStartErr = nil
	c.delayCoreMu.Unlock()

	startErr := c.startDelayCoreAndWaitReady(startCtx, profileID)

	ok := c.finishDelayCoreStart(ready, startErr)
	if !ok {

		if startErr == nil && !c.IsUserRunning() {
			clash.Stop()
			c.SyncState()
		}
		return nil, false, fmt.Errorf("Тест задержки отменён")
	}

	if startErr != nil {
		return nil, false, startErr
	}

	c.delayCoreMu.Lock()
	c.delayCoreStarted = true
	c.delayCoreRefs = 1
	c.delayCoreMu.Unlock()

	c.mu.Lock()
	c.coreStartedAt = time.Now()
	c.mu.Unlock()

	return c.releaseDelayCore, true, nil
}

func (c *Controller) finishDelayCoreStart(ready chan struct{}, err error) bool {
	c.delayCoreMu.Lock()
	defer c.delayCoreMu.Unlock()

	if c.delayCoreReady != ready {
		return false
	}

	c.delayCoreStarting = false
	c.delayCoreStartErr = err
	c.delayCoreStartCancel = nil
	close(ready)
	c.delayCoreReady = nil

	return true
}

func (c *Controller) startDelayCoreAndWaitReady(ctx context.Context, profileID string) error {
	started := false
	ready := false

	if err := c.StartCoreOnly(ctx, profileID); err != nil {
		return fmt.Errorf("Не удалось запустить ядро для теста задержки: %w", err)
	}
	started = true

	defer func() {
		if started && !ready && !c.IsUserRunning() {
			clash.Stop()
			c.SyncState()
		}
	}()

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeout.C:
			return fmt.Errorf("Таймаут запуска ядра для теста задержки")

		case <-ticker.C:
			if _, err := clash.GetInitialData(); err == nil {
				ready = true
				return nil
			}
		}
	}
}

const DelayCoreIdleTTL = 15 * time.Second

func (c *Controller) releaseDelayCore() {
	c.delayCoreMu.Lock()

	if c.delayCoreRefs > 0 {
		c.delayCoreRefs--
	}

	shouldScheduleStop := false
	if c.delayCoreRefs == 0 && c.delayCoreStarted && !c.IsUserRunning() {
		shouldScheduleStop = true
	}

	if shouldScheduleStop {
		if c.delayCoreStopTimer != nil {
			c.delayCoreStopTimer.Stop()
		}

		c.delayCoreStopTimer = time.AfterFunc(DelayCoreIdleTTL, func() {
			c.delayCoreMu.Lock()

			if c.delayCoreRefs > 0 || !c.delayCoreStarted || c.IsUserRunning() {
				c.delayCoreMu.Unlock()
				return
			}

			c.delayCoreStarted = false
			c.delayCoreStopTimer = nil
			c.delayCoreMu.Unlock()

			c.mu.Lock()
			c.coreStartedAt = time.Time{}
			c.mu.Unlock()

			clash.Stop()
			c.SyncState()
		})
	}

	c.delayCoreMu.Unlock()
}

func (c *Controller) IsUserRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userCoreRunning
}

const DelayWarmupWindow = 20 * time.Second

func (c *Controller) NeedsDelayWarmup() bool {
	c.mu.RLock()
	startedAt := c.coreStartedAt
	c.mu.RUnlock()

	if startedAt.IsZero() {
		return false
	}

	return time.Since(startedAt) <= DelayWarmupWindow
}

func (c *Controller) StartCoreOnly(ctx context.Context, id string) error {
	behavior := c.Behavior.Get()

	if err := clash.BuildRuntimeConfig(id, behavior.ActiveMode, behavior.LogLevel, false); err != nil {
		return err
	}
	return clash.Start(ctx, false)
}

func (c *Controller) StopCoreProcess() {
	c.coreLifecycleMu.Lock()
	defer c.coreLifecycleMu.Unlock()
	c.stopCoreProcessLocked()
}

func (c *Controller) fillDesiredTarget(d *DesiredState) {
	b := c.Behavior.Get()
	d.ActiveConfig = b.ActiveConfig
	d.Mode = b.ActiveMode
}

func (c *Controller) ToggleSystemProxy(ctx context.Context, enable bool) error {
	previousDesired := c.Desired.Get()

	desired := previousDesired
	desired.SystemProxy = enable

	if enable {
		desired.CoreRunning = true
	} else if !desired.Tun {
		desired.CoreRunning = false
	}

	c.fillDesiredTarget(&desired)

	c.Desired.SetAndSave(desired)

	if err := c.Supervisor.Reconcile(ctx, "system-proxy-toggle"); err != nil {

		c.Desired.SetAndSave(previousDesired)
		c.SyncState()
		return err
	}
	return nil
}

func (c *Controller) EnsureHelperReady(reason string) error {
	_, err, _ := helperReadyGroup.Do("helper-ready", func() (any, error) {
		return nil, c.ensureHelperReadySlow(reason)
	})
	return err
}

func (c *Controller) ensureHelperReadySlow(reason string) error {
	client := sys.NewHelperClient()

	if err := client.Ping(); err == nil {
		return nil
	}

	if !sys.CheckAdmin() {
		helperStatus := sys.CheckHelperService()
		// Если проверить состояние службы не удалось — НЕ выдаём это за «служба не установлена»,
		// иначе пользователю на пустом месте предлагается переустановка (и так каждый запуск).
		if helperStatus.Error != "" && !helperStatus.Installed {
			c.logWarn("Не удалось достоверно проверить службу helper: %s", helperStatus.Error)
			return ErrHelperRepairRequired
		}
		if !helperStatus.Installed {
			return ErrHelperInstallRequired
		}
		// Служба ставится с типом запуска «вручную», поэтому после перезагрузки ПК она
		// установлена, но остановлена. DACL службы даёт обычному пользователю право
		// SERVICE_START — пробуем поднять её без UAC (раньше сразу требовали админа,
		// из-за чего TUN не поднимался при автозапуске).
		if !helperStatus.Running {
			if err := sys.StartHelperService(); err != nil {
				c.logWarn("Не удалось запустить службу helper без прав администратора: %v", err)
			} else if err := sys.WaitForHelperReady(10, 300*time.Millisecond); err != nil {
				c.logWarn("Служба helper запущена, но не отвечает: %v", err)
			} else {
				c.logInfo("Служба helper запущена без прав администратора (reason: %s)", reason)
				return nil
			}
		}
		return ErrHelperRepairRequired
	}

	helperExe := filepath.Join(utils.GetAppDir(), "GoclashZHelper.exe")
	sid, err := sys.CurrentUserSID()
	if err != nil {
		return fmt.Errorf("Не удалось получить SID текущего пользователя: %w", err)
	}

	if err := sys.RecoverHelperServiceForUser(helperExe, sid); err != nil {
		return fmt.Errorf("Не удалось восстановить фоновую службу: %w", err)
	}

	c.logInfo("Служба helper восстановлена и готова (reason: %s)", reason)
	return nil
}

func (c *Controller) ToggleTunMode(ctx context.Context, enable bool) error {
	if enable {

		behavior := c.Behavior.Get()
		if behavior.ActiveConfig == "" {
			return ErrNoActiveConfig
		}

		client := sys.NewHelperClient()
		helperReachable := client.Ping() == nil

		if !helperReachable && !sys.CheckAdmin() {

			helperStatus := sys.CheckHelperService()
			if !helperStatus.Installed {
				return ErrHelperInstallRequired
			}

			return ErrHelperRepairRequired
		}

		if !helperReachable && sys.CheckAdmin() {
			if err := c.EnsureHelperReady("tun-first-install"); err != nil {
				return fmt.Errorf("Не удалось установить фоновую службу: %w", err)
			}
		}

		assetStatus, assetErr := runtimeassets.EnsureReady(ctx, runtimeassets.RequireTun, runtimeassets.RepairInvalid)
		if assetErr != nil {
			w := assetStatus.Assets[runtimeassets.AssetWintun]
			if !w.Ready {
				return fmt.Errorf("Отсутствует зависимость Wintun: %s (%s)", w.Error, w.Path)
			}
			return assetErr
		}
	}

	previousDesired := c.Desired.Get()
	desired := previousDesired
	desired.Tun = enable

	if enable {
		desired.CoreRunning = true
	} else if !desired.SystemProxy {
		desired.CoreRunning = false
	}

	c.fillDesiredTarget(&desired)
	c.Desired.SetAndSave(desired)

	if err := c.Supervisor.Reconcile(ctx, "tun-toggle"); err != nil {
		c.Desired.SetAndSave(previousDesired)
		c.SyncState()
		return err
	}
	return nil
}

func (c *Controller) RestartCore(ctx context.Context) error {
	return c.RestartCoreWithReason(ctx, "internal")
}

func (c *Controller) RestartCoreWithReason(ctx context.Context, reason string) error {
	if err := c.guardDisruptiveAction(reason); err != nil {
		c.queuePendingDisruptiveAction(reason)
		return err
	}

	c.coreLifecycleMu.Lock()
	defer c.coreLifecycleMu.Unlock()

	c.stopCoreProcessLocked()

	desired := c.Desired.Get()
	if desired.ActiveConfig == "" {
		behavior := c.Behavior.Get()
		desired.ActiveConfig = behavior.ActiveConfig
		desired.Mode = behavior.ActiveMode
	}

	needCore := desired.CoreRunning || desired.SystemProxy || desired.Tun

	if err := c.ensureCoreRunningWithDesiredState(ctx, desired); err != nil {
		c.SyncState()
		return err
	}

	if !needCore {

		c.stopCoreProcessLocked()
	}

	c.events.Emit("core-restarted", map[string]any{
		"reason": reason,
	})

	switch reason {
	case "manual", "config-switch", "subscription-update", "restore", "config-save":
		c.events.Emit("delay-cache-clear", reason)
	}

	c.SyncState()
	return nil
}

func (c *Controller) UpdateClashMode(ctx context.Context, mode string) error {
	behavior := c.Behavior.Get()
	if behavior.ActiveMode == mode {
		return nil
	}

	behavior.ActiveMode = mode
	if err := c.Behavior.SetAndSave(behavior); err != nil {
		c.events.Emit("notify-error", "Не удалось сохранить режим: "+err.Error())
		c.SyncState()
		return err
	}

	desired := c.Desired.Get()
	desired.Mode = mode
	c.fillDesiredTarget(&desired)
	c.Desired.SetAndSave(desired)

	if clash.IsRunning() {
		if err := clash.UpdateModeWithContext(ctx, mode); err != nil {
			c.SyncState()
			return fmt.Errorf("Не удалось переключить режим ядра: %v", err)
		}

		st := c.runtimeState.Get()
		if st.Running && st.Purpose == "user" {
			st.Mode = mode
			c.runtimeState.Set(st)
		}

		if mode == "global" {
			c.syncGlobalSelectionOnModeEnter(ctx, behavior.ActiveConfig)
		}
	}

	c.SyncState()
	return nil
}

func (c *Controller) RefreshAutoDelayTest(opts AutoDelayRefreshOptions) {
	c.autoTestMu.Lock()

	if c.autoTestQuit != nil {
		close(c.autoTestQuit)
		c.autoTestQuit = nil
	}

	behavior := c.Behavior.Get()
	if !behavior.AutoDelayTest {
		c.autoTestMu.Unlock()
		return
	}

	intervalMin := behavior.AutoDelayTestInterval
	if intervalMin <= 0 {
		intervalMin = 60
	}

	quit := make(chan struct{})
	c.autoTestQuit = quit
	c.autoTestMu.Unlock()

	if opts.Immediate {
		delay := autoDelayInitialDelay(opts.Reason)
		go c.runAutoDelayTestOnceDelayed(quit, opts.Reason, delay)
	}

	go func(quit <-chan struct{}, intervalMin int) {
		ticker := time.NewTicker(time.Duration(intervalMin) * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-quit:
				return

			case <-ticker.C:
				c.runAutoDelayTestOnce("scheduled")
			}
		}
	}(quit, intervalMin)
}

func autoDelayInitialDelay(reason string) time.Duration {
	switch reason {
	case "startup":
		return 7 * time.Second
	case "restore":
		return 15 * time.Second
	case "enabled":
		return 3 * time.Second
	default:
		return 5 * time.Second
	}
}

func (c *Controller) runAutoDelayTestOnceDelayed(
	quit <-chan struct{},
	reason string,
	delay time.Duration,
) {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-quit:
		return

	case <-timer.C:
		c.runAutoDelayTestOnce(reason)
	}
}

func (c *Controller) runAutoDelayTestOnce(reason string) {
	if c.ctx == nil {
		return
	}

	behavior := c.Behavior.Get()
	if !behavior.AutoDelayTest {
		return
	}

	c.logInfo("Triggering auto delay test, reason: %s", reason)
	go c.Delay.TestAllProxiesAuto(c.ctx, reason)
}

func (c *Controller) RefreshAppAutoUpdate() {
	if c.ctx == nil {
		return
	}

	behavior := c.Behavior.Get()
	if !behavior.AutoUpdate {
		return
	}

	now := time.Now().Unix()
	shouldCheck := false

	switch behavior.UpdateMethod {
	case "startup":
		shouldCheck = true

	case "scheduled":
		intervalDays := behavior.UpdateInterval
		if intervalDays <= 0 {
			intervalDays = 1
		}

		if behavior.LastUpdateCheck == 0 ||
			now-behavior.LastUpdateCheck >= int64(intervalDays)*24*3600 {
			shouldCheck = true
		}

	default:
		shouldCheck = true
	}

	if !shouldCheck {
		return
	}

	behavior.LastUpdateCheck = now
	_ = c.Behavior.SetAndSave(behavior)

	go c.AutoCheckAndDownloadAppUpdateAsync(c.ctx, c.version)
}

func (c *Controller) SaveAppBehavior(b AppBehavior) error {
	old := c.Behavior.Get()

	if err := c.Behavior.SetAndSave(b); err != nil {
		return err
	}

	next := c.Behavior.Get()

	if old.AutoDelayTest != next.AutoDelayTest ||
		old.AutoDelayTestInterval != next.AutoDelayTestInterval {
		c.RefreshAutoDelayTest(AutoDelayRefreshOptions{
			Immediate: !old.AutoDelayTest && next.AutoDelayTest,
			Reason:    "enabled",
		})
	}

	if old.AutoUpdate != next.AutoUpdate ||
		old.UpdateMethod != next.UpdateMethod ||
		old.UpdateInterval != next.UpdateInterval {
		c.RefreshAppAutoUpdate()
	}

	if old.LogLevel != next.LogLevel {
		c.StartLogStream(c.ctx)
	}

	if old.AppLogLevel != next.AppLogLevel {
		c.SyncState()
	}

	if old.ProxyTrafficOnly != next.ProxyTrafficOnly {
		c.traffic.Stop()
		c.traffic.ResetRuntimeState()
		c.events.Emit("traffic-stat-mode-changed", next.ProxyTrafficOnly)
	}

	if old.StartupWithOS != next.StartupWithOS {
		c.handleStartupWithOSChange(next.StartupWithOS)
	}

	c.SyncState()
	return nil
}

func (c *Controller) handleStartupWithOSChange(enable bool) {
	exePath, err := os.Executable()
	if err != nil {
		c.events.Emit("app-state-sync", c.GetAppState())
		return
	}

	_ = sys.DeleteStartupTask()

	if enable {
		if createTaskErr := sys.CreateStartupTask(exePath); createTaskErr != nil {

			behavior := c.Behavior.Get()
			behavior.StartupWithOS = false
			_ = c.Behavior.SetAndSave(behavior)
			c.setLastError("Не удалось настроить автозапуск: " + createTaskErr.Error())
			c.events.Emit("app-state-sync", c.GetAppState())
		}
	}
}

func (c *Controller) StopTrafficStream() {
	c.traffic.Stop()
}

func (c *Controller) ResetTrafficTotals() {
	c.traffic.ResetRuntimeState()
}

func (c *Controller) StartLogStream(ctx context.Context) {
	logLevel := c.Behavior.Get().LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	c.logs.Start(ctx, logLevel)
}

func (c *Controller) StopLogStream() {
	c.logs.Stop()
}

func (c *Controller) GetRecentLogs() []logger.LogEntry {
	return logger.AppLogs.GetAll()
}

func (c *Controller) SearchLogs(keyword string) []logger.LogEntry {
	return logger.AppLogs.Search(keyword)
}

func (c *Controller) ClearLogs() {
	logger.AppLogs.Clear()
}

func (c *Controller) GetConnections() (ConnectionsSnapshot, error) {
	return c.connections.GetSnapshot()
}

func (c *Controller) StartConnectionMonitor(ctx context.Context) {
	c.connections.Start(ctx)
}

func (c *Controller) StopConnectionMonitor() {
	c.connections.Stop()
}

func (c *Controller) SetUpdateStatus(ready bool, version string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateReady = ready
	c.newAppVersion = version
}

func (c *Controller) SetDownloadedAppUpdate(path, version string) {
	c.mu.Lock()
	changed := c.downloadedUpdatePath != path || c.downloadedUpdateVersion != version
	c.downloadedUpdatePath = path
	c.downloadedUpdateVersion = version
	c.mu.Unlock()

	if changed {
		c.SyncState()
	}
}

func (c *Controller) SelectProxyWithModeSync(
	ctx context.Context,
	profileID string,
	mode string,
	groupName string,
	proxyName string,
) error {

	if mode == "global" {

		if err := c.validateProxySelection(ctx, groupName, proxyName, true); err != nil {
			return fmt.Errorf("Предварительная проверка глобальной синхронизации не пройдена: %w", err)
		}

		if err := clash.SelectProxyWithContext(ctx, groupName, proxyName); err != nil {
			return fmt.Errorf("Не удалось переключить прокси-группу: %w", err)
		}
		if err := clash.SelectProxyWithContext(ctx, "GLOBAL", proxyName); err != nil {
			return fmt.Errorf("Не удалось синхронизировать глобальный выход: %w", err)
		}

		c.Offline.Mark(profileID, groupName, proxyName)
		c.Offline.Mark(profileID, "GLOBAL", proxyName)

	} else {

		if err := clash.SelectProxyWithContext(ctx, groupName, proxyName); err != nil {
			return err
		}
		c.Offline.Mark(profileID, groupName, proxyName)
	}

	c.SyncProxyStateOnce()
	return nil
}

func (c *Controller) validateProxySelection(ctx context.Context, groupName, nodeName string, checkGlobal bool) error {
	if nodeName == "" || nodeName == "DIRECT" || nodeName == "REJECT" {
		return fmt.Errorf("Недопустимое имя узла: %s", nodeName)
	}

	data, err := clash.GetInitialDataWithContext(ctx)
	if err != nil {
		return err
	}

	groups, ok := data["groups"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("Недопустимые данные прокси-группы")
	}

	if g, ok := groups[groupName].(map[string]interface{}); ok {
		if !proxyGroupContainsNode(g, nodeName) {
			return fmt.Errorf("Прокси-группа %s не содержит узел %s", groupName, nodeName)
		}
	} else {
		return fmt.Errorf("Прокси-группа %s не существует", groupName)
	}

	if checkGlobal {
		if g, ok := groups["GLOBAL"].(map[string]interface{}); ok {
			if !proxyGroupContainsNode(g, nodeName) {
				return fmt.Errorf("Глобальный выход (GLOBAL) не поддерживает узел %s", nodeName)
			}
		} else {
			return fmt.Errorf("Группа GLOBAL не существует, синхронизация невозможна")
		}
	}

	return nil
}

func (c *Controller) syncGlobalSelectionOnModeEnter(ctx context.Context, profileID string) {

	targetNode, exists := c.Offline.Get(profileID, "GLOBAL")

	if !exists {

		selections := c.Offline.Snapshot(profileID)
		for gName, node := range selections {
			if gName == "GLOBAL" {
				continue
			}

			if err := c.selectGlobalProxyIfValid(ctx, node); err == nil {
				c.Offline.Mark(profileID, "GLOBAL", node)
				return
			}
		}
		return
	}

	if err := c.selectGlobalProxyIfValid(ctx, targetNode); err != nil {
		c.logError("Не удалось выполнить синхронизацию при входе в глобальный режим: %v", err)
	}
}

func (c *Controller) SelectOfflineProxyWithModeSync(
	profileID string,
	mode string,
	groupName string,
	proxyName string,
) {
	c.Offline.Mark(profileID, groupName, proxyName)

	if mode == "global" {
		c.Offline.Mark(profileID, "GLOBAL", proxyName)
	}
}

func (c *Controller) selectGlobalProxyIfValid(ctx context.Context, proxyName string) error {
	if proxyName == "" || proxyName == "DIRECT" || proxyName == "REJECT" {
		return fmt.Errorf("Недопустимый узел глобального выхода: %s", proxyName)
	}

	data, err := clash.GetInitialDataWithContext(ctx)
	if err != nil {
		return err
	}

	groups, ok := data["groups"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("Недопустимые данные прокси-групп ядра")
	}

	raw, ok := groups["GLOBAL"]
	if !ok {
		return fmt.Errorf("Группа GLOBAL не существует")
	}

	globalGroup, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Недопустимые данные группы GLOBAL")
	}

	if !proxyGroupContainsNode(globalGroup, proxyName) {
		return fmt.Errorf("GLOBAL не содержит узел: %s", proxyName)
	}

	return clash.SelectProxyWithContext(ctx, "GLOBAL", proxyName)
}

func (c *Controller) applyStoredProxySelections(ctx context.Context, profileID string) {
	selected := c.Offline.Snapshot(profileID)
	if len(selected) == 0 {
		return
	}

	data, err := clash.GetInitialDataWithContext(ctx)
	if err != nil {
		c.logError("Не удалось прочитать прокси-группы ядра, воспроизведение выбора узлов пропущено: %v", err)
		return
	}

	groups, ok := data["groups"].(map[string]interface{})
	if !ok {
		return
	}

	for groupName, nodeName := range selected {
		if groupName == "GLOBAL" {
			continue
		}

		raw, ok := groups[groupName]
		if !ok {
			continue
		}

		group, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		groupType, _ := group["type"].(string)
		if !isUserSelectableProxyGroupType(groupType) {
			continue
		}

		if !proxyGroupContainsNode(group, nodeName) {
			continue
		}

		_ = clash.SelectProxyWithContext(ctx, groupName, nodeName)
	}

	behavior := c.Behavior.Get()
	if behavior.ActiveMode == "global" {
		if globalNode, ok := selected["GLOBAL"]; ok && globalNode != "" {
			if err := c.selectGlobalProxyIfValid(ctx, globalNode); err != nil {
				c.logError("Не удалось воспроизвести выбор выхода GLOBAL: %v", err)
			}
		}
	}
}

func (c *Controller) SyncProxyStateOnce() {
	if c.proxyState != nil {
		c.proxyState.SyncOnce()
	}
}

func isUserSelectableProxyGroupType(t string) bool {
	switch t {
	case "Selector", "Fallback":
		return true
	default:
		return false
	}
}

func proxyGroupContainsNode(group map[string]interface{}, nodeName string) bool {
	all, ok := group["all"].([]interface{})
	if !ok {
		return false
	}
	for _, v := range all {
		if s, ok := v.(string); ok && s == nodeName {
			return true
		}
	}
	return false
}

func (c *Controller) GetDiagnosticInfo() DiagnosticInfo {
	return GetDiagnosticInfo()
}

func (c *Controller) ExportDiagnostics() error {
	path, err := runtime.SaveFileDialog(c.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "goclashz_diagnostics.json",
		Title:           "Экспорт диагностики",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return err
	}
	return ExportDiagnosticsToFile(path)
}
