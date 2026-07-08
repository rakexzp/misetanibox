package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type AppUpdateInfo struct {
	HasUpdate   bool   `json:"hasUpdate"`
	Version     string `json:"version"`
	Body        string `json:"body"`
	ReleaseURL  string `json:"releaseUrl"`
	DownloadURL string `json:"downloadUrl"`
	AssetName   string `json:"assetName"`
}

var strictVersionRe = regexp.MustCompile(`(?i)(?:^|[^0-9])v?(\d+\.\d+(?:\.\d+)?(?:\.\d+)?)`)

func CheckAppUpdate(ctx context.Context, currentVersion string, strategy func() DownloadStrategy) (*AppUpdateInfo, error) {

	apiURL := "http://files.geodema.network/misetani/update.json"

	clients := BuildOrderedClients(strategy, 60*time.Second)

	var release struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	var lastErr error
	for _, client := range clients {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if reqErr != nil {
			lastErr = reqErr
			continue
		}
		req.Header.Set("User-Agent", "Misetanibox-Updater")

		resp, reqErr := client.Do(req)
		if reqErr != nil {
			lastErr = reqErr
			continue
		}

		func() {
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				lastErr = fmt.Errorf("GitHub API вернул HTTP %d", resp.StatusCode)
				return
			}
			lastErr = json.NewDecoder(resp.Body).Decode(&release)
		}()

		if lastErr == nil && release.TagName != "" {
			break
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	cmp, _ := CompareAppVersion(release.TagName, currentVersion)
	assetName, downloadURL := selectWindowsAsset(release.Assets)

	return &AppUpdateInfo{
		HasUpdate:   cmp > 0,
		Version:     release.TagName,
		Body:        release.Body,
		ReleaseURL:  release.HTMLURL,
		DownloadURL: downloadURL,
		AssetName:   assetName,
	}, nil
}

func selectWindowsAsset(assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}) (string, string) {
	type candidate struct {
		name string
		url  string
		rank int
	}

	var candidates []candidate

	for _, asset := range assets {
		lower := strings.ToLower(asset.Name)

		if !strings.HasSuffix(lower, ".exe") {
			continue
		}

		if !strings.Contains(lower, "misetani") {
			continue
		}

		if strings.Contains(lower, "sha256") ||
			strings.Contains(lower, "checksum") ||
			strings.Contains(lower, "symbols") ||
			strings.Contains(lower, "debug") {
			continue
		}

		rank := 10
		if strings.Contains(lower, "setup") || strings.Contains(lower, "installer") {
			rank = 0
		} else if strings.Contains(lower, "windows") || strings.Contains(lower, "win") {
			rank = 1
		} else if strings.Contains(lower, "x64") || strings.Contains(lower, "amd64") {
			rank = 2
		}

		candidates = append(candidates, candidate{
			name: asset.Name,
			url:  asset.BrowserDownloadURL,
			rank: rank,
		})
	}

	if len(candidates) == 0 {
		return "", ""
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.rank < best.rank {
			best = c
		}
	}

	return best.name, best.url
}

func CompareAppVersion(remote, current string) (int, error) {
	aa := parseVersionParts(remote)
	bb := parseVersionParts(current)
	if len(aa) == 0 || len(bb) == 0 {
		return 0, nil
	}
	for i := 0; i < 3; i++ {
		var a, b int
		if i < len(aa) {
			a = aa[i]
		}
		if i < len(bb) {
			b = bb[i]
		}
		if a > b {
			return 1, nil
		}
		if a < b {
			return -1, nil
		}
	}
	return 0, nil
}

func parseVersionParts(v string) []int {
	m := strictVersionRe.FindStringSubmatch(v)
	if len(m) < 2 {
		return nil
	}
	parts := strings.Split(m[1], ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}

func DownloadAppUpdate(
	ctx context.Context,
	info *AppUpdateInfo,
	destDir string,
	onProgress func(bytesDone, totalBytes, speedBps int64, etaSec int64),
	strategy func() DownloadStrategy,
) (string, error) {
	if info == nil {
		return "", fmt.Errorf("информация об обновлении пуста")
	}
	if strings.TrimSpace(info.DownloadURL) == "" {
		return "", fmt.Errorf("нет доступного адреса загрузки обновления приложения")
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	fileName := sanitizeUpdateAssetName(info.AssetName)
	if fileName == "" {

		fileName = fmt.Sprintf("Misetanibox_%s_Setup.exe", strings.TrimPrefix(info.Version, "v"))
	}

	destPath := filepath.Join(destDir, fileName)

	err := DownloadLargeAssetAtomic(ctx, Options{
		URLs:       []string{info.DownloadURL},
		DestPath:   destPath,
		UserAgent:  "Misetanibox-Updater",
		MaxBytes:   300 << 20,
		Strategy:   strategy,
		OnProgress: onProgress,
		Validator: func(tmpPath string) error {
			return ValidateWindowsExecutable(tmpPath)
		},
	})
	if err != nil {
		return "", err
	}

	return destPath, nil
}

func ValidateWindowsExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.Size() < 1024*1024 {
		return fmt.Errorf("пакет обновления: аномальный размер (меньше 1MB)")
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, 2)
	if _, err := io.ReadFull(f, header); err != nil {
		return err
	}

	if string(header) != "MZ" {
		return fmt.Errorf("пакет обновления не является допустимым исполняемым файлом Windows (нет сигнатуры MZ)")
	}

	return nil
}

func sanitizeUpdateAssetName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" {
		return ""
	}

	if !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return ""
	}
	return name
}
