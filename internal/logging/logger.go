package logging

import (
	"fmt"
	"io"
	"os"
	"sync"
)

type Logger struct {
	debug bool
	out   io.Writer
	mu    sync.Mutex
}

func New(debug bool, debugFile string) (*Logger, func() error, error) {
	logger := &Logger{debug: debug, out: os.Stderr}
	if debugFile == "" {
		return logger, func() error { return nil }, nil
	}
	fh, err := os.OpenFile(debugFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open debug file: %w", err)
	}
	logger.out = fh
	closeFn := func() error { return fh.Close() }
	return logger, closeFn, nil
}

func (l *Logger) Debugf(format string, args ...any) {
	if l == nil || !l.debug {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.out, "[debug] "+sanitize(format)+"\n", sanitizeArgs(args)...)
}

func (l *Logger) Infof(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(os.Stdout, sanitize(format)+"\n", sanitizeArgs(args)...)
}

func sanitize(input string) string {
	if input == "" {
		return input
	}
	return input
}

func sanitizeArgs(args []any) []any {
	if len(args) == 0 {
		return args
	}
	out := make([]any, len(args))
	for i, arg := range args {
		out[i] = arg
	}
	return out
}
