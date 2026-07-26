package downloader

import (
	"net/http"
)

type Options struct {
	URLs      []string
	DestPath  string
	UserAgent string
	Headers   map[string]string
	MaxBytes  int64

	Client *http.Client

	Strategy func() DownloadStrategy

	VerifyGitHubSHA    bool
	RequireGitHubSHA   bool
	InsecureSkipVerify bool

	AttemptsPerEndpoint int
	OnResponse          func(resp *http.Response)
	// Transform (если задан) вызывается ДО Validator и может переписать файл
	// (напр. сконвертировать Xray-JSON в mihomo-YAML на месте).
	Transform func(tmpPath string) error
	Validator func(tmpPath string) error

	OnProgress func(bytesDone, totalBytes, speedBps int64, etaSec int64)
}

type DownloadStrategy struct {
	ProxyURL    string
	PreferProxy bool
}
