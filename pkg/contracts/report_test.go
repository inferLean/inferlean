package contracts

import (
	"testing"
	"time"
)

func TestFinalReportValidateAcceptsCanonicalShape(t *testing.T) {
	report := validFinalReport()
	report.Diagnosis.BaseDiagnosis.CapacitySnapshot = &CapacitySnapshot{
		Confidence: "medium",
		Observed: CapacityRates{
			RequestThroughput: floatPointer(2),
		},
	}
	report.Diagnosis.BaseDiagnosis.Recommendation.Actions = []Action{{
		ID:            "action:set-max-num-batched-tokens",
		Title:         "Raise `--max-num-batched-tokens`",
		CurrentValue:  "2048",
		ProposedValue: "4096",
		ValueKind:     "number",
	}}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFinalReportValidateAcceptsSecondaryOpportunity(t *testing.T) {
	report := validFinalReport()
	report.UIHints.SecondaryOpportunity = &SecondaryOpportunity{
		IssueID:      "issue:kv_footprint_heavy",
		IssueFamily:  "kv_footprint_heavy",
		PriorityNote: "Lower priority than the primary recommendation.",
		Recommendation: &Recommendation{
			Decision: "reduce_kv_footprint",
			Title:    "Reduce KV footprint",
			Actions: []Action{{
				ID:            "action:reduce-max-model-len",
				Title:         "Reduce `--max-model-len`",
				CurrentValue:  "8192",
				ProposedValue: "4096",
			}},
		},
	}

	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFinalReportValidateRejectsIncompleteActionDelta(t *testing.T) {
	report := validFinalReport()
	report.Diagnosis.BaseDiagnosis.Recommendation.Actions = []Action{{
		ID:           "action:set-max-num-batched-tokens",
		Title:        "Raise `--max-num-batched-tokens`",
		CurrentValue: "2048",
	}}

	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want incomplete action delta failure")
	}
}

func TestFinalReportValidateAcceptsFollowUpStepsWithoutDelta(t *testing.T) {
	report := validFinalReport()
	report.Diagnosis.BaseDiagnosis.Recommendation.FollowUpSteps = []FollowUpStep{{
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

func TestFinalReportValidateRejectsIncompleteSecondaryOpportunity(t *testing.T) {
	report := validFinalReport()
	report.UIHints.SecondaryOpportunity = &SecondaryOpportunity{
		IssueFamily: "kv_footprint_heavy",
		Recommendation: &Recommendation{
			Decision: "reduce_kv_footprint",
			Title:    "Reduce KV footprint",
		},
	}

	if err := report.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid secondary opportunity")
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
		DiagnosticLenses: DiagnosticLenses{
			Quantization: &QuantizationLens{
				ID: "lens:quantization",
				CurrentPosture: QuantizationCurrentPosture{
					ModelID:      "Qwen/Qwen3-32B",
					DType:        "bfloat16",
					Quantization: "none",
					KVCacheDType: "auto",
					GPUFamily:    "hopper",
				},
				SelectedCandidate: QuantizationCandidate{
					Family:     "fp8",
					Repo:       "Qwen/Qwen3-32B-FP8",
					Source:     "verified_allowlist",
					Confidence: "medium",
				},
				TargetOverlay: QuantizationScenarioOverlay{Target: "throughput"},
			},
		},
		Diagnosis: DiagnosisSection{
			BaseDiagnosis: BaseDiagnosis{
				ID: "base",
				Recommendation: &Recommendation{
					Decision: "widen_scheduler_posture",
					Title:    "Widen scheduler posture",
				},
			},
			TargetOverlay: ScenarioOverlay{Target: "throughput"},
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
				Status:     "detected",
			}},
		},
		Issues: []Issue{{
			ID:   "scheduler_conservative",
			Rank: 1,
		}},
		UIHints: UIHints{
			AvailableModes:    []string{"brief", "full"},
			DefaultMode:       "brief",
			HighlightIssueIDs: []string{"scheduler_conservative"},
		},
	}
}
