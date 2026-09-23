package appcore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"goclashz/core/clash"
)

func TestDelayUniqueTargetSummary(t *testing.T) {
	for _, tc := range []struct {
		name, status              string
		good, zero, retry, cancel bool
		want                      DelayBatchSummary
	}{
		{name: "all HTTP 503", status: "error", want: DelayBatchSummary{4, 2, 2, 0, 2, 0}},
		{name: "partial", status: "partial", good: true, want: DelayBatchSummary{4, 2, 2, 1, 1, 0}},
		{name: "zero is not success", status: "error", zero: true, want: DelayBatchSummary{4, 2, 2, 0, 2, 0}},
		{name: "retry counts targets not attempts", status: "partial", good: true, retry: true, want: DelayBatchSummary{4, 2, 2, 1, 1, 0}},
		{name: "cancel skips pending targets", status: "cancelled", cancel: true, want: DelayBatchSummary{4, 2, 0, 0, 0, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldURL := clash.APIURL("")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/configs":
					fmt.Fprint(w, `{}`)
				case "/proxies":
					fmt.Fprint(w, `{"proxies":{"a":{"type":"Vless"},"b":{"type":"Vless"},"alias":{"type":"Fallback","now":"a","all":["a"]}}}`)
					if tc.cancel {
						cancel()
					}
				default:
					if tc.good && strings.Contains(r.URL.Path, "/a/") {
						fmt.Fprint(w, `{"delay":42}`)
					} else if tc.zero {
						fmt.Fprint(w, `{"delay":0}`)
					} else {
						w.WriteHeader(503)
						fmt.Fprint(w, "token=SECRET")
					}
				}
			}))
			clash.UpdateAPIBaseURL(server.URL)
			defer func() { clash.UpdateAPIBaseURL(oldURL); clash.ResetAPIConnections(); server.Close() }()
			sink := &delayRecordingSink{}
			manager := NewDelayTestManager(sink, &Controller{})
			opts := manualDelayOptions()
			opts.RetryFailed = tc.retry
			stage, summary, err := manager.runPreparedBatch(ctx, []string{"alias", "a", "b", "DIRECT"}, opts)
			if !reflect.DeepEqual(summary, tc.want) {
				t.Fatalf("summary=%+v want=%+v", summary, tc.want)
			}
			manager.finishBatch(opts, stage, err, summary)
			event := sink.events[len(sink.events)-1]
			if _, ok := event.args[0].(string); !ok {
				t.Fatal("legacy string missing")
			}
			meta := event.args[1].(map[string]interface{})
			if meta["status"] != tc.status {
				t.Fatalf("completion: %v", event)
			}
			for key, want := range map[string]int{"requested": tc.want.Requested, "targets": tc.want.Targets, "completed": tc.want.Completed, "success": tc.want.Success, "failed": tc.want.Failed, "skipped": tc.want.Skipped} {
				if meta[key] != want {
					t.Fatalf("%s=%v want=%d", key, meta[key], want)
				}
			}
			if strings.Contains(fmt.Sprint(sink.events), "SECRET") {
				t.Fatal("unsafe diagnostic")
			}
		})
	}
}
