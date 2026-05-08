package types

import "time"

type Configurations struct {
	ParsedArgs          map[string]string `json:"parsed_args,omitempty"`
	ParsedArgSources    map[string]string `json:"parsed_arg_sources,omitempty"`
	EnvironmentHints    map[string]string `json:"environment_hints,omitempty"`
	OS                  string            `json:"os,omitempty"`
	Kernel              string            `json:"kernel,omitempty"`
	CPUModel            string            `json:"cpu_model,omitempty"`
	CPUCores            int               `json:"cpu_cores,omitempty"`
	RAMBytes            uint64            `json:"ram_bytes,omitempty"`
	GPUModel            string            `json:"gpu_model,omitempty"`
	GPUCount            int               `json:"gpu_count,omitempty"`
	DriverVersion       string            `json:"driver_version,omitempty"`
	CUDARuntimeVersion  string            `json:"cuda_runtime_version,omitempty"`
	NvidiaSMIStaticText string            `json:"nvidia_smi_static_text,omitempty"`
}

type UserIntent struct {
	DeclaredWorkloadMode    string `json:"declared_workload_mode,omitempty"`
	DeclaredWorkloadTarget  string `json:"declared_workload_target,omitempty"`
	PrefixHeavy             bool   `json:"prefix_heavy"`
	Multimodal              bool   `json:"multimodal"`
	RepeatedMultimodalMedia bool   `json:"repeated_multimodal_media"`
}

type CollectionQuality struct {
	SourceStatus       map[string]string         `json:"source_status,omitempty"`
	SourceMetadata     map[string]SourceMetadata `json:"source_metadata,omitempty"`
	TelemetryMode      string                    `json:"telemetry_mode,omitempty"`
	Fallbacks          []string                  `json:"fallbacks,omitempty"`
	CollectionDuration time.Duration             `json:"collection_duration,omitempty"`
	ScrapeInterval     time.Duration             `json:"scrape_interval,omitempty"`
}

type SourceMetadata struct {
	Endpoint  string   `json:"endpoint,omitempty"`
	Transport string   `json:"transport,omitempty"`
	Fallback  bool     `json:"fallback,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
}
