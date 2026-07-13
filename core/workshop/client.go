// Package workshop — клиент каталога авторских тем (Мастерская).
// Ходит к бэкенду на зеркале, личность = HWID устройства (заголовок x-hwid).
package workshop

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const BaseURL = "https://files.geodema.network/workshop/api"

func client() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func do(req *http.Request, hwid string) (string, error) {
	if hwid != "" {
		req.Header.Set("x-hwid", hwid)
	}
	resp, err := client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("каталог вернул %d: %s", resp.StatusCode, string(b))
	}
	return string(b), nil
}

func get(hwid, path string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, BaseURL+path, nil)
	if err != nil {
		return "", err
	}
	return do(req, hwid)
}

func postForm(hwid, path string, form url.Values) (string, error) {
	req, err := http.NewRequest(http.MethodPost, BaseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(req, hwid)
}

// List — лента тем. sort: popular|new. Возвращает сырой JSON.
func List(hwid, sort, q string, page int) (string, error) {
	return get(hwid, fmt.Sprintf("/themes?sort=%s&page=%d&q=%s",
		url.QueryEscape(sort), page, url.QueryEscape(q)))
}

func Get(hwid string, id int) (string, error)      { return get(hwid, fmt.Sprintf("/themes/%d", id)) }
func Download(hwid string, id int) (string, error) { return get(hwid, fmt.Sprintf("/themes/%d/download", id)) }
func Like(hwid string, id int) (string, error)     { return postForm(hwid, fmt.Sprintf("/themes/%d/like", id), nil) }

func Report(hwid string, id int, reason string) (string, error) {
	return postForm(hwid, fmt.Sprintf("/themes/%d/report", id), url.Values{"reason": {reason}})
}

// decodeDataURI разбирает "data:image/png;base64,...." → байты + расширение.
func decodeDataURI(s string) ([]byte, string, error) {
	i := strings.Index(s, ",")
	if !strings.HasPrefix(s, "data:") || i < 0 {
		return nil, "", fmt.Errorf("не data-URI")
	}
	meta := s[5:i]
	ext := "png"
	if strings.Contains(meta, "jpeg") || strings.Contains(meta, "jpg") {
		ext = "jpg"
	} else if strings.Contains(meta, "webp") {
		ext = "webp"
	}
	data, err := base64.StdEncoding.DecodeString(s[i+1:])
	return data, ext, err
}

// Publish — публикует тему: manifest (JSON) + картинки (data-URI по порядку asset:N).
func Publish(hwid, manifestJSON string, imagesDataURI []string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("manifest", manifestJSON)
	for i, du := range imagesDataURI {
		data, ext, err := decodeDataURI(du)
		if err != nil {
			return "", fmt.Errorf("картинка %d: %v", i, err)
		}
		fw, err := w.CreateFormFile("images", fmt.Sprintf("img%d.%s", i, ext))
		if err != nil {
			return "", err
		}
		if _, err := fw.Write(data); err != nil {
			return "", err
		}
	}
	w.Close()
	req, err := http.NewRequest(http.MethodPost, BaseURL+"/themes", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return do(req, hwid)
}

// FetchImage качает картинку темы по URL и возвращает data-URI (для применения фона карточки).
func FetchImage(rawurl string) (string, error) {
	resp, err := client().Get(rawurl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("картинка %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return "", err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
