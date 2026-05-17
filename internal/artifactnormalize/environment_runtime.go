package artifactnormalize

import (
	"strings"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/types"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func normalizeEnvironment(input Input) contracts.Environment {
	cfg := input.Configurations
	return contracts.Environment{
		OS:             cfg.OS,
		Kernel:         cfg.Kernel,
		CPUModel:       cfg.CPUModel,
		CPUCores:       cfg.CPUCores,
		MemoryBytes:    int64(cfg.RAMBytes),
		GPUModel:       cfg.GPUModel,
		GPUCount:       cfg.GPUCount,
		DriverVersion:  cfg.DriverVersion,
		RuntimeVersion: firstNonEmpty(cfg.CUDARuntimeVersion, "unknown"),
	}
}

func normalizeRuntimeConfig(input Input) contracts.RuntimeConfig {
	args := input.Configurations.ParsedArgs
	argSources := input.Configurations.ParsedArgSources
	hints := input.Configurations.EnvironmentHints
	vllmSamples := input.Observations.Prometheus["vllm"]
	host, port := runtimeHostPort(args, input.Target.MetricsEndpoint)
	prefixCaching, prefixCachingSource := resolvePrefixCaching(args, vllmSamples)
	chunkedPrefill, _ := parseBool(args, []string{"enable-chunked-prefill", "chunked-prefill"})
	flashInfer, _ := parseBool(args, []string{"flashinfer-present", "enable-flashinfer", "flashinfer"})
	attentionBackend := strings.TrimSpace(args["attention-backend"])
	var flashAttention *bool
	if attentionBackend != "" {
		flashAttention = boolPtr(strings.Contains(strings.ToLower(attentionBackend), "flash"))
	}
	multimodalFlags := buildMultimodalFlags(input)
	runtime := contracts.RuntimeConfig{
		Model:                 firstNonEmpty(args["model"], hints["vllm_model"]),
		ServedModelName:       firstNonEmpty(args["served-model-name"], args["model"]),
		Host:                  host,
		Port:                  port,
		TensorParallelSize:    parseInt(args, []string{"tensor-parallel-size", "tp"}, 1),
		DataParallelSize:      parseInt(args, []string{"data-parallel-size", "dp"}, 1),
		PipelineParallelSize:  parseInt(args, []string{"pipeline-parallel-size", "pp"}, 1),
		MaxModelLen:           parseInt(args, []string{"max-model-len"}, 0),
		MaxNumBatchedTokens:   parseInt(args, []string{"max-num-batched-tokens"}, 0),
		MaxNumSeqs:            parseInt(args, []string{"max-num-seqs"}, 0),
		Scheduler:             normalizeSchedulerConfig(args),
		GPUMemoryUtilization:  parseFloat(args, []string{"gpu-memory-utilization"}, 0),
		KVCacheDType:          firstNonEmpty(args["kv-cache-dtype"], "auto"),
		Cache:                 normalizeCacheConfig(args, vllmSamples),
		ChunkedPrefill:        chunkedPrefill,
		PrefixCaching:         prefixCaching,
		Quantization:          firstNonEmpty(args["quantization"], "none"),
		DType:                 firstNonEmpty(args["dtype"], "auto"),
		MultimodalFlags:       multimodalFlags,
		EnvHints:              hints,
		VLLMVersion:           firstNonEmpty(hints["vllm_version_hint"], "unknown"),
		TorchVersion:          firstNonEmpty(hints["torch_version"], "unknown"),
		CUDARuntimeVersion:    firstNonEmpty(input.Configurations.CUDARuntimeVersion, "unknown"),
		NvidiaDriverVersion:   firstNonEmpty(input.Configurations.DriverVersion, "unknown"),
		AttentionBackend:      attentionBackend,
		FlashinferPresent:     flashInfer,
		FlashAttentionPresent: flashAttention,
		ImageProcessor:        imageProcessorValue(args, input.UserIntent),
		ValueSources:          runtimeValueSources(argSources, prefixCachingSource),
	}
	runtime.Coverage = runtimeCoverage(runtime, input)
	return runtime
}

func runtimeHostPort(args map[string]string, endpoint string) (string, int) {
	host, port := parseHostPort(endpoint)
	if explicitHost := strings.TrimSpace(args["host"]); explicitHost != "" {
		host = explicitHost
	}
	if explicitPort := parseInt(args, []string{"port"}, 0); explicitPort > 0 {
		port = explicitPort
	}
	return host, port
}

func runtimeCoverage(runtime contracts.RuntimeConfig, input Input) contracts.SourceCoverage {
	present := map[string]bool{}
	appendPresent(present, "max_model_len", runtime.MaxModelLen > 0)
	appendPresent(present, "max_num_batched_tokens", runtime.MaxNumBatchedTokens > 0)
	appendPresent(present, "max_num_seqs", runtime.MaxNumSeqs > 0)
	appendPresent(present, "scheduler_details", runtime.Scheduler.HasData())
	appendPresent(present, "gpu_memory_utilization", runtime.GPUMemoryUtilization > 0)
	appendPresent(present, "cache_details", runtime.Cache.HasData())
	appendPresent(present, "parallelism_settings", runtime.TensorParallelSize > 0 || runtime.DataParallelSize > 0 || runtime.PipelineParallelSize > 0)
	appendPresent(present, "quantization_mode", runtime.Quantization != "")
	appendPresent(present, "prefix_caching_state", runtime.PrefixCaching != nil)
	appendPresent(present, "chunked_prefill_state", runtime.ChunkedPrefill != nil)
	appendPresent(present, "multimodal_runtime_hints", len(runtime.MultimodalFlags) > 0)
	appendPresent(present, "vllm_version", runtime.VLLMVersion != "")
	appendPresent(present, "torch_version", runtime.TorchVersion != "")
	appendPresent(present, "cuda_runtime_version", runtime.CUDARuntimeVersion != "")
	appendPresent(present, "nvidia_driver_version", runtime.NvidiaDriverVersion != "")
	appendPresent(present, "attention_backend", runtime.AttentionBackend != "")
	appendPresent(present, "flashinfer_presence", runtime.FlashinferPresent != nil)
	appendPresent(present, "flash_attention_presence", runtime.FlashAttentionPresent != nil)
	appendPresent(present, "image_processor", runtime.ImageProcessor != "")
	coverage := newCoverage(present, runtimeRequiredFields())
	if multimodalRuntimeHintsUnsupported(runtime, input) {
		coverage = markUnsupported(coverage, "multimodal_runtime_hints")
	}
	return coverage
}

func runtimeRequiredFields() []string {
	return []string{
		"max_model_len",
		"max_num_batched_tokens",
		"max_num_seqs",
		"scheduler_details",
		"gpu_memory_utilization",
		"cache_details",
		"parallelism_settings",
		"quantization_mode",
		"prefix_caching_state",
		"chunked_prefill_state",
		"multimodal_runtime_hints",
		"vllm_version",
		"torch_version",
		"cuda_runtime_version",
		"nvidia_driver_version",
		"attention_backend",
		"flashinfer_presence",
		"flash_attention_presence",
		"image_processor",
	}
}

func runtimeValueSources(argSources map[string]string, prefixCachingSource string) map[string]string {
	sources := map[string]string{}
	copyIfPresent := func(contractKey, argKey string) {
		value := strings.TrimSpace(argSources[argKey])
		if value == "" {
			return
		}
		sources[contractKey] = value
	}
	copyIfPresent("max_model_len", "max-model-len")
	copyIfPresent("max_num_batched_tokens", "max-num-batched-tokens")
	copyIfPresent("max_num_seqs", "max-num-seqs")
	copyIfPresent("scheduler.policy", "scheduler-policy")
	copyIfPresent("scheduler.policy", "scheduling-policy")
	copyIfPresent("scheduler.max_num_partial_prefills", "max-num-partial-prefills")
	copyIfPresent("scheduler.max_long_partial_prefills", "max-long-partial-prefills")
	copyIfPresent("scheduler.long_prefill_token_threshold", "long-prefill-token-threshold")
	copyIfPresent("gpu_memory_utilization", "gpu-memory-utilization")
	copyIfPresent("cache.block_size", "block-size")
	copyIfPresent("cache.kv_offloading_backend", "kv-offloading-backend")
	copyIfPresent("prefix_caching", "enable-prefix-caching")
	copyIfPresent("chunked_prefill", "enable-chunked-prefill")
	copyIfPresent("flashinfer_presence", "flashinfer-present")
	if strings.TrimSpace(prefixCachingSource) != "" {
		sources["prefix_caching"] = strings.TrimSpace(prefixCachingSource)
	}
	if len(sources) == 0 {
		return nil
	}
	return sources
}

func resolvePrefixCaching(args map[string]string, samples []promcollector.Sample) (*bool, string) {
	if value, ok := cacheConfigPrefixCaching(samples); ok {
		return boolPtr(value), "metrics.vllm.cache_config_info.enable_prefix_caching"
	}
	if value, ok := parseBool(args, []string{"enable-prefix-caching", "prefix-caching"}); ok {
		return value, ""
	}
	return nil, ""
}

func cacheConfigPrefixCaching(samples []promcollector.Sample) (bool, bool) {
	if len(samples) == 0 {
		return false, false
	}
	latest := samples[len(samples)-1]
	for _, metric := range latest.Metrics {
		if metric.Name != "vllm:cache_config_info" {
			continue
		}
		flag := strings.TrimSpace(strings.ToLower(metric.Labels["enable_prefix_caching"]))
		switch flag {
		case "true", "1":
			return true, true
		case "false", "0":
			return false, true
		}
	}
	return false, false
}

func imageProcessorValue(args map[string]string, intent types.UserIntent) string {
	if value := strings.TrimSpace(args["image-processor"]); value != "" {
		return value
	}
	if !intent.Multimodal {
		return "none"
	}
	return "unknown"
}
