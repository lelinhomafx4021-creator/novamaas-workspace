package runtimewatch

import "sync/atomic"

var monitoringDisabled atomic.Bool

// Enabled reports whether runtime instrumentation is configured for this
// process. Instrumentation is enabled by default.
func Enabled() bool { return !monitoringDisabled.Load() }

// SetEnabled configures instrumentation at startup, before clients, database
// connections and middleware are created. Runtime switching is unsupported:
// existing clients and byte counters retain their startup configuration.
func SetEnabled(enabled bool) { monitoringDisabled.Store(!enabled) }
