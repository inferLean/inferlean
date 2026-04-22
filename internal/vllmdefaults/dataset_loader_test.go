package vllmdefaults

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDefaultsUnderRootFindsLegacyBackendPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	defaultsDir := filepath.Join(root, "backend", "internal", "analysis", "data", "vllm_defaults")
	writeDefaultsManifest(t, defaultsDir)

	got := findDefaultsUnderRoot(filepath.Join(root, "backend", "internal"))
	if got != defaultsDir {
		t.Fatalf("findDefaultsUnderRoot() = %q, want %q", got, defaultsDir)
	}
}

func TestFindDefaultsUnderRootFindsRepoRootDefaultsPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	defaultsDir := filepath.Join(root, "vllm_defaults")
	writeDefaultsManifest(t, defaultsDir)

	got := findDefaultsUnderRoot(filepath.Join(root, "cmd", "inferlean"))
	if got != defaultsDir {
		t.Fatalf("findDefaultsUnderRoot() = %q, want %q", got, defaultsDir)
	}
}

func TestFindDefaultsUnderRootFindsMainRepoCLISubmodulePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	defaultsDir := filepath.Join(root, "cli", "vllm_defaults")
	writeDefaultsManifest(t, defaultsDir)

	got := findDefaultsUnderRoot(filepath.Join(root, "backend", "cmd"))
	if got != defaultsDir {
		t.Fatalf("findDefaultsUnderRoot() = %q, want %q", got, defaultsDir)
	}
}

func writeDefaultsManifest(t *testing.T, defaultsDir string) {
	t.Helper()
	manifestPath := filepath.Join(defaultsDir, "simple_cuda_by_tag", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("mkdir defaults dir: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}
