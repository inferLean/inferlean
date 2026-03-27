package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureWritesDebugOutputToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inferlean-debug.log")

	if err := Configure(false, path); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}
	if !Enabled() {
		t.Fatal("Enabled() = false, want true when debug-file is set")
	}

	Debugf("hello %s", "world")

	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(content); !strings.Contains(got, "debug: hello world\n") {
		t.Fatalf("debug log = %q, want file output", got)
	}
}
