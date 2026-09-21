package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAppVersionStrict(t *testing.T) {
	for _, s := range []string{"garbage", "release 1.2.3", "1.2.3.4", "v01.2.3", "1.2.3-junk!"} {
		if _, e := CompareAppVersion(s, "1.2.3"); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
	for _, tc := range []struct {
		a, b string
		cmp  int
	}{{"1.2.3-rc.2", "1.2.3-rc.10", -1}, {"v1.2.3", "1.2.3-rc.1", 1}, {"1.2.3+build", "1.2.3", 0}} {
		if c, e := CompareAppVersion(tc.a, tc.b); e != nil || c != tc.cmp {
			t.Errorf("%+v: %d %v", tc, c, e)
		}
	}
}
func TestAppMirrorsSelectLatest(t *testing.T) {
	server := func(version string, delay time.Duration) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
			fmt.Fprintf(w, `{"tag_name":%q,"body":%q,"assets":[{"name":"Misetanibox_Setup.exe","browser_download_url":"https://example.org/%s.exe"}]}`, version, version, version)
		}))
	}
	a := server("v1.5.28", 0)
	defer a.Close()
	b := server("v1.5.29", 0)
	defer b.Close()
	info, e := checkAppUpdateSources(context.Background(), "v1.5.28", []string{a.URL, b.URL}, []*http.Client{a.Client()})
	if e != nil || !info.HasUpdate || info.Version != "v1.5.29" || info.Body != "v1.5.29" || info.Partial {
		t.Fatalf("%+v %v", info, e)
	}
	bad := server("invalid", 0)
	defer bad.Close()
	info, e = checkAppUpdateSources(context.Background(), "v1.5.29", []string{bad.URL, b.URL}, []*http.Client{a.Client()})
	if e != nil || !info.Partial || info.HasUpdate {
		t.Fatalf("partial %+v %v", info, e)
	}
	if _, e = checkAppUpdateSources(context.Background(), "v1.5.29", []string{bad.URL}, []*http.Client{a.Client()}); e == nil {
		t.Fatal("all failures accepted")
	}
	slow := server("v1.5.30", time.Second)
	defer slow.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	info, e = checkAppUpdateSources(ctx, "v1.5.28", []string{slow.URL, b.URL}, []*http.Client{a.Client()})
	if e != nil || !info.Partial || info.Version != "v1.5.29" {
		t.Fatalf("timeout %+v %v", info, e)
	}
}
