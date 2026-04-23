package vllmdefaults

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

const defaultsScriptEnv = "INFERLEAN_VLLM_DEFAULTS_SCRIPT"

type RuntimeInput struct {
	Input
	Target   shared.Candidate
	DumpPath string
}

type runtimeDumpFile struct {
	Metadata struct {
		VLLMVersion string `json:"vllm_version"`
	} `json:"metadata"`
	EffectiveServeParameters map[string]any    `json:"effective_serve_parameters"`
	Errors                   map[string]string `json:"errors"`
	Warnings                 map[string]string `json:"warnings"`
}

type runtimeExecution struct {
	Source string
	PID    int32
}

var allowedEffectiveKeys = map[string]bool{
	"model":                  true,
	"max-model-len":          true,
	"max-num-batched-tokens": true,
	"max-num-seqs":           true,
	"gpu-memory-utilization": true,
	"enable-chunked-prefill": true,
	"attention-backend":      true,
}

func ResolveFromRuntime(ctx context.Context, in RuntimeInput) (Output, error) {
	dumpPath, err := prepareDumpPath(in.DumpPath)
	if err != nil {
		return Output{}, err
	}
	scriptPath, err := discoverDumpScriptPath()
	if err != nil {
		return Output{}, err
	}

	execMeta, err := runDumpScript(ctx, in.Target, scriptPath, dumpPath)
	if err != nil {
		return Output{}, err
	}
	dump, err := loadRuntimeDump(dumpPath)
	if err != nil {
		return Output{}, err
	}

	out, err := resolveFromDump(in.Input, dump)
	if err != nil {
		return Output{}, err
	}

	out.RuntimeSource = execMeta.Source
	out.RuntimePID = execMeta.PID
	out.RuntimeDumpPath = dumpPath
	out.RuntimeScriptPath = scriptPath
	out.RuntimeWarnings = flattenStatusMap(dump.Warnings)
	out.RuntimeErrors = flattenStatusMap(dump.Errors)
	if strings.TrimSpace(dump.Metadata.VLLMVersion) != "" {
		out.ResolvedVersion = strings.TrimSpace(dump.Metadata.VLLMVersion)
	}
	return out, nil
}

func prepareDumpPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		file, err := os.CreateTemp("", "inferlean-vllm-defaults-*.json")
		if err != nil {
			return "", fmt.Errorf("create defaults dump temp file: %w", err)
		}
		if err := file.Close(); err != nil {
			return "", fmt.Errorf("close defaults dump temp file: %w", err)
		}
		return file.Name(), nil
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve defaults dump path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return "", fmt.Errorf("create defaults dump directory: %w", err)
	}
	return abs, nil
}

func discoverDumpScriptPath() (string, error) {
	if custom := strings.TrimSpace(os.Getenv(defaultsScriptEnv)); custom != "" {
		if isRegularFile(custom) {
			return custom, nil
		}
		return "", fmt.Errorf("invalid %s path: %s", defaultsScriptEnv, custom)
	}
	for _, root := range collectSearchRoots() {
		if path := findDumpScriptUnderRoot(root); path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("unable to locate scripts/dump_vllm_defaults.py")
}

func findDumpScriptUnderRoot(start string) string {
	current := start
	for depth := 0; depth < 8; depth++ {
		candidates := []string{
			filepath.Join(current, "scripts", "dump_vllm_defaults.py"),
			filepath.Join(current, "cli", "scripts", "dump_vllm_defaults.py"),
		}
		for _, candidate := range candidates {
			if isRegularFile(candidate) {
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

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func loadRuntimeDump(path string) (runtimeDumpFile, error) {
	var dump runtimeDumpFile
	payload, err := os.ReadFile(path)
	if err != nil {
		return dump, fmt.Errorf("read defaults dump: %w", err)
	}
	if err := json.Unmarshal(payload, &dump); err != nil {
		return dump, fmt.Errorf("parse defaults dump: %w", err)
	}
	return dump, nil
}

func resolveFromDump(input Input, dump runtimeDumpFile) (Output, error) {
	if len(dump.EffectiveServeParameters) == 0 {
		return Output{}, fmt.Errorf("defaults dump does not include effective_serve_parameters")
	}
	explicit := normalizeArgs(input.ExplicitArgs)
	model := inferModel(explicit, input.RawCommandLine)
	requestedVersion := inferRequestedVersion(input, explicit)

	resolved := copyStringMap(explicit)
	applied := applyEffectiveDefaults(resolved, dump.EffectiveServeParameters, model)

	return Output{
		Args:             resolved,
		SelectedModel:    model,
		RequestedVersion: requestedVersion,
		AppliedDefaults:  applied,
	}, nil
}

func applyEffectiveDefaults(target map[string]string, effective map[string]any, model string) int {
	applied := 0
	for rawKey, rawValue := range effective {
		if strings.HasPrefix(strings.TrimSpace(rawKey), "_") {
			continue
		}
		key := normalizeKey(strings.ReplaceAll(rawKey, "_", "-"))
		if key == "" {
			continue
		}
		if !allowedEffectiveKeys[key] {
			continue
		}
		if key == "model" && strings.TrimSpace(model) == "" {
			continue
		}
		if _, exists := target[key]; exists {
			continue
		}
		value := stringifyValue(rawValue)
		if strings.TrimSpace(value) == "" {
			continue
		}
		target[key] = value
		applied++
	}
	return applied
}

func flattenStatusMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(values[key])
		if value == "" {
			continue
		}
		items = append(items, key+"="+value)
	}
	return strings.Join(items, "; ")
}
