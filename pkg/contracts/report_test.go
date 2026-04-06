package contracts

import (
	"testing"
	"time"
)

func TestFinalReportValidateAcceptsCanonicalShape(t *testing.T) {
	report := validFinalReport()
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
				Recommendation: &Recommendation{
					Decision: "widen_scheduler_posture",
					Title:    "Widen scheduler posture",
				},
			},
			ScenarioOverlays: ScenarioOverlays{
				Latency:    ScenarioOverlay{Target: "latency"},
				Balanced:   ScenarioOverlay{Target: "balanced"},
				Throughput: ScenarioOverlay{Target: "throughput"},
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
				Status:     "detected",
			}},
		},
		Issues: []Issue{{
			ID:   "scheduler_conservative",
			Rank: 1,
		}},
	}
}
