package clash

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetProxyDelayProviderOnly(t *testing.T) {
	old := APIURL("")
	defer func() { UpdateAPIBaseURL(old); ResetAPIConnections() }()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method %s", r.Method)
		}
		switch r.URL.EscapedPath() {
		case "/proxies/leaf%2F%D0%A0%D0%A4/delay":
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"Resource not found"}`)
		case "/providers/proxies":
			fmt.Fprint(w, `{"providers":{"source/#?":{"proxies":[{"name":"leaf/РФ"}]}}}`)
		case "/providers/proxies/source%2F%23%3F/leaf%2F%D0%A0%D0%A4/healthcheck":
			if r.URL.Query().Get("url") != "http://127.0.0.1/check" || r.URL.Query().Get("timeout") != "1000" {
				t.Error("lost delay query")
			}
			fmt.Fprint(w, `{"delay":42}`)
		default:
			t.Errorf("unexpected route %s", r.URL.EscapedPath())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	UpdateAPIBaseURL(server.URL)
	delay, err := GetProxyDelay(context.Background(), "leaf/РФ", "http://127.0.0.1/check", 1000)
	if err != nil || delay != 42 {
		t.Fatalf("delay=%d err=%v", delay, err)
	}
}

func TestGetProxyDelayUnknownOrAmbiguousProvider(t *testing.T) {
	for _, body := range []string{
		`{"providers":{}}`,
		`{"providers":{"a":{"proxies":[{"name":"leaf"}]},"b":{"proxies":[{"name":"leaf"}]}}}`,
		`{"providers":{"a":{"proxies":[{"name":" leaf "}]}}}`,
	} {
		t.Run(body, func(t *testing.T) {
			old := APIURL("")
			defer func() { UpdateAPIBaseURL(old); ResetAPIConnections() }()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/proxies/leaf/delay":
					w.WriteHeader(404)
				case "/providers/proxies":
					fmt.Fprint(w, body)
				default:
					t.Error("must not guess provider or normalize names")
					fmt.Fprint(w, `{"delay":42}`)
				}
			}))
			defer server.Close()
			UpdateAPIBaseURL(server.URL)
			_, err := GetProxyDelay(context.Background(), "leaf", "http://127.0.0.1/check", 1000)
			if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
				t.Fatal(err)
			}
		})
	}
}

func TestGetProxyDelayDoesNotFallbackOnOtherErrors(t *testing.T) {
	old := APIURL("")
	defer func() { UpdateAPIBaseURL(old); ResetAPIConnections() }()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/proxies/") {
			t.Error("unexpected provider lookup")
		}
		w.WriteHeader(503)
	}))
	defer server.Close()
	UpdateAPIBaseURL(server.URL)
	_, err := GetProxyDelay(context.Background(), "leaf", "http://127.0.0.1/check", 1000)
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatal(err)
	}
}
