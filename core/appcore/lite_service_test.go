package appcore

import (
	"encoding/json"
	"strings"
	"testing"

	"goclashz/core/clash"
)

func TestLiteProfilesDoNotExposeSubscriptionCredentials(t *testing.T) {
	clash.IndexLock.Lock()
	before := clash.SubIndex
	clash.SubIndex = []clash.SubIndexItem{{ID: "p", Name: "Profile", Type: "remote", URL: "https://example.com/private-token", Headers: map[string]string{"Authorization": "secret-header"}, FallbackURLs: []string{"https://fallback/private-token"}, WebPageURL: "https://renew/private-token"}}
	clash.IndexLock.Unlock()
	t.Cleanup(func() {
		clash.IndexLock.Lock()
		clash.SubIndex = before
		clash.IndexLock.Unlock()
	})
	service := NewLiteService(nil)
	data, err := json.Marshal(service.Profiles())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-token", "secret-header", "url", "headers", "fallback", "webPage"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("DTO leaked %q: %s", forbidden, data)
		}
	}
	if !strings.Contains(string(data), `"id":"p"`) {
		t.Fatalf("missing ID: %s", data)
	}
}

func TestLiteServersDoNotExposeProxyCredentials(t *testing.T) {
	servers, err := parseLiteServers("p", []byte("proxies:\n  - {name: Node, type: ss, server: hidden.example, password: secret-password}\nproxy-providers:\n  p: {url: 'https://hidden.example/token'}\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(servers)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"hidden.example", "secret-password", "password", "token"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("DTO leaked %q", forbidden)
		}
	}
}
