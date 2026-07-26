//go:build windows

package sys

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// connectSCMLimited подключается к диспетчеру служб с минимальными правами.
//
// ВАЖНО: mgr.Connect() из x/sys открывает SCM с SC_MANAGER_ALL_ACCESS, что требует прав
// администратора. Приложение из автозапуска работает БЕЗ элевации, поэтому такой вызов падал
// с ACCESS_DENIED, проверка «установлена ли служба» возвращала ошибку, а вызывающий код считал
// это за «служба не установлена» — и после каждой перезагрузки просил установить её заново.
// Для проверки состояния и запуска/остановки достаточно SC_MANAGER_CONNECT, доступного всем.
func connectSCMLimited() (*mgr.Mgr, error) {
	handle, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, fmt.Errorf("SCM connect failed: %w", err)
	}
	return &mgr.Mgr{Handle: handle}, nil
}

func openServiceWithAccess(m *mgr.Mgr, name string, access uint32) (*mgr.Service, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}

	handle, err := windows.OpenService(m.Handle, namePtr, access)
	if err != nil {
		return nil, err
	}

	return &mgr.Service{
		Name:   name,
		Handle: handle,
	}, nil
}

func isServiceInstalledSCM(name string) (bool, error) {
	m, err := connectSCMLimited()
	if err != nil {
		return false, fmt.Errorf("SCM connect failed: %w", err)
	}
	defer m.Disconnect()

	s, err := openServiceWithAccess(m, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return false, nil
		}
		return false, fmt.Errorf("open service failed: %w", err)
	}
	defer s.Close()
	return true, nil
}

func isServiceRunningSCM(name string) (bool, error) {
	m, err := connectSCMLimited()
	if err != nil {
		return false, fmt.Errorf("SCM connect failed: %w", err)
	}
	defer m.Disconnect()

	s, err := openServiceWithAccess(m, name, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return false, nil
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return false, fmt.Errorf("query service status failed: %w", err)
	}

	return status.State == svc.Running, nil
}

func installOrUpdateServiceSCM(name, exePath, description string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("SCM connect failed: %w", err)
	}
	defer m.Disconnect()

	s, err := m.CreateService(name, exePath, mgr.Config{
		DisplayName:  HelperDisplayName,
		Description:  description,
		StartType:    mgr.StartAutomatic,
		ErrorControl: mgr.ErrorNormal,
	})

	if err != nil {
		if !errors.Is(err, windows.ERROR_SERVICE_EXISTS) {
			return fmt.Errorf("create service failed: %w", err)
		}

		s, err = openServiceWithAccess(
			m,
			name,
			windows.SERVICE_QUERY_CONFIG|windows.SERVICE_CHANGE_CONFIG,
		)
		if err != nil {
			return fmt.Errorf("open existing service failed: %w", err)
		}
	}
	defer s.Close()

	cfg, err := s.Config()
	if err != nil {
		return fmt.Errorf("query service config failed: %w", err)
	}

	changed := false

	if cfg.BinaryPathName != exePath {
		cfg.BinaryPathName = exePath
		changed = true
	}
	if cfg.DisplayName != HelperDisplayName {
		cfg.DisplayName = HelperDisplayName
		changed = true
	}
	if cfg.Description != description {
		cfg.Description = description
		changed = true
	}
	if cfg.StartType != mgr.StartAutomatic {
		cfg.StartType = mgr.StartAutomatic
		changed = true
	}
	if cfg.ErrorControl != mgr.ErrorNormal {
		cfg.ErrorControl = mgr.ErrorNormal
		changed = true
	}

	if changed {
		if err := s.UpdateConfig(cfg); err != nil {
			return fmt.Errorf("update service config failed: %w", err)
		}
	}

	return nil
}

func uninstallServiceSCM(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("SCM connect failed: %w", err)
	}
	defer m.Disconnect()

	s, err := openServiceWithAccess(m, name, windows.SERVICE_QUERY_STATUS|windows.SERVICE_STOP|windows.DELETE)
	if err != nil {
		return fmt.Errorf("open service failed: %w", err)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service failed: %w", err)
	}

	return nil
}

func startServiceSCM(name string) error {
	m, err := connectSCMLimited()
	if err != nil {
		return fmt.Errorf("SCM connect failed: %w", err)
	}
	defer m.Disconnect()

	s, err := openServiceWithAccess(m, name, windows.SERVICE_QUERY_STATUS|windows.SERVICE_START|windows.SERVICE_INTERROGATE)
	if err != nil {
		return fmt.Errorf("open service failed: %w", err)
	}
	defer s.Close()

	status, err := s.Query()
	if err == nil && status.State == svc.Running {
		return nil
	}

	if err := s.Start(); err != nil {
		if err == windows.ERROR_SERVICE_ALREADY_RUNNING {
			return nil
		}
		return fmt.Errorf("start service failed: %w", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := s.Query()
		if err != nil {
			return fmt.Errorf("query service status failed: %w", err)
		}
		if status.State == svc.Running {
			return nil
		}
		if status.State != svc.StartPending && status.State != svc.ContinuePending {
			return fmt.Errorf("service entered unexpected state: %v", status.State)
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("service did not start within timeout")
}

func stopServiceSCM(name string) error {
	m, err := connectSCMLimited()
	if err != nil {
		return fmt.Errorf("SCM connect failed: %w", err)
	}
	defer m.Disconnect()

	s, err := openServiceWithAccess(m, name, windows.SERVICE_QUERY_STATUS|windows.SERVICE_STOP|windows.SERVICE_INTERROGATE)
	if err != nil {
		return fmt.Errorf("open service failed: %w", err)
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return fmt.Errorf("query service status failed: %w", err)
	}

	if status.State == svc.Stopped {
		return nil
	}

	if _, err := s.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop service failed: %w", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, err := s.Query()
		if err != nil {
			return nil
		}
		if status.State == svc.Stopped {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("service did not stop within timeout")
}

func grantServiceControlToUser(serviceName string, userSID string) error {
	adminRights := "CCDCLCSWRPWPDTLOCRSDRCWDWO"
	userRights := "LCRPWPLOCRRC"

	sddl := fmt.Sprintf(
		"D:(A;;%s;;;SY)(A;;%s;;;BA)(A;;%s;;;%s)",
		adminRights,
		adminRights,
		userRights,
		userSID,
	)
	cmd := exec.Command("sc", "sdset", serviceName, sddl)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set service DACL failed: %w, output: %s", err, string(output))
	}
	return nil
}
