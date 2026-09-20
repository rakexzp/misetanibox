package clash

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goclashz/core/utils"
)

// TunCapabilities describes parser support, not a working TUN or network test.
type TunCapabilities struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

var tunProbeCache struct {
	sync.Mutex
	identity string
	result   TunCapabilities
}

func GetTunCapabilities() TunCapabilities {
	coreBinaryMu.Lock()
	defer coreBinaryMu.Unlock()
	return probeTunCapabilities(context.Background(), coreExePath(), 3*time.Second)
}

func probeTunCapabilities(ctx context.Context, path string, timeout time.Duration) TunCapabilities {
	unknown := func(reason string) TunCapabilities { return TunCapabilities{"unknown", reason} }
	path, err := filepath.Abs(path)
	if err != nil {
		return unknown(err.Error())
	}
	f, err := os.Open(path)
	if err != nil {
		return unknown("Ядро недоступно: " + err.Error())
	}
	hash := sha256.New()
	_, err = io.Copy(hash, f)
	f.Close()
	if err != nil {
		return unknown(err.Error())
	}
	identity := fmt.Sprintf("%s:%x", path, hash.Sum(nil))
	tunProbeCache.Lock()
	defer tunProbeCache.Unlock()
	if tunProbeCache.identity == identity {
		return tunProbeCache.result
	}
	dir, err := os.MkdirTemp("", "misetanibox-tun-probe-")
	if err != nil {
		return unknown(err.Error())
	}
	defer os.RemoveAll(dir)
	probe := func(stack string) (string, error) {
		probeCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		config := "mode: direct\nlog-level: silent\ngeodata-mode: false\ndns:\n  enable: false\ntun:\n  enable: false\n  stack: " + stack + "\nrules: []\n"
		// --config bypasses config.Init (and its possible geodata downloads).
		cmd := exec.CommandContext(probeCtx, path, "-t", "-config", base64.StdEncoding.EncodeToString([]byte(config)), "-d", dir)
		cmd.Dir = dir
		for _, entry := range os.Environ() {
			key := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
			if !strings.HasPrefix(key, "CLASH_") && !strings.HasPrefix(key, "MIHOMO_") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		utils.HideCommandWindow(cmd, 0)
		cmd.WaitDelay = 100 * time.Millisecond
		out, err := cmd.CombinedOutput()
		if probeCtx.Err() != nil {
			return "", probeCtx.Err()
		}
		return strings.ToLower(string(out)), err
	}
	if _, err := probe("gvisor"); err != nil {
		return unknown("Проверка gVisor не выполнена: " + err.Error())
	}
	invalidOut, invalidErr := probe("misetanibox-invalid-stack")
	if invalidErr == nil || !strings.Contains(invalidOut, "invalid tun stack") {
		return unknown("Ядро не подтвердило проверку поля tun.stack")
	}
	out, err := probe("mips")
	result := TunCapabilities{"supported", "Ядро принимает MIPS (-t); работа TUN не проверена"}
	if err != nil {
		if !strings.Contains(out, "invalid tun stack") {
			return unknown("Проверка MIPS не выполнена: " + err.Error())
		}
		result = TunCapabilities{"unsupported", "Установленное ядро не поддерживает MIPS; выберите gVisor"}
	}
	tunProbeCache.identity, tunProbeCache.result = identity, result
	return result
}

func ValidateTunConfig(cfg *TunConfig) error {
	if cfg == nil {
		return fmt.Errorf("настройки TUN отсутствуют")
	}
	switch cfg.Stack {
	case "gvisor", "system", "mixed":
		return nil
	case "mips":
		capability := GetTunCapabilities()
		if capability.Status != "supported" {
			return fmt.Errorf("MIPS: %s", capability.Reason)
		}
		return nil
	default:
		return fmt.Errorf("неизвестный стек TUN: %q", cfg.Stack)
	}
}

func validateReplacementTun(ctx context.Context, path string) error {
	cfg, err := GetTunConfig()
	if err != nil {
		return err
	}
	if cfg.Stack != "mips" {
		return nil
	}
	capability := probeTunCapabilities(ctx, path, 3*time.Second)
	if capability.Status != "supported" {
		return fmt.Errorf("замена ядра заблокирована при выбранном MIPS: %s", capability.Reason)
	}
	return nil
}

// Smart switches are blocked before stopping the live core. Users can explicitly
// select gVisor first; never silently switch the Smart branch to stock.
func CheckSmartCoreSwitch() error {
	cfg, err := GetTunConfig()
	if err != nil {
		return err
	}
	if cfg.Stack == "mips" {
		return fmt.Errorf("перед переключением Smart/stock выберите gVisor: поддержка MIPS новым ядром не подтверждена")
	}
	return nil
}

func resolveTunRuntime(cfg TunConfig, capability TunCapabilities) (TunConfig, string) {
	if cfg.Stack == "mips" && capability.Status != "supported" {
		cfg.Stack = "gvisor"
		return cfg, "MIPS сохранён, но runtime использует gVisor: " + capability.Reason
	}
	return cfg, ""
}
