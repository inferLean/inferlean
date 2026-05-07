package vllmdefaults

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func discoverDefaultsDir() (string, error) {
	if custom := strings.TrimSpace(os.Getenv(defaultsDirEnv)); custom != "" {
		if isDefaultsDir(custom) {
			return custom, nil
		}
		return "", fmt.Errorf("invalid %s path: %s", defaultsDirEnv, custom)
	}
	searchRoots := collectSearchRoots()
	for _, root := range searchRoots {
		if path := findDefaultsUnderRoot(root); path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("unable to locate vllm_defaults data directory")
}

func collectSearchRoots() []string {
	roots := []string{}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	return dedupeStrings(roots)
}

func findDefaultsUnderRoot(start string) string {
	current := start
	for depth := 0; depth < 8; depth++ {
		candidates := []string{
			filepath.Join(current, "internal", "analysis", "data", "vllm_defaults"),
			filepath.Join(current, "vllm_defaults"),
			filepath.Join(current, "cli", "vllm_defaults"),
		}
		for _, candidate := range candidates {
			if isDefaultsDir(candidate) {
				return candidate
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
	return ""
}

func isDefaultsDir(path string) bool {
	manifestPath := filepath.Join(path, "simple_cuda_by_tag", "manifest.json")
	info, err := os.Stat(manifestPath)
	return err == nil && !info.IsDir()
}

func dedupeStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

func loadManifest(path string) (manifestFile, error) {
	return loadJSONFile[manifestFile](path, "defaults manifest")
}

func loadTagDefaults(path string) (tagDefaultsFile, error) {
	return loadJSONFile[tagDefaultsFile](path, "defaults tag file")
}

func loadRuntimeDump(path string) (runtimeDumpFile, error) {
	return loadJSONFile[runtimeDumpFile](path, "defaults dump")
}

func loadJSONFile[T any](path, label string) (T, error) {
	var file T
	payload, err := os.ReadFile(path)
	if err != nil {
		return file, fmt.Errorf("read %s: %w", label, err)
	}
	if err := json.Unmarshal(payload, &file); err != nil {
		return file, fmt.Errorf("parse %s: %w", label, err)
	}
	return file, nil
}
