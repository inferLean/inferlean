package collector

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/inferLean/inferlean/internal/discovery"
	"github.com/inferLean/inferlean/pkg/contracts"
)

type runtimeProbeResult struct {
	VLLMVersion           string `json:"vllm_version,omitempty"`
	TorchVersion          string `json:"torch_version,omitempty"`
	CUDARuntimeVersion    string `json:"cuda_runtime_version,omitempty"`
	AttentionBackend      string `json:"attention_backend,omitempty"`
	FlashinferPresent     *bool  `json:"flashinfer_present,omitempty"`
	FlashAttentionPresent *bool  `json:"flash_attention_present,omitempty"`
}

func probeRuntimeConfig(ctx context.Context, target discovery.CandidateGroup, rawPath, driverVersion string) contracts.RuntimeConfig {
	cfg := baseRuntimeConfig(target.RuntimeConfig)
	cfg.NvidiaDriverVersion = driverVersion
	cfg.ProbeEvidenceRef = relativeRawArtifact(rawPath)

	inspected := inspectRuntimeContext(target)
	result, warnings := runRuntimeProbe(ctx, target, inspected)
	cfg.VLLMVersion = firstNonEmpty(result.VLLMVersion, cfg.VLLMVersion)
	cfg.TorchVersion = firstNonEmpty(result.TorchVersion, cfg.TorchVersion)
	cfg.CUDARuntimeVersion = firstNonEmpty(result.CUDARuntimeVersion, cfg.CUDARuntimeVersion)
	cfg.AttentionBackend = firstNonEmpty(result.AttentionBackend, cfg.AttentionBackend)
	cfg.FlashinferPresent = mergeBoolPointer(cfg.FlashinferPresent, result.FlashinferPresent)
	cfg.FlashAttentionPresent = mergeBoolPointer(cfg.FlashAttentionPresent, result.FlashAttentionPresent)
	cfg.ProbeWarnings = warnings
	cfg.Coverage = runtimeCoverage(cfg, relativeRawArtifact(rawPath))
	_ = writeJSONFile(rawPath, map[string]any{
		"runtime_config":  cfg,
		"probe":           result,
		"warnings":        warnings,
		"process_context": inspected,
	})
	return cfg
}

func runRuntimeProbe(ctx context.Context, target discovery.CandidateGroup, inspected runtimeProbeContext) (runtimeProbeResult, []string) {
	pythonExe := inferPythonExecutable(target, inspected)
	if pythonExe == "" {
		result := fallbackRuntimeProbe(target, inspected)
		return result, []string{"could not infer a python executable from the target process"}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := pythonRuntimeProbe(probeCtx, pythonExe, inspected, target.PrimaryPID)
	if err == nil {
		return mergeRuntimeProbe(result, fallbackRuntimeProbe(target, inspected)), nil
	}

	fallback := fallbackRuntimeProbe(target, inspected)
	return mergeRuntimeProbe(result, fallback), []string{err.Error()}
}

func inferPythonExecutable(target discovery.CandidateGroup, inspected runtimeProbeContext) string {
	if strings.Contains(filepath.Base(inspected.ResolvedExecutable), "python") {
		return inspected.ResolvedExecutable
	}
	if strings.Contains(filepath.Base(target.Executable), "python") {
		return target.Executable
	}
	for _, candidate := range []string{
		filepath.Join(inspected.Environment["VIRTUAL_ENV"], "bin", "python"),
		filepath.Join(inspected.Environment["CONDA_PREFIX"], "bin", "python"),
	} {
		if strings.TrimSpace(candidate) != "" {
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
				return candidate
			}
		}
	}
	return ""
}

func pythonRuntimeProbe(ctx context.Context, pythonExe string, inspected runtimeProbeContext, pid int32) (runtimeProbeResult, error) {
	script := strings.Join([]string{
		"import importlib.util, json, os, sys",
		"out = {}",
		"try:\n import vllm; out['vllm_version'] = getattr(vllm, '__version__', '')\nexcept Exception:\n pass",
		"try:\n import torch; out['torch_version'] = getattr(torch, '__version__', ''); out['cuda_runtime_version'] = getattr(torch.version, 'cuda', '')\nexcept Exception:\n pass",
		"out['attention_backend'] = os.environ.get('VLLM_ATTENTION_BACKEND', '')",
		"out['flashinfer_present'] = importlib.util.find_spec('flashinfer') is not None",
		"out['flash_attention_present'] = importlib.util.find_spec('flash_attn') is not None",
		"print(json.dumps(out))",
	}, "\n")
	cmd := exec.CommandContext(ctx, pythonExe, "-c", script)
	cmd.Dir = firstNonEmpty(inspected.WorkingDirectory, cmd.Dir)
	cmd.Env = append(os.Environ(), "INFERLEAN_TARGET_PID="+strconv.Itoa(int(pid)))
	output, err := cmd.Output()
	if err != nil {
		return runtimeProbeResult{}, err
	}
	var result runtimeProbeResult
	return result, json.Unmarshal(output, &result)
}

func fallbackRuntimeProbe(target discovery.CandidateGroup, inspected runtimeProbeContext) runtimeProbeResult {
	result := runtimeProbeResult{AttentionBackend: target.RuntimeConfig.AttentionBackend}
	for _, siteDir := range inspected.SitePackages {
		if result.VLLMVersion == "" {
			result.VLLMVersion = distInfoVersion(siteDir, "vllm")
		}
		if result.TorchVersion == "" {
			result.TorchVersion = distInfoVersion(siteDir, "torch")
		}
	}
	return result
}

func distInfoVersion(root, name string) string {
	patterns := []string{
		filepath.Join(root, "python*", "site-packages", name+"-*.dist-info"),
		filepath.Join(root, name+"-*.dist-info"),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			base := filepath.Base(match)
			base = strings.TrimSuffix(base, ".dist-info")
			if strings.HasPrefix(base, name+"-") {
				return strings.TrimPrefix(base, name+"-")
			}
		}
	}
	return ""
}

func mergeRuntimeProbe(primary, fallback runtimeProbeResult) runtimeProbeResult {
	return runtimeProbeResult{
		VLLMVersion:           firstNonEmpty(primary.VLLMVersion, fallback.VLLMVersion),
		TorchVersion:          firstNonEmpty(primary.TorchVersion, fallback.TorchVersion),
		CUDARuntimeVersion:    firstNonEmpty(primary.CUDARuntimeVersion, fallback.CUDARuntimeVersion),
		AttentionBackend:      firstNonEmpty(primary.AttentionBackend, fallback.AttentionBackend),
		FlashinferPresent:     mergeBoolPointer(primary.FlashinferPresent, fallback.FlashinferPresent),
		FlashAttentionPresent: mergeBoolPointer(primary.FlashAttentionPresent, fallback.FlashAttentionPresent),
	}
}

func runtimeCoverage(cfg contracts.RuntimeConfig, rawRef string) contracts.SourceCoverage {
	coverage := newCoverageBuilder(rawRef)
	markRuntimeString(coverage, "max_model_len", strconv.Itoa(cfg.MaxModelLen))
	markRuntimeString(coverage, "max_num_batched_tokens", strconv.Itoa(cfg.MaxNumBatchedTokens))
	markRuntimeString(coverage, "max_num_seqs", strconv.Itoa(cfg.MaxNumSeqs))
	markRuntimeFloat(coverage, "gpu_memory_utilization", cfg.GPUMemoryUtilization)
	if cfg.TensorParallelSize != 0 || cfg.DataParallelSize != 0 || cfg.PipelineParallelSize != 0 {
		coverage.Present("parallelism_settings")
	} else {
		coverage.Missing("parallelism_settings")
	}
	markRuntimeString(coverage, "quantization_mode", cfg.Quantization)
	markRuntimeBool(coverage, "prefix_caching_state", cfg.PrefixCaching)
	markRuntimeBool(coverage, "chunked_prefill_state", cfg.ChunkedPrefill)
	markRuntimeSlice(coverage, "multimodal_runtime_hints", cfg.MultimodalFlags)
	markRuntimeString(coverage, "vllm_version", cfg.VLLMVersion)
	markRuntimeString(coverage, "torch_version", cfg.TorchVersion)
	markRuntimeString(coverage, "cuda_runtime_version", cfg.CUDARuntimeVersion)
	markRuntimeString(coverage, "nvidia_driver_version", cfg.NvidiaDriverVersion)
	markRuntimeString(coverage, "attention_backend", cfg.AttentionBackend)
	markRuntimeBool(coverage, "flashinfer_presence", cfg.FlashinferPresent)
	markRuntimeBool(coverage, "flash_attention_presence", cfg.FlashAttentionPresent)
	markRuntimeString(coverage, "image_processor", cfg.ImageProcessor)
	markRuntimeSlice(coverage, "multimodal_cache_hints", cfg.MultimodalCacheHints)
	return coverage.Build()
}

func markRuntimeString(coverage *coverageBuilder, field, value string) {
	if strings.TrimSpace(value) == "" {
		coverage.Missing(field)
		return
	}
	coverage.Present(field)
}

func markRuntimeFloat(coverage *coverageBuilder, field string, value float64) {
	if value == 0 {
		coverage.Missing(field)
		return
	}
	coverage.Present(field)
}

func markRuntimeBool(coverage *coverageBuilder, field string, value *bool) {
	if value == nil {
		coverage.Missing(field)
		return
	}
	coverage.Present(field)
}

func markRuntimeSlice(coverage *coverageBuilder, field string, values []string) {
	if len(values) == 0 {
		coverage.Missing(field)
		return
	}
	coverage.Present(field)
}

func mergeBoolPointer(primary, secondary *bool) *bool {
	if primary != nil {
		return primary
	}
	return secondary
}

func firstNonEmpty(primary, secondary string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return secondary
}
