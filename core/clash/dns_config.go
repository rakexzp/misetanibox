package clash

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var dohResolvers = []string{
	"https://cloudflare-dns.com/dns-query",
	"https://dns.google/resolve",
	"https://dns.quad9.net/dns-query",
}

type dohAnswer struct {
	Type int    `json:"type"`
	Data string `json:"data"`
}

type dohResponse struct {
	Status int         `json:"Status"`
	Answer []dohAnswer `json:"Answer"`
}

func ResolveConfigViaDNS(ctx context.Context, domain string) (string, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("пустой домен")
	}

	if strings.Contains(domain, "://") {
		if u, err := url.Parse(domain); err == nil && u.Host != "" {
			domain = u.Host
		}
	}

	client := &http.Client{Timeout: 8 * time.Second}
	var lastErr error

	for _, resolver := range dohResolvers {
		txts, err := queryTXT(ctx, client, resolver, domain)
		if err != nil {
			lastErr = err
			continue
		}

		best := ""
		for _, t := range txts {
			if len(t) > len(best) {
				best = t
			}
		}
		if best != "" {
			return best, nil
		}
		lastErr = fmt.Errorf("TXT-записи не найдены для %s", domain)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("не удалось получить TXT-запись")
	}
	return "", lastErr
}

func queryTXT(ctx context.Context, client *http.Client, resolver, domain string) ([]string, error) {
	q := fmt.Sprintf("%s?name=%s&type=TXT", resolver, url.QueryEscape(domain))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, q, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH %s: HTTP %d", resolver, resp.StatusCode)
	}

	var parsed dohResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if parsed.Status != 0 {
		return nil, fmt.Errorf("DoH статус %d", parsed.Status)
	}

	var out []string
	for _, a := range parsed.Answer {
		if a.Type != 16 {
			continue
		}
		out = append(out, cleanTXTData(a.Data))
	}
	return out, nil
}

func cleanTXTData(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "\"") {
		return raw
	}
	var b strings.Builder
	inQuote := false
	escaped := false
	for _, r := range raw {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuote = !inQuote
		case inQuote:
			b.WriteRune(r)
		}
	}
	return b.String()
}
