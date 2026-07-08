package clash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultDelayTestURL = "http://www.gstatic.com/generate_204"

var noProxyTransport = &http.Transport{
	Proxy: nil,

	DialContext: (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 15 * time.Second,
	}).DialContext,
	MaxIdleConns:        100,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
}

var localAPIClient = &http.Client{
	Transport: noProxyTransport,
	Timeout:   2 * time.Second,
}

var speedTestClient = &http.Client{
	Transport: noProxyTransport,
}

var streamClient = &http.Client{
	Transport: noProxyTransport,
}

var apiBase = struct {
	sync.RWMutex
	value string
}{
	value: "http://127.0.0.1:9090",
}

func NormalizeControllerHostPort(controller string) string {
	controller = strings.TrimSpace(controller)
	if controller == "" {
		return "127.0.0.1:9090"
	}

	if _, err := strconv.Atoi(controller); err == nil {
		return "127.0.0.1:" + controller
	}

	if strings.Contains(controller, "://") {
		u, err := url.Parse(controller)
		if err == nil && u.Host != "" {
			controller = u.Host
		}
	}
	host, port, err := net.SplitHostPort(controller)
	if err != nil || host == "" || port == "" {
		return "127.0.0.1:9090"
	}

	if host == "" || port == "" {
		return "127.0.0.1:9090"
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), port)
}

func normalizeAPIBaseURL(controller string) string {
	controller = NormalizeControllerHostPort(controller)

	if !strings.HasPrefix(controller, "http://") && !strings.HasPrefix(controller, "https://") {
		controller = "http://" + controller
	}

	u, err := url.Parse(controller)
	if err != nil || u.Host == "" {
		return "http://127.0.0.1:9090"
	}

	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""

	return strings.TrimRight(u.String(), "/")
}

func UpdateAPIBaseURL(controller string) {
	apiBase.Lock()
	apiBase.value = normalizeAPIBaseURL(controller)
	apiBase.Unlock()
}

func APIURL(path string) string {
	apiBase.RLock()
	base := apiBase.value
	apiBase.RUnlock()

	if base == "" {
		base = "http://127.0.0.1:9090"
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return base + path
}

func APIWSURL(path string) string {
	apiBase.RLock()
	base := apiBase.value
	apiBase.RUnlock()

	if base == "" {
		base = "http://127.0.0.1:9090"
	}

	u, err := url.Parse(base)
	if err != nil {
		return "ws://127.0.0.1:9090" + path
	}

	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	u.Path = path
	u.RawQuery = ""

	return u.String()
}

func APIWSURLWithRawQuery(path string, rawQuery string) string {
	u, _ := url.Parse(APIWSURL(path))
	u.RawQuery = rawQuery
	return u.String()
}

func FetchLogs(ctx context.Context, level string, onLog func(data interface{})) {
	if level == "" {
		level = "info"
	}

	for {

		select {
		case <-ctx.Done():
			return
		default:
		}

		apiURL := APIURL("/logs?level=" + url.QueryEscape(level))
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {

			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}

		resp, err := streamClient.Do(req)
		if err != nil {

			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}

		const logIdleTimeout = 70 * time.Second
		decoder := json.NewDecoder(resp.Body)
		idleTimer := time.NewTimer(logIdleTimeout)
		stopIdle := make(chan struct{})

		go func() {
			select {
			case <-ctx.Done():
				resp.Body.Close()
			case <-idleTimer.C:

				resp.Body.Close()
			case <-stopIdle:

			}
		}()

		for {
			var logData map[string]interface{}
			if err := decoder.Decode(&logData); err != nil {
				close(stopIdle)
				if !idleTimer.Stop() {
					select {
					case <-idleTimer.C:
					default:
					}
				}
				resp.Body.Close()
				break
			}

			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(logIdleTimeout)

			onLog(logData)
		}
	}
}

func GetProxyDelay(ctx context.Context, proxyName string, testUrl string, timeoutMs int) (int, error) {
	encodedName := url.PathEscape(proxyName)

	if testUrl == "" {
		testUrl = DefaultDelayTestURL
	}

	if timeoutMs <= 0 {
		timeoutMs = 7000
	}

	apiURL := fmt.Sprintf("%s?timeout=%d&url=%s",
		APIURL("/proxies/"+encodedName+"/delay"),
		timeoutMs,
		url.QueryEscape(testUrl))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return -1, err
	}

	resp, err := speedTestClient.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return -1, fmt.Errorf("mihomo delay failed: HTTP %d: %s", resp.StatusCode, msg)
	}

	var result struct {
		Delay int `json:"delay"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return -1, err
	}

	if result.Delay <= 0 {
		return -1, fmt.Errorf("invalid delay format")
	}

	return result.Delay, nil
}

func TestProxy(name string) (int, error) {
	testUrl := DefaultDelayTestURL
	if netCfg, err := GetNetworkConfig(); err == nil && netCfg != nil {
		if u := strings.TrimSpace(netCfg.TestURL); u != "" {
			testUrl = u
		}
	}

	timeoutMs := 8000

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs+1500)*time.Millisecond)
	defer cancel()

	return GetProxyDelay(ctx, name, testUrl, timeoutMs)
}

func require2xx(resp *http.Response, endpoint string) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s failed: HTTP %d", endpoint, resp.StatusCode)
	}
	return nil
}

func doKernelGet[T any](path string) (T, error) {
	return doKernelGetWithContext[T](context.Background(), path)
}

func doKernelGetWithContext[T any](ctx context.Context, path string) (T, error) {
	var result T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, APIURL(path), nil)
	if err != nil {
		return result, err
	}

	resp, err := localAPIClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if err := require2xx(resp, path); err != nil {
		return result, err
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func doKernelRequest(ctx context.Context, method, path string, body any, okStatus ...int) error {
	var reader io.Reader

	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, APIURL(path), reader)
	if err != nil {
		return err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := localAPIClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for _, status := range okStatus {
		if resp.StatusCode == status {
			return nil
		}
	}

	return fmt.Errorf("не удалось вызвать API ядра: %s %s -> HTTP %d", method, path, resp.StatusCode)
}

func GetInitialData() (map[string]interface{}, error) {
	return GetInitialDataWithContext(context.Background())
}

func PingAPIWithContext(ctx context.Context) error {
	_, err := doKernelGetWithContext[map[string]interface{}](ctx, "/configs")
	return err
}

func GetInitialDataWithContext(ctx context.Context) (map[string]interface{}, error) {
	configData, err := doKernelGetWithContext[map[string]interface{}](ctx, "/configs")
	if err != nil {
		return nil, err
	}

	proxiesData, err := doKernelGetWithContext[map[string]interface{}](ctx, "/proxies")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"mode":   configData["mode"],
		"groups": proxiesData["proxies"],
	}, nil
}

func UpdateMode(mode string) error {
	return UpdateModeWithContext(context.Background(), mode)
}

func UpdateModeWithContext(ctx context.Context, mode string) error {
	return doKernelRequest(
		ctx,
		http.MethodPatch,
		"/configs",
		map[string]string{"mode": mode},
		http.StatusOK,
		http.StatusNoContent,
	)
}

func SelectProxy(groupName, proxyName string) error {
	return SelectProxyWithContext(context.Background(), groupName, proxyName)
}

func SelectProxyWithContext(ctx context.Context, groupName, proxyName string) error {
	return doKernelRequest(
		ctx,
		http.MethodPut,
		"/proxies/"+url.PathEscape(groupName),
		map[string]string{"name": proxyName},
		http.StatusOK,
		http.StatusNoContent,
	)
}

func GetConnectionsRaw() ([]byte, error) {
	resp, err := localAPIClient.Get(APIURL("/connections"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := require2xx(resp, "/connections"); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	return buf.Bytes(), err
}

func CloseConnection(id string) error {
	return doKernelRequest(
		context.Background(),
		http.MethodDelete,
		"/connections/"+url.PathEscape(id),
		nil,
		http.StatusOK,
		http.StatusNoContent,
	)
}

func CloseAllConnections() error {
	return doKernelRequest(
		context.Background(),
		http.MethodDelete,
		"/connections",
		nil,
		http.StatusOK,
		http.StatusNoContent,
	)
}

func GetVersion() string {
	data, err := doKernelGet[map[string]string]("/version")
	if err != nil {
		return ""
	}
	return data["version"]
}

func FlushFakeIP() error {

	dnsCfg, err := GetDNSConfig()
	if err != nil || dnsCfg == nil {
		return fmt.Errorf("сбой связи: не удалось получить текущее состояние DNS, проверьте, запущено ли ядро")
	}

	if dnsCfg.EnhancedMode != "fake-ip" {
		return fmt.Errorf("текущий режим: %s; сброс кэша поддерживается только в режиме Fake-IP", dnsCfg.EnhancedMode)
	}

	return doKernelRequest(
		context.Background(),
		http.MethodPost,
		"/cache/fakeip/flush",
		nil,
		http.StatusOK,
		http.StatusNoContent,
	)
}
