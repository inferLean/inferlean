package contracts

import "time"

const SchemaVersion = "v2.4"

type RunArtifact struct {
	SchemaVersion        string               `json:"schema_version"`
	Job                  Job                  `json:"job"`
	Environment          Environment          `json:"environment"`
	RuntimeConfig        RuntimeConfig        `json:"runtime_config"`
	Metrics              Metrics              `json:"metrics"`
	ProcessInspection    ProcessInspection    `json:"process_inspection"`
	WorkloadObservations WorkloadObservations `json:"workload_observations"`
	CollectionQuality    CollectionQuality    `json:"collection_quality"`
}

type Job struct {
	RunID            string    `json:"run_id"`
	InstallationID   string    `json:"installation_id"`
	CollectorVersion string    `json:"collector_version"`
	SchemaVersion    string    `json:"schema_version"`
	CollectedAt      time.Time `json:"collected_at"`
}

type Environment struct {
	OS             string `json:"os,omitempty"`
	Kernel         string `json:"kernel,omitempty"`
	CPUModel       string `json:"cpu_model,omitempty"`
	CPUCores       int    `json:"cpu_cores,omitempty"`
	MemoryBytes    int64  `json:"memory_bytes,omitempty"`
	GPUModel       string `json:"gpu_model,omitempty"`
	GPUCount       int    `json:"gpu_count,omitempty"`
	DriverVersion  string `json:"driver_version,omitempty"`
	RuntimeVersion string `json:"runtime_version,omitempty"`
}

type RuntimeConfig struct {
	Model                string            `json:"model,omitempty"`
	ServedModelName      string            `json:"served_model_name,omitempty"`
	Host                 string            `json:"host,omitempty"`
	Port                 int               `json:"port,omitempty"`
	TensorParallelSize   int               `json:"tensor_parallel_size,omitempty"`
	DataParallelSize     int               `json:"data_parallel_size,omitempty"`
	PipelineParallelSize int               `json:"pipeline_parallel_size,omitempty"`
	MaxModelLen          int               `json:"max_model_len,omitempty"`
	MaxNumBatchedTokens  int               `json:"max_num_batched_tokens,omitempty"`
	MaxNumSeqs           int               `json:"max_num_seqs,omitempty"`
	GPUMemoryUtilization float64           `json:"gpu_memory_utilization,omitempty"`
	KVCacheDType         string            `json:"kv_cache_dtype,omitempty"`
	ChunkedPrefill       *bool             `json:"chunked_prefill,omitempty"`
	PrefixCaching        *bool             `json:"prefix_caching,omitempty"`
	Quantization         string            `json:"quantization,omitempty"`
	DType                string            `json:"dtype,omitempty"`
	GenerationConfig     string            `json:"generation_config,omitempty"`
	APIKeyConfigured     bool              `json:"api_key_configured,omitempty"`
	MultimodalFlags      []string          `json:"multimodal_flags,omitempty"`
	EnvHints             map[string]string `json:"env_hints,omitempty"`
}

type Metrics struct {
	VLLM      map[string]any `json:"vllm,omitempty"`
	Host      map[string]any `json:"host,omitempty"`
	GPU       map[string]any `json:"gpu,omitempty"`
	NvidiaSmi map[string]any `json:"nvidia_smi,omitempty"`
}

type ProcessInspection struct {
	TargetProcess    TargetProcess     `json:"target_process"`
	RelatedProcesses []ObservedProcess `json:"related_processes,omitempty"`
	ParseWarnings    []string          `json:"parse_warnings,omitempty"`
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

type WorkloadObservations struct {
	Mode         string            `json:"mode,omitempty"`
	Target       string            `json:"target,omitempty"`
	Summary      string            `json:"summary,omitempty"`
	Hints        map[string]string `json:"hints,omitempty"`
	Measurements map[string]any    `json:"measurements,omitempty"`
}

type CollectionQuality struct {
	SourceStates     map[string]SourceState `json:"source_states,omitempty"`
	MissingEvidence  []string               `json:"missing_evidence,omitempty"`
	DegradedEvidence []string               `json:"degraded_evidence,omitempty"`
	Completeness     float64                `json:"completeness,omitempty"`
	Summary          string                 `json:"summary,omitempty"`
}

type SourceState struct {
	Status    string   `json:"status,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
}
