package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func TestRenderReportSummaryShowsVerdict(t *testing.T) {
	var buf bytes.Buffer
	RenderReportSummary(&buf, contracts.FinalReport{
		Entitlement: contracts.ReportEntitlement{Tier: "free"},
		Diagnosis: contracts.DiagnosisSection{
			BaseDiagnosis: contracts.BaseDiagnosis{
				CurrentLimiter: contracts.CurrentLimiter{Label: "queue-bound"},
				Recommendation: &contracts.Recommendation{
					Title:    "Widen scheduler posture",
					Tradeoff: contracts.RecommendationTradeoff{Summary: "Tail latency can worsen."},
				},
				Confidence: "medium",
			},
		},
	})

	output := buf.String()
	for _, want := range []string{"Report summary", "queue-bound", "Widen scheduler posture", "Tail latency can worsen."} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRenderSummaryPreviewShowsAnonymousHint(t *testing.T) {
	var buf bytes.Buffer
	RenderSummaryPreview(&buf, &contracts.SummaryPreview{
		Headline:              "Queue pressure is limiting throughput.",
		PrimaryRecommendation: "Widen scheduler posture",
	})

	output := buf.String()
	for _, want := range []string{"Report preview", "Queue pressure is limiting throughput.", "Full report: run inferlean login"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}
