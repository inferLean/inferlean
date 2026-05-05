package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func ResolveBinary(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("binary name is required")
	}
	for _, toolsDir := range toolDirs() {
		if path, ok := binaryFromTools(toolsDir, trimmed); ok {
			return path, nil
		}
	}
	if path, err := exec.LookPath(trimmed); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found in InferLean tool directories or PATH", trimmed)
}

func binaryFromTools(toolsDir, name string) (string, bool) {
	if strings.TrimSpace(toolsDir) == "" {
		return "", false
	}
	platformDir := runtime.GOOS + "_" + runtime.GOARCH
	candidates := []string{
		filepath.Join(toolsDir, name),
		filepath.Join(toolsDir, "bin", name),
		filepath.Join(toolsDir, name, name),
		filepath.Join(toolsDir, platformDir, name),
		filepath.Join(toolsDir, platformDir, "bin", name),
		filepath.Join(toolsDir, platformDir, name, name),
	}
	for _, path := range candidates {
		if isExecutable(path) {
			return path, true
		}
	}
	return "", false
}

func toolDirs() []string {
	dirs := make([]string, 0, 2)
	if toolsDir, err := ToolsDir(); err == nil {
		dirs = append(dirs, toolsDir)
	}
	if bundled := bundledToolsDir(); bundled != "" {
		dirs = append(dirs, bundled)
	}
	return uniqueDirs(dirs)
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

func bundledToolsDir() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err == nil {
		executable = resolved
	}
	return filepath.Join(filepath.Dir(executable), "tools")
}

func uniqueDirs(dirs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		cleaned := filepath.Clean(strings.TrimSpace(dir))
		if cleaned == "." || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		out = append(out, cleaned)
	}
	return out
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
