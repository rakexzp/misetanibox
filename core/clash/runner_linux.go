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

// ExitEvent 描述内核退出事件
type ExitEvent struct {
	Intentional bool
	Message     string
}

// OnExitFunc 是内核异常退出时的回调函数类型
type OnExitFunc func(event ExitEvent)

var (
	mu                sync.Mutex
	clashCmd          *exec.Cmd
	isRunning         atomic.Bool
	isIntentionalStop atomic.Bool
	processExitCh     chan struct{}
	onExitCallback    OnExitFunc
)

// SetOnExitCallback 注册内核异常退出的回调（由 appcore 层在启动时设置）
func SetOnExitCallback(fn OnExitFunc) {
	mu.Lock()
	defer mu.Unlock()
	onExitCallback = fn
}

// killProcessIfClash 安全杀进程：验证 PID 对应进程确为目标执行文件，防止 PID 复用误杀
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

// cleanupResidualClashProcess 清理残余内核进程
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

// Start 启动内核
// 在 Linux 上没有 helper：TUN 模式同样直接启动进程。
// TUN 需要 root/CAP_NET_ADMIN —— 若权限不足，内核自身会在日志中报错。
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
		// Сначала мягко (SIGTERM), даём внутренним обработчикам корректно свернуться
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

// StartedViaHelper — на Linux helper-службы нет, всегда false
func StartedViaHelper() bool {
	return false
}
