package logger

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type LogSource string

const (
	LogSourceCore LogSource = "core"
	LogSourceApp  LogSource = "app"
)

type LogEntry struct {
	Type    string    `json:"type"`
	Source  LogSource `json:"source"`
	Payload string    `json:"payload"`
	Time    string    `json:"time"`
}

type RingBuffer struct {
	mu   sync.RWMutex
	data []LogEntry
	max  int
}

func NewRingBuffer(max int) *RingBuffer {
	return &RingBuffer{
		data: make([]LogEntry, 0, max),
		max:  max,
	}
}

func (r *RingBuffer) Add(entry LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.data) >= r.max {

		r.data = append(r.data[1:], entry)
	} else {
		r.data = append(r.data, entry)
	}
}

func (r *RingBuffer) GetAll() []LogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]LogEntry, len(r.data))
	copy(result, r.data)
	return result
}

func (r *RingBuffer) Search(keyword string) []LogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []LogEntry
	lowerKey := strings.ToLower(keyword)
	for _, entry := range r.data {
		if strings.Contains(strings.ToLower(entry.Payload), lowerKey) ||
			strings.Contains(strings.ToLower(entry.Type), lowerKey) {
			result = append(result, entry)
		}
	}
	return result
}

var AppLogs = NewRingBuffer(500)

func (r *RingBuffer) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = make([]LogEntry, 0, r.max)
}

func Infof(format string, args ...any) {
	AppLogs.Add(LogEntry{
		Type:    "info",
		Source:  LogSourceApp,
		Payload: fmt.Sprintf(format, args...),
		Time:    time.Now().Format("15:04:05"),
	})
}

func Errorf(format string, args ...any) {
	AppLogs.Add(LogEntry{
		Type:    "error",
		Source:  LogSourceApp,
		Payload: fmt.Sprintf(format, args...),
		Time:    time.Now().Format("15:04:05"),
	})
}

func Warnf(format string, args ...any) {
	AppLogs.Add(LogEntry{
		Type:    "warn",
		Source:  LogSourceApp,
		Payload: fmt.Sprintf(format, args...),
		Time:    time.Now().Format("15:04:05"),
	})
}

func Debugf(format string, args ...any) {
	AppLogs.Add(LogEntry{
		Type:    "debug",
		Source:  LogSourceApp,
		Payload: fmt.Sprintf(format, args...),
		Time:    time.Now().Format("15:04:05"),
	})
}
