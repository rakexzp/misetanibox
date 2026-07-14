package mobilecore

import "sync/atomic"

// tiny wrapper so older mental models map cleanly (atomic.Bool needs go1.19+).
type atomicBool struct{ v atomic.Bool }

func (b *atomicBool) set(x bool) { b.v.Store(x) }
func (b *atomicBool) get() bool  { return b.v.Load() }
