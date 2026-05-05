package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBinaryFromToolsFindsBundledPlatformLayout(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, runtime.GOOS+"_"+runtime.GOARCH, "node_exporter", "node_exporter")
	writeExecutable(t, binary)

	path, ok := binaryFromTools(root, "node_exporter")
	if !ok {
		t.Fatal("expected bundled node_exporter to resolve")
	}
	if path != binary {
		t.Fatalf("path = %q, want %q", path, binary)
	}
}

func TestBinaryFromToolsKeepsFlatToolsLayout(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "prometheus")
	writeExecutable(t, binary)

	path, ok := binaryFromTools(root, "prometheus")
	if !ok {
		t.Fatal("expected flat prometheus to resolve")
	}
	if path != binary {
		t.Fatalf("path = %q, want %q", path, binary)
	}
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}
