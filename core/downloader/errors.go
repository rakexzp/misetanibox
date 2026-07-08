package downloader

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

func SanitizeDownloadError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) {
		return err
	}

	msg := err.Error()

	reAsset := regexp.MustCompile(`https://release-assets\.githubusercontent\.com/[^\s"]+`)
	msg = reAsset.ReplaceAllString(msg, "адрес загрузки ассета GitHub Release")

	reGithub := regexp.MustCompile(`https://github\.com/[^\s"]+/releases/download/[^\s"]+`)
	msg = reGithub.ReplaceAllStringFunc(msg, func(s string) string {
		if u, err := url.Parse(s); err == nil {
			u.RawQuery = ""
			return u.String()
		}
		return "адрес загрузки GitHub Release"
	})

	reQuery := regexp.MustCompile(`([?&](sp|sv|se|sr|sig|skoid|sktid|skt|ske|sks|skv)=[^\s]+)`)
	msg = reQuery.ReplaceAllString(msg, "")

	if len(msg) > 360 {
		msg = msg[:360] + "..."
	}

	return fmt.Errorf("%s", msg)
}
