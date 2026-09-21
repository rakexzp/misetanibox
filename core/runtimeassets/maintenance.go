package runtimeassets

import "sync/atomic"

var bundledCoreMaintenance atomic.Bool

// EnableBundledCoreMaintenance is called only by Windows Wails before bootstrap.
// Other hosts retain their existing runtime asset policy.
func EnableBundledCoreMaintenance() { bundledCoreMaintenance.Store(true) }
