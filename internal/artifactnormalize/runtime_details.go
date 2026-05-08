package artifactnormalize

import (
	"strconv"
	"strings"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func normalizeSchedulerConfig(args map[string]string) contracts.SchedulerConfig {
	asyncScheduling, _ := parseBool(args, []string{"async-scheduling"})
	reserveFullISL, _ := parseBool(args, []string{"scheduler-reserve-full-isl"})
	disableChunkedMM, _ := parseBool(args, []string{"disable-chunked-mm-input"})
	disableHybridKV, _ := parseBool(args, []string{"disable-hybrid-kv-cache-manager"})
	return contracts.SchedulerConfig{
		AsyncScheduling:             asyncScheduling,
		Policy:                      firstNonEmpty(args["scheduler-policy"], args["scheduling-policy"], args["policy"]),
		MaxNumPartialPrefills:       parseInt(args, []string{"max-num-partial-prefills"}, 0),
		MaxLongPartialPrefills:      parseInt(args, []string{"max-long-partial-prefills"}, 0),
		LongPrefillTokenThreshold:   parseInt(args, []string{"long-prefill-token-threshold"}, 0),
		MaxNumScheduledTokens:       parseInt(args, []string{"max-num-scheduled-tokens"}, 0),
		MaxNumEncoderInputTokens:    parseInt(args, []string{"max-num-encoder-input-tokens"}, 0),
		SchedulerReserveFullISL:     reserveFullISL,
		DisableChunkedMMInput:       disableChunkedMM,
		DisableHybridKVCacheManager: disableHybridKV,
	}
}

func normalizeCacheConfig(args map[string]string, samples []promcollector.Sample) contracts.CacheConfig {
	labels := latestCacheConfigLabels(samples)
	kvSharingFastPrefill := parseBoolValue(firstNonEmpty(labels["kv_sharing_fast_prefill"], args["kv-sharing-fast-prefill"]))
	calculateKVScales := parseBoolValue(firstNonEmpty(labels["calculate_kv_scales"], args["calculate-kv-scales"]))
	return contracts.CacheConfig{
		BlockSize:             parseIntValue(firstNonEmpty(labels["block_size"], args["block-size"])),
		CacheDType:            firstNonEmpty(labels["cache_dtype"], args["kv-cache-dtype"]),
		NumGPUBlocks:          parseIntValue(labels["num_gpu_blocks"]),
		NumCPUBlocks:          parseIntValue(labels["num_cpu_blocks"]),
		KVCacheMemoryBytes:    parseInt64Value(firstNonEmpty(labels["kv_cache_memory_bytes"], args["kv-cache-memory-bytes"])),
		KVOffloadingBackend:   firstNonEmpty(labels["kv_offloading_backend"], args["kv-offloading-backend"]),
		KVOffloadingSizeBytes: parseInt64Value(firstNonEmpty(labels["kv_offloading_size"], args["kv-offloading-size"])),
		KVSharingFastPrefill:  kvSharingFastPrefill,
		SlidingWindow:         parseIntValue(firstNonEmpty(labels["sliding_window"], args["sliding-window"])),
		PrefixCachingHashAlgo: firstNonEmpty(labels["prefix_caching_hash_algo"], args["prefix-caching-hash-algo"]),
		CalculateKVScales:     calculateKVScales,
		GPUMemoryUtilization:  parseFloatValue(firstNonEmpty(labels["gpu_memory_utilization"], args["gpu-memory-utilization"])),
	}
}

func latestCacheConfigLabels(samples []promcollector.Sample) map[string]string {
	if len(samples) == 0 {
		return nil
	}
	for idx := len(samples) - 1; idx >= 0; idx-- {
		for _, metric := range samples[idx].Metrics {
			if metric.Name == "vllm:cache_config_info" {
				return metric.Labels
			}
		}
	}
	return nil
}

func parseBoolValue(raw string) *bool {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" || value == "none" || value == "null" {
		return nil
	}
	switch value {
	case "1", "true", "yes", "on", "enabled":
		return boolPtr(true)
	case "0", "false", "no", "off", "disabled":
		return boolPtr(false)
	default:
		return nil
	}
}

func parseIntValue(raw string) int {
	parsed, ok := parseInt64(raw)
	if !ok {
		return 0
	}
	return int(parsed)
}

func parseInt64Value(raw string) int64 {
	parsed, ok := parseInt64(raw)
	if !ok {
		return 0
	}
	return parsed
}

func parseFloatValue(raw string) float64 {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "null") {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseInt64(raw string) (int64, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "none") || strings.EqualFold(value, "null") {
		return 0, false
	}
	if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
		return parsed, true
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return int64(parsed), true
}
