//go:build windows

package sys

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Microsoft/go-winio"
	"goclashz/core/logger"
	"golang.org/x/sys/windows/registry"
)

const (
	helperConnectTimeout = 150 * time.Millisecond
)

func getMethodTimeout(method string) time.Duration {
	switch method {
	case "ping":
		return 200 * time.Millisecond
	case "core-status":
		return 500 * time.Millisecond
	case "start-core", "stop-core":
		return 3 * time.Second
	default:
		return 10 * time.Second
	}
}

type HelperClient struct {
	pipeName string
}

func NewHelperClient() *HelperClient {
	return &HelperClient{
		pipeName: GetHelperPipeName(),
	}
}

func (c *HelperClient) Ping() error {
	resp, err := c.sendRequest("ping", nil)
	if err != nil {
		return fmt.Errorf("helper ping failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("helper ping error: %s", resp.Error)
	}
	return nil
}

func (c *HelperClient) Shutdown() error {
	resp, err := c.sendRequest("shutdown", nil)
	if err != nil {
		return fmt.Errorf("helper shutdown failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("helper shutdown error: %s", resp.Error)
	}
	return nil
}

func (c *HelperClient) StartCore(params StartCoreParams) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	resp, err := c.sendRequest("start-core", data)
	if err != nil {
		return fmt.Errorf("helper start-core failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("helper start-core error: %s", resp.Error)
	}
	return nil
}

func (c *HelperClient) StopCore() error {
	resp, err := c.sendRequest("stop-core", nil)
	if err != nil {
		return fmt.Errorf("helper stop-core failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("helper stop-core error: %s", resp.Error)
	}
	return nil
}

func (c *HelperClient) CoreStatus() (CoreStatusData, error) {
	var status CoreStatusData
	resp, err := c.sendRequest("core-status", nil)
	if err != nil {
		return status, fmt.Errorf("helper core-status failed: %w", err)
	}
	if !resp.OK {
		return status, fmt.Errorf("helper core-status error: %s", resp.Error)
	}
	if err := json.Unmarshal(resp.Data, &status); err != nil {
		return status, fmt.Errorf("helper core-status decode failed: %w", err)
	}
	return status, nil
}

func (c *HelperClient) RepairPermission(dataDir string) error {
	data, _ := json.Marshal(map[string]string{"dataDir": dataDir})
	resp, err := c.sendRequest("repair-permission", data)
	if err != nil {
		return fmt.Errorf("helper repair-permission failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("helper repair-permission error: %s", resp.Error)
	}
	return nil
}

func (c *HelperClient) ReplaceCoreFile(params ReplaceCoreFileParams) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	resp, err := c.sendRequest("replace-core-file", data)
	if err != nil {
		return fmt.Errorf("helper replace-core-file failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("helper replace-core-file error: %s", resp.Error)
	}
	return nil
}

func (c *HelperClient) InstallWintun(source, target string) error {
	data, err := json.Marshal(InstallWintunParams{Source: source, Target: target})
	if err != nil {
		return err
	}
	resp, err := c.sendRequest("install-wintun", data)
	if err != nil {
		return fmt.Errorf("helper install-wintun failed: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("helper install-wintun error: %s", resp.Error)
	}
	return nil
}

func (c *HelperClient) sendRequest(method string, params json.RawMessage) (*HelperResponse, error) {
	timeout := helperConnectTimeout
	conn, err := winio.DialPipe(c.pipeName, &timeout)
	if err != nil {
		return nil, fmt.Errorf("connect to helper pipe failed: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(getMethodTimeout(method))); err != nil {
		return nil, fmt.Errorf("set deadline failed: %w", err)
	}

	req := HelperRequest{
		Method: method,
		Params: params,
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("encode request failed: %w", err)
	}

	decoder := json.NewDecoder(conn)
	var resp HelperResponse
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	return &resp, nil
}

func CheckHelperService() HelperStatusData {
	status := HelperStatusData{}

	installed, err := isServiceInstalled(HelperServiceName)
	if err != nil {
		status.Error = fmt.Sprintf("не удалось проверить регистрацию службы: %v", err)
		return status
	}
	status.Installed = installed

	if !installed {
		return status
	}

	running, err := isServiceRunning(HelperServiceName)
	if err != nil {
		status.Error = fmt.Sprintf("не удалось проверить состояние службы: %v", err)
		return status
	}
	status.Running = running

	if !running {
		return status
	}

	client := NewHelperClient()
	if err := client.Ping(); err != nil {
		status.Error = fmt.Sprintf("проверка доступности службы не удалась: %v", err)
		return status
	}
	status.Reachable = true

	return status
}

func isServiceInstalled(name string) (bool, error) {
	return isServiceInstalledSCM(name)
}

func isServiceRunning(name string) (bool, error) {
	return isServiceRunningSCM(name)
}

func InstallHelperService(exePath string) error {
	return errors.New("deprecated: use InstallOrRepairHelperServiceForUser")
}

func InstallOrRepairHelperServiceForUser(exePath string, userSID string) error {
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("helper executable not found: %s: %w", exePath, err)
	}

	if err := installOrUpdateServiceSCM(HelperServiceName, exePath, HelperDescription); err != nil {
		return err
	}

	if userSID != "" {
		if err := writeAllowedSidToRegistry(userSID); err != nil {
			return fmt.Errorf("не удалось записать AllowedSids в реестр: %w", err)
		}

		if err := grantServiceControlToUser(HelperServiceName, userSID); err != nil {

			logger.Warnf("не удалось задать DACL службы, продолжаем попытку запуска: %v", err)
		}
	}

	return nil
}

func RecoverHelperServiceForUser(exePath string, userSID string) error {
	if !CheckAdmin() {
		return fmt.Errorf("для исправления службы Helper требуются права администратора")
	}

	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("программа helper не найдена: %s: %w", exePath, err)
	}

	_ = StopHelperService()

	if err := InstallOrRepairHelperServiceForUser(exePath, userSID); err != nil {
		return fmt.Errorf("не удалось установить/исправить службу Helper: %w", err)
	}

	if err := StartHelperService(); err != nil {

		_ = StopHelperService()
		time.Sleep(500 * time.Millisecond)

		if err2 := StartHelperService(); err2 != nil {

			if delErr := UninstallHelperService(); delErr == nil {
				time.Sleep(1200 * time.Millisecond)

				if rebuildErr := InstallOrRepairHelperServiceForUser(exePath, userSID); rebuildErr != nil {
					return fmt.Errorf("не удалось пересоздать службу Helper: startErr=%v retryErr=%v rebuildErr=%w", err, err2, rebuildErr)
				}

				if startErr := StartHelperService(); startErr != nil {
					return fmt.Errorf("не удалось запустить службу Helper после пересоздания: first=%v retry=%v final=%w", err, err2, startErr)
				}
			} else {
				return fmt.Errorf("не удалось запустить службу Helper: first=%v retry=%v deleteErr=%w", err, err2, delErr)
			}
		}
	}

	if err := WaitForHelperReady(30, 300*time.Millisecond); err != nil {
		return fmt.Errorf("служба Helper недоступна после запуска: %w", err)
	}

	return nil
}

func writeAllowedSidToRegistry(sid string) error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\GoclashZHelper`, registry.SET_VALUE)
	if err != nil {
		key, _, err = registry.CreateKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\GoclashZHelper`, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("не удалось открыть/создать раздел реестра: %w", err)
		}
	}
	defer key.Close()
	if err := key.SetStringValue("AllowedSids", sid); err != nil {
		return fmt.Errorf("не удалось задать значение AllowedSids: %w", err)
	}
	return nil
}

func UninstallHelperService() error {
	_ = StopHelperService()
	return uninstallServiceSCM(HelperServiceName)
}

func StartHelperService() error {
	return startServiceSCM(HelperServiceName)
}

func StopHelperService() error {
	return stopServiceSCM(HelperServiceName)
}

func WaitForHelperReady(maxRetries int, interval time.Duration) error {
	client := NewHelperClient()
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(interval)
		}
		if err := client.Ping(); err == nil {
			logger.Infof("служба helper готова (attempt %d/%d)", i+1, maxRetries)
			return nil
		}
	}
	return fmt.Errorf("служба helper не готова после %d попыток", maxRetries)
}
