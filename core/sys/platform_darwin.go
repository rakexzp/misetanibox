//go:build darwin

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

// relaunchElevated перезапускает программу с правами администратора.
// На macOS нет pkexec — используем системный запрос пароля через osascript.
func relaunchElevated(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	quoted := make([]string, 0, len(args)+1)
	for _, a := range append([]string{exe}, args...) {
		quoted = append(quoted, "'"+strings.ReplaceAll(a, "'", "'\\''")+"'")
	}
	script := fmt.Sprintf(`do shell script "%s > /dev/null 2>&1 &" with administrator privileges`,
		strings.ReplaceAll(strings.Join(quoted, " "), `"`, `\"`))
	return exec.Command("osascript", "-e", script).Start()
}

// RunElevatedWithArgsWait выполняет программу с правами администратора и ждёт завершения.
func RunElevatedWithArgsWait(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	quoted := make([]string, 0, len(args)+1)
	for _, a := range append([]string{exe}, args...) {
		quoted = append(quoted, "'"+strings.ReplaceAll(a, "'", "'\\''")+"'")
	}
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`,
		strings.ReplaceAll(strings.Join(quoted, " "), `"`, `\"`))
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("не удалось выполнить с правами администратора: %v: %s", err, strings.TrimSpace(string(out)))
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

// networksetupPath — путь к утилите настройки сети macOS.
func networksetupPath() string {
	p, err := exec.LookPath("networksetup")
	if err != nil {
		return ""
	}
	return p
}

// activeNetworkServices возвращает включённые сетевые сервисы (Wi-Fi, Ethernet и т.д.).
// В macOS прокси настраивается отдельно для каждого сервиса, единого переключателя нет.
func activeNetworkServices(ns string) []string {
	out, err := exec.Command(ns, "-listallnetworkservices").Output()
	if err != nil {
		return nil
	}
	var services []string
	for i, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// первая строка — пояснительный текст; "*" помечает отключённый сервис
		if i == 0 || line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		services = append(services, line)
	}
	return services
}

func networksetupRun(ns string, args ...string) error {
	out, err := exec.Command(ns, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("networksetup %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func networksetupGet(ns string, args ...string) (string, error) {
	out, err := exec.Command(ns, args...).Output()
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
	ns := networksetupPath()
	if ns == "" {
		return fmt.Errorf("networksetup не найден — не удалось настроить системный прокси")
	}
	services := activeNetworkServices(ns)
	if len(services) == 0 {
		return fmt.Errorf("не найдено активных сетевых сервисов для настройки прокси")
	}

	portStr := strconv.Itoa(port)
	var lastErr error
	applied := 0
	for _, svc := range services {
		okSvc := true
		for _, kind := range []string{"-setwebproxy", "-setsecurewebproxy"} {
			if err := networksetupRun(ns, kind, svc, host, portStr); err != nil {
				lastErr = err
				okSvc = false
			}
		}
		if bypass := bypassDomainList(bypassDomains); len(bypass) > 0 {
			args := append([]string{"-setproxybypassdomains", svc}, bypass...)
			if err := networksetupRun(ns, args...); err != nil {
				lastErr = err
			}
		}
		if okSvc {
			applied++
		}
	}
	if applied == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("не удалось включить системный прокси")
		}
		return lastErr
	}

	markSystemProxyOwnedLocked(host, port)
	return nil
}

// bypassDomainList — домены-исключения отдельными аргументами (формат networksetup).
func bypassDomainList(bypass string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(bypass, func(r rune) bool { return r == ';' || r == ',' }) {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
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
	ns := networksetupPath()
	if ns == "" {
		unmarkSystemProxyOwnedLocked()
		return nil
	}
	var lastErr error
	for _, svc := range activeNetworkServices(ns) {
		for _, kind := range []string{"-setwebproxystate", "-setsecurewebproxystate"} {
			if err := networksetupRun(ns, kind, svc, "off"); err != nil {
				lastErr = err
			}
		}
	}
	unmarkSystemProxyOwnedLocked()
	return lastErr
}
func RefreshSystemProxy() {}

// GetSystemProxyState читает текущее состояние системного прокси у первого активного сервиса.
func GetSystemProxyState() (SystemProxyState, error) {
	st := SystemProxyState{}
	ns := networksetupPath()
	if ns == "" {
		return st, nil
	}
	services := activeNetworkServices(ns)
	if len(services) == 0 {
		return st, nil
	}

	out, err := networksetupGet(ns, "-getwebproxy", services[0])
	if err != nil {
		return st, err
	}
	var host, port string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "Enabled:"); ok {
			st.Enabled = strings.EqualFold(strings.TrimSpace(v), "Yes")
		}
		if v, ok := strings.CutPrefix(line, "Server:"); ok {
			host = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "Port:"); ok {
			port = strings.TrimSpace(v)
		}
	}
	if host != "" && port != "" && port != "0" {
		st.Server = host + ":" + port
	}
	return st, nil
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
			OS:    "macOS",
			OSVer: readMacVersion(),
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

// readMachineID берёт IOPlatformUUID — стабильный идентификатор конкретного Mac.
func readMachineID() string {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, "IOPlatformUUID") {
				continue
			}
			if i := strings.Index(line, "= \""); i >= 0 {
				v := line[i+3:]
				if j := strings.Index(v, "\""); j >= 0 {
					return strings.TrimSpace(v[:j])
				}
			}
		}
	}
	return ""
}

// readMacVersion — версия macOS (sw_vers -productVersion).
func readMacVersion() string {
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

// readDeviceModel — модель железа (например MacBookPro18,3).
func readDeviceModel() string {
	out, err := exec.Command("sysctl", "-n", "hw.model").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

type AppInfo struct {
	Name    string `json:"name"`
	Exe     string `json:"exe"`
	Path    string `json:"path"`
	IconPNG string `json:"iconPng"`
}

// ListInstalledApps перечисляет приложения macOS (.app в стандартных каталогах).
func ListInstalledApps() ([]AppInfo, error) {
	dirs := []string{"/Applications", "/System/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}

	seen := map[string]bool{}
	var apps []AppInfo
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".app")
			bundle := filepath.Join(dir, e.Name())
			// исполняемый файл лежит внутри бандла
			exe := filepath.Join(bundle, "Contents", "MacOS", name)
			if _, err := os.Stat(exe); err != nil {
				exe = bundle
			}
			if seen[exe] {
				continue
			}
			seen[exe] = true
			apps = append(apps, AppInfo{Name: name, Path: exe})
		}
	}
	sort.Slice(apps, func(i, j int) bool { return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name) })
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
	name := filepath.Base(path)
	if i := strings.Index(path, ".app/"); i >= 0 {
		name = strings.TrimSuffix(filepath.Base(path[:i+4]), ".app")
	}
	return AppInfo{Name: name, Path: path}
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

// launchAgentPath — путь к пользовательскому LaunchAgent (автозапуск macOS).
func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", "network.geodema.misetanibox.plist"), nil
}

func CheckStartupTask() (StartupTaskInfo, error) {
	exe, _ := os.Executable()
	expected, _ := filepath.Abs(exe)
	info := StartupTaskInfo{Mode: StartupDisabled, ExpectedPath: expected}

	plistPath, err := launchAgentPath()
	if err != nil {
		info.LastError = err.Error()
		return info, err
	}

	data, err := os.ReadFile(plistPath)
	if err != nil {
		info.LastError = "startup task not found"
		return info, nil
	}
	info.Exists = true
	info.Mode = StartupNormal

	text := string(data)
	// Disabled=true means the agent is registered but switched off
	info.Enabled = !strings.Contains(text, "<key>Disabled</key>")

	// первый <string> внутри ProgramArguments — путь к программе
	if i := strings.Index(text, "<key>ProgramArguments</key>"); i >= 0 {
		rest := text[i:]
		var tokens []string
		for {
			a := strings.Index(rest, "<string>")
			if a < 0 {
				break
			}
			b := strings.Index(rest[a:], "</string>")
			if b < 0 {
				break
			}
			tokens = append(tokens, strings.TrimSpace(rest[a+len("<string>"):a+b]))
			rest = rest[a+b:]
			if strings.HasPrefix(strings.TrimSpace(rest), "</array>") {
				break
			}
		}
		if len(tokens) > 0 {
			info.Path = tokens[0]
			info.Arguments = strings.Join(tokens[1:], " ")
		}
	}
	info.ActualPath = info.Path
	info.ActualArgs = info.Arguments
	info.IsHealthy = info.Exists && info.Enabled && info.Path == expected
	return info, nil
}

func CreateStartupTask(exePath string) error {
	plistPath, err := launchAgentPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return err
	}
	if strings.TrimSpace(exePath) == "" {
		exePath, _ = os.Executable()
	}
	exePath, _ = filepath.Abs(exePath)

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>network.geodema.misetanibox</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + exePath + `</string>
		<string>--startup</string>
		<string>--silent</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`
	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return err
	}
	// перезагружаем агент, чтобы изменения применились без перезахода в систему
	_ = exec.Command("launchctl", "unload", plistPath).Run()
	_ = exec.Command("launchctl", "load", plistPath).Run()
	return nil
}

func DeleteStartupTask() error {
	plistPath, err := launchAgentPath()
	if err != nil {
		return err
	}
	_ = exec.Command("launchctl", "unload", plistPath).Run()
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
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
