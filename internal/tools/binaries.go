package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ResolveBinary(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("binary name is required")
	}
	if path, ok := binaryFromTools(trimmed); ok {
		return path, nil
	}
	if path, err := exec.LookPath(trimmed); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found in ~/.inferlean/tools or PATH", trimmed)
}

func binaryFromTools(name string) (string, bool) {
	toolsDir, err := ToolsDir()
	if err != nil {
		return "", false
	}
	candidates := []string{
		filepath.Join(toolsDir, name),
		filepath.Join(toolsDir, "bin", name),
	}
	for _, path := range candidates {
		if isExecutable(path) {
			return path, true
		}
	}
	return "", false
}

func ToolsDir() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("INFERLEAN_TOOLS_DIR")); custom != "" {
		return custom, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".inferlean", "tools"), nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
