package contracts

import "time"

type RuntimeConfig struct {
	Model                 string            `json:"model,omitempty"`
	ServedModelName       string            `json:"served_model_name,omitempty"`
	Host                  string            `json:"host,omitempty"`
	Port                  int               `json:"port,omitempty"`
	TensorParallelSize    int               `json:"tensor_parallel_size,omitempty"`
	DataParallelSize      int               `json:"data_parallel_size,omitempty"`
	PipelineParallelSize  int               `json:"pipeline_parallel_size,omitempty"`
	MaxModelLen           int               `json:"max_model_len,omitempty"`
	MaxNumBatchedTokens   int               `json:"max_num_batched_tokens,omitempty"`
	MaxNumSeqs            int               `json:"max_num_seqs,omitempty"`
	GPUMemoryUtilization  float64           `json:"gpu_memory_utilization,omitempty"`
	KVCacheDType          string            `json:"kv_cache_dtype,omitempty"`
	ChunkedPrefill        *bool             `json:"chunked_prefill,omitempty"`
	PrefixCaching         *bool             `json:"prefix_caching,omitempty"`
	Quantization          string            `json:"quantization,omitempty"`
	DType                 string            `json:"dtype,omitempty"`
	GenerationConfig      string            `json:"generation_config,omitempty"`
	APIKeyConfigured      bool              `json:"api_key_configured,omitempty"`
	MultimodalFlags       []string          `json:"multimodal_flags,omitempty"`
	EnvHints              map[string]string `json:"env_hints,omitempty"`
	VLLMVersion           string            `json:"vllm_version,omitempty"`
	TorchVersion          string            `json:"torch_version,omitempty"`
	CUDARuntimeVersion    string            `json:"cuda_runtime_version,omitempty"`
	NvidiaDriverVersion   string            `json:"nvidia_driver_version,omitempty"`
	AttentionBackend      string            `json:"attention_backend,omitempty"`
	FlashinferPresent     *bool             `json:"flashinfer_present,omitempty"`
	FlashAttentionPresent *bool             `json:"flash_attention_present,omitempty"`
	ImageProcessor        string            `json:"image_processor,omitempty"`
	MultimodalCacheHints  []string          `json:"multimodal_cache_hints,omitempty"`
	ProbeWarnings         []string          `json:"probe_warnings,omitempty"`
	ProbeEvidenceRef      string            `json:"probe_evidence_ref,omitempty"`
	Coverage              SourceCoverage    `json:"coverage,omitempty"`
}

type ProcessInspection struct {
	TargetProcess    TargetProcess     `json:"target_process"`
	RelatedProcesses []ObservedProcess `json:"related_processes,omitempty"`
	ParseWarnings    []string          `json:"parse_warnings,omitempty"`
	ProbeWarnings    []string          `json:"probe_warnings,omitempty"`
	ProbeEvidenceRef string            `json:"probe_evidence_ref,omitempty"`
	Coverage         SourceCoverage    `json:"coverage,omitempty"`
}

type TargetProcess struct {
	PID            int32      `json:"pid,omitempty"`
	Executable     string     `json:"executable,omitempty"`
	RawCommandLine string     `json:"raw_command_line,omitempty"`
	EntryPoint     string     `json:"entry_point,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
}

type ObservedProcess struct {
	PID            int32      `json:"pid,omitempty"`
	PPID           int32      `json:"ppid,omitempty"`
	Executable     string     `json:"executable,omitempty"`
	RawCommandLine string     `json:"raw_command_line,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
}
