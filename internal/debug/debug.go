package debug

import (
	"fmt"
	"os"
	"sync/atomic"
)

var enabled atomic.Bool

// SetEnabled configures whether Debugf writes messages to stderr.
func SetEnabled(v bool) {
	enabled.Store(v)
}

// Enabled reports whether debug logging is active.
func Enabled() bool {
	return enabled.Load()
}

// Debugf emits bounded step-oriented diagnostics for discovery flows.
func Debugf(format string, args ...any) {
	if !enabled.Load() {
		return
	}

	fmt.Fprintf(os.Stderr, "debug: "+format+"\n", args...)
}
