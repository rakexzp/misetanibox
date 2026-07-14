package clash

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"goclashz/core/downloader"
	"goclashz/core/sys"
	"goclashz/core/utils"

	"gopkg.in/yaml.v3"
)

// resolveSubInput accepts:
//   - https://… subscription URLs
//   - local paths / file:// URLs to a .txt that contains an http(s) URL
//   - local paths to a Clash/mihomo YAML (imported as body, no network)
//
// Returns either remoteURL (download) or localBody (inline config). The caller's
// original input should be stored in the index so refresh re-reads the file.
func resolveSubInput(input string) (remoteURL string, localBody []byte, err error) {
	input = strings.TrimSpace(input)
	input = strings.Trim(input, `"'«»`)
	if input == "" {
		return "", nil, fmt.Errorf("пустая ссылка или путь")
	}

	// Direct remote URL — no file involved.
	low := strings.ToLower(input)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return input, nil, nil
	}

	path, isLocal, pathErr := localPathFromInput(input)
	if pathErr != nil {
		return "", nil, pathErr
	}
	if !isLocal {
		// Domain-like or opaque string — let HTTP downloader try (may fail clearly).
		return input, nil, nil
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", nil, fmt.Errorf("не удалось прочитать файл %s: %w", path, readErr)
	}
	// Strip UTF-8 BOM
	data = bytesTrimBOM(data)
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", nil, fmt.Errorf("файл пуст: %s", path)
	}

	// File is a pointer to a remote subscription (one or more lines).
	if u := firstHTTPURL(content); u != "" {
		return u, nil, nil
	}

	// base64:… or raw base64 blob of YAML
	trimmed := strings.TrimPrefix(content, "base64:")
	if dec, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(trimmed)); decErr == nil && len(dec) > 0 {
		if StrictVerifyClashConfig(dec) == nil {
			return "", dec, nil
		}
	}

	// Inline Clash/mihomo YAML (or proxies-only body).
	if StrictVerifyClashConfig(data) == nil {
		return "", data, nil
	}

	return "", nil, fmt.Errorf(
		"в файле %s нет https-ссылки и это не YAML Clash/mihomo. "+
			"Положите в .txt одну строку с URL подписки или полный конфиг",
		path,
	)
}

func bytesTrimBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

// localPathFromInput detects Windows/Unix/file:// paths that exist on disk.
func localPathFromInput(input string) (path string, ok bool, err error) {
	s := strings.TrimSpace(input)
	low := strings.ToLower(s)

	if strings.HasPrefix(low, "file:") {
		p, perr := pathFromFileURL(s)
		if perr != nil {
			return "", false, perr
		}
		if st, e := os.Stat(p); e != nil {
			return "", false, fmt.Errorf("локальный файл не найден: %s", p)
		} else if st.IsDir() {
			return "", false, fmt.Errorf("нужен файл, а не папка: %s", p)
		}
		return p, true, nil
	}

	// Never treat real URLs as paths.
	if strings.Contains(s, "://") {
		return "", false, nil
	}

	candidate := s
	// Normalize quotes / stray spaces already trimmed.
	if looksLikeFilesystemPath(candidate) {
		if st, e := os.Stat(candidate); e == nil {
			if st.IsDir() {
				return "", false, fmt.Errorf("нужен файл, а не папка: %s", candidate)
			}
			return candidate, true, nil
		}
		// Path-shaped but missing — surface a clear error instead of HTTP fail.
		if filepath.IsAbs(candidate) || looksLikeWindowsPath(candidate) {
			return "", false, fmt.Errorf("локальный файл не найден: %s", candidate)
		}
	}

	// Relative path that exists (e.g. .\subs\my.txt)
	if st, e := os.Stat(candidate); e == nil && !st.IsDir() {
		abs, _ := filepath.Abs(candidate)
		return abs, true, nil
	}
	return "", false, nil
}

func looksLikeWindowsPath(s string) bool {
	if strings.HasPrefix(s, `\\`) {
		return true // UNC
	}
	if len(s) >= 3 {
		drive := s[0]
		if ((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) && s[1] == ':' {
			return s[2] == '\\' || s[2] == '/'
		}
	}
	return false
}

func looksLikeFilesystemPath(s string) bool {
	if looksLikeWindowsPath(s) {
		return true
	}
	if filepath.IsAbs(s) {
		return true
	}
	// Relative with separators or known config extensions.
	if strings.ContainsAny(s, `/\`) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(s))
	switch ext {
	case ".txt", ".yaml", ".yml", ".json", ".conf", ".ini", ".list":
		return true
	}
	return false
}

func pathFromFileURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("некорректный file:// URL: %w", err)
	}
	p := u.Path
	if u.Host != "" && u.Host != "localhost" {
		// file://server/share → UNC on Windows
		if runtime.GOOS == "windows" {
			p = `\\` + u.Host + filepath.FromSlash(p)
			return p, nil
		}
	}
	if runtime.GOOS == "windows" {
		// file:///C:/Users/... → /C:/Users/... → C:\Users\...
		if strings.HasPrefix(p, "/") && len(p) >= 3 && p[2] == ':' {
			p = p[1:]
		}
		// file:///C|/Users (legacy)
		if len(p) >= 2 && p[1] == '|' {
			p = string(p[0]) + ":" + p[2:]
		}
		p = filepath.FromSlash(p)
	}
	if p == "" {
		return "", fmt.Errorf("пустой путь в file:// URL")
	}
	return p, nil
}

func firstHTTPURL(content string) string {
	// Whole content is a single URL
	one := strings.TrimSpace(content)
	one = strings.Trim(one, `"'`)
	low := strings.ToLower(one)
	if (strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://")) && !strings.Contains(one, "\n") {
		return one
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, `"'`)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		// Allow "url: https://..." style
		if idx := strings.Index(strings.ToLower(line), "http://"); idx >= 0 {
			return strings.TrimSpace(line[idx:])
		}
		if idx := strings.Index(strings.ToLower(line), "https://"); idx >= 0 {
			return strings.TrimSpace(line[idx:])
		}
	}
	return ""
}

func parseSubUserInfo(header string) (upload, download, total, expire int64) {
	if header == "" {
		return
	}
	parts := strings.Split(header, ";")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {

			key := strings.ToLower(strings.TrimSpace(kv[0]))
			valStr := strings.TrimSpace(kv[1])

			val, _ := strconv.ParseInt(valStr, 10, 64)
			switch key {
			case "upload":
				upload = val
			case "download":
				download = val
			case "total":
				total = val
			case "expire":
				expire = val
			}
		}
	}
	return
}

func parseSubProfileName(h http.Header) string {
	if pt := strings.TrimSpace(h.Get("profile-title")); pt != "" {
		if strings.HasPrefix(strings.ToLower(pt), "base64:") {
			if dec, err := base64.StdEncoding.DecodeString(pt[len("base64:"):]); err == nil {
				return strings.TrimSpace(string(dec))
			}
		}
		return pt
	}
	if cd := h.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn := strings.TrimSpace(params["filename"]); fn != "" {
				fn = strings.TrimSuffix(fn, filepath.Ext(fn))
				return fn
			}
		}
	}
	return ""
}

func DownloadSub(ctx context.Context, name, url, existingId, userAgent string) (string, error) {
	id := existingId
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().UnixMilli())
	}

	// Keep the original input (may be a local path to a .txt with the real URL).
	sourceKey := strings.TrimSpace(url)
	remoteURL, localBody, resolveErr := resolveSubInput(sourceKey)
	if resolveErr != nil {
		return id, resolveErr
	}

	dir := utils.GetSubscriptionsDir()
	os.MkdirAll(dir, 0755)

	safeId, err := utils.SanitizeFilename(id)
	if err != nil {
		return id, err
	}

	originPath := filepath.Join(OriginDir(), safeId+".yaml")
	workingPath := filepath.Join(SubscriptionsDir(), safeId+".yaml")

	os.MkdirAll(filepath.Dir(originPath), 0755)

	var upload, download, total, expire int64
	var headerName string

	if _, statErr := os.Stat(originPath); statErr == nil {
		originData, _ := os.ReadFile(originPath)
		_ = utils.WriteFileAtomic(originPath+".bak", originData, 0644)
	}

	if len(localBody) > 0 {
		// Local YAML / decoded config — no network.
		if err := StrictVerifyClashConfig(localBody); err != nil {
			return safeId, fmt.Errorf("проверка конфигурации не пройдена: %v", err)
		}
		if err := ValidateClashReferencesBytes(localBody); err != nil {
			return safeId, fmt.Errorf("проверка ссылок конфигурации не пройдена: %v", err)
		}
		if err := utils.WriteFileAtomic(originPath, localBody, 0644); err != nil {
			return safeId, err
		}
	} else {
		fetchURL := remoteURL
		if fetchURL == "" {
			fetchURL = sourceKey
		}
		err = downloader.FetchSmallFileAtomic(ctx, downloader.Options{
			URLs:      []string{fetchURL},
			DestPath:  originPath,
			UserAgent: userAgent,
			Headers:   sys.SubscriptionHeaders(),
			MaxBytes:  50 * 1024 * 1024,
			Strategy: func() downloader.DownloadStrategy {
				var pUrl string
				if IsRunning() {
					if netCfg, err := GetNetworkConfig(); err == nil && netCfg.MixedPort != 0 {
						pUrl = fmt.Sprintf("http://127.0.0.1:%d", netCfg.MixedPort)
					}
				}
				return downloader.DownloadStrategy{
					ProxyURL:    pUrl,
					PreferProxy: pUrl != "",
				}
			},
			InsecureSkipVerify: true,
			OnResponse: func(resp *http.Response) {

				if info := resp.Header.Get("Subscription-Userinfo"); info != "" {
					upload, download, total, expire = parseSubUserInfo(info)
				}

				headerName = parseSubProfileName(resp.Header)
			},
			Validator: func(tmpPath string) error {

				data, err := os.ReadFile(tmpPath)
				if err != nil {
					return err
				}
				if err := StrictVerifyClashConfig(data); err != nil {
					return fmt.Errorf("проверка конфигурации подписки не пройдена: %v (возможно, скачалась веб-страница, HTML или мусор)", err)
				}
				if err := ValidateClashReferencesBytes(data); err != nil {
					return fmt.Errorf("проверка ссылок конфигурации подписки не пройдена: %v", err)
				}
				return nil
			},
		})

		if err != nil {

			if _, statErr := os.Stat(originPath + ".bak"); statErr == nil {
				_ = os.Rename(originPath+".bak", originPath)
			}
			return safeId, err
		}
	}

	err = WithRuleStorageLock(func() error {
		originData, readErr := os.ReadFile(originPath)
		if readErr != nil {
			return readErr
		}
		if writeErr := utils.WriteFileAtomic(workingPath, originData, 0644); writeErr != nil {
			return fmt.Errorf("не удалось перезаписать рабочий файл: %w", writeErr)
		}

		if err := EnsureEmptyOverlay(safeId); err != nil {
			return fmt.Errorf("не удалось инициализировать конфигурацию правил: %w", err)
		}

		return nil
	})

	if err != nil {

		if _, statErr := os.Stat(originPath + ".bak"); statErr == nil {
			_ = os.Rename(originPath+".bak", originPath)
		}
		return safeId, err
	}

	_ = os.Remove(originPath + ".bak")

	resolvedName := strings.TrimSpace(name)
	if resolvedName == "" {
		resolvedName = strings.TrimSpace(headerName)
	}
	if resolvedName == "" {
		if base := filepath.Base(sourceKey); base != "" && base != "." && looksLikeFilesystemPath(sourceKey) {
			resolvedName = strings.TrimSuffix(base, filepath.Ext(base))
			if resolvedName == "" {
				resolvedName = base
			}
		} else {
			resolvedName = "Подписка"
		}
	}

	IndexLock.Lock()
	found := false
	for i, item := range SubIndex {
		if item.ID == safeId {
			SubIndex[i].Upload = upload
			SubIndex[i].Download = download
			SubIndex[i].Total = total
			SubIndex[i].Expire = expire
			SubIndex[i].Updated = time.Now().Unix()
			if sourceKey != "" {
				SubIndex[i].URL = sourceKey
			}

			if strings.TrimSpace(name) == "" && strings.TrimSpace(headerName) != "" {
				SubIndex[i].Name = strings.TrimSpace(headerName)
			}
			found = true
			break
		}
	}
	if !found {
		SubIndex = append(SubIndex, SubIndexItem{
			ID:       safeId,
			Name:     resolvedName,
			URL:      sourceKey, // original input (http URL or local path to .txt)
			Type:     "remote",
			Upload:   upload,
			Download: download,
			Total:    total,
			Expire:   expire,
			Updated:  time.Now().Unix(),
		})
	}
	IndexLock.Unlock()

	return safeId, SaveIndex()
}

func RenameConfig(id, newName string) error {
	IndexLock.Lock()
	for i, item := range SubIndex {
		if item.ID == id {
			SubIndex[i].Name = newName
			break
		}
	}
	IndexLock.Unlock()

	return SaveIndex()
}

func DeleteConfig(id string) error {

	safeId, err := utils.SanitizeFilename(id)
	if err != nil {
		return err
	}

	dir := utils.GetSubscriptionsDir()
	yamlPath := filepath.Join(dir, safeId+".yaml")
	rulesPath := filepath.Join(dir, safeId+"_rules.json")
	originPath := filepath.Join(dir, "origin", safeId+".yaml")
	overlayPath := filepath.Join(dir, safeId+"_overlay.json")

	var removeErr error
	WithRuleStorageLock(func() error {
		if err := os.Remove(yamlPath); err != nil && !os.IsNotExist(err) {
			removeErr = fmt.Errorf("не удалось удалить файл конфигурации: возможно, он занят ядром; остановите прокси и повторите: %v", err)
			return nil
		}

		_ = os.Remove(rulesPath)
		_ = os.Remove(originPath)
		_ = os.Remove(overlayPath)
		return nil
	})
	if removeErr != nil {
		return removeErr
	}

	IndexLock.Lock()
	for i, item := range SubIndex {
		if item.ID == safeId {
			SubIndex = append(SubIndex[:i], SubIndex[i+1:]...)
			break
		}
	}
	IndexLock.Unlock()

	return SaveIndex()
}

func ReloadConfig() error {
	return doKernelRequest(
		context.Background(),
		http.MethodPut,
		"/configs?force=true",
		nil,
		http.StatusOK,
		http.StatusNoContent,
	)
}

func StrictVerifyClashConfig(data []byte) error {
	var root map[string]interface{}

	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("не удалось разобрать файл: это не корректный YAML (возможно, скачалась веб-страница, HTML или мусор)")
	}

	if len(root) == 0 {
		return fmt.Errorf("формат отклонён: файл конфигурации пуст")
	}

	hasProxies := root["proxies"] != nil || root["Proxy"] != nil
	hasProxyGroups := root["proxy-groups"] != nil || root["Proxy Group"] != nil
	hasProxyProviders := root["proxy-providers"] != nil

	if !hasProxies && !hasProxyGroups && !hasProxyProviders {
		return fmt.Errorf("формат отклонён: не найдены proxies или proxy-groups. Это не стандартный файл подписки Clash")
	}

	if proxiesNode := root["proxies"]; proxiesNode != nil {
		proxiesList, ok := proxiesNode.([]interface{})
		if !ok {
			return fmt.Errorf("критическая ошибка структуры: [proxies] должен быть списком узлов (Array)")
		}

		if len(proxiesList) > 0 {
			firstProxy, isMap := proxiesList[0].(map[string]interface{})
			if !isMap {
				return fmt.Errorf("критическая ошибка структуры: элементы списка [proxies] должны быть объектами узлов (Object)")
			}

			requiredKeys := []string{"name", "type", "server", "port"}
			for _, key := range requiredKeys {
				if _, exists := firstProxy[key]; !exists {
					return fmt.Errorf("семантическая проверка не пройдена: у прокси-узла отсутствует обязательное поле Clash [%s]", key)
				}
			}
		}
	}

	if groupsNode := root["proxy-groups"]; groupsNode != nil {
		groupsList, ok := groupsNode.([]interface{})
		if !ok {
			return fmt.Errorf("критическая ошибка структуры: [proxy-groups] должен быть списком групп (Array)")
		}

		if len(groupsList) > 0 {
			firstGroup, isMap := groupsList[0].(map[string]interface{})
			if !isMap {
				return fmt.Errorf("критическая ошибка структуры: элементы [proxy-groups] должны быть объектами (Object)")
			}

			if _, ok := firstGroup["name"]; !ok {
				return fmt.Errorf("семантическая проверка не пройдена: у группы отсутствует обязательное поле [name]")
			}
			if _, ok := firstGroup["type"]; !ok {
				return fmt.Errorf("семантическая проверка не пройдена: у группы отсутствует обязательное поле [type]")
			}
		}
	}

	return nil
}

func ImportLocalConfig(srcPath, name string) (string, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}

	if err := StrictVerifyClashConfig(data); err != nil {
		return "", err
	}

	if err := ValidateClashReferencesBytes(data); err != nil {
		return "", err
	}

	id := fmt.Sprintf("%d", time.Now().UnixMilli())
	safeId, _ := utils.SanitizeFilename(id)

	originPath := filepath.Join(OriginDir(), safeId+".yaml")
	workingPath := filepath.Join(SubscriptionsDir(), safeId+".yaml")

	os.MkdirAll(filepath.Dir(originPath), 0755)

	if err := WithRuleStorageLock(func() error {
		if err := utils.WriteFileAtomic(originPath, data, 0644); err != nil {
			return err
		}
		if err := utils.WriteFileAtomic(workingPath, data, 0644); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}

	IndexLock.Lock()
	SubIndex = append(SubIndex, SubIndexItem{
		ID:      safeId,
		Name:    name,
		URL:     "",
		Type:    "local",
		Updated: time.Now().Unix(),
	})
	IndexLock.Unlock()

	return safeId, SaveIndex()
}
