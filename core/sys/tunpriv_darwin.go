//go:build darwin

package sys

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"goclashz/core/logger"
	"goclashz/core/utils"
)

func corePathForTun() string {
	return filepath.Join(utils.GetCoreBinDir(), "clash")
}

// hasRootSetuid — бинарь принадлежит root и помечен setuid: utun поднимется без sudo.
func hasRootSetuid(st os.FileInfo) bool {
	if st.Mode()&os.ModeSetuid == 0 {
		return false
	}
	raw, ok := st.Sys().(*syscall.Stat_t)
	return ok && raw.Uid == 0
}

// EnsureTunPrivilege: utun требует root → один раз root:wheel+setuid на ядро через системный диалог пароля.
func EnsureTunPrivilege() error {
	path := corePathForTun()
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("бинарь ядра не найден: %s", path)
	}

	if os.Geteuid() == 0 || hasRootSetuid(st) {
		return nil
	}

	if err := grantSetuidWithAdminPrompt(path); err != nil {
		return err
	}

	st, err = os.Stat(path)
	if err != nil || !hasRootSetuid(st) {
		return errors.New("не удалось выдать ядру права root (setuid не применился). Либо пользуйтесь режимом системного прокси — он работает без прав администратора")
	}
	logger.Infof("ядру выданы права root для TUN (setuid): %s", path)
	return nil
}

// диалог пароля администратора; отмена = код -128
func grantSetuidWithAdminPrompt(path string) error {
	// путь экранируем для shell внутри AppleScript-строки
	shellCmd := fmt.Sprintf("chown root:wheel '%s' && chmod u+s '%s'", path, path)
	script := fmt.Sprintf(
		`do shell script %q with prompt "Misetanibox: режиму TUN нужны права администратора, чтобы ядро могло создать сетевой интерфейс." with administrator privileges`,
		shellCmd,
	)

	cmd := exec.Command("/usr/bin/osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	msg := strings.TrimSpace(string(out))
	if strings.Contains(msg, "-128") || strings.Contains(strings.ToLower(msg), "user canceled") {
		return errors.New("выдача прав отменена — без них режим TUN недоступен. Либо пользуйтесь режимом системного прокси, он работает без прав администратора")
	}
	logger.Warnf("osascript setuid: %v: %s", err, msg)
	return fmt.Errorf("не удалось выдать ядру права root: %s", firstLine(msg))
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	if s == "" {
		return "ошибка osascript"
	}
	return s
}
