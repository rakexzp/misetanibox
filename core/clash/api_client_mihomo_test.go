package clash

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt in with MIHOMO_TEST_BINARY: no downloads, subscriptions or remote probes.
func TestGetProxyDelayRealMihomo(t *testing.T) {
	binary := os.Getenv("MIHOMO_TEST_BINARY")
	if binary == "" {
		t.Skip("set MIHOMO_TEST_BINARY to run local core integration")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(20 * time.Millisecond); w.WriteHeader(204) }))
	defer target.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	controller := listener.Addr().String()
	listener.Close()
	dir := t.TempDir()
	names := []string{"plain", "leaf/Россия", "hash#tag", "percent%tag", "question?tag", "Россия 🇷🇺", " spaced ", "plus+tag"}
	proxies := []map[string]string{}
	for _, name := range names {
		proxies = append(proxies, map[string]string{"name": name, "type": "direct"})
	}
	config := map[string]any{
		"external-controller": controller, "log-level": "silent", "proxies": proxies,
		"proxy-providers": map[string]any{"source/#?": map[string]any{"type": "file", "path": "./provider.yaml", "health-check": map[string]any{"enable": false}}},
		"proxy-groups": []map[string]any{
			{"name": "country", "type": "fallback", "proxies": []string{"leaf/Россия"}, "url": target.URL, "lazy": true},
			{"name": "nested", "type": "select", "proxies": []string{"country"}},
			{"name": "provided", "type": "select", "use": []string{"source/#?"}},
		}, "rules": []string{"MATCH,DIRECT"},
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "provider.yaml"), []byte(`{"proxies":[{"name":"provider/Россия","type":"direct"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-d", dir)
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); _ = cmd.Wait() }()
	old := APIURL("")
	UpdateAPIBaseURL(controller)
	defer func() { UpdateAPIBaseURL(old); ResetAPIConnections() }()
	ready := false
	for i := 0; i < 200; i++ {
		data, probeErr := doKernelGetWithContext[struct {
			Proxies map[string]json.RawMessage `json:"proxies"`
		}](ctx, "/proxies")
		providers, providerErr := doKernelGetWithContext[struct {
			Providers map[string]struct {
				Proxies []struct {
					Name string `json:"name"`
				} `json:"proxies"`
			} `json:"providers"`
		}](ctx, "/providers/proxies")
		if probeErr == nil && data.Proxies["plain"] != nil && providerErr == nil {
			for _, proxy := range providers.Providers["source/#?"].Proxies {
				if proxy.Name == "provider/Россия" {
					ready = true
				}
			}
		}
		if ready {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !ready {
		t.Fatal("core not ready")
	}
	// Establish that the core really requires the separate provider route.
	resp, err := http.Get(APIURL("/proxies/provider%2F%D0%A0%D0%BE%D1%81%D1%81%D0%B8%D1%8F/delay?timeout=1000&url=" + target.URL))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 404 || !strings.Contains(string(body), "Resource not found") {
		t.Fatalf("raw provider route: %d %s", resp.StatusCode, body)
	}
	for _, name := range append(names, "country", "nested", "provided", "provider/Россия") {
		t.Run(name, func(t *testing.T) {
			delay, err := GetProxyDelay(ctx, name, target.URL, 1000)
			if err != nil || delay <= 0 {
				t.Fatalf("%q delay=%d err=%v", name, delay, err)
			}
			t.Logf("%q delay=%d", name, delay)
		})
	}
	_, err = GetProxyDelay(ctx, "missing", target.URL, 1000)
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatal(fmt.Sprint("unknown proxy: ", err))
	}
}
