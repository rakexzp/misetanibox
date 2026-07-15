package clash

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"goclashz/core/utils"

	"golang.org/x/crypto/curve25519"
)

// WARP-выход: Cloudflare WARP как wireguard-аутбаунд для конструктора цепочек.
// Регистрируем устройство в Cloudflare один раз (получаем ключи/адреса/reserved),
// кешируем локально и инжектим узел "WARP" в proxies — дальше он выбирается в цепочке
// как обычный сервер, юзеру ничего настраивать не нужно.

const (
	// WarpNodeName — имя узла WARP в списке прокси (по нему ссылается relay-цепочка).
	WarpNodeName = "WARP"

	warpEnabledKey = "warp_enabled"
	warpCredsKey   = "warp_creds"
	warpRegURL     = "https://api.cloudflareclient.com/v0a2158/reg"

	// Стабильный публичный endpoint WARP (надёжнее возвращаемого хоста).
	warpServer = "engage.cloudflareclient.com"
	warpPort   = 2408
)

// WarpCreds — сохранённые креды зарегистрированного WARP-устройства.
type WarpCreds struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"` // публичный ключ пира (Cloudflare)
	Address4   string `json:"address4"`
	Address6   string `json:"address6"`
	Reserved   []int  `json:"reserved"`
}

// IsWarpEnabled — включён ли WARP-выход.
func IsWarpEnabled() bool {
	v, err := utils.LoadSetting(warpEnabledKey, false)
	if err != nil || v == nil {
		return false
	}
	return *v
}

func setWarpEnabled(on bool) error {
	return utils.SaveSetting(warpEnabledKey, &on)
}

func loadWarpCreds() *WarpCreds {
	v, err := utils.LoadSetting(warpCredsKey, WarpCreds{})
	if err != nil || v == nil || v.PrivateKey == "" || v.PublicKey == "" {
		return nil
	}
	return v
}

func saveWarpCreds(c *WarpCreds) error {
	return utils.SaveSetting(warpCredsKey, c)
}

// genWGKeypair — генерирует пару ключей WireGuard (Curve25519).
func genWGKeypair() (priv, pub string, err error) {
	var privKey [32]byte
	if _, err = rand.Read(privKey[:]); err != nil {
		return
	}
	// clamp по спецификации Curve25519
	privKey[0] &= 248
	privKey[31] &= 127
	privKey[31] |= 64
	pubKey, err := curve25519.X25519(privKey[:], curve25519.Basepoint)
	if err != nil {
		return
	}
	return base64.StdEncoding.EncodeToString(privKey[:]),
		base64.StdEncoding.EncodeToString(pubKey), nil
}

// registerWarp — регистрирует новое WARP-устройство в Cloudflare и возвращает креды.
func registerWarp() (*WarpCreds, error) {
	priv, pub, err := genWGKeypair()
	if err != nil {
		return nil, fmt.Errorf("генерация ключей: %w", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"key":        pub,
		"install_id": "",
		"fcm_token":  "",
		"tos":        time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"model":      "PC",
		"type":       "Android",
		"locale":     "en_US",
	})

	req, err := http.NewRequest(http.MethodPost, warpRegURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "okhttp/3.12.1")
	req.Header.Set("CF-Client-Version", "a-6.30-2158")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("запрос к Cloudflare: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Cloudflare вернул HTTP %d", resp.StatusCode)
	}

	var reg struct {
		Config struct {
			ClientID string `json:"client_id"`
			Peers    []struct {
				PublicKey string `json:"public_key"`
			} `json:"peers"`
			Interface struct {
				Addresses struct {
					V4 string `json:"v4"`
					V6 string `json:"v6"`
				} `json:"addresses"`
			} `json:"interface"`
		} `json:"config"`
	}
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("разбор ответа Cloudflare: %w", err)
	}
	if len(reg.Config.Peers) == 0 || reg.Config.Peers[0].PublicKey == "" {
		return nil, fmt.Errorf("Cloudflare не вернул публичный ключ пира")
	}

	reserved := []int{0, 0, 0}
	if cid, err := base64.StdEncoding.DecodeString(reg.Config.ClientID); err == nil && len(cid) >= 3 {
		reserved = []int{int(cid[0]), int(cid[1]), int(cid[2])}
	}

	return &WarpCreds{
		PrivateKey: priv,
		PublicKey:  reg.Config.Peers[0].PublicKey,
		Address4:   stripCIDR(reg.Config.Interface.Addresses.V4),
		Address6:   stripCIDR(reg.Config.Interface.Addresses.V6),
		Reserved:   reserved,
	}, nil
}

func stripCIDR(addr string) string {
	if i := strings.IndexByte(addr, '/'); i >= 0 {
		return addr[:i]
	}
	return addr
}

// EnableWarpNode — включает WARP-выход: при необходимости регистрирует устройство,
// кеширует креды и помечает WARP включённым. Возвращает ошибку, если регистрация не удалась.
func EnableWarpNode() error {
	if loadWarpCreds() == nil {
		creds, err := registerWarp()
		if err != nil {
			return err
		}
		if err := saveWarpCreds(creds); err != nil {
			return err
		}
	}
	return setWarpEnabled(true)
}

// DisableWarpNode — выключает WARP-выход (креды остаются в кеше для повторного включения).
func DisableWarpNode() error {
	return setWarpEnabled(false)
}

// injectWarp — добавляет узел WARP (type: wireguard) в root["proxies"], если включён и есть креды.
// Вызывается до injectChains, чтобы relay-цепочки могли на него ссылаться, и до валидатора.
func injectWarp(root map[string]interface{}) {
	if !IsWarpEnabled() {
		return
	}
	creds := loadWarpCreds()
	if creds == nil {
		return
	}

	proxies, _ := root["proxies"].([]interface{})
	// не дублируем, если уже есть
	for _, p := range proxies {
		if pm, ok := p.(map[string]interface{}); ok {
			if n, _ := pm["name"].(string); n == WarpNodeName {
				return
			}
		}
	}

	reserved := make([]interface{}, 0, 3)
	for _, r := range creds.Reserved {
		reserved = append(reserved, r)
	}

	warp := map[string]interface{}{
		"name":               WarpNodeName,
		"type":               "wireguard",
		"server":             warpServer,
		"port":               warpPort,
		"ip":                 creds.Address4,
		"ipv6":               creds.Address6,
		"private-key":        creds.PrivateKey,
		"public-key":         creds.PublicKey,
		"reserved":           reserved,
		"udp":                true,
		"mtu":                1280,
		"remote-dns-resolve": true,
	}

	root["proxies"] = append(proxies, warp)
}
