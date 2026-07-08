package sys

import (
	"encoding/json"
	"fmt"
	"os/user"
)

const (
	HelperServiceName = "GoclashZHelper"
	HelperDisplayName = "Misetanibox Helper Service"
	HelperDescription = "Привилегированные операции для Misetanibox: запуск TUN, установка Wintun, замена файлов ядра, исправление прав"

	HelperPipeName = `\\.\pipe\GoclashZ.Helper`
)

func GetHelperPipeName() string {
	return HelperPipeName
}

func CurrentUserSID() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("get current user failed: %w", err)
	}
	return u.Uid, nil
}

type HelperRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type HelperResponse struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

type StartCoreParams struct {
	CorePath      string   `json:"corePath"`
	BinDir        string   `json:"binDir"`
	RuntimeConfig string   `json:"runtimeConfig"`
	Args          []string `json:"args"`
}

type ReplaceCoreFileParams struct {
	Source string `json:"source"`
	Target string `json:"target"`
	SHA256 string `json:"sha256"`
}

type InstallWintunParams struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type CoreStatusData struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Error   string `json:"error,omitempty"`
}

type HelperStatusData struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}
