//go:build windows

package clash

import (
	"context"
	"errors"
	"fmt"
	"goclashz/core/logger"
	"goclashz/core/sys"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"goclashz/core/utils"

	"golang.org/x/sys/windows"
)

type ExitEvent struct {
	Intentional bool
	Message     string
}

type OnExitFunc func(event ExitEvent)

var (
	mu                sync.Mutex
	clashCmd          *exec.Cmd
	isRunning         atomic.Bool
	isIntentionalStop atomic.Bool
	processExitCh     chan struct{}
	onExitCallback    OnExitFunc
	startedViaHelper  atomic.Bool
	// поколение запуска через Helper — чтобы наблюдатель от прошлого запуска не срабатывал
	helperCoreGen atomic.Uint64
)

func SetOnExitCallback(fn OnExitFunc) {
	mu.Lock()
	defer mu.Unlock()
	onExitCallback = fn
}

func killProcessIfClash(pid int, expectedExeName string) {
	hProcess, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(hProcess)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	err = windows.QueryFullProcessImageName(hProcess, 0, &buf[0], &size)
	if err == nil {
		imageName := windows.UTF16ToString(buf[:size])
		targetSuffix := "\\" + strings.ToLower(expectedExeName)
		if strings.HasSuffix(strings.ToLower(imageName), targetSuffix) {
			_ = windows.TerminateProcess(hProcess, 1)
		}
	}
}

func cleanupResidualClashProcess(pidFile string, expectedExeName string) {
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		return
	}

	pidStr := strings.TrimSpace(string(pidData))
	if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
		killProcessIfClash(pid, expectedExeName)
		time.Sleep(300 * time.Millisecond)
	}

	_ = os.Remove(pidFile)
}

func Start(ctx context.Context, tun bool) error {
	mu.Lock()
	defer mu.Unlock()

	if isRunning.Load() {
		return nil
	}

	binDir := utils.GetCoreBinDir()
	exePath := filepath.Join(binDir, coreExeName())
	targetExeName := filepath.Base(exePath)

	pidFile := utils.GetPidFilePath()
	runtimeConfig := utils.GetRuntimeConfigPath()

	cleanupResidualClashProcess(pidFile, targetExeName)

	if err := PrepareEnv(ctx); err != nil {
		return err
	}

	if tun {

		return startCoreViaHelper(ctx, exePath, binDir, runtimeConfig, pidFile)
	}

	return startCoreDirect(ctx, exePath, binDir, runtimeConfig, pidFile)
}

func startCoreViaHelper(_ context.Context, exePath, binDir, runtimeConfig, _ string) error {
	client := sys.NewHelperClient()

	if err := client.StartCore(sys.StartCoreParams{
		CorePath:      exePath,
		BinDir:        binDir,
		RuntimeConfig: runtimeConfig,
		Args:          []string{"-d", binDir, "-f", runtimeConfig},
	}); err != nil {
		return fmt.Errorf("не удалось запустить ядро через Helper (режим TUN требует службу Helper): %w", err)
	}

	startedViaHelper.Store(true)
	isRunning.Store(true)
	isIntentionalStop.Store(false)
	logger.Infof("ядро запущено через службу Helper (TUN)")

	// При запуске через Helper процесс ядра нам не принадлежит, поэтому cmd.Wait() недоступен.
	// Без наблюдателя падение ядра оставалось незамеченным: isRunning навсегда true и
	// интерфейс показывал «подключено» при мёртвом туннеле. Опрашиваем статус у службы.
	watchHelperCore(helperCoreGen.Add(1))
	return nil
}

func watchHelperCore(gen uint64) {
	go func() {
		client := sys.NewHelperClient()
		const interval = 3 * time.Second
		// даём ядру время на инициализацию TUN-адаптера
		time.Sleep(interval)

		misses := 0
		for {
			if helperCoreGen.Load() != gen || !isRunning.Load() || isIntentionalStop.Load() {
				return
			}
			if st, err := client.CoreStatus(); err != nil || !st.Running {
				misses++
			} else {
				misses = 0
			}
			// два промаха подряд, чтобы не среагировать на разовый сбой опроса
			if misses >= 2 {
				break
			}
			time.Sleep(interval)
		}

		if helperCoreGen.Load() != gen || isIntentionalStop.Load() {
			return
		}

		mu.Lock()
		if helperCoreGen.Load() != gen {
			mu.Unlock()
			return
		}
		isRunning.Store(false)
		startedViaHelper.Store(false)
		cb := onExitCallback
		mu.Unlock()

		logger.Warnf("ядро, запущенное через службу Helper, больше не работает")
		if cb != nil {
			cb(ExitEvent{Intentional: false, Message: "ядро аварийно завершилось (режим TUN)"})
		}
	}()
}

func startCoreDirect(ctx context.Context, exePath, binDir, runtimeConfig, pidFile string) error {
	startedViaHelper.Store(false)

	cmd, err := startCoreProcessWithRetry(ctx, exePath, binDir, runtimeConfig)
	if err != nil {
		if isAccessDenied(err) {
			return fmt.Errorf(
				"не удалось запустить ядро: Windows отказала в выполнении %s. Возможные причины: ядро лежит в записываемом каталоге data, файл ещё сканируется антивирусом или заблокирован политикой прав: %w",
				exePath, err,
			)
		}
		return fmt.Errorf("не удалось запустить ядро: %w", err)
	}

	clashCmd = cmd
	isRunning.Store(true)
	isIntentionalStop.Store(false)
	processExitCh = make(chan struct{})
	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644)

	localExitCh := processExitCh

	go func(c *exec.Cmd, ch chan struct{}) {
		c.Wait()

		mu.Lock()
		if clashCmd == c {
			isRunning.Store(false)
			clashCmd = nil
			os.Remove(pidFile)
		}
		cb := onExitCallback
		mu.Unlock()

		close(ch)

		if !isIntentionalStop.Load() && cb != nil {
			cb(ExitEvent{Intentional: false, Message: "ядро аварийно завершилось"})
		}
	}(cmd, localExitCh)

	return nil
}

func Stop() error {
	mu.Lock()
	isIntentionalStop.Store(true)

	if startedViaHelper.Load() {
		mu.Unlock()
		client := sys.NewHelperClient()
		if err := client.StopCore(); err != nil {
			logger.Warnf("не удалось остановить ядро через Helper: %v", err)
		}
		startedViaHelper.Store(false)
		isRunning.Store(false)
		return nil
	}

	var exitCh chan struct{}
	var proc *os.Process
	var pid int

	if clashCmd != nil && clashCmd.Process != nil {
		proc = clashCmd.Process
		pid = clashCmd.Process.Pid
		exitCh = processExitCh
	}

	targetExeName := filepath.Base(filepath.Join(utils.GetCoreBinDir(), coreExeName()))
	mu.Unlock()

	if proc != nil {
		if err := proc.Kill(); err != nil {
			logger.Errorf("не удалось остановить процесс ядра: %v", err)
			if pid > 0 {
				killProcessIfClash(pid, targetExeName)
			}
		}
	}

	if exitCh != nil {
		select {
		case <-exitCh:
		case <-time.After(3 * time.Second):
			if pid > 0 {
				killProcessIfClash(pid, targetExeName)
			}
		}
	}

	isRunning.Store(false)
	return nil
}

func IsRunning() bool {
	return isRunning.Load()
}

func StartedViaHelper() bool {
	return startedViaHelper.Load()
}

func startCoreProcessWithRetry(ctx context.Context, exePath, binDir, runtimeConfig string) (*exec.Cmd, error) {
	var lastErr error

	for i := 0; i < 8; i++ {
		cmd := exec.CommandContext(ctx, exePath, "-d", binDir, "-f", runtimeConfig)
		cmd.Dir = binDir
		utils.HideCommandWindow(cmd, 0)

		err := cmd.Start()
		if err == nil {
			return cmd, nil
		}

		lastErr = err

		if !isAccessDenied(err) {
			return nil, err
		}

		time.Sleep(time.Duration(250+i*250) * time.Millisecond)
	}

	return nil, fmt.Errorf("система отказала в запуске ядра: возможно, файл ещё сканируется антивирусом или заблокирован политикой каталога: %w", lastErr)
}

func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, os.ErrPermission) {
		return true
	}

	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return errors.Is(pathErr.Err, windows.ERROR_ACCESS_DENIED)
	}

	return strings.Contains(strings.ToLower(err.Error()), "access is denied")
}
