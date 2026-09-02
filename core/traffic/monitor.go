package traffic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type ConnectionCallback func(vos []ConnectionVO)

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

type ConnectionMetadata struct {
	Network         string `json:"network"`
	Type            string `json:"type"`
	SourceIP        string `json:"sourceIP"`
	DestinationIP   string `json:"destinationIP"`
	SourcePort      string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Host            string `json:"host"`
}

type RawConnection struct {
	ID          string             `json:"id"`
	Metadata    ConnectionMetadata `json:"metadata"`
	Upload      int64              `json:"upload"`
	Download    int64              `json:"download"`
	Start       string             `json:"start"`
	Chains      []string           `json:"chains"`
	Rule        string             `json:"rule"`
	RulePayload string             `json:"rulePayload"`
}

type ConnectionVO struct {
	RawConnection
	UploadStr   string `json:"uploadStr"`
	DownloadStr string `json:"downloadStr"`
	DurationStr string `json:"durationStr"`
}

// секрет API ядра (ставит пакет clash; traffic не может его импортировать — цикл)
var AuthSecret func() string

var trafficStreamClient = &http.Client{
	Transport: &http.Transport{
		Proxy: nil,

		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 15 * time.Second,
		}).DialContext,
	},
}

func StreamTraffic(ctx context.Context, apiURL string, callback func(upRaw, downRaw float64, upStr, downStr string), onError func(error)) {

	for {

		select {
		case <-ctx.Done():
			return
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err == nil && AuthSecret != nil {
			if s := AuthSecret(); s != "" {
				req.Header.Set("Authorization", "Bearer "+s)
			}
		}
		if err != nil {
			if !sleepOrDone(ctx, 2*time.Second) {
				return
			}
			continue
		}

		resp, err := trafficStreamClient.Do(req)
		if err != nil {
			if onError != nil && ctx.Err() == nil {
				onError(fmt.Errorf("traffic stream request failed: %w", err))
			}
			if !sleepOrDone(ctx, 2*time.Second) {
				return
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			err := fmt.Errorf("traffic stream unexpected status: %s", resp.Status)
			resp.Body.Close()
			if onError != nil && ctx.Err() == nil {
				onError(err)
			}
			if !sleepOrDone(ctx, 2*time.Second) {
				return
			}
			continue
		}

		const trafficIdleTimeout = 70 * time.Second
		decoder := json.NewDecoder(resp.Body)
		idleTimer := time.NewTimer(trafficIdleTimeout)
		stopIdle := make(chan struct{})

		go func() {
			select {
			case <-ctx.Done():
				resp.Body.Close()
			case <-idleTimer.C:

				resp.Body.Close()
			case <-stopIdle:

			}
		}()

		for {
			var data struct {
				Up   float64 `json:"up"`
				Down float64 `json:"down"`
			}

			if err := decoder.Decode(&data); err != nil {

				close(stopIdle)
				if !idleTimer.Stop() {
					select {
					case <-idleTimer.C:
					default:
					}
				}
				resp.Body.Close()

				if ctx.Err() == nil && !errors.Is(err, io.EOF) && onError != nil {
					onError(fmt.Errorf("traffic stream decode failed: %w", err))
				}

				break
			}

			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(trafficIdleTimeout)

			callback(data.Up, data.Down, FormatBytes(int64(data.Up)), FormatBytes(int64(data.Down)))
		}
	}
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(startStr string) string {
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return "00:00"
	}
	d := time.Since(start)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func ProcessConnections(rawConnections []RawConnection) []ConnectionVO {
	var vos []ConnectionVO
	for _, conn := range rawConnections {
		vos = append(vos, ConnectionVO{
			RawConnection: conn,
			UploadStr:     FormatBytes(conn.Upload),
			DownloadStr:   FormatBytes(conn.Download),
			DurationStr:   formatDuration(conn.Start),
		})
	}
	return vos
}

func EmitConnections(ctx context.Context, rawConnections []RawConnection, callback ConnectionCallback) {
	vos := ProcessConnections(rawConnections)
	if callback != nil {
		callback(vos)
	}
}
