//go:build !windows

package sys

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"goclashz/core/utils"
)

type FLASHWINFO struct {
	CbSize    uint32
	Hwnd      uintptr
	DwFlags   uint32
	UCount    uint32
	DwTimeout uint32
}

type MainWindowState struct {
	Hwnd       uintptr
	Visible    bool
	Minimized  bool
	Foreground bool
}

func FindMainWindow() uintptr { return 0 }

func GetMainWindowState() MainWindowState { return MainWindowState{} }

func IsMainWindowShowing() bool { return false }

func StopTaskbarFlash(hwnd uintptr) {}

func FocusWindow(hwnd uintptr) {}

func FlashWindowTwice(hwnd uintptr) {}

func WaitMainWindowHandle() uintptr { return 0 }

func FocusMainWindowAndFlashTwiceWin32Only() {}

type ShellExecuteInfo struct {
	CbSize       uint32
	FMask        uint32
	Hwnd         uintptr
	LpVerb       *uint16
	LpFile       *uint16
	LpParameters *uint16
	LpDirectory  *uint16
	NShow        int32
	HInstApp     uintptr
	LpIDList     uintptr
	LpClass      *uint16
	HkeyClass    uintptr
	HotKey       uint32
	Union        uintptr
	HProcess     uintptr
}

func CheckAdmin() bool {
	return os.Geteuid() == 0
}

func IsAdmin() bool {
	return CheckAdmin()
}

func RequestAdmin() error {
	if CheckAdmin() {
		return nil
	}
	return relaunchElevated(os.Args[1:]...)
}

func RequestAdminWithArgs(extraArgs string) error {
	if CheckAdmin() {
		return nil
	}
	return relaunchElevated(strings.Fields(extraArgs)...)
}

func relaunchElevated(args ...string) error {
	pkexec, err := exec.LookPath("pkexec")
	if err != nil {
		return fmt.Errorf("требуются права root: pkexec не найден, запустите программу от root")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(pkexec, append([]string{exe}, args...)...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запросить права root: %w", err)
	}
	os.Exit(0)
	return nil
}

func RunElevatedWithArgsWait(args ...string) error {
	if CheckAdmin() {
		return nil
	}

	pkexec, err := exec.LookPath("pkexec")
	if err != nil {
		return fmt.Errorf("требуются права root: pkexec не найден")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	taskID := fmt.Sprintf("%d_%d", time.Now().UnixNano(), os.Getpid())
	os.Setenv("GOCLASHZ_ADMIN_TASK_ID", taskID)

	cmdArgs := append([]string{"env", "GOCLASHZ_ADMIN_TASK_ID=" + taskID, exe}, args...)
	cmd := exec.Command(pkexec, cmdArgs...)
	if err := cmd.Run(); err != nil {
		if msg := readAdminTaskError(); msg != "" {
			return fmt.Errorf("административный подпроцесс завершился с ошибкой: %s", msg)
		}
		return fmt.Errorf("не удалось запросить права root или запрос отменён: %w", err)
	}
	if msg := readAdminTaskError(); msg != "" {
		return fmt.Errorf("административный подпроцесс завершился с ошибкой: %s", msg)
	}
	return nil
}

type AdminTaskResult struct {
	OK        bool   `json:"ok"`
	Operation string `json:"operation"`
	Error     string `json:"error,omitempty"`
}

func getAdminTaskResultPath() string {
	id := os.Getenv("GOCLASHZ_ADMIN_TASK_ID")
	if id == "" {
		id = "default"
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("goclashz_admin_task_%s.json", id))
}

func WriteAdminTaskResult(operation string, err error) {
	res := AdminTaskResult{
		OK:        err == nil,
		Operation: operation,
	}
	if err != nil {
		res.Error = err.Error()
	}
	data, _ := json.Marshal(res)
	_ = os.WriteFile(getAdminTaskResultPath(), data, 0666)
}

func readAdminTaskError() string {
	path := getAdminTaskResultPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	_ = os.Remove(path)
	var res AdminTaskResult
	if err := json.Unmarshal(data, &res); err == nil && !res.OK {
		return res.Error
	}
	return ""
}

var systemProxyMu sync.Mutex

type SystemProxyState struct {
	Enabled bool
	Server  string
}

func gsettingsPath() string {
	p, err := exec.LookPath("gsettings")
	if err != nil {
		return ""
	}
	return p
}

func gsettingsSet(gs string, args ...string) error {
	out, err := exec.Command(gs, append([]string{"set"}, args...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gsettings set %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func gsettingsGet(gs string, args ...string) (string, error) {
	out, err := exec.Command(gs, append([]string{"get"}, args...)...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func EnableSystemProxy(host string, port int, bypassDomains string) error {
	systemProxyMu.Lock()
	defer systemProxyMu.Unlock()

	return enableSystemProxyLocked(host, port, bypassDomains)
}

func enableSystemProxyLocked(host string, port int, bypassDomains string) error {
	gs := gsettingsPath()
	if gs == "" {

		markSystemProxyOwnedLocked(host, port)
		return nil
	}

	portStr := strconv.Itoa(port)
	for _, schema := range []string{
		"org.gnome.system.proxy.http",
		"org.gnome.system.proxy.https",
		"org.gnome.system.proxy.socks",
	} {
		if err := gsettingsSet(gs, schema, "host", host); err != nil {
			return err
		}
		if err := gsettingsSet(gs, schema, "port", portStr); err != nil {
			return err
		}
	}

	if ignore := bypassToIgnoreHosts(bypassDomains); ignore != "" {
		_ = gsettingsSet(gs, "org.gnome.system.proxy", "ignore-hosts", ignore)
	}

	if err := gsettingsSet(gs, "org.gnome.system.proxy", "mode", "manual"); err != nil {
		return err
	}

	markSystemProxyOwnedLocked(host, port)
	return nil
}

func bypassToIgnoreHosts(bypass string) string {
	var items []string
	for _, part := range strings.FieldsFunc(bypass, func(r rune) bool { return r == ';' || r == ',' }) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "<local>" {
			part = "localhost"
		}
		items = append(items, "'"+strings.ReplaceAll(part, "'", "")+"'")
	}
	if len(items) == 0 {
		return ""
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func DisableSystemProxy() error {
	systemProxyMu.Lock()
	defer systemProxyMu.Unlock()

	return disableSystemProxyLocked()
}

func disableSystemProxyLocked() error {
	unmarkSystemProxyOwnedLocked()

	gs := gsettingsPath()
	if gs == "" {
		return nil
	}
	return gsettingsSet(gs, "org.gnome.system.proxy", "mode", "none")
}

func RefreshSystemProxy() {}

func GetSystemProxyState() (SystemProxyState, error) {
	gs := gsettingsPath()
	if gs == "" {
		return SystemProxyState{}, nil
	}

	mode, err := gsettingsGet(gs, "org.gnome.system.proxy", "mode")
	if err != nil {
		return SystemProxyState{}, err
	}
	mode = strings.Trim(mode, "'\"")
	if mode != "manual" {
		return SystemProxyState{}, nil
	}

	host, _ := gsettingsGet(gs, "org.gnome.system.proxy.http", "host")
	port, _ := gsettingsGet(gs, "org.gnome.system.proxy.http", "port")
	host = strings.Trim(host, "'\"")

	server := ""
	if host != "" {
		server = host + ":" + port
	}
	return SystemProxyState{Enabled: true, Server: server}, nil
}

type ownedProxyState struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	EnabledAt int64  `json:"enabledAt"`
}

func proxyStatePath() string {
	return filepath.Join(utils.GetDataDir(), "system_proxy_state.json")
}

func MarkSystemProxyOwned(host string, port int) {
	systemProxyMu.Lock()
	defer systemProxyMu.Unlock()

	markSystemProxyOwnedLocked(host, port)
}

func markSystemProxyOwnedLocked(host string, port int) {
	state := ownedProxyState{
		Host:      host,
		Port:      port,
		EnabledAt: time.Now().Unix(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}

	_ = utils.WriteFileAtomic(proxyStatePath(), data, 0644)
}

func UnmarkSystemProxyOwned() {
	systemProxyMu.Lock()
	defer systemProxyMu.Unlock()

	unmarkSystemProxyOwnedLocked()
}

func unmarkSystemProxyOwnedLocked() {
	_ = os.Remove(proxyStatePath())
}

func readOwnedProxyStateLocked() (ownedProxyState, bool) {
	data, err := os.ReadFile(proxyStatePath())
	if err != nil {
		return ownedProxyState{}, false
	}

	var state ownedProxyState
	if err := json.Unmarshal(data, &state); err != nil {
		return ownedProxyState{}, false
	}

	if state.Host == "" || state.Port <= 0 {
		return ownedProxyState{}, false
	}

	return state, true
}

func currentProxyMatches(host string, port int) bool {
	state, err := GetSystemProxyState()
	if err != nil || !state.Enabled {
		return false
	}
	return strings.Contains(state.Server, fmt.Sprintf("%s:%d", host, port))
}

func ClearOwnedSystemProxy() {
	systemProxyMu.Lock()
	defer systemProxyMu.Unlock()

	state, ok := readOwnedProxyStateLocked()
	if !ok {
		return
	}

	if currentProxyMatches(state.Host, state.Port) {
		_ = disableSystemProxyLocked()
	}

	unmarkSystemProxyOwnedLocked()
}

type DeviceInfo struct {
	HWID  string
	OS    string
	OSVer string
	Model string
}

var (
	deviceInfoOnce   sync.Once
	cachedDeviceInfo DeviceInfo
)

func GetDeviceInfo() DeviceInfo {
	deviceInfoOnce.Do(func() {
		cachedDeviceInfo = DeviceInfo{
			HWID:  readMachineID(),
			OS:    "Linux",
			OSVer: readLinuxVersion(),
			Model: readDeviceModel(),
		}
	})
	return cachedDeviceInfo
}

func SubscriptionHeaders() map[string]string {
	info := GetDeviceInfo()
	h := map[string]string{}
	if info.HWID != "" {
		h["x-hwid"] = info.HWID
	}
	h["x-device-os"] = info.OS
	if info.OSVer != "" {
		h["x-ver-os"] = info.OSVer
	}
	if info.Model != "" {
		h["x-device-model"] = info.Model
	}
	return h
}

func readMachineID() string {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if data, err := os.ReadFile(p); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id
			}
		}
	}
	return ""
}

func readLinuxVersion() string {
	parts := []string{}

	if pretty := readOSReleasePrettyName(); pretty != "" {
		parts = append(parts, pretty)
	} else {
		parts = append(parts, "Linux")
	}

	if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		if kernel := strings.TrimSpace(string(data)); kernel != "" {
			parts = append(parts, "kernel "+kernel)
		}
	}

	return strings.Join(parts, " ")
}

func readOSReleasePrettyName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}

func readDMI(name string) string {
	data, err := os.ReadFile(filepath.Join("/sys/devices/virtual/dmi/id", name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readDeviceModel() string {
	manufacturer := readDMI("sys_vendor")
	model := readDMI("product_name")
	switch {
	case manufacturer != "" && model != "" && !strings.HasPrefix(strings.ToLower(model), strings.ToLower(manufacturer)):
		return manufacturer + " " + model
	case model != "":
		return model
	default:
		return manufacturer
	}
}

type AppInfo struct {
	Name    string `json:"name"`
	Exe     string `json:"exe"`
	Path    string `json:"path"`
	IconPNG string `json:"iconPng"`
}

func ListInstalledApps() ([]AppInfo, error) {
	dirs := []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "applications"))
	}

	var apps []AppInfo
	seen := make(map[string]struct{})

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			info, ok := parseDesktopFile(filepath.Join(dir, e.Name()))
			if !ok {
				continue
			}
			key := strings.ToLower(info.Path)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			apps = append(apps, info)
		}
	}

	sort.Slice(apps, func(i, j int) bool {
		li, lj := strings.ToLower(apps[i].Name), strings.ToLower(apps[j].Name)
		if li != lj {
			return li < lj
		}
		return apps[i].Path < apps[j].Path
	})
	return apps, nil
}

func parseDesktopFile(path string) (AppInfo, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return AppInfo{}, false
	}

	var name, execLine, entryType string
	noDisplay := false
	inEntry := false

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		if !inEntry || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "Name":
			if name == "" {
				name = strings.TrimSpace(v)
			}
		case "Exec":
			if execLine == "" {
				execLine = strings.TrimSpace(v)
			}
		case "Type":
			entryType = strings.TrimSpace(v)
		case "NoDisplay":
			noDisplay = strings.EqualFold(strings.TrimSpace(v), "true")
		}
	}

	if noDisplay || (entryType != "" && entryType != "Application") || execLine == "" {
		return AppInfo{}, false
	}

	cmd := firstExecToken(execLine)
	if cmd == "" {
		return AppInfo{}, false
	}

	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".desktop")
	}

	return AppInfo{
		Name: name,
		Exe:  strings.ToLower(filepath.Base(cmd)),
		Path: cmd,
	}, true
}

func firstExecToken(execLine string) string {
	tokens := splitExecLine(execLine)

	i := 0

	for i < len(tokens) {
		t := tokens[i]
		if t == "env" || strings.Contains(t, "=") {
			i++
			continue
		}
		break
	}
	if i >= len(tokens) {
		return ""
	}
	cmd := tokens[i]
	if strings.HasPrefix(cmd, "%") {
		return ""
	}
	return cmd
}

func splitExecLine(s string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := false

	flush := func() {
		t := cur.String()
		cur.Reset()
		if t == "" {
			return
		}

		if len(t) == 2 && t[0] == '%' {
			return
		}
		tokens = append(tokens, t)
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case c == ' ' && !inQuote:
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return tokens
}

func AppInfoForExe(path string) AppInfo {
	path = strings.TrimSpace(path)
	if path != "" {
		path = filepath.Clean(path)
	}
	base := filepath.Base(path)
	return AppInfo{
		Name: strings.TrimSuffix(base, filepath.Ext(base)),
		Exe:  strings.ToLower(base),
		Path: path,
	}
}

type StartupMode string

const (
	StartupDisabled StartupMode = "disabled"
	StartupNormal   StartupMode = "normal"
)

type StartupTaskInfo struct {
	Exists          bool        `json:"exists"`
	Enabled         bool        `json:"enabled"`
	Mode            StartupMode `json:"mode"`
	Path            string      `json:"path"`
	Arguments       string      `json:"arguments"`
	RunLevel        int         `json:"runLevel"`
	LastError       string      `json:"lastError"`
	ExpectedPath    string      `json:"expectedPath"`
	ActualPath      string      `json:"actualPath"`
	ActualArgs      string      `json:"actualArgs"`
	ExpectedDataDir string      `json:"expectedDataDir"`
	ActualDataDir   string      `json:"actualDataDir"`
	IsHealthy       bool        `json:"isHealthy"`
}

func autostartDesktopPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить домашний каталог: %w", err)
	}
	return filepath.Join(home, ".config", "autostart", "misetanibox.desktop"), nil
}

func CheckStartupTask() (StartupTaskInfo, error) {
	exe, _ := os.Executable()
	expected, _ := filepath.Abs(exe)
	info := StartupTaskInfo{Mode: StartupDisabled, ExpectedPath: expected}

	desktopPath, err := autostartDesktopPath()
	if err != nil {
		info.LastError = err.Error()
		return info, err
	}

	data, err := os.ReadFile(desktopPath)
	if err != nil {
		info.Exists = false
		info.Enabled = false
		info.IsHealthy = false
		info.LastError = "startup task not found"
		return info, nil
	}
	info.Exists = true
	info.Mode = StartupNormal

	enabled := true
	var execLine string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "Exec="); ok {
			execLine = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "Hidden="); ok && strings.EqualFold(strings.TrimSpace(v), "true") {
			enabled = false
		}
		if v, ok := strings.CutPrefix(line, "X-GNOME-Autostart-enabled="); ok && strings.EqualFold(strings.TrimSpace(v), "false") {
			enabled = false
		}
	}
	info.Enabled = enabled

	tokens := splitExecLine(execLine)
	if len(tokens) > 0 {
		info.Path = tokens[0]
		info.Arguments = strings.Join(tokens[1:], " ")
	}
	info.ActualPath = info.Path
	info.ActualArgs = info.Arguments

	expectedDataDir := filepath.Clean(utils.GetDataDir())
	info.ExpectedDataDir = expectedDataDir

	actualDataDir := ""
	for i, t := range tokens {
		if t == "--data-dir" && i+1 < len(tokens) {
			actualDataDir = filepath.Clean(strings.Trim(tokens[i+1], `"`))
			break
		}
		if v, ok := strings.CutPrefix(t, "--data-dir="); ok {
			actualDataDir = filepath.Clean(strings.Trim(v, `"`))
			break
		}
	}
	info.ActualDataDir = actualDataDir

	pathMismatch := info.Path == "" || filepath.Clean(info.Path) != filepath.Clean(expected)
	hasStartup := strings.Contains(info.Arguments, "--startup")
	hasSilent := strings.Contains(info.Arguments, "--silent")
	dataDirMismatch := actualDataDir == "" || actualDataDir != expectedDataDir

	if pathMismatch || !hasStartup || !hasSilent || dataDirMismatch {
		info.Enabled = false
		info.LastError = "path mismatch or incomplete arguments"
	}

	info.IsHealthy = info.Exists && info.Enabled && info.LastError == ""

	return info, nil
}

func CreateStartupTask(exePath string) error {
	absPath, err := filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("не удалось получить абсолютный путь: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("исполняемый файл не найден: %s", absPath)
	}

	desktopPath, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(desktopPath), 0755); err != nil {
		return fmt.Errorf("не удалось создать каталог автозапуска: %w", err)
	}

	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Misetanibox
Comment=Автозапуск прокси-клиента Misetanibox
Exec="%s" --startup --silent --data-dir "%s"
Terminal=false
X-GNOME-Autostart-enabled=true
`, absPath, utils.GetDataDir())

	if err := os.WriteFile(desktopPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("не удалось записать файл автозапуска: %w", err)
	}
	return nil
}

func DeleteStartupTask() error {
	desktopPath, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func GetFileVersion(path string) (string, error) {
	return "", nil
}

func RequestExistingInstanceQuit() {}

func ListenForShutdownEvent() {
	select {}
}

type HelperClient struct{}

func NewHelperClient() *HelperClient {
	return &HelperClient{}
}

func (c *HelperClient) Ping() error { return nil }

func (c *HelperClient) Shutdown() error { return nil }

func (c *HelperClient) StartCore(params StartCoreParams) error { return nil }

func (c *HelperClient) StopCore() error { return nil }

func (c *HelperClient) CoreStatus() (CoreStatusData, error) {
	return CoreStatusData{}, nil
}

func (c *HelperClient) RepairPermission(dataDir string) error { return nil }

func (c *HelperClient) ReplaceCoreFile(params ReplaceCoreFileParams) error { return nil }

func (c *HelperClient) InstallWintun(source, target string) error { return nil }

func CheckHelperService() HelperStatusData {
	return HelperStatusData{}
}

func InstallHelperService(exePath string) error { return nil }

func InstallOrRepairHelperServiceForUser(exePath string, userSID string) error { return nil }

func RecoverHelperServiceForUser(exePath string, userSID string) error { return nil }

func UninstallHelperService() error { return nil }

func StartHelperService() error { return nil }

func StopHelperService() error { return nil }

func WaitForHelperReady(maxRetries int, interval time.Duration) error { return nil }

type UwpApp struct {
	DisplayName       string `json:"displayName"`
	PackageFamilyName string `json:"packageFamilyName"`
	SID               string `json:"sid"`
	IsEnabled         bool   `json:"isEnabled"`
}

func GetUwpAppList() ([]UwpApp, error) { return nil, nil }

func SaveUwpExemptions(targetSids []string) error { return nil }

func ExemptAllUWP() error { return nil }
