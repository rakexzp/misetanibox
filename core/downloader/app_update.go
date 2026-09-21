package downloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type AppUpdateInfo struct {
	HasUpdate   bool   `json:"hasUpdate"`
	Partial     bool   `json:"partial"`
	Version     string `json:"version"`
	Body        string `json:"body"`
	ReleaseURL  string `json:"releaseUrl"`
	DownloadURL string `json:"downloadUrl"`
	AssetName   string `json:"assetName"`
}

var strictVersionRe = regexp.MustCompile(`(?i)(?:^|[^0-9])v?(\d+\.\d+(?:\.\d+)?(?:\.\d+)?)`)

func CheckAppUpdate(ctx context.Context, currentVersion string, strategy func() DownloadStrategy) (*AppUpdateInfo, error) {
	return checkAppUpdateSources(ctx, currentVersion, []string{
		"https://files.misetani.app/misetani/update.json",
		"https://files.geodema.network/misetani/update.json",
	}, BuildOrderedClients(strategy, 10*time.Second))
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
	return compareReleaseVersions(remote, current)
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
