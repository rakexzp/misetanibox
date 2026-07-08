//go:build !windows

package clash

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"goclashz/core/logger"
	"goclashz/core/utils"
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
)

func SetOnExitCallback(fn OnExitFunc) {
	mu.Lock()
	defer mu.Unlock()
	onExitCallback = fn
}

func killProcessIfClash(pid int, expectedExeName string) {
	exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return
	}
	if filepath.Base(exePath) != expectedExeName {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
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
	exePath := filepath.Join(binDir, "clash")
	targetExeName := filepath.Base(exePath)

	pidFile := utils.GetPidFilePath()
	runtimeConfig := utils.GetRuntimeConfigPath()

	cleanupResidualClashProcess(pidFile, targetExeName)

	if err := PrepareEnv(ctx); err != nil {
		return err
	}

	if tun && os.Geteuid() != 0 {
		logger.Warnf("режим TUN запрошен без прав root/CAP_NET_ADMIN — ядро может не поднять интерфейс")
	}

	cmd := exec.CommandContext(ctx, exePath, "-d", binDir, "-f", runtimeConfig)
	cmd.Dir = binDir

	if err := cmd.Start(); err != nil {
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

	var exitCh chan struct{}
	var proc *os.Process
	var pid int

	if clashCmd != nil && clashCmd.Process != nil {
		proc = clashCmd.Process
		pid = clashCmd.Process.Pid
		exitCh = processExitCh
	}

	targetExeName := filepath.Base(filepath.Join(utils.GetCoreBinDir(), "clash"))
	mu.Unlock()

	if proc != nil {

		if err := proc.Signal(syscall.SIGTERM); err != nil {
			logger.Errorf("не удалось остановить процесс ядра: %v", err)
			_ = proc.Kill()
		}
	}

	if exitCh != nil {
		select {
		case <-exitCh:
		case <-time.After(3 * time.Second):
			if proc != nil {
				_ = proc.Kill()
			}
			select {
			case <-exitCh:
			case <-time.After(2 * time.Second):
				if pid > 0 {
					killProcessIfClash(pid, targetExeName)
				}
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
	return false
}
