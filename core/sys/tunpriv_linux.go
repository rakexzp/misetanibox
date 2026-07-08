//go:build !windows

package sys

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"goclashz/core/utils"
)

func corePathForTun() string {
	return filepath.Join(utils.GetCoreBinDir(), "clash")
}

func hasNetAdminCap(path string) bool {
	out, err := exec.Command("getcap", path).Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "cap_net_admin")
}

func EnsureTunPrivilege() error {
	path := corePathForTun()
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("бинарь ядра не найден: %s", path)
	}

	if os.Geteuid() == 0 {
		return nil
	}

	if hasNetAdminCap(path) {
		return nil
	}

	caps := "cap_net_admin,cap_net_raw+ep"

	if pk, err := exec.LookPath("pkexec"); err == nil {
		cmd := exec.Command(pk, "setcap", caps, path)
		if err := cmd.Run(); err == nil {
			if hasNetAdminCap(path) {
				return nil
			}
		}
	}

	return fmt.Errorf(
		"режиму TUN нужны права ядру. Выполните один раз в терминале:\n\nsudo setcap %s %s\n\nлибо установите polkit (pkexec) для графического запроса.",
		caps, path,
	)
}
