package contracts

import "time"

const ReportSchemaVersion = "report-v5"

type FinalReport struct {
	SchemaVersion      string                  `json:"schema_version"`
	Job                ReportJob               `json:"job"`
	Entitlement        ReportEntitlement       `json:"entitlement"`
	Environment        ReportEnvironment       `json:"environment"`
	Diagnosis          DiagnosisSection        `json:"diagnosis"`
	Saturation         SaturationReport        `json:"saturation"`
	DiagnosticCoverage DiagnosticCoverage      `json:"diagnostic_coverage"`
	Issues             []Issue                 `json:"issues,omitempty"`
	Opportunities      []Opportunity           `json:"opportunities,omitempty"`
	CollectionQuality  ReportCollectionQuality `json:"collection_quality"`
}

type ReportJob struct {
	RunID                 string    `json:"run_id"`
	InstallationID        string    `json:"installation_id,omitempty"`
	CollectorVersion      string    `json:"collector_version,omitempty"`
	ArtifactSchemaVersion string    `json:"artifact_schema_version,omitempty"`
	CollectedAt           time.Time `json:"collected_at,omitempty"`
	ReceivedAt            time.Time `json:"received_at,omitempty"`
	ReportedAt            time.Time `json:"reported_at,omitempty"`
}

type ReportEntitlement struct {
	Tier         string   `json:"tier"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type ReportEnvironment struct {
	OS                 string `json:"os,omitempty"`
	Kernel             string `json:"kernel,omitempty"`
	CPUModel           string `json:"cpu_model,omitempty"`
	CPUCores           int    `json:"cpu_cores,omitempty"`
	MemoryBytes        int64  `json:"memory_bytes,omitempty"`
	GPUModel           string `json:"gpu_model,omitempty"`
	GPUCount           int    `json:"gpu_count,omitempty"`
	DriverVersion      string `json:"driver_version,omitempty"`
	RuntimeVersion     string `json:"runtime_version,omitempty"`
	Model              string `json:"model,omitempty"`
	ServedModelName    string `json:"served_model_name,omitempty"`
	Host               string `json:"host,omitempty"`
	Port               int    `json:"port,omitempty"`
	VLLMVersion        string `json:"vllm_version,omitempty"`
	TorchVersion       string `json:"torch_version,omitempty"`
	CUDARuntimeVersion string `json:"cuda_runtime_version,omitempty"`
}

type DiagnosisSection struct {
	BaseDiagnosis BaseDiagnosis `json:"base_diagnosis"`
}

type BaseDiagnosis struct {
	ID                         string          `json:"id,omitempty"`
	WorkloadSummary            WorkloadSummary `json:"workload_summary"`
	RealLoadSummary            RealLoadSummary `json:"real_load_summary"`
	Confidence                 string          `json:"confidence,omitempty"`
	Caveats                    []string        `json:"caveats,omitempty"`
	NoSafeRecommendationReason string          `json:"no_safe_recommendation_reason,omitempty"`
}

type WorkloadSummary struct {
	DeclaredWorkloadMode  string `json:"declared_workload_mode,omitempty"`
	ObservedWorkloadShape string `json:"observed_workload_shape,omitempty"`
	ConfiguredPosture     string `json:"configured_posture,omitempty"`
	Multimodal            bool   `json:"multimodal,omitempty"`
	Summary               string `json:"summary,omitempty"`
}

type RealLoadSummary struct {
	ComputePressure         string `json:"compute_pressure,omitempty"`
	MemoryBandwidthPressure string `json:"memory_bandwidth_pressure,omitempty"`
	KVPressure              string `json:"kv_pressure,omitempty"`
	QueuePressure           string `json:"queue_pressure,omitempty"`
	HostPipelinePressure    string `json:"host_pipeline_pressure,omitempty"`
	Summary                 string `json:"summary,omitempty"`
}

type SaturationReport struct {
	Version    string             `json:"version,omitempty"`
	Generic    SaturationMetric   `json:"generic,omitempty"`
	Dimensions []SaturationMetric `json:"dimensions,omitempty"`
}

type SaturationMetric struct {
	ID                           string       `json:"id,omitempty"`
	Label                        string       `json:"label,omitempty"`
	BottleneckType               string       `json:"bottleneck_type,omitempty"`
	Status                       string       `json:"status,omitempty"`
	Score                        MetricWindow `json:"score,omitempty"`
	HeadroomPercent              *float64     `json:"headroom_percent,omitempty"`
	WorstObservedHeadroomPercent *float64     `json:"worst_observed_headroom_percent,omitempty"`
	EvidenceRefs                 []string     `json:"evidence_refs,omitempty"`
	MissingEvidence              []string     `json:"missing_evidence,omitempty"`
	Reason                       string       `json:"reason,omitempty"`
}

type Recommendation struct {
	Decision        string          `json:"decision"`
	Title           string          `json:"title"`
	Rationale       string          `json:"rationale,omitempty"`
	ProjectedEffect ProjectedEffect `json:"projected_effect"`
	Confidence      string          `json:"confidence,omitempty"`
	Actions         []Action        `json:"actions,omitempty"`
	FollowUpSteps   []FollowUpStep  `json:"follow_up_steps,omitempty"`
}

type ProjectedEffect struct {
	Summary    string                    `json:"summary,omitempty"`
	Latency    ProjectedMetricEffect     `json:"latency"`
	Throughput ProjectedThroughputEffect `json:"throughput"`
}

type ProjectedThroughputEffect struct {
	Requests     ProjectedMetricEffect `json:"requests"`
	OutputTokens ProjectedMetricEffect `json:"output_tokens"`
}

type ProjectedMetricEffect struct {
	Metric       string   `json:"metric,omitempty"`
	Unit         string   `json:"unit,omitempty"`
	Current      *float64 `json:"current"`
	Projected    *float64 `json:"projected"`
	Delta        *float64 `json:"delta"`
	PercentDelta *float64 `json:"percent_delta"`
	Direction    string   `json:"direction,omitempty"`
	Confidence   string   `json:"confidence,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

type Action struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Why                  string `json:"why,omitempty"`
	How                  string `json:"how,omitempty"`
	CurrentValue         string `json:"current_value,omitempty"`
	ProposedValue        string `json:"proposed_value,omitempty"`
	ValueKind            string `json:"value_kind,omitempty"`
	ValueRequired        bool   `json:"value_required,omitempty"`
	ExpectedSignalChange string `json:"expected_signal_change,omitempty"`
	Risk                 string `json:"risk,omitempty"`
	Confidence           string `json:"confidence,omitempty"`
}

type FollowUpStep struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Why                  string `json:"why,omitempty"`
	How                  string `json:"how,omitempty"`
	ExpectedSignalChange string `json:"expected_signal_change,omitempty"`
	Risk                 string `json:"risk,omitempty"`
	Confidence           string `json:"confidence,omitempty"`
}

type DiagnosticCoverage struct {
	CoverageVersion              string                    `json:"coverage_version,omitempty"`
	EligibleForRequiredDetectors bool                      `json:"eligible_for_required_detectors"`
	IneligibleReason             string                    `json:"ineligible_reason,omitempty"`
	RequiredDetectorSet          string                    `json:"required_detector_set,omitempty"`
	Summary                      DiagnosticCoverageSummary `json:"summary"`
	DetectorResults              []DetectorResult          `json:"detector_results,omitempty"`
	MissingEvidenceAreas         []string                  `json:"missing_evidence_areas,omitempty"`
	ConfidenceImpactSummary      string                    `json:"confidence_impact_summary,omitempty"`
}

type DiagnosticCoverageSummary struct {
	RequiredTotal  int    `json:"required_total,omitempty"`
	Attempted      int    `json:"attempted,omitempty"`
	Detected       int    `json:"detected,omitempty"`
	RuledOut       int    `json:"ruled_out,omitempty"`
	NotEvaluable   int    `json:"not_evaluable,omitempty"`
	CoverageStatus string `json:"coverage_status,omitempty"`
}

type DetectorResult struct {
	DetectorID           string   `json:"detector_id"`
	Rank                 int      `json:"rank"`
	Status               string   `json:"status"`
	Reason               string   `json:"reason,omitempty"`
	RequiredEvidenceRefs []string `json:"required_evidence_refs,omitempty"`
	Confidence           string   `json:"confidence,omitempty"`
}

type Issue struct {
	ID             string          `json:"id"`
	Rank           int             `json:"rank"`
	DetectorID     string          `json:"detector_id,omitempty"`
	Family         string          `json:"family,omitempty"`
	Label          string          `json:"label,omitempty"`
	EvidenceRefs   []string        `json:"evidence_refs,omitempty"`
	Recommendation *Recommendation `json:"recommendation,omitempty"`
	Confidence     string          `json:"confidence,omitempty"`
}

type Opportunity struct {
	ID             string          `json:"id"`
	Rank           int             `json:"rank"`
	DetectorID     string          `json:"detector_id,omitempty"`
	Category       string          `json:"category,omitempty"`
	Title          string          `json:"title,omitempty"`
	EvidenceRefs   []string        `json:"evidence_refs,omitempty"`
	Recommendation *Recommendation `json:"recommendation,omitempty"`
	Confidence     string          `json:"confidence,omitempty"`
}

type ReportCollectionQuality struct {
	SourceStates              map[string]SourceState `json:"source_states,omitempty"`
	MissingEvidence           []string               `json:"missing_evidence,omitempty"`
	DegradedEvidence          []string               `json:"degraded_evidence,omitempty"`
	Completeness              float64                `json:"completeness,omitempty"`
	Summary                   string                 `json:"summary,omitempty"`
	SelectedGPUPath           string                 `json:"selected_gpu_path,omitempty"`
	TelemetryMode             string                 `json:"telemetry_mode,omitempty"`
	Fallbacks                 []string               `json:"fallbacks,omitempty"`
	CollectionDurationSeconds float64                `json:"collection_duration_seconds,omitempty"`
	ScrapeIntervalSeconds     float64                `json:"scrape_interval_seconds,omitempty"`
	ConfidenceImpactSummary   string                 `json:"confidence_impact_summary,omitempty"`
}

type SummaryPreview struct {
	Headline              string `json:"headline,omitempty"`
	PrimaryRecommendation string `json:"primary_recommendation,omitempty"`
	KeyTradeoff           string `json:"key_tradeoff,omitempty"`
	Confidence            string `json:"confidence,omitempty"`
}
