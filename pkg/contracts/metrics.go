package contracts

import "time"

type Metrics struct {
	VLLM      VLLMMetrics      `json:"vllm,omitempty"`
	Host      HostMetrics      `json:"host,omitempty"`
	GPU       GPUTelemetry     `json:"gpu,omitempty"`
	NvidiaSmi NvidiaSMIMetrics `json:"nvidia_smi,omitempty"`
}

type MetricSample struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type MetricWindow struct {
	Latest  *float64       `json:"latest,omitempty"`
	Min     *float64       `json:"min,omitempty"`
	Max     *float64       `json:"max,omitempty"`
	Avg     *float64       `json:"avg,omitempty"`
	Samples []MetricSample `json:"samples,omitempty"`
}

type DistributionBucket struct {
	UpperBound string  `json:"upper_bound"`
	Value      float64 `json:"value"`
}

type DistributionSnapshot struct {
	Count   *float64             `json:"count,omitempty"`
	Sum     *float64             `json:"sum,omitempty"`
	Buckets []DistributionBucket `json:"buckets,omitempty"`
}

type LabeledDelta struct {
	Labels map[string]string `json:"labels,omitempty"`
	Value  float64           `json:"value"`
}

type DeltaSnapshot struct {
	Total  *float64       `json:"total,omitempty"`
	Series []LabeledDelta `json:"series,omitempty"`
}

type CacheSnapshot struct {
	Hits    *float64 `json:"hits,omitempty"`
	Queries *float64 `json:"queries,omitempty"`
	HitRate *float64 `json:"hit_rate,omitempty"`
}

type SourceCoverage struct {
	PresentFields     []string `json:"present_fields,omitempty"`
	MissingFields     []string `json:"missing_fields,omitempty"`
	UnsupportedFields []string `json:"unsupported_fields,omitempty"`
	DerivedFields     []string `json:"derived_fields,omitempty"`
	RawEvidenceRef    string   `json:"raw_evidence_ref,omitempty"`
}

type MemoryMetrics struct {
	Used     MetricWindow `json:"used,omitempty"`
	Free     MetricWindow `json:"free,omitempty"`
	Reserved MetricWindow `json:"reserved,omitempty"`
	Total    MetricWindow `json:"total,omitempty"`
}

type ClockMetrics struct {
	SM     MetricWindow `json:"sm,omitempty"`
	Memory MetricWindow `json:"memory,omitempty"`
}

type LoadMetrics struct {
	Load1  MetricWindow `json:"load1,omitempty"`
	Load5  MetricWindow `json:"load5,omitempty"`
	Load15 MetricWindow `json:"load15,omitempty"`
}

type ThroughputMetrics struct {
	RX MetricWindow `json:"rx,omitempty"`
	TX MetricWindow `json:"tx,omitempty"`
}

type DiskIOMetrics struct {
	ReadBytes  MetricWindow `json:"read_bytes,omitempty"`
	WriteBytes MetricWindow `json:"write_bytes,omitempty"`
}

type ReliabilityMetrics struct {
	XID MetricWindow `json:"xid,omitempty"`
	ECC MetricWindow `json:"ecc,omitempty"`
}

type VLLMMetrics struct {
	RequestsRunning           MetricWindow            `json:"requests_running,omitempty"`
	RequestsWaiting           MetricWindow            `json:"requests_waiting,omitempty"`
	RequestsWaitingByReason   map[string]MetricWindow `json:"requests_waiting_by_reason,omitempty"`
	RequestThroughput         MetricWindow            `json:"request_throughput,omitempty"`
	CompletedRequests         DeltaSnapshot           `json:"completed_requests,omitempty"`
	LatencyE2E                MetricWindow            `json:"latency_e2e,omitempty"`
	LatencyTTFT               MetricWindow            `json:"latency_ttft,omitempty"`
	LatencyQueue              MetricWindow            `json:"latency_queue,omitempty"`
	LatencyPrefill            MetricWindow            `json:"latency_prefill,omitempty"`
	LatencyDecode             MetricWindow            `json:"latency_decode,omitempty"`
	PromptTokens              MetricWindow            `json:"prompt_tokens,omitempty"`
	PromptTokensProcessed     DeltaSnapshot           `json:"prompt_tokens_processed,omitempty"`
	PromptTokensBySource      DeltaSnapshot           `json:"prompt_tokens_by_source,omitempty"`
	CachedPromptTokens        DeltaSnapshot           `json:"cached_prompt_tokens,omitempty"`
	GenerationTokens          MetricWindow            `json:"generation_tokens,omitempty"`
	GenerationTokensProcessed DeltaSnapshot           `json:"generation_tokens_processed,omitempty"`
	PromptLength              DistributionSnapshot    `json:"prompt_length,omitempty"`
	GenerationLength          DistributionSnapshot    `json:"generation_length,omitempty"`
	KVCacheUsage              MetricWindow            `json:"kv_cache_usage,omitempty"`
	GPUKVCacheUsage           MetricWindow            `json:"gpu_kv_cache_usage,omitempty"`
	CPUKVCacheUsage           MetricWindow            `json:"cpu_kv_cache_usage,omitempty"`
	CPUKVBlocks               MetricWindow            `json:"cpu_kv_blocks,omitempty"`
	Preemptions               MetricWindow            `json:"preemptions,omitempty"`
	SwappedRequests           MetricWindow            `json:"swapped_requests,omitempty"`
	RecomputedPromptTokens    MetricWindow            `json:"recomputed_prompt_tokens,omitempty"`
	KVOffloadActivity         MetricWindow            `json:"kv_offload_activity,omitempty"`
	PrefixCache               CacheSnapshot           `json:"prefix_cache,omitempty"`
	MultimodalCache           CacheSnapshot           `json:"multimodal_cache,omitempty"`
	Coverage                  SourceCoverage          `json:"coverage,omitempty"`
}

type HostMetrics struct {
	CPUUtilization          MetricWindow   `json:"cpu_utilization,omitempty"`
	CPULoad                 MetricWindow   `json:"cpu_load,omitempty"`
	CPULoadAverages         LoadMetrics    `json:"cpu_load_averages,omitempty"`
	MemoryUsed              MetricWindow   `json:"memory_used,omitempty"`
	MemoryAvailable         MetricWindow   `json:"memory_available,omitempty"`
	MemoryTotal             MetricWindow   `json:"memory_total,omitempty"`
	SwapPressure            MetricWindow   `json:"swap_pressure,omitempty"`
	SwapUsed                MetricWindow   `json:"swap_used,omitempty"`
	ProcessCPU              MetricWindow   `json:"process_cpu,omitempty"`
	ProcessMemory           MetricWindow   `json:"process_memory,omitempty"`
	DiskIO                  DiskIOMetrics  `json:"disk_io,omitempty"`
	NetworkRX               MetricWindow   `json:"network_rx,omitempty"`
	NetworkTX               MetricWindow   `json:"network_tx,omitempty"`
	KubernetesCPUThrottling MetricWindow   `json:"kubernetes_cpu_throttling,omitempty"`
	Coverage                SourceCoverage `json:"coverage,omitempty"`
}

type GPUTelemetry struct {
	GPUUtilizationOrSMActivity MetricWindow       `json:"gpu_utilization_or_sm_activity,omitempty"`
	GPUUtilization             MetricWindow       `json:"gpu_utilization,omitempty"`
	SMActive                   MetricWindow       `json:"sm_active,omitempty"`
	SMOccupancy                MetricWindow       `json:"sm_occupancy,omitempty"`
	TensorCoreActive           MetricWindow       `json:"tensor_core_active,omitempty"`
	DRAMActive                 MetricWindow       `json:"dram_active,omitempty"`
	FramebufferMemory          MemoryMetrics      `json:"framebuffer_memory,omitempty"`
	MemoryBandwidth            MetricWindow       `json:"memory_bandwidth,omitempty"`
	Clocks                     ClockMetrics       `json:"clocks,omitempty"`
	Power                      MetricWindow       `json:"power,omitempty"`
	Temperature                MetricWindow       `json:"temperature,omitempty"`
	ClockThrottleReasons       MetricWindow       `json:"clock_throttle_reasons,omitempty"`
	PCIeThroughput             ThroughputMetrics  `json:"pcie_throughput,omitempty"`
	NVLinkThroughput           ThroughputMetrics  `json:"nvlink_throughput,omitempty"`
	ReliabilityErrors          ReliabilityMetrics `json:"reliability_errors,omitempty"`
	Coverage                   SourceCoverage     `json:"coverage,omitempty"`
}

type NvidiaSMIMetrics struct {
	GPUUtilization   MetricWindow   `json:"gpu_utilization,omitempty"`
	MemoryUsed       MetricWindow   `json:"memory_used,omitempty"`
	MemoryTotal      MetricWindow   `json:"memory_total,omitempty"`
	PowerDraw        MetricWindow   `json:"power_draw,omitempty"`
	PowerLimit       MetricWindow   `json:"power_limit,omitempty"`
	Temperature      MetricWindow   `json:"temperature,omitempty"`
	SMClock          MetricWindow   `json:"sm_clock,omitempty"`
	MemClock         MetricWindow   `json:"mem_clock,omitempty"`
	ProcessGPUMemory MetricWindow   `json:"process_gpu_memory,omitempty"`
	PerformanceState string         `json:"performance_state,omitempty"`
	ThrottleReasons  []string       `json:"throttle_reasons,omitempty"`
	Coverage         SourceCoverage `json:"coverage,omitempty"`
}
