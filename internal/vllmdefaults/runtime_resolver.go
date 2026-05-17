package vllmdefaults

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

const defaultsScriptEnv = "INFERLEAN_VLLM_DEFAULTS_SCRIPT"

const (
	runtimeDefaultsEffectiveTimeoutSeconds = 20
	runtimeDefaultsScriptTimeout           = 90 * time.Second
)

type RuntimeInput struct {
	Input
	Target            shared.Candidate
	DumpPath          string
	ModelPathOverride string
}

type runtimeDumpFile struct {
	Metadata struct {
		VLLMVersion  string `json:"vllm_version"`
		TorchVersion string `json:"torch_version"`
	} `json:"metadata"`
	PIDProcess               runtimePIDProcess `json:"pid_process"`
	EffectiveServeParameters map[string]any    `json:"effective_serve_parameters"`
	Errors                   map[string]string `json:"errors"`
	Warnings                 map[string]string `json:"warnings"`
}

type runtimePIDProcess struct {
	PID                      int32  `json:"pid"`
	ModelPathOverride        string `json:"model_path_override"`
	ModelPathOverrideApplied bool   `json:"model_path_override_applied"`
}

type runtimeExecution struct {
	Source string
	PID    int32
}

var allowedEffectiveKeys = map[string]bool{
	"model":                           true,
	"served-model-name":               true,
	"tensor-parallel-size":            true,
	"data-parallel-size":              true,
	"pipeline-parallel-size":          true,
	"max-model-len":                   true,
	"max-num-batched-tokens":          true,
	"max-num-seqs":                    true,
	"gpu-memory-utilization":          true,
	"kv-cache-dtype":                  true,
	"enable-prefix-caching":           true,
	"enable-chunked-prefill":          true,
	"async-scheduling":                true,
	"scheduler-policy":                true,
	"scheduling-policy":               true,
	"max-num-partial-prefills":        true,
	"max-long-partial-prefills":       true,
	"long-prefill-token-threshold":    true,
	"max-num-scheduled-tokens":        true,
	"max-num-encoder-input-tokens":    true,
	"scheduler-reserve-full-isl":      true,
	"disable-chunked-mm-input":        true,
	"disable-hybrid-kv-cache-manager": true,
	"block-size":                      true,
	"kv-cache-memory-bytes":           true,
	"kv-offloading-backend":           true,
	"kv-offloading-size":              true,
	"kv-sharing-fast-prefill":         true,
	"sliding-window":                  true,
	"prefix-caching-hash-algo":        true,
	"calculate-kv-scales":             true,
	"quantization":                    true,
	"dtype":                           true,
	"attention-backend":               true,
	"flashinfer-present":              true,
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

	execCtx, cancel := context.WithTimeout(ctx, runtimeDefaultsScriptTimeout)
	defer cancel()
	modelPathOverride := strings.TrimSpace(in.ModelPathOverride)
	execMeta, err := runDumpScript(execCtx, in.Target, scriptPath, dumpPath, modelPathOverride)
	if err != nil {
		return resolveRuntimeExecutionFailure(in, scriptPath, dumpPath, modelPathOverride, err)
	}
	dump, err := loadRuntimeDump(dumpPath)
	if err != nil {
		return Output{}, err
	}

	statusWarnings := copyStatusMap(dump.Warnings)
	statusErrors := copyStatusMap(dump.Errors)
	out, err := resolveFromDumpWithGeneratedFallback(
		in.Input,
		dump,
		statusWarnings,
		statusErrors,
		Resolve,
	)
	if err != nil {
		return Output{}, err
	}

	out.RuntimeSource = execMeta.Source
	out.RuntimePID = execMeta.PID
	if dump.PIDProcess.PID > 0 {
		out.RuntimePID = dump.PIDProcess.PID
	}
	out.RuntimeDumpPath = dumpPath
	out.RuntimeScriptPath = scriptPath
	out.RuntimeModelPath = modelPathOverride
	if out.RuntimeModelPath == "" {
		out.RuntimeModelPath = strings.TrimSpace(dump.PIDProcess.ModelPathOverride)
	}
	out.RuntimeWarnings = flattenStatusMap(statusWarnings)
	out.RuntimeErrors = flattenStatusMap(statusErrors)
	if strings.TrimSpace(dump.Metadata.VLLMVersion) != "" {
		out.ResolvedVersion = strings.TrimSpace(dump.Metadata.VLLMVersion)
	}
	if strings.TrimSpace(dump.Metadata.TorchVersion) != "" {
		out.ResolvedTorchVersion = strings.TrimSpace(dump.Metadata.TorchVersion)
	}
	return out, nil
}

func resolveRuntimeExecutionFailure(
	in RuntimeInput,
	scriptPath string,
	dumpPath string,
	modelPathOverride string,
	execErr error,
) (Output, error) {
	statusWarnings := map[string]string{
		"defaults.generated_fallback": "used generated vLLM defaults because runtime defaults script failed",
	}
	statusErrors := map[string]string{
		"runtime_dump.execute": execErr.Error(),
	}
	out, fallbackErr := Resolve(in.Input)
	if fallbackErr != nil {
		statusErrors["defaults.generated_fallback"] = fallbackErr.Error()
		return Output{}, fmt.Errorf("execute defaults script: %w; generated fallback: %v", execErr, fallbackErr)
	}
	source := runtimeSourceForTarget(in.Target)
	out.RuntimeSource = source
	if pid, err := runtimePID(in.Target, source); err == nil {
		out.RuntimePID = pid
	}
	out.RuntimeDumpPath = dumpPath
	out.RuntimeScriptPath = scriptPath
	out.RuntimeModelPath = modelPathOverride
	out.RuntimeEffectiveMode = "unavailable"
	out.RuntimeWarnings = flattenStatusMap(statusWarnings)
	out.RuntimeErrors = flattenStatusMap(statusErrors)
	return out, nil
}

func runtimeSourceForTarget(target shared.Candidate) string {
	source := strings.ToLower(strings.TrimSpace(target.Source))
	switch source {
	case "docker":
		return "docker"
	case "pod", "kubernetes":
		return "pod"
	default:
		return "process"
	}
}

func resolveFromDumpWithGeneratedFallback(
	input Input,
	dump runtimeDumpFile,
	warnings map[string]string,
	errors map[string]string,
	staticResolve func(Input) (Output, error),
) (Output, error) {
	out, err := resolveFromDump(input, dump)
	if err != nil {
		errors["runtime_dump.resolve"] = err.Error()
	}
	if err == nil && !shouldUseGeneratedDefaultsFallback(out.RuntimeEffectiveMode) {
		return out, nil
	}

	fallbackInput := input
	if runtimeVersion := strings.TrimSpace(dump.Metadata.VLLMVersion); runtimeVersion != "" {
		fallbackInput.VLLMVersion = runtimeVersion
	}
	fallbackOut, fallbackErr := staticResolve(fallbackInput)
	if fallbackErr != nil {
		errors["defaults.generated_fallback"] = fallbackErr.Error()
		if err != nil {
			return Output{}, err
		}
		out.AppliedDefaults += applyTrustedRuntimeObservedDefaults(&out, dump.EffectiveServeParameters)
		return out, nil
	}
	warnings["defaults.generated_fallback"] = generatedFallbackReason(out.RuntimeEffectiveMode, err)
	fallbackOut.AppliedDefaults += applyTrustedRuntimeObservedDefaults(&fallbackOut, dump.EffectiveServeParameters)
	fallbackOut.RuntimeEffectiveMode = out.RuntimeEffectiveMode
	return fallbackOut, nil
}

func shouldUseGeneratedDefaultsFallback(mode string) bool {
	trimmed := strings.TrimSpace(mode)
	return trimmed == "fallback" || trimmed == "unavailable"
}

func generatedFallbackReason(mode string, resolveErr error) string {
	trimmedMode := strings.TrimSpace(mode)
	if trimmedMode == "" {
		trimmedMode = "unknown"
	}
	if resolveErr != nil {
		return "used generated vLLM defaults because runtime defaults dump could not be resolved: " + resolveErr.Error()
	}
	return "used generated vLLM defaults because runtime effective config mode was " + trimmedMode
}

func copyStatusMap(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
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

func resolveFromDump(input Input, dump runtimeDumpFile) (Output, error) {
	if len(dump.EffectiveServeParameters) == 0 {
		return Output{}, fmt.Errorf("defaults dump does not include effective_serve_parameters")
	}
	explicit := normalizeArgs(input.ExplicitArgs)
	model := inferModel(explicit, input.RawCommandLine)
	requestedVersion := inferRequestedVersion(input, explicit)
	effective := restoreModelLabelsForOverride(dump.EffectiveServeParameters, model, dump.PIDProcess)
	effectiveMode := runtimeEffectiveMode(effective)

	resolved := copyStringMap(explicit)
	sources := explicitArgSources(explicit)
	applied := 0
	if effectiveMode == "" || effectiveMode == "full_vllm_config" {
		applied = applyEffectiveDefaults(resolved, sources, effective, model)
	}

	return Output{
		Args:                 resolved,
		ArgSources:           sources,
		SelectedModel:        model,
		RequestedVersion:     requestedVersion,
		AppliedDefaults:      applied,
		RuntimeEffectiveMode: effectiveMode,
	}, nil
}

func restoreModelLabelsForOverride(
	effective map[string]any,
	model string,
	pidProcess runtimePIDProcess,
) map[string]any {
	override := strings.TrimSpace(pidProcess.ModelPathOverride)
	if !pidProcess.ModelPathOverrideApplied || override == "" || strings.TrimSpace(model) == "" {
		return effective
	}
	out := make(map[string]any, len(effective))
	for key, value := range effective {
		out[key] = value
	}
	for _, key := range []string{"model", "served_model_name"} {
		if strings.TrimSpace(stringifyValue(out[key])) == override {
			out[key] = model
		}
	}
	return out
}

func runtimeEffectiveMode(effective map[string]any) string {
	return strings.TrimSpace(stringifyValue(effective["_effective_mode"]))
}

func applyEffectiveDefaults(target, sources map[string]string, effective map[string]any, model string) int {
	applied := 0
	effectiveSources := normalizeEffectiveSources(effective["_sources"])
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
		if sources == nil {
			sources = map[string]string{}
		}
		sources[key] = sourceLabel(effectiveSources[key])
		applied++
	}
	return applied
}

func normalizeEffectiveSources(raw any) map[string]string {
	input, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	sources := make(map[string]string, len(input))
	for key, value := range input {
		normalizedKey := normalizeKey(strings.ReplaceAll(key, "_", "-"))
		if normalizedKey == "" {
			continue
		}
		sources[normalizedKey] = stringifyValue(value)
	}
	return sources
}

func sourceLabel(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "effective_default"
	}
	return trimmed
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
