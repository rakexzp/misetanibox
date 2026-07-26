package xrayconv

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Разбор «классической» подписки — base64 со списком ссылок vless:// / vmess:// /
// trojan:// / ss:// / hysteria2://. Конвертер XrayMi умеет только Xray-JSON, а этот
// формат отдаёт большинство панелей (Remnawave, Marzban и т.п.).

// decodeMaybeBase64 разворачивает base64, если весь ввод им закодирован.
func decodeMaybeBase64(raw []byte) []byte {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return raw
	}
	// уже похоже на список ссылок или YAML — не трогаем
	if strings.Contains(s, "://") || strings.Contains(s, "proxies:") {
		return raw
	}
	// пробуем оба варианта base64 (std и url-safe, с/без паддинга)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if dec, err := enc.DecodeString(strings.ReplaceAll(s, "\n", "")); err == nil && strings.Contains(string(dec), "://") {
			return dec
		}
	}
	return raw
}

// LooksLikeURIList — это список прокси-ссылок (в т.ч. под base64)?
func LooksLikeURIList(raw []byte) bool {
	s := string(decodeMaybeBase64(raw))
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		for _, p := range []string{"vless://", "vmess://", "trojan://", "ss://", "hysteria2://", "hy2://", "tuic://"} {
			if strings.HasPrefix(l, p) {
				return true
			}
		}
	}
	return false
}

// URIListToProxies парсит ссылки в прокси-объекты mihomo.
func URIListToProxies(raw []byte) ([]map[string]interface{}, error) {
	s := string(decodeMaybeBase64(raw))
	var proxies []map[string]interface{}
	names := map[string]int{}

	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var (
			p   map[string]interface{}
			err error
		)
		switch {
		case strings.HasPrefix(line, "vless://"):
			p, err = parseVLESS(line)
		case strings.HasPrefix(line, "vmess://"):
			p, err = parseVMess(line)
		case strings.HasPrefix(line, "trojan://"):
			p, err = parseTrojan(line)
		case strings.HasPrefix(line, "ss://"):
			p, err = parseSS(line)
		case strings.HasPrefix(line, "hysteria2://"), strings.HasPrefix(line, "hy2://"):
			p, err = parseHysteria2(line)
		default:
			continue // tuic и прочее пока пропускаем, а не роняем всю подписку
		}
		if err != nil || p == nil {
			continue
		}
		// уникализируем имена
		name, _ := p["name"].(string)
		if name == "" {
			name = fmt.Sprintf("%v:%v", p["server"], p["port"])
		}
		if n := names[name]; n > 0 {
			name = fmt.Sprintf("%s (%d)", name, n+1)
		}
		names[name]++
		p["name"] = name
		proxies = append(proxies, p)
	}
	if len(proxies) == 0 {
		return nil, fmt.Errorf("в подписке нет распознанных ссылок")
	}
	return proxies, nil
}

// URIListToMihomoYAML собирает из ссылок полноценный clash-YAML.
func URIListToMihomoYAML(raw []byte) (string, error) {
	proxies, err := URIListToProxies(raw)
	if err != nil {
		return "", err
	}
	names := make([]interface{}, 0, len(proxies))
	for _, p := range proxies {
		names = append(names, p["name"])
	}
	proxiesAny := make([]interface{}, len(proxies))
	for i, p := range proxies {
		proxiesAny[i] = p
	}
	doc := map[string]interface{}{
		"mixed-port": 7890,
		"mode":       "rule",
		"proxies":    proxiesAny,
		"proxy-groups": []interface{}{
			map[string]interface{}{"name": "PROXY", "type": "select", "proxies": names},
		},
		"rules": []interface{}{"MATCH,PROXY"},
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return "# Сконвертировано из ссылок подписки клиентом Misetanibox\n" + string(out), nil
}

func frag(u *url.URL) string {
	if u.Fragment != "" {
		if d, err := url.QueryUnescape(u.Fragment); err == nil {
			return d
		}
		return u.Fragment
	}
	return ""
}

func portInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// applyStream заполняет транспорт (tls/reality/network) по query-параметрам xray-ссылки.
func applyStream(p map[string]interface{}, q url.Values) {
	security := q.Get("security")
	sni := q.Get("sni")
	if sni == "" {
		sni = q.Get("peer")
	}
	if security == "tls" || security == "reality" {
		p["tls"] = true
		if sni != "" {
			p["servername"] = sni
		}
		if fp := q.Get("fp"); fp != "" {
			p["client-fingerprint"] = fp
		}
		if alpn := q.Get("alpn"); alpn != "" {
			p["alpn"] = strings.Split(alpn, ",")
		}
	}
	if security == "reality" {
		ro := map[string]interface{}{}
		if pbk := q.Get("pbk"); pbk != "" {
			ro["public-key"] = pbk
		}
		if sid := q.Get("sid"); sid != "" {
			ro["short-id"] = sid
		}
		p["reality-opts"] = ro
	}

	net := q.Get("type")
	switch net {
	case "ws":
		p["network"] = "ws"
		opts := map[string]interface{}{}
		if path := q.Get("path"); path != "" {
			opts["path"] = path
		}
		if host := q.Get("host"); host != "" {
			opts["headers"] = map[string]interface{}{"Host": host}
		}
		p["ws-opts"] = opts
	case "grpc":
		p["network"] = "grpc"
		if sn := q.Get("serviceName"); sn != "" {
			p["grpc-opts"] = map[string]interface{}{"grpc-service-name": sn}
		}
	case "http":
		p["network"] = "http"
	case "", "tcp", "raw":
		// tcp — сеть по умолчанию, поле не нужно
	default:
		p["network"] = net
	}
}

func parseVLESS(line string) (map[string]interface{}, error) {
	u, err := url.Parse(line)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	p := map[string]interface{}{
		"name":   frag(u),
		"type":   "vless",
		"server": u.Hostname(),
		"port":   portInt(u.Port()),
		"uuid":   u.User.Username(),
		"udp":    true,
	}
	if flow := q.Get("flow"); flow != "" {
		p["flow"] = flow
	}
	applyStream(p, q)
	return p, nil
}

func parseTrojan(line string) (map[string]interface{}, error) {
	u, err := url.Parse(line)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	p := map[string]interface{}{
		"name":     frag(u),
		"type":     "trojan",
		"server":   u.Hostname(),
		"port":     portInt(u.Port()),
		"password": u.User.Username(),
		"udp":      true,
	}
	if sni := q.Get("sni"); sni != "" {
		p["sni"] = sni
	}
	if q.Get("allowInsecure") == "1" {
		p["skip-cert-verify"] = true
	}
	applyStream(p, q)
	delete(p, "servername") // у trojan поле называется sni, не servername
	if sni := q.Get("sni"); sni != "" {
		p["sni"] = sni
	}
	return p, nil
}

func parseHysteria2(line string) (map[string]interface{}, error) {
	u, err := url.Parse(line)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	p := map[string]interface{}{
		"name":     frag(u),
		"type":     "hysteria2",
		"server":   u.Hostname(),
		"port":     portInt(u.Port()),
		"password": u.User.Username(),
	}
	if sni := q.Get("sni"); sni != "" {
		p["sni"] = sni
	}
	if q.Get("insecure") == "1" {
		p["skip-cert-verify"] = true
	}
	if obfs := q.Get("obfs"); obfs != "" {
		p["obfs"] = obfs
		if op := q.Get("obfs-password"); op != "" {
			p["obfs-password"] = op
		}
	}
	return p, nil
}

func parseSS(line string) (map[string]interface{}, error) {
	// ss://base64(method:pass)@host:port#name  ИЛИ  ss://base64(method:pass@host:port)#name
	rest := strings.TrimPrefix(line, "ss://")
	name := ""
	if i := strings.Index(rest, "#"); i >= 0 {
		name, _ = url.QueryUnescape(rest[i+1:])
		rest = rest[:i]
	}
	if i := strings.Index(rest, "?"); i >= 0 {
		rest = rest[:i]
	}

	var method, password, host, port string
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		// userinfo может быть base64 или открытым method:pass
		userinfo := rest[:at]
		hostport := rest[at+1:]
		if dec := b64Try(userinfo); dec != "" {
			userinfo = dec
		}
		mp := strings.SplitN(userinfo, ":", 2)
		if len(mp) == 2 {
			method, password = mp[0], mp[1]
		}
		if h, pt, ok := splitHostPort(hostport); ok {
			host, port = h, pt
		}
	} else {
		// весь блок в base64
		dec := b64Try(rest)
		if at := strings.LastIndex(dec, "@"); at >= 0 {
			mp := strings.SplitN(dec[:at], ":", 2)
			if len(mp) == 2 {
				method, password = mp[0], mp[1]
			}
			if h, pt, ok := splitHostPort(dec[at+1:]); ok {
				host, port = h, pt
			}
		}
	}
	if host == "" || method == "" {
		return nil, fmt.Errorf("не удалось разобрать ss")
	}
	return map[string]interface{}{
		"name":     name,
		"type":     "ss",
		"server":   host,
		"port":     portInt(port),
		"cipher":   method,
		"password": password,
		"udp":      true,
	}, nil
}

func parseVMess(line string) (map[string]interface{}, error) {
	dec := b64Try(strings.TrimPrefix(line, "vmess://"))
	if dec == "" {
		return nil, fmt.Errorf("vmess: не base64")
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(dec), &v); err != nil {
		return nil, err
	}
	str := func(k string) string {
		switch x := v[k].(type) {
		case string:
			return x
		case float64:
			return strconv.Itoa(int(x))
		}
		return ""
	}
	p := map[string]interface{}{
		"name":    str("ps"),
		"type":    "vmess",
		"server":  str("add"),
		"port":    portInt(str("port")),
		"uuid":    str("id"),
		"alterId": portInt(str("aid")),
		"cipher":  orStr(str("scy"), "auto"),
		"udp":     true,
	}
	if str("tls") == "tls" {
		p["tls"] = true
		if sni := str("sni"); sni != "" {
			p["servername"] = sni
		}
	}
	switch str("net") {
	case "ws":
		p["network"] = "ws"
		opts := map[string]interface{}{}
		if path := str("path"); path != "" {
			opts["path"] = path
		}
		if host := str("host"); host != "" {
			opts["headers"] = map[string]interface{}{"Host": host}
		}
		p["ws-opts"] = opts
	case "grpc":
		p["network"] = "grpc"
		if sn := str("path"); sn != "" {
			p["grpc-opts"] = map[string]interface{}{"grpc-service-name": sn}
		}
	}
	return p, nil
}

func b64Try(s string) string {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if dec, err := enc.DecodeString(s); err == nil {
			return string(dec)
		}
	}
	return ""
}

func splitHostPort(s string) (host, port string, ok bool) {
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return "", "", false
}

func orStr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
