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

// corePathForTun возвращает путь к бинарю ядра (на Linux — без .exe).
func corePathForTun() string {
	return filepath.Join(utils.GetCoreBinDir(), "clash")
}

// hasNetAdminCap проверяет, есть ли у бинаря ядра CAP_NET_ADMIN (через getcap).
func hasNetAdminCap(path string) bool {
	out, err := exec.Command("getcap", path).Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "cap_net_admin")
}

// EnsureTunPrivilege выдаёт бинарю ядра права CAP_NET_ADMIN+CAP_NET_RAW, чтобы оно могло
// создавать TUN-устройство и править маршруты без запуска всего приложения от root.
// Если права уже есть — ничего не делает. Иначе запрашивает их через pkexec (графический
// диалог polkit) — разово. root не нужен, если запущены root'ом.
func EnsureTunPrivilege() error {
	path := corePathForTun()
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("бинарь ядра не найден: %s", path)
	}

	// уже root — ядро и так сможет поднять TUN
	if os.Geteuid() == 0 {
		return nil
	}

	if hasNetAdminCap(path) {
		return nil
	}

	caps := "cap_net_admin,cap_net_raw+ep"

	// пробуем через pkexec (polkit-диалог с запросом пароля админа)
	if pk, err := exec.LookPath("pkexec"); err == nil {
		cmd := exec.Command(pk, "setcap", caps, path)
		if err := cmd.Run(); err == nil {
			if hasNetAdminCap(path) {
				return nil
			}
		}
	}

	// pkexec недоступен/отклонён — понятная инструкция пользователю
	return fmt.Errorf(
		"режиму TUN нужны права ядру. Выполните один раз в терминале:\n\nsudo setcap %s %s\n\nлибо установите polkit (pkexec) для графического запроса.",
		caps, path,
	)
}
