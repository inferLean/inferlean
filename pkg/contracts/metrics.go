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
	Used  MetricWindow `json:"used,omitempty"`
	Free  MetricWindow `json:"free,omitempty"`
	Total MetricWindow `json:"total,omitempty"`
}

type ClockMetrics struct {
	SM     MetricWindow `json:"sm,omitempty"`
	Memory MetricWindow `json:"memory,omitempty"`
}

type ThroughputMetrics struct {
	RX MetricWindow `json:"rx,omitempty"`
	TX MetricWindow `json:"tx,omitempty"`
}

type ReliabilityMetrics struct {
	XID MetricWindow `json:"xid,omitempty"`
	ECC MetricWindow `json:"ecc,omitempty"`
}

type VLLMMetrics struct {
	RequestsRunning        MetricWindow         `json:"requests_running,omitempty"`
	RequestsWaiting        MetricWindow         `json:"requests_waiting,omitempty"`
	RequestThroughput      MetricWindow         `json:"request_throughput,omitempty"`
	LatencyE2E             MetricWindow         `json:"latency_e2e,omitempty"`
	LatencyTTFT            MetricWindow         `json:"latency_ttft,omitempty"`
	LatencyQueue           MetricWindow         `json:"latency_queue,omitempty"`
	LatencyPrefill         MetricWindow         `json:"latency_prefill,omitempty"`
	LatencyDecode          MetricWindow         `json:"latency_decode,omitempty"`
	PromptTokens           MetricWindow         `json:"prompt_tokens,omitempty"`
	GenerationTokens       MetricWindow         `json:"generation_tokens,omitempty"`
	PromptLength           DistributionSnapshot `json:"prompt_length,omitempty"`
	GenerationLength       DistributionSnapshot `json:"generation_length,omitempty"`
	KVCacheUsage           MetricWindow         `json:"kv_cache_usage,omitempty"`
	Preemptions            MetricWindow         `json:"preemptions,omitempty"`
	RecomputedPromptTokens MetricWindow         `json:"recomputed_prompt_tokens,omitempty"`
	PrefixCache            CacheSnapshot        `json:"prefix_cache,omitempty"`
	MultimodalCache        CacheSnapshot        `json:"multimodal_cache,omitempty"`
	Coverage               SourceCoverage       `json:"coverage,omitempty"`
}

type HostMetrics struct {
	CPUUtilization  MetricWindow   `json:"cpu_utilization,omitempty"`
	CPULoad         MetricWindow   `json:"cpu_load,omitempty"`
	MemoryUsed      MetricWindow   `json:"memory_used,omitempty"`
	MemoryAvailable MetricWindow   `json:"memory_available,omitempty"`
	SwapPressure    MetricWindow   `json:"swap_pressure,omitempty"`
	ProcessCPU      MetricWindow   `json:"process_cpu,omitempty"`
	ProcessMemory   MetricWindow   `json:"process_memory,omitempty"`
	NetworkRX       MetricWindow   `json:"network_rx,omitempty"`
	NetworkTX       MetricWindow   `json:"network_tx,omitempty"`
	Coverage        SourceCoverage `json:"coverage,omitempty"`
}

type GPUTelemetry struct {
	GPUUtilizationOrSMActivity MetricWindow       `json:"gpu_utilization_or_sm_activity,omitempty"`
	FramebufferMemory          MemoryMetrics      `json:"framebuffer_memory,omitempty"`
	MemoryBandwidth            MetricWindow       `json:"memory_bandwidth,omitempty"`
	Clocks                     ClockMetrics       `json:"clocks,omitempty"`
	Power                      MetricWindow       `json:"power,omitempty"`
	Temperature                MetricWindow       `json:"temperature,omitempty"`
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
	Temperature      MetricWindow   `json:"temperature,omitempty"`
	SMClock          MetricWindow   `json:"sm_clock,omitempty"`
	MemClock         MetricWindow   `json:"mem_clock,omitempty"`
	ProcessGPUMemory MetricWindow   `json:"process_gpu_memory,omitempty"`
	Coverage         SourceCoverage `json:"coverage,omitempty"`
}
