package types

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Artifact struct {
	Job               Job               `json:"job"`
	Target            Target            `json:"target"`
	Configurations    Configurations    `json:"configurations"`
	Observations      Observations      `json:"observations"`
	RawProcessIO      map[string]any    `json:"raw_process_io"`
	UserIntent        UserIntent        `json:"user_intent"`
	CollectionQuality CollectionQuality `json:"collection_quality"`
}

type Job struct {
	RunID            string    `json:"run_id"`
	InstallationID   string    `json:"installation_id"`
	SchemaVersion    string    `json:"schema_version"`
	CollectorVersion string    `json:"collector_version"`
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
}

type Target struct {
	PID             int32  `json:"pid,omitempty"`
	Executable      string `json:"executable,omitempty"`
	RawCommandLine  string `json:"raw_command_line,omitempty"`
	MetricsEndpoint string `json:"metrics_endpoint,omitempty"`
	ContainerID     string `json:"container_id,omitempty"`
	PodName         string `json:"pod_name,omitempty"`
}

type Configurations struct {
	ParsedArgs          map[string]string `json:"parsed_args,omitempty"`
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

type Observations map[string]any

type UserIntent struct {
	WorkloadMode    string `json:"workload_mode,omitempty"`
	WorkloadTarget  string `json:"workload_target,omitempty"`
	PrefixHeavy     bool   `json:"prefix_heavy"`
	Multimodal      bool   `json:"multimodal"`
	MultimodalCache bool   `json:"multimodal_cache"`
}

type CollectionQuality struct {
	SourceStatus      map[string]string `json:"source_status,omitempty"`
	TelemetryMode     string            `json:"telemetry_mode,omitempty"`
	FallbacksUsed     []string          `json:"fallbacks_used,omitempty"`
	MissingSources    []string          `json:"missing_sources,omitempty"`
	DegradedSources   []string          `json:"degraded_sources,omitempty"`
	CollectionSeconds float64           `json:"collection_seconds,omitempty"`
	ScrapeIntervalSec float64           `json:"scrape_interval_seconds,omitempty"`
}

func (a Artifact) Validate() error {
	var errs []error
	errs = append(errs, validateJob(a.Job)...)
	errs = append(errs, validateTarget(a.Target)...)
	errs = append(errs, validateQuality(a.CollectionQuality)...)
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("artifact validation failed: %w", errors.Join(errs...))
}

func validateJob(job Job) []error {
	var errs []error
	if strings.TrimSpace(job.RunID) == "" {
		errs = append(errs, errors.New("job.run_id is required"))
	}
	if strings.TrimSpace(job.InstallationID) == "" {
		errs = append(errs, errors.New("job.installation_id is required"))
	}
	if strings.TrimSpace(job.SchemaVersion) == "" {
		errs = append(errs, errors.New("job.schema_version is required"))
	}
	if strings.TrimSpace(job.CollectorVersion) == "" {
		errs = append(errs, errors.New("job.collector_version is required"))
	}
	if job.StartedAt.IsZero() {
		errs = append(errs, errors.New("job.started_at is required"))
	}
	if job.FinishedAt.IsZero() {
		errs = append(errs, errors.New("job.finished_at is required"))
	}
	return errs
}

func validateTarget(target Target) []error {
	if strings.TrimSpace(target.RawCommandLine) == "" {
		return []error{errors.New("target.raw_command_line is required")}
	}
	return nil
}

func validateQuality(quality CollectionQuality) []error {
	var errs []error
	if quality.CollectionSeconds <= 0 {
		errs = append(errs, errors.New("collection_quality.collection_seconds must be > 0"))
	}
	if len(quality.SourceStatus) == 0 {
		errs = append(errs, errors.New("collection_quality.source_status is required"))
	}
	return errs
}
