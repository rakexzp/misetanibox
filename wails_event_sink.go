package main

import (
	"context"
	"goclashz/core/logger"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type WailsEventSink struct {
	mu  sync.RWMutex
	ctx context.Context
}

func (s *WailsEventSink) SetContext(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx = ctx
}

func (s *WailsEventSink) Emit(name string, args ...any) {
	if s == nil {
		return
	}

	s.mu.RLock()
	ctx := s.ctx
	s.mu.RUnlock()

	if ctx == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Wails event emit panic: %s: %v", name, r)
		}
	}()

	runtime.EventsEmit(ctx, name, args...)
}
