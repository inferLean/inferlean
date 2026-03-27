package debug

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	enabled atomic.Bool
	mu      sync.Mutex
	output  io.Writer = os.Stderr
	closer  io.Closer
)

// SetEnabled configures whether Debugf writes messages to stderr.
func SetEnabled(v bool) {
	_ = Configure(v, "")
}

// Configure enables debug logging and optionally redirects it to a file.
// When outputPath is set, debug logging is enabled even if v is false.
func Configure(v bool, outputPath string) error {
	path := strings.TrimSpace(outputPath)
	writer := io.Writer(os.Stderr)
	var nextCloser io.Closer

	if path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		writer = file
		nextCloser = file
	}

	mu.Lock()
	defer mu.Unlock()

	if err := closeLocked(); err != nil {
		if nextCloser != nil {
			_ = nextCloser.Close()
		}
		return err
	}

	output = writer
	closer = nextCloser
	enabled.Store(v || path != "")
	return nil
}

// Close flushes and releases any configured debug file sink.
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	err := closeLocked()
	output = os.Stderr
	enabled.Store(false)
	return err
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

	mu.Lock()
	defer mu.Unlock()

	fmt.Fprintf(output, "debug: "+format+"\n", args...)
}

func closeLocked() error {
	if closer == nil {
		return nil
	}
	current := closer
	closer = nil
	return current.Close()
}
