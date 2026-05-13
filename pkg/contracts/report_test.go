package contracts

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFinalReportValidateAcceptsCanonicalShape(t *testing.T) {
	report := validFinalReport()
	report.Issues[0].Recommendation.Actions = []Action{{
		ID:            "action:set-max-num-batched-tokens",
		Title:         "Raise `--max-num-batched-tokens`",
		CurrentValue:  "2048",
		ProposedValue: "4096",
		ValueKind:     "number",
		ValueRequired: true,
	}}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFinalReportValidateAcceptsOpportunities(t *testing.T) {
	report := validFinalReport()
	report.Opportunities = []Opportunity{{
		ID:         "opportunity:insufficient_offered_load",
		Rank:       1,
		DetectorID: "insufficient_offered_load",
		Category:   "cost_rightsizing",
		Recommendation: &Recommendation{
			Decision: "right_size_gpu_capacity",
			Title:    "Right-size GPU capacity",
			Actions: []Action{{
				ID:            "action:right-size-gpu-capacity",
				Title:         "Reduce GPU count or GPU size",
				CurrentValue:  "4",
				ProposedValue: "3",
				ValueRequired: true,
			}},
			ProjectedEffect: validProjectedEffect(),
		},
	}}

	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFinalReportValidateRejectsIncompleteActionDelta(t *testing.T) {
	report := validFinalReport()
	report.Issues[0].Recommendation.Actions = []Action{{
		ID:            "action:set-max-num-batched-tokens",
		Title:         "Raise `--max-num-batched-tokens`",
		CurrentValue:  "2048",
		ValueRequired: true,
	}}

	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want incomplete action delta failure")
	}
}

func TestFinalReportValidateAcceptsFollowUpStepsWithoutDelta(t *testing.T) {
	report := validFinalReport()
	report.Issues[0].Recommendation.FollowUpSteps = []FollowUpStep{{
		ID:    "action:rerun-under-same-load",
		Title: "Rerun at the same offered load",
		How:   "Keep prompt/output mix and client concurrency stable for the comparison run.",
	}}

	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFinalReportValidateRequiresRankOneIssue(t *testing.T) {
	report := validFinalReport()
	report.Issues[0].Rank = 2

	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want rank-one failure")
	}
}

func TestFinalReportValidateRequiresIssueRecommendation(t *testing.T) {
	report := validFinalReport()
	report.Issues[0].Recommendation = nil

	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want issue recommendation failure")
	}
}

func TestFinalReportValidateRequiresIssueRecommendationActions(t *testing.T) {
	report := validFinalReport()
	report.Issues[0].Recommendation.Actions = nil
	report.Issues[0].Recommendation.FollowUpSteps = nil

	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want issue recommendation actions failure")
	}
}

func TestFinalReportValidateRequiresIssueDetectorID(t *testing.T) {
	report := validFinalReport()
	report.Issues[0].DetectorID = ""

	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want issue detector id failure")
	}
}

func TestFinalReportValidateAllowsNonRequiredActionWithoutDelta(t *testing.T) {
	report := validFinalReport()
	report.Issues[0].Recommendation.Actions = []Action{{
		ID:    "action:prioritize-latency-target",
		Title: "Apply latency-prioritized tuning order",
	}}

	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFinalReportJSONOmitsRemovedTargetOverlay(t *testing.T) {
	payload, err := json.Marshal(validFinalReport())
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	rendered := string(payload)
	if strings.Contains(rendered, "target_overlay") {
		t.Fatalf("final report JSON includes removed target_overlay field: %s", rendered)
	}
	if strings.Contains(rendered, "diagnostic_lenses") {
		t.Fatalf("final report JSON includes removed diagnostic_lenses field: %s", rendered)
	}
	if strings.Contains(rendered, "situation") {
		t.Fatalf("final report JSON includes removed base_diagnosis.situation field: %s", rendered)
	}
	if strings.Contains(rendered, "pressures") {
		t.Fatalf("final report JSON includes removed capacity_snapshot.pressures field: %s", rendered)
	}
	if strings.Contains(rendered, "capacity_snapshot") {
		t.Fatalf("final report JSON includes removed capacity_snapshot field: %s", rendered)
	}
	if strings.Contains(rendered, "frontier") {
		t.Fatalf("final report JSON includes removed frontier field: %s", rendered)
	}
	if strings.Contains(rendered, "expected_effect") {
		t.Fatalf("final report JSON includes removed expected_effect field: %s", rendered)
	}
	if strings.Contains(rendered, "ui_hints") {
		t.Fatalf("final report JSON includes removed ui_hints field: %s", rendered)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	diagnosis := decoded["diagnosis"].(map[string]any)
	base := diagnosis["base_diagnosis"].(map[string]any)
	if _, ok := base["current_limiter"]; ok {
		t.Fatalf("final report JSON includes removed base_diagnosis.current_limiter field: %s", rendered)
	}
	if _, ok := base["recommendation"]; ok {
		t.Fatalf("final report JSON includes removed base_diagnosis.recommendation field: %s", rendered)
	}
}

func TestFinalReportValidateRequiresDetectorRanks(t *testing.T) {
	report := validFinalReport()
	report.DiagnosticCoverage.DetectorResults[0].Rank = 0

	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want detector rank failure")
	}
}

func TestArtifactUploadAckValidateChecksSummaryPreview(t *testing.T) {
	ack := ArtifactUploadAck{
		UploadID:       "upl-123",
		RunID:          "run-123",
		InstallationID: "inst-123",
		Status:         "accepted",
		ReceivedAt:     time.Unix(1700000000, 0).UTC(),
		SummaryPreview: &SummaryPreview{Headline: "Queue pressure is limiting throughput."},
	}
	if err := ack.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	ack.SummaryPreview = &SummaryPreview{}
	if err := ack.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid summary preview")
	}
}

func validFinalReport() FinalReport {
	reportedAt := time.Unix(1700000100, 0).UTC()
	return FinalReport{
		SchemaVersion: ReportSchemaVersion,
		Job: ReportJob{
			RunID:                 "run-123",
			InstallationID:        "inst-123",
			CollectorVersion:      "0.2.0",
			ArtifactSchemaVersion: SchemaVersion,
			CollectedAt:           time.Unix(1700000000, 0).UTC(),
			ReceivedAt:            time.Unix(1700000005, 0).UTC(),
			ReportedAt:            reportedAt,
		},
		Entitlement: ReportEntitlement{Tier: "free"},
		Diagnosis: DiagnosisSection{
			BaseDiagnosis: BaseDiagnosis{
				ID: "base",
			},
		},
		DiagnosticCoverage: DiagnosticCoverage{
			EligibleForRequiredDetectors: true,
			Summary: DiagnosticCoverageSummary{
				RequiredTotal:  16,
				Attempted:      16,
				CoverageStatus: "complete",
			},
			DetectorResults: []DetectorResult{{
				DetectorID: "underbatching_for_throughput_traffic",
				Rank:       1,
				Status:     "detected",
			}},
		},
		Issues: []Issue{{
			ID:         "issue:underbatching_for_throughput_traffic",
			DetectorID: "underbatching_for_throughput_traffic",
			Family:     "scheduler_conservative",
			Rank:       1,
			Recommendation: &Recommendation{
				Decision:        "widen_scheduler_posture",
				Title:           "Widen scheduler posture",
				ProjectedEffect: validProjectedEffect(),
				Actions: []Action{{
					ID:    "action:set-max-num-batched-tokens",
					Title: "Raise `--max-num-batched-tokens`",
				}},
			},
		}},
	}
}

func validProjectedEffect() ProjectedEffect {
	currentLatency := 1.4
	projectedLatency := 1.26
	latencyDelta := projectedLatency - currentLatency
	latencyPercent := -10.0
	currentRequests := 2.0
	projectedRequests := 2.2
	requestDelta := projectedRequests - currentRequests
	requestPercent := 10.0
	currentOutput := 256.0
	projectedOutput := 281.6
	outputDelta := projectedOutput - currentOutput
	outputPercent := 10.0
	return ProjectedEffect{
		Summary: "Likely improvement: +5% to +15% throughput under the observed workload.",
		Latency: ProjectedMetricEffect{
			Metric:       "latency_e2e_seconds",
			Unit:         "s",
			Current:      &currentLatency,
			Projected:    &projectedLatency,
			Delta:        &latencyDelta,
			PercentDelta: &latencyPercent,
			Direction:    "lower_is_better",
			Confidence:   "medium",
		},
		Throughput: ProjectedThroughputEffect{
			Requests: ProjectedMetricEffect{
				Metric:       "request_throughput",
				Unit:         "req/s",
				Current:      &currentRequests,
				Projected:    &projectedRequests,
				Delta:        &requestDelta,
				PercentDelta: &requestPercent,
				Direction:    "higher_is_better",
				Confidence:   "medium",
			},
			OutputTokens: ProjectedMetricEffect{
				Metric:       "generation_tokens_per_second",
				Unit:         "tok/s",
				Current:      &currentOutput,
				Projected:    &projectedOutput,
				Delta:        &outputDelta,
				PercentDelta: &outputPercent,
				Direction:    "higher_is_better",
				Confidence:   "medium",
			},
		},
	}
}
