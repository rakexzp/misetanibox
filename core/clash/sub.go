package clash

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"goclashz/core/downloader"
	"goclashz/core/sys"
	"goclashz/core/utils"
	"goclashz/core/xrayconv"

	"gopkg.in/yaml.v3"
)

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

// Заголовки, которыми панель может отдать запасные адреса подписки.
// Значение — список ссылок через запятую/точку с запятой/перевод строки.
var subFallbackHeaders = []string{
	"x-sub-fallback",
	"x-fallback-url",
	"profile-fallback-url",
}

// parseSubFallbacks достаёт запасные ссылки из заголовков ответа подписки.
// Так фолбек настраивается на стороне панели и не требует ничего от пользователя.
func parseSubFallbacks(h http.Header) []string {
	out := make([]string, 0, 4)
	seen := map[string]bool{}
	for _, name := range subFallbackHeaders {
		for _, raw := range h.Values(name) {
			for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
				return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
			}) {
				u := strings.TrimSpace(part)
				if u == "" || seen[u] {
					continue
				}
				if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
					continue // мусор/относительные пути игнорируем
				}
				seen[u] = true
				out = append(out, u)
			}
		}
	}
	return out
}

// Заголовки со ссылкой на личный кабинет / продление подписки.
// Порядок важен: явно заданная ссылка продления побеждает автоматическую
// profile-web-page-url, которую панель отдаёт сама (она ведёт на страницу подписки).
var subWebPageHeaders = []string{
	"x-renew-url",
	"x-subscription-web-page",
	"profile-web-page-url",
}

// parseSubWebPage достаёт ссылку на страницу продления из заголовков ответа.
func parseSubWebPage(h http.Header) string {
	for _, name := range subWebPageHeaders {
		u := strings.TrimSpace(h.Get(name))
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			return u
		}
	}
	return ""
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

// SubFetchOptions — дополнительные параметры загрузки подписки.
type SubFetchOptions struct {
	// Запасные ссылки: идут в загрузку вместе с основной, побеждает первая ответившая.
	FallbackURLs []string
	// Свои заголовки: перекрывают стандартные (x-hwid и т.п.).
	Headers map[string]string
	// Convert: если подписка отдаёт Xray-JSON вместо clash-YAML — сконвертировать.
	Convert bool
}

// resolveFallbackURL достраивает запасной адрес.
//
// Заголовок с запасными адресами панель отдаёт ОДИН на всех пользователей, поэтому
// персональный токен в него положить нельзя. Договорённость: если в запасном адресе
// указан только хост (без пути) — берём путь и параметры из основной ссылки юзера.
// Тогда одна строка «https://зеркало» обслуживает всю базу.
// Если путь указан явно — используем адрес как есть.
func resolveFallbackURL(primary, fallback string) string {
	fb, err := neturl.Parse(fallback)
	if err != nil || fb.Host == "" {
		return ""
	}
	if strings.Trim(fb.Path, "/") != "" || fb.RawQuery != "" {
		return fallback
	}
	pr, err := neturl.Parse(primary)
	if err != nil || pr.Path == "" {
		return fallback
	}
	fb.Path = pr.Path
	fb.RawQuery = pr.RawQuery
	return fb.String()
}

// subFetchURLs — основная ссылка + запасные, без пустых и дублей.
func subFetchURLs(url string, fallbacks []string) []string {
	out := make([]string, 0, len(fallbacks)+1)
	seen := map[string]bool{}
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	add(url)
	for _, f := range fallbacks {
		add(resolveFallbackURL(url, strings.TrimSpace(f)))
	}
	return out
}

// subFetchHeaders — стандартные заголовки устройства + пользовательские (последние в приоритете).
func subFetchHeaders(custom map[string]string) map[string]string {
	h := sys.SubscriptionHeaders()
	if h == nil {
		h = map[string]string{}
	}
	for k, v := range custom {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		h[k] = v
	}
	return h
}

func DownloadSub(ctx context.Context, name, url, existingId, userAgent string, opts *SubFetchOptions) (string, error) {
	if opts == nil {
		opts = &SubFetchOptions{}
	}
	id := existingId
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().UnixMilli())
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
	var headerFallbacks []string
	var headerWebPage string

	if _, statErr := os.Stat(originPath); statErr == nil {
		originData, _ := os.ReadFile(originPath)
		_ = utils.WriteFileAtomic(originPath+".bak", originData, 0644)
	}

	err = downloader.FetchSmallFileAtomic(ctx, downloader.Options{
		URLs:      subFetchURLs(url, opts.FallbackURLs),
		DestPath:  originPath,
		UserAgent: userAgent,
		Headers:   subFetchHeaders(opts.Headers),
		MaxBytes:  50 * 1024 * 1024,
		Strategy: func() downloader.DownloadStrategy {
			var pUrl string
			if IsRunning() {
				if p := GetProxyPort(); p != 0 {
					pUrl = fmt.Sprintf("http://127.0.0.1:%d", p)
				}
			}
			return downloader.DownloadStrategy{
				ProxyURL:    pUrl,
				PreferProxy: pUrl != "",
			}
		},
		InsecureSkipVerify: true,
		Transform: func(tmpPath string) error {
			if !opts.Convert {
				return nil
			}
			raw, err := os.ReadFile(tmpPath)
			if err != nil {
				return err
			}
			// уже валидный clash — не трогаем
			if StrictVerifyClashConfig(raw) == nil {
				return nil
			}
			if !xrayconv.LooksLikeXray(raw) {
				return nil // не xray — пусть Validator ругнётся как обычно
			}
			yaml, cerr := xrayconv.ToMihomoYAML(raw)
			if cerr != nil {
				return fmt.Errorf("конвертация Xray→mihomo не удалась: %w", cerr)
			}
			return utils.WriteFileAtomic(tmpPath, []byte(yaml), 0644)
		},
		OnResponse: func(resp *http.Response) {

			if info := resp.Header.Get("Subscription-Userinfo"); info != "" {
				upload, download, total, expire = parseSubUserInfo(info)
			}

			headerName = parseSubProfileName(resp.Header)
			if fb := parseSubFallbacks(resp.Header); len(fb) > 0 {
				headerFallbacks = fb
			}
			if wp := parseSubWebPage(resp.Header); wp != "" {
				headerWebPage = wp
			}
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
		resolvedName = "Подписка"
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

			if strings.TrimSpace(name) == "" && strings.TrimSpace(headerName) != "" {
				SubIndex[i].Name = strings.TrimSpace(headerName)
			}
			// фолбеки приходят из заголовка панели; если панель их не прислала — оставляем прежние
			if len(headerFallbacks) > 0 {
				SubIndex[i].FallbackURLs = headerFallbacks
			}
			if headerWebPage != "" {
				SubIndex[i].WebPageURL = headerWebPage
			}
			SubIndex[i].Convert = opts.Convert
			SubIndex[i].Headers = opts.Headers
			found = true
			break
		}
	}
	if !found {
		SubIndex = append(SubIndex, SubIndexItem{
			ID:       safeId,
			Name:     resolvedName,
			URL:      url,
			Type:     "remote",
			Upload:   upload,
			Download: download,
			Total:    total,
			Expire:   expire,
			Updated:  time.Now().Unix(),

			FallbackURLs: headerFallbacks,
			WebPageURL:   headerWebPage,
			Headers:      opts.Headers,
			Convert:      opts.Convert,
		})
	}
	IndexLock.Unlock()

	return safeId, SaveIndex()
}

// SaveSubHeaders сохраняет свои заголовки запроса подписки, не перезакачивая её.
// Запасные ссылки здесь намеренно не трогаются — их присылает панель заголовком ответа.
func SaveSubHeaders(id string, headers map[string]string) error {
	IndexLock.Lock()
	found := false
	for i, item := range SubIndex {
		if item.ID == id {
			SubIndex[i].Headers = headers
			found = true
			break
		}
	}
	IndexLock.Unlock()
	if !found {
		return fmt.Errorf("подписка не найдена")
	}
	return SaveIndex()
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

// SubProbe — результат разведки ссылки подписки.
type SubProbe struct {
	Kind      string `json:"kind"`      // "clash" | "xray" | "unknown"
	NodeCount int    `json:"nodeCount"` // сколько узлов удастся получить
	Error     string `json:"error,omitempty"`
}

// ProbeSubURL скачивает подписку один раз и определяет её формат — чтобы предложить
// конвертацию, когда панель отдаёт Xray-JSON вместо clash-YAML.
func ProbeSubURL(ctx context.Context, url, userAgent string, headers map[string]string) SubProbe {
	tmp, err := os.CreateTemp("", "mise-probe-*")
	if err != nil {
		return SubProbe{Kind: "unknown", Error: err.Error()}
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	ferr := downloader.FetchSmallFileAtomic(ctx, downloader.Options{
		URLs:               []string{url},
		DestPath:           tmpPath,
		UserAgent:          userAgent,
		Headers:            subFetchHeaders(headers),
		MaxBytes:           50 * 1024 * 1024,
		InsecureSkipVerify: true,
	})
	if ferr != nil {
		return SubProbe{Kind: "unknown", Error: ferr.Error()}
	}
	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return SubProbe{Kind: "unknown", Error: err.Error()}
	}

	if StrictVerifyClashConfig(raw) == nil {
		return SubProbe{Kind: "clash", NodeCount: countClashProxies(raw)}
	}
	if xrayconv.LooksLikeXray(raw) {
		n := 0
		if y, cerr := xrayconv.ToMihomoYAML(raw); cerr == nil {
			n = strings.Count(y, "\n  - name:") + strings.Count(y, "\n  - {name:")
		}
		return SubProbe{Kind: "xray", NodeCount: n}
	}
	return SubProbe{Kind: "unknown"}
}

func countClashProxies(data []byte) int {
	var doc struct {
		Proxies []interface{} `yaml:"proxies"`
	}
	_ = yaml.Unmarshal(data, &doc)
	return len(doc.Proxies)
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
