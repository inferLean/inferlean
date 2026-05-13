package artifactnormalize

import (
	"strings"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func normalizeVLLMMetrics(samples []promcollector.Sample) contracts.VLLMMetrics {
	cpuKVBlocks, derivedCPUKVBlocks := cpuKVBlocksWindow(samples)
	gpuKVUsage, derivedGPUKVUsage := gpuKVCacheUsageWindow(samples)
	cpuKVUsage, derivedCPUKVUsage := cpuKVCacheUsageWindow(samples, derivedCPUKVBlocks)
	swappedRequests, derivedSwappedRequests := swappedRequestsWindow(samples, derivedCPUKVBlocks)
	kvOffloadActivity, derivedKVOffloadActivity := kvOffloadActivityWindow(samples, swappedRequests, cpuKVUsage, derivedCPUKVUsage || derivedSwappedRequests)
	metrics := contracts.VLLMMetrics{
		RequestsRunning:           windowFromMetric(samples, "vllm:num_requests_running"),
		RequestsWaiting:           windowFromMetric(samples, "vllm:num_requests_waiting"),
		RequestsWaitingByReason:   windowsByLabel(samples, "vllm:num_requests_waiting_by_reason", "reason"),
		RequestThroughput:         deltaRateWindow(samples, "vllm:request_success_total", 1),
		CompletedRequests:         deltaSnapshot(samples, "vllm:request_success_total"),
		LatencyE2E:                histogramMeanWindow(samples, "vllm:e2e_request_latency_seconds"),
		LatencyTTFT:               histogramMeanWindow(samples, "vllm:time_to_first_token_seconds"),
		LatencyQueue:              histogramMeanWindow(samples, "vllm:request_queue_time_seconds"),
		LatencyPrefill:            histogramMeanWindow(samples, "vllm:request_prefill_time_seconds"),
		LatencyDecode:             histogramMeanWindow(samples, "vllm:request_decode_time_seconds"),
		PromptTokens:              histogramMeanWindow(samples, "vllm:request_prompt_tokens"),
		PromptTokensProcessed:     deltaSnapshot(samples, "vllm:prompt_tokens_total"),
		PromptTokensBySource:      deltaSnapshot(samples, "vllm:prompt_tokens_by_source_total"),
		CachedPromptTokens:        deltaSnapshot(samples, "vllm:prompt_tokens_cached_total"),
		GenerationTokens:          histogramMeanWindow(samples, "vllm:request_generation_tokens"),
		GenerationTokensProcessed: deltaSnapshot(samples, "vllm:generation_tokens_total"),
		PromptLength:              histogramDistribution(samples, "vllm:request_prompt_tokens"),
		GenerationLength:          histogramDistribution(samples, "vllm:request_generation_tokens"),
		KVCacheUsage: windowFromAnyMetric(
			samples,
			"vllm:kv_cache_usage_perc",
			"vllm:gpu_cache_usage_perc",
		),
		GPUKVCacheUsage: gpuKVUsage,
		CPUKVCacheUsage: cpuKVUsage,
		CPUKVBlocks:     cpuKVBlocks,
		Preemptions: windowFromAnyMetric(
			samples,
			"vllm:num_preemptions_total",
			"vllm:num_preemptions",
		),
		SwappedRequests:        swappedRequests,
		RecomputedPromptTokens: windowFromMetric(samples, "vllm:prompt_tokens_recomputed_total"),
		KVOffloadActivity:      kvOffloadActivity,
		PrefixCache: cacheSnapshot(
			samples,
			"vllm:prefix_cache_hits_total",
			"vllm:prefix_cache_queries_total",
		),
		MultimodalCache: cacheSnapshot(
			samples,
			"vllm:mm_cache_hits_total",
			"vllm:mm_cache_queries_total",
		),
	}
	metrics.Coverage = vllmCoverage(metrics, len(samples) > 0 && !hasMetric(samples, "vllm:prompt_tokens_recomputed_total"))
	metrics.Coverage.DerivedFields = derivedVLLMFields(
		derivedCPUKVBlocks,
		derivedGPUKVUsage,
		derivedCPUKVUsage,
		derivedSwappedRequests,
		derivedKVOffloadActivity,
	)
	return metrics
}

func gpuKVCacheUsageWindow(samples []promcollector.Sample) (contracts.MetricWindow, bool) {
	window := windowFromMetric(samples, "vllm:gpu_cache_usage_perc")
	if window.HasData() {
		return window, false
	}
	window = windowFromMetric(samples, "vllm:kv_cache_usage_perc")
	if window.HasData() {
		return window, true
	}
	return contracts.MetricWindow{}, false
}

func cpuKVBlocksWindow(samples []promcollector.Sample) (contracts.MetricWindow, bool) {
	window := windowFromInfoLabel(samples, "vllm:cache_config_info", "num_cpu_blocks")
	if window.HasData() {
		return window, false
	}
	if cacheInfoLabelIsNone(samples, "num_cpu_blocks") {
		return zeroWindowForMetricPresence(samples, "vllm:cache_config_info"), true
	}
	return contracts.MetricWindow{}, false
}

func cpuKVCacheUsageWindow(samples []promcollector.Sample, noCPUBlocks bool) (contracts.MetricWindow, bool) {
	window := windowFromMetric(samples, "vllm:cpu_cache_usage_perc")
	if window.HasData() {
		return window, false
	}
	if noCPUBlocks {
		return zeroWindowForMetricPresence(samples, "vllm:cache_config_info"), true
	}
	return contracts.MetricWindow{}, false
}

func swappedRequestsWindow(samples []promcollector.Sample, noCPUBlocks bool) (contracts.MetricWindow, bool) {
	window := windowFromMetric(samples, "vllm:num_requests_swapped")
	if window.HasData() {
		return window, false
	}
	if noCPUBlocks {
		return zeroWindowForMetricPresence(samples, "vllm:cache_config_info"), true
	}
	return contracts.MetricWindow{}, false
}

func kvOffloadActivityWindow(samples []promcollector.Sample, swapped, cpuUsage contracts.MetricWindow, derivedFromNoCPUBlocks bool) (contracts.MetricWindow, bool) {
	window := firstWindow(swapped, cpuUsage)
	if window.HasData() {
		return window, derivedFromNoCPUBlocks
	}
	if derivedFromNoCPUBlocks {
		return zeroWindowForMetricPresence(samples, "vllm:cache_config_info"), true
	}
	return contracts.MetricWindow{}, false
}

func cacheInfoLabelIsNone(samples []promcollector.Sample, label string) bool {
	value := strings.TrimSpace(strings.ToLower(latestInfoLabel(samples, "vllm:cache_config_info", label)))
	return value == "none" || value == "null" || value == "0"
}

func derivedVLLMFields(cpuBlocks, gpuUsage, cpuUsage, swappedRequests, offloadActivity bool) []string {
	fields := []string{}
	if cpuBlocks {
		fields = append(fields, "cpu_kv_blocks")
	}
	if gpuUsage {
		fields = append(fields, "gpu_kv_cache_usage")
	}
	if cpuUsage {
		fields = append(fields, "cpu_kv_cache_usage")
	}
	if swappedRequests {
		fields = append(fields, "swapped_requests")
	}
	if offloadActivity {
		fields = append(fields, "kv_offload_activity")
	}
	return fields
}

func vllmCoverage(metrics contracts.VLLMMetrics, recomputedUnsupported bool) contracts.SourceCoverage {
	present := map[string]bool{}
	appendPresent(present, "requests_running", metrics.RequestsRunning.HasData())
	appendPresent(present, "requests_waiting", metrics.RequestsWaiting.HasData())
	appendPresent(present, "requests_waiting_by_reason", len(metrics.RequestsWaitingByReason) > 0)
	appendPresent(present, "request_throughput", metrics.RequestThroughput.HasData())
	appendPresent(present, "completed_requests", metrics.CompletedRequests.HasData())
	appendPresent(present, "latency_e2e", metrics.LatencyE2E.HasData())
	appendPresent(present, "latency_ttft", metrics.LatencyTTFT.HasData())
	appendPresent(present, "latency_queue", metrics.LatencyQueue.HasData())
	appendPresent(present, "latency_prefill", metrics.LatencyPrefill.HasData())
	appendPresent(present, "latency_decode", metrics.LatencyDecode.HasData())
	appendPresent(present, "prompt_tokens", metrics.PromptTokens.HasData())
	appendPresent(present, "prompt_tokens_processed", metrics.PromptTokensProcessed.HasData())
	appendPresent(present, "prompt_tokens_by_source", metrics.PromptTokensBySource.HasData())
	appendPresent(present, "cached_prompt_tokens", metrics.CachedPromptTokens.HasData())
	appendPresent(present, "generation_tokens", metrics.GenerationTokens.HasData())
	appendPresent(present, "generation_tokens_processed", metrics.GenerationTokensProcessed.HasData())
	appendPresent(present, "prompt_length", metrics.PromptLength.HasData())
	appendPresent(present, "generation_length", metrics.GenerationLength.HasData())
	appendPresent(present, "kv_cache_usage", metrics.KVCacheUsage.HasData())
	appendPresent(present, "gpu_kv_cache_usage", metrics.GPUKVCacheUsage.HasData())
	appendPresent(present, "cpu_kv_cache_usage", metrics.CPUKVCacheUsage.HasData())
	appendPresent(present, "cpu_kv_blocks", metrics.CPUKVBlocks.HasData())
	appendPresent(present, "preemptions", metrics.Preemptions.HasData())
	appendPresent(present, "swapped_requests", metrics.SwappedRequests.HasData())
	appendPresent(present, "recomputed_prompt_tokens", metrics.RecomputedPromptTokens.HasData())
	appendPresent(present, "kv_offload_activity", metrics.KVOffloadActivity.HasData())
	appendPresent(present, "prefix_cache", metrics.PrefixCache.HasData())
	appendPresent(present, "multimodal_cache", metrics.MultimodalCache.HasData())
	coverage := newCoverage(present, vllmRequiredFields())
	if recomputedUnsupported {
		coverage = markUnsupported(coverage, "recomputed_prompt_tokens")
	}
	if metrics.RequestThroughput.HasData() {
		coverage.PresentFields = append(coverage.PresentFields, "request_throughput")
	} else {
		coverage.MissingFields = append(coverage.MissingFields, "request_throughput")
	}
	return coverage
}

func vllmRequiredFields() []string {
	return []string{
		"requests_running",
		"requests_waiting",
		"requests_waiting_by_reason",
		"completed_requests",
		"latency_e2e",
		"latency_ttft",
		"latency_queue",
		"latency_prefill",
		"latency_decode",
		"prompt_tokens",
		"prompt_tokens_processed",
		"prompt_tokens_by_source",
		"cached_prompt_tokens",
		"generation_tokens",
		"generation_tokens_processed",
		"prompt_length",
		"generation_length",
		"kv_cache_usage",
		"gpu_kv_cache_usage",
		"cpu_kv_cache_usage",
		"cpu_kv_blocks",
		"preemptions",
		"swapped_requests",
		"recomputed_prompt_tokens",
		"kv_offload_activity",
		"prefix_cache",
		"multimodal_cache",
	}
}
