//go:build darwin

package sys

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"goclashz/core/utils"
)

func corePathForTun() string {
	return filepath.Join(utils.GetCoreBinDir(), "clash")
}

// EnsureTunPrivilege проверяет, что ядру хватит прав для создания TUN-интерфейса.
//
// В macOS нет аналога capabilities (setcap): создание utun-интерфейса требует
// именно root. Варианты дать эти права:
//   - запустить приложение из-под root (не рекомендуется);
//   - выставить владельца root и setuid-бит на бинарь ядра (делается один раз,
//     требует ввода пароля администратора).
//
// Автоматически setuid не выставляем: это заметное изменение прав в системе,
// поэтому просим пользователя подтвердить команду осознанно.
func EnsureTunPrivilege() error {
	path := corePathForTun()
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("бинарь ядра не найден: %s", path)
	}

	if os.Geteuid() == 0 {
		return nil
	}

	// ядро помечено setuid и принадлежит root — прав достаточно
	if st.Mode()&os.ModeSetuid != 0 {
		if raw, ok := st.Sys().(*syscall.Stat_t); ok && raw.Uid == 0 {
			return nil
		}
	}

	return fmt.Errorf(
		"режиму TUN нужны права root для ядра. Выполните один раз в терминале:\n\n"+
			"sudo chown root:wheel %s && sudo chmod u+s %s\n\n"+
			"после этого перезапустите приложение. Либо пользуйтесь режимом системного прокси — он работает без прав администратора.",
		path, path,
	)
}
