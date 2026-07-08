package clash

import (
	"context"
	"encoding/json"
	"fmt"
	"goclashz/core/logger"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type LogMessage struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type LogCallback func(log LogMessage)

func StartLogStream(ctx context.Context, onLog LogCallback) {
	const (
		pongWait   = 60 * time.Second
		pingPeriod = 30 * time.Second
		writeWait  = 5 * time.Second
	)

	wsURL := APIWSURLWithRawQuery("/logs", "level=info")

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		logger.Errorf("не удалось подключиться к потоку логов: %v", err)
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var log LogMessage
			if err := json.Unmarshal(message, &log); err != nil {
				continue
			}

			if onLog != nil {
				onLog(log)
			}
		}
	}()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():

			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(writeWait),
			)
			return

		case <-done:
			return

		case <-ticker.C:

			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}

func PatchConfig(settings map[string]interface{}) error {
	payload, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("не удалось сериализовать конфигурацию: %v", err)
	}

	req, err := http.NewRequest("PATCH", APIURL("/configs"), strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("не удалось построить PATCH-запрос: %v", err)
	}

	resp, err := localAPIClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ядро вернуло код ошибки: %d", resp.StatusCode)
	}
	return nil
}
