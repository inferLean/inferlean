package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inferLean/inferlean/internal/debug"
)

func TestRootCommandDebugFileEnablesFileLogging(t *testing.T) {
	t.Cleanup(func() {
		_ = debug.Close()
	})

	path := filepath.Join(t.TempDir(), "inferlean-debug.log")
	var stdout bytes.Buffer

	cmd := newRootCommand(context.Background())
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--debug-file", path, "version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !debug.Enabled() {
		t.Fatal("Enabled() = false, want true after --debug-file configuration")
	}

	debug.Debugf("root debug message")

	if err := debug.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != version {
		t.Fatalf("stdout = %q, want %q", got, version)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(content); !strings.Contains(got, "debug: root debug message\n") {
		t.Fatalf("debug log = %q, want root-configured file output", got)
	}
}
