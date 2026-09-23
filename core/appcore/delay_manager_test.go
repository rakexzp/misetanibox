package appcore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"goclashz/core/clash"
	"goclashz/core/logger"
)

type delayRecordedEvent struct {
	name string
	args []any
}

type delayRecordingSink struct {
	mu     sync.Mutex
	events []delayRecordedEvent
}

func (s *delayRecordingSink) Emit(name string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, delayRecordedEvent{name, args})
}

func mockDelayAPI(t *testing.T) *httptest.Server {
	t.Helper()
	oldURL := clash.APIURL("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/configs":
			fmt.Fprint(w, `{"mode":"rule"}`)
		case "/proxies":
			fmt.Fprint(w, `{"proxies":{
				"DIRECT":{"type":"Direct"},
				"leaf/Россия":{"type":"Vless"},
				"leaf-2":{"type":"Vless"},
				"Россия":{"type":"Fallback","now":"leaf/Россия","all":["leaf/Россия","leaf-2"]},
				"Fastest":{"type":"URLTest","now":"Россия","all":["Россия"]},
				"Российские сайты":{"type":"Selector","now":"Fastest","all":["DIRECT","Fastest","Россия"]}
			}}`)
		case "/proxies/leaf/Россия/delay":
			if r.URL.Query().Get("timeout") != "8000" || r.URL.Query().Get("url") == "" {
				t.Errorf("invalid delay query: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"delay":42}`)
		default:
			t.Errorf("unexpected API route: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	clash.UpdateAPIBaseURL(server.URL)
	t.Cleanup(func() {
		clash.UpdateAPIBaseURL(oldURL)
		clash.ResetAPIConnections()
		server.Close()
	})
	return server
}

func TestDelayFallbackURLTestTopologyAndEvents(t *testing.T) {
	mockDelayAPI(t)
	topo, err := buildDelayTopology()
	if err != nil {
		t.Fatal(err)
	}
	targets := topo.normalizeTargets([]string{"DIRECT", "Fastest", "Россия"})
	if !reflect.DeepEqual(targets, []string{"leaf/Россия"}) {
		t.Fatalf("targets: %v", targets)
	}
	for _, group := range []string{"Россия", "Fastest", "Российские сайты"} {
		if topo.SelectedLeafByGroup[group] != "leaf/Россия" {
			t.Fatalf("selected leaf of %s: %q", group, topo.SelectedLeafByGroup[group])
		}
	}
	sink := &delayRecordingSink{}
	manager := NewDelayTestManager(sink, &Controller{})
	manager.runBatch(context.Background(), topo, targets, manualDelayOptions())
	updates := map[string]bool{}
	starts := 0
	for _, event := range sink.events {
		switch event.name {
		case "proxy-test-start":
			starts++
			if event.args[0] != "leaf/Россия" {
				t.Fatalf("start: %v", event.args)
			}
		case "proxy-delay-update":
			data := event.args[0].(map[string]interface{})
			name := data["name"].(string)
			if data["delay"] != 42 || data["status"] != "success" {
				t.Fatalf("result: %v", data)
			}
			if name != "leaf/Россия" && (data["source"] != "group-derived" || data["from"] != "leaf/Россия") {
				t.Fatalf("derived result: %v", data)
			}
			updates[name] = true
		}
	}
	if starts != 1 || len(updates) != 4 {
		t.Fatalf("starts=%d updates=%v", starts, updates)
	}
	for _, name := range []string{"leaf/Россия", "Россия", "Fastest", "Российские сайты"} {
		if !updates[name] {
			t.Errorf("missing update: %s", name)
		}
	}
}

func TestDelayBatchPreparationFailureFinishesAndUnlocks(t *testing.T) {
	sink := &delayRecordingSink{}
	controller := &Controller{Behavior: &BehaviorStore{}}
	manager := NewDelayTestManager(sink, controller)
	for i := 0; i < 2; i++ {
		manager.TestAllProxies(context.Background(), []string{"Fastest", "Россия"})
		if manager.state != DelayIdle || manager.batchDone != nil || manager.batchCancel != nil {
			t.Fatal("failed preparation left the batch locked")
		}
	}
	if len(sink.events) != 2 {
		t.Fatalf("events: %v", sink.events)
	}
	for _, event := range sink.events {
		if event.name != "proxy-test-finished" || !strings.Contains(event.args[0].(string), "Сначала выберите") {
			t.Fatalf("missing actionable failure: %v", event)
		}
		if len(event.args) != 2 {
			t.Fatalf("missing completion status: %v", event)
		}
		if event.args[1].(map[string]string)["status"] != "error" {
			t.Fatalf("incorrect completion status: %v", event)
		}
	}
}

func TestDelaySinglePreparationFailureDoesNotLockNextRequest(t *testing.T) {
	manager := NewDelayTestManager(&delayRecordingSink{}, &Controller{Behavior: &BehaviorStore{}})
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := manager.TestProxy(ctx, "Россия")
		cancel()
		if err == nil || !strings.Contains(err.Error(), "Сначала выберите") {
			t.Fatalf("preparation error: %v", err)
		}
		if len(manager.activeSingles) != 0 || len(manager.sem) != 0 {
			t.Fatal("single request leaked slot")
		}
	}
}

func TestDelayPreparedBatchCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, stage, status string
		apiStatus           int
		targets             []string
	}{
		{"success", "probe", "success", 200, []string{"leaf"}},
		{"topology", "topology", "error", 401, []string{"leaf"}},
		{"empty group", "targets", "error", 200, []string{"empty"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldURL := clash.APIURL("")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.apiStatus != 200 {
					w.WriteHeader(tc.apiStatus)
					fmt.Fprint(w, "token=SECRET")
					return
				}
				switch r.URL.Path {
				case "/configs":
					fmt.Fprint(w, `{}`)
				case "/proxies":
					fmt.Fprint(w, `{"proxies":{"leaf":{"type":"Vless"},"empty":{"type":"Selector","all":[]}}}`)
				default:
					fmt.Fprint(w, `{"delay":42}`)
				}
			}))
			clash.UpdateAPIBaseURL(server.URL)
			defer func() { clash.UpdateAPIBaseURL(oldURL); clash.ResetAPIConnections(); server.Close() }()
			sink := &delayRecordingSink{}
			manager := NewDelayTestManager(sink, &Controller{})
			opts := manualDelayOptions()
			stage, err := manager.runPreparedBatch(context.Background(), tc.targets, opts)
			manager.finishBatch(opts, stage, err)
			event := sink.events[len(sink.events)-1]
			meta := event.args[1].(map[string]string)
			if stage != tc.stage || meta["status"] != tc.status || event.name != "proxy-test-finished" {
				t.Fatalf("completion: %v", event)
			}
			if tc.apiStatus == 401 && !strings.Contains(event.args[0].(string), "HTTP 401") {
				t.Fatal(event)
			}
		})
	}
}

func TestDelayCancellationCompletionAndRetry(t *testing.T) {
	sink := &delayRecordingSink{}
	manager := NewDelayTestManager(sink, &Controller{Behavior: &BehaviorStore{}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager.TestAllProxies(ctx, []string{"leaf"})
	if sink.events[0].args[1].(map[string]string)["status"] != "cancelled" || manager.state != DelayIdle {
		t.Fatal(sink.events)
	}
	_, err := manager.TestProxy(ctx, "leaf")
	if err == nil || err.Error() != "Тест задержки отменён" {
		t.Fatal(err)
	}
	manager.TestAllProxies(context.Background(), []string{"leaf"})
	if sink.events[1].args[1].(map[string]string)["status"] != "error" {
		t.Fatal(sink.events)
	}
}

func TestDelayDiagnosticsNeverExposeErrorBody(t *testing.T) {
	sink := &delayRecordingSink{}
	manager := NewDelayTestManager(sink, &Controller{})
	err := fmt.Errorf("mihomo delay failed: HTTP 503: https://private/sub/SECRET?token=SECRET\npassword: SECRET")
	manager.finishBatch(manualDelayOptions(), "topology", err)
	manager.emitDelayResult(nil, DelayResult{Name: "leaf", Status: "test-error", Err: err, Message: err.Error()})
	for _, event := range sink.events {
		text := fmt.Sprint(event.args)
		if strings.Contains(text, "SECRET") || !strings.Contains(text, "HTTP 503") {
			t.Fatalf("unsafe/unhelpful event: %s", text)
		}
	}
	logs := logger.AppLogs.Search("[DelayTest]")
	if len(logs) == 0 {
		t.Fatal("missing application diagnostic")
	}
	for _, entry := range logs {
		if strings.Contains(entry.Payload, "SECRET") {
			t.Fatal("error body leaked to application log")
		}
	}
}
