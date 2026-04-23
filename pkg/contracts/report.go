package contracts

import "time"

const ReportSchemaVersion = "report-v2"

type FinalReport struct {
	SchemaVersion      string                  `json:"schema_version"`
	Job                ReportJob               `json:"job"`
	Entitlement        ReportEntitlement       `json:"entitlement"`
	Environment        ReportEnvironment       `json:"environment"`
	Diagnosis          DiagnosisSection        `json:"diagnosis"`
	DiagnosticCoverage DiagnosticCoverage      `json:"diagnostic_coverage"`
	DiagnosticLenses   map[string]any          `json:"diagnostic_lenses,omitempty"`
	Issues             []Issue                 `json:"issues,omitempty"`
	Evidence           Evidence                `json:"evidence,omitempty"`
	CollectionQuality  ReportCollectionQuality `json:"collection_quality"`
	UIHints            UIHints                 `json:"ui_hints"`
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
	BaseDiagnosis    BaseDiagnosis    `json:"base_diagnosis"`
	ScenarioOverlays ScenarioOverlays `json:"scenario_overlays"`
}

type BaseDiagnosis struct {
	ID                         string            `json:"id,omitempty"`
	WorkloadSummary            WorkloadSummary   `json:"workload_summary"`
	CurrentLimiter             CurrentLimiter    `json:"current_limiter"`
	RealLoadSummary            RealLoadSummary   `json:"real_load_summary"`
	CapacitySnapshot           *CapacitySnapshot `json:"capacity_snapshot,omitempty"`
	Situation                  Situation         `json:"situation"`
	Frontier                   FrontierBundle    `json:"frontier"`
	Recommendation             *Recommendation   `json:"recommendation,omitempty"`
	Confidence                 string            `json:"confidence,omitempty"`
	Caveats                    []string          `json:"caveats,omitempty"`
	NoSafeRecommendationReason string            `json:"no_safe_recommendation_reason,omitempty"`
}

type WorkloadSummary struct {
	DeclaredWorkloadMode  string `json:"declared_workload_mode,omitempty"`
	ObservedWorkloadShape string `json:"observed_workload_shape,omitempty"`
	ConfiguredPosture     string `json:"configured_posture,omitempty"`
	Multimodal            bool   `json:"multimodal,omitempty"`
	Summary               string `json:"summary,omitempty"`
}

type CurrentLimiter struct {
	Family  string `json:"family,omitempty"`
	Label   string `json:"label,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type RealLoadSummary struct {
	ComputePressure         string `json:"compute_pressure,omitempty"`
	MemoryBandwidthPressure string `json:"memory_bandwidth_pressure,omitempty"`
	KVPressure              string `json:"kv_pressure,omitempty"`
	QueuePressure           string `json:"queue_pressure,omitempty"`
	HostPipelinePressure    string `json:"host_pipeline_pressure,omitempty"`
	Summary                 string `json:"summary,omitempty"`
}

type CapacitySnapshot struct {
	Pressures       CapacityPressures `json:"pressures,omitempty"`
	Observed        CapacityRates     `json:"observed,omitempty"`
	CurrentFrontier CapacityRates     `json:"current_frontier,omitempty"`
	Confidence      string            `json:"confidence,omitempty"`
	Summary         string            `json:"summary,omitempty"`
	Notes           []string          `json:"notes,omitempty"`
}

type CapacityPressures struct {
	Compute         string `json:"compute,omitempty"`
	MemoryBandwidth string `json:"memory_bandwidth,omitempty"`
	KV              string `json:"kv,omitempty"`
	Queue           string `json:"queue,omitempty"`
	Host            string `json:"host,omitempty"`
}

type CapacityRates struct {
	PromptTokensPerSecond     *float64 `json:"prompt_tokens_per_second,omitempty"`
	GenerationTokensPerSecond *float64 `json:"generation_tokens_per_second,omitempty"`
	RequestThroughput         *float64 `json:"request_throughput,omitempty"`
}

type Situation struct {
	Headline    string `json:"headline,omitempty"`
	Summary     string `json:"summary,omitempty"`
	KeyTradeoff string `json:"key_tradeoff,omitempty"`
}

type FrontierBundle struct {
	CurrentPracticalFrontier                    FrontierEstimate `json:"current_practical_frontier"`
	SafeHeadroom                                FrontierEstimate `json:"safe_headroom"`
	ProjectedFrontierAfterPrimaryRecommendation FrontierEstimate `json:"projected_frontier_after_primary_recommendation"`
	LikelyGainRange                             GainRange        `json:"likely_gain_range"`
}

type FrontierEstimate struct {
	Target          string          `json:"target,omitempty"`
	WorkloadContext WorkloadContext `json:"workload_context,omitempty"`
	EstimateSummary string          `json:"estimate_summary,omitempty"`
	Value           EstimateValue   `json:"value,omitempty"`
	Confidence      string          `json:"confidence,omitempty"`
	Notes           []string        `json:"notes,omitempty"`
}

type GainRange struct {
	Target      string   `json:"target,omitempty"`
	Metric      string   `json:"metric,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Estimate    *float64 `json:"estimate,omitempty"`
	RangeLow    *float64 `json:"range_low,omitempty"`
	RangeHigh   *float64 `json:"range_high,omitempty"`
	PercentLow  *float64 `json:"percent_low,omitempty"`
	PercentHigh *float64 `json:"percent_high,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

type WorkloadContext struct {
	DeclaredWorkloadMode  string `json:"declared_workload_mode,omitempty"`
	ObservedWorkloadShape string `json:"observed_workload_shape,omitempty"`
}

type EstimateValue struct {
	Metric    string   `json:"metric,omitempty"`
	Estimate  *float64 `json:"estimate,omitempty"`
	RangeLow  *float64 `json:"range_low,omitempty"`
	RangeHigh *float64 `json:"range_high,omitempty"`
}

type Recommendation struct {
	Decision       string                 `json:"decision"`
	Title          string                 `json:"title"`
	Rationale      string                 `json:"rationale,omitempty"`
	Mechanism      string                 `json:"mechanism,omitempty"`
	ExpectedEffect RecommendationEffect   `json:"expected_effect,omitempty"`
	Tradeoff       RecommendationTradeoff `json:"tradeoff,omitempty"`
	Effort         string                 `json:"effort,omitempty"`
	Risk           string                 `json:"risk,omitempty"`
	Reversibility  string                 `json:"reversibility,omitempty"`
	Confidence     string                 `json:"confidence,omitempty"`
	Actions        []Action               `json:"actions,omitempty"`
}

type RecommendationEffect struct {
	PrimaryMetric          string `json:"primary_metric,omitempty"`
	Summary                string `json:"summary,omitempty"`
	ProjectedFrontierDelta string `json:"projected_frontier_delta,omitempty"`
}

type RecommendationTradeoff struct {
	Summary string `json:"summary,omitempty"`
}

type Action struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Why                  string `json:"why,omitempty"`
	How                  string `json:"how,omitempty"`
	CurrentValue         string `json:"current_value,omitempty"`
	ProposedValue        string `json:"proposed_value,omitempty"`
	ValueKind            string `json:"value_kind,omitempty"`
	ExpectedSignalChange string `json:"expected_signal_change,omitempty"`
	Risk                 string `json:"risk,omitempty"`
	Confidence           string `json:"confidence,omitempty"`
}

type ScenarioOverlays struct {
	Latency    ScenarioOverlay `json:"latency"`
	Balanced   ScenarioOverlay `json:"balanced"`
	Throughput ScenarioOverlay `json:"throughput"`
}

type ScenarioOverlay struct {
	Target         string                 `json:"target,omitempty"`
	Summary        string                 `json:"summary,omitempty"`
	Frontier       FrontierBundle         `json:"frontier"`
	Recommendation *Recommendation        `json:"recommendation,omitempty"`
	Tradeoff       RecommendationTradeoff `json:"tradeoff,omitempty"`
	Confidence     string                 `json:"confidence,omitempty"`
	Caveats        []string               `json:"caveats,omitempty"`
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
	Status               string   `json:"status"`
	Reason               string   `json:"reason,omitempty"`
	RequiredEvidenceRefs []string `json:"required_evidence_refs,omitempty"`
	Confidence           string   `json:"confidence,omitempty"`
}

type Issue struct {
	ID                string      `json:"id"`
	Rank              int         `json:"rank"`
	Family            string      `json:"family,omitempty"`
	Label             string      `json:"label,omitempty"`
	Summary           string      `json:"summary,omitempty"`
	Impact            IssueImpact `json:"impact,omitempty"`
	EvidenceRefs      []string    `json:"evidence_refs,omitempty"`
	RecommendationRef string      `json:"recommendation_ref,omitempty"`
	Confidence        string      `json:"confidence,omitempty"`
}

type IssueImpact struct {
	Summary string `json:"summary,omitempty"`
}

type Evidence struct {
	Highlights []EvidenceHighlight `json:"highlights,omitempty"`
}

type EvidenceHighlight struct {
	ID         string `json:"id"`
	Title      string `json:"title,omitempty"`
	Summary    string `json:"summary,omitempty"`
	DetectorID string `json:"detector_id,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

type ReportCollectionQuality struct {
	SourceStates            map[string]SourceState `json:"source_states,omitempty"`
	MissingEvidence         []string               `json:"missing_evidence,omitempty"`
	DegradedEvidence        []string               `json:"degraded_evidence,omitempty"`
	Completeness            float64                `json:"completeness,omitempty"`
	Summary                 string                 `json:"summary,omitempty"`
	SelectedGPUPath         string                 `json:"selected_gpu_path,omitempty"`
	TelemetryMode           string                 `json:"telemetry_mode,omitempty"`
	ConfidenceImpactSummary string                 `json:"confidence_impact_summary,omitempty"`
}

type UIHints struct {
	AvailableModes    []string `json:"available_modes,omitempty"`
	DefaultMode       string   `json:"default_mode,omitempty"`
	DefaultTarget     string   `json:"default_target,omitempty"`
	HighlightIssueIDs []string `json:"highlight_issue_ids,omitempty"`
}

type SummaryPreview struct {
	Headline              string `json:"headline,omitempty"`
	CurrentLimiterLabel   string `json:"current_limiter_label,omitempty"`
	PrimaryRecommendation string `json:"primary_recommendation,omitempty"`
	KeyTradeoff           string `json:"key_tradeoff,omitempty"`
	Confidence            string `json:"confidence,omitempty"`
}
