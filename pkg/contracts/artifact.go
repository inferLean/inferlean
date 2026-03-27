package contracts

import "time"

const SchemaVersion = "v2.5"

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
