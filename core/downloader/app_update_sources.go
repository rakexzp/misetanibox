package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func checkAppUpdateSources(ctx context.Context, current string, urls []string, clients []*http.Client) (*AppUpdateInfo, error) {
	if _, err := CompareAppVersion(current, current); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	type result struct {
		info *AppUpdateInfo
		err  error
	}
	results := make(chan result, len(urls))
	for _, source := range urls {
		go func(source string) {
			var last error = fmt.Errorf("нет клиентов")
			for _, client := range clients {
				attempt, cancel := context.WithTimeout(ctx, 5*time.Second)
				info, err := fetchAppRelease(attempt, client, source)
				cancel()
				if err == nil {
					results <- result{info: info}
					return
				}
				last = err
			}
			results <- result{err: fmt.Errorf("%s: %w", source, last)}
		}(source)
	}
	var best *AppUpdateInfo
	var failures []string
	for range urls {
		r := <-results
		if r.err != nil {
			failures = append(failures, r.err.Error())
			continue
		}
		if best == nil {
			best = r.info
		} else if cmp, _ := CompareAppVersion(r.info.Version, best.Version); cmp > 0 {
			best = r.info
		}
	}
	if best == nil {
		return nil, fmt.Errorf("не удалось проверить источники обновлений: %s", strings.Join(failures, "; "))
	}
	cmp, err := CompareAppVersion(best.Version, current)
	if err != nil {
		return nil, err
	}
	best.HasUpdate = cmp > 0
	best.Partial = len(failures) > 0
	return best, nil
}

func fetchAppRelease(ctx context.Context, client *http.Client, source string) (*AppUpdateInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Misetanibox-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var release struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("manifest слишком большой")
	}
	if err = json.Unmarshal(data, &release); err != nil {
		return nil, err
	}
	if _, err = CompareAppVersion(release.TagName, release.TagName); err != nil {
		return nil, err
	}
	name, download := selectWindowsAsset(release.Assets)
	u, err := url.Parse(download)
	if name == "" || err != nil || u.Host == "" || u.User != nil || u.Scheme != "https" {
		return nil, fmt.Errorf("нет корректного Windows installer")
	}
	return &AppUpdateInfo{Version: release.TagName, Body: release.Body, ReleaseURL: release.HTMLURL, DownloadURL: download, AssetName: name}, nil
}
