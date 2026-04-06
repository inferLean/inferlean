package reportview

import (
	"strings"
	"testing"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func TestRenderBriefUsesBaseDiagnosisAndSelectedOverlay(t *testing.T) {
	report := sampleReport()

	got := Render(report, "brief", "throughput")

	for _, want := range []string{
		"Verdict",
		"Headline: Queue pressure is limiting progress",
		"Dominant limiter: queue-bound",
		"Current practical frontier: requests_per_second: 42.0",
		"Target Overlay: Throughput",
		"Overlay recommendation: Widen scheduler posture for throughput",
		"Scenario At A Glance",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render() missing %q\n%s", want, got)
		}
	}
}

func TestRenderFullIncludesCoverageIssuesAndEnvironment(t *testing.T) {
	report := sampleReport()

	got := Render(report, "full", "balanced")

	for _, want := range []string{
		"Coverage",
		"Confidence impact: Richer GPU telemetry was unavailable.",
		"Issues",
		"#1 Useful batching is too conservative",
		"Collection Quality",
		"Telemetry mode: standard",
		"Environment",
		"vLLM: 0.7.2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render() missing %q\n%s", want, got)
		}
	}
}

func sampleReport() contracts.FinalReport {
	value := 42.0
	rangeLow := 38.0
	rangeHigh := 46.0
	percentLow := 15.0
	percentHigh := 35.0

	return contracts.FinalReport{
		Job:         contracts.ReportJob{RunID: "run-123"},
		Entitlement: contracts.ReportEntitlement{Tier: "free"},
		Environment: contracts.ReportEnvironment{
			OS:                 "linux",
			GPUModel:           "NVIDIA H100",
			GPUCount:           8,
			VLLMVersion:        "0.7.2",
			CUDARuntimeVersion: "12.4",
		},
		Diagnosis: contracts.DiagnosisSection{
			BaseDiagnosis: contracts.BaseDiagnosis{
				WorkloadSummary: contracts.WorkloadSummary{Summary: "mixed workload with long-input requests"},
				CurrentLimiter:  contracts.CurrentLimiter{Label: "queue-bound"},
				Situation: contracts.Situation{
					Headline:    "Queue pressure is limiting progress",
					Summary:     "The scheduler posture is too conservative for the observed load.",
					KeyTradeoff: "Tail latency can worsen modestly.",
				},
				Confidence: "high",
				Frontier: contracts.FrontierBundle{
					CurrentPracticalFrontier: contracts.FrontierEstimate{
						EstimateSummary: "current balanced frontier",
						Value: contracts.EstimateValue{
							Metric:    "requests_per_second",
							Estimate:  &value,
							RangeLow:  &rangeLow,
							RangeHigh: &rangeHigh,
						},
					},
					SafeHeadroom: contracts.FrontierEstimate{EstimateSummary: "moderate safe headroom remains"},
				},
				Recommendation: &contracts.Recommendation{
					Title:     "Widen scheduler posture",
					Rationale: "Queue pressure is high while GPU pressure is still moderate.",
					Mechanism: "Increase useful batching and concurrency carefully.",
					Actions: []contracts.Action{
						{Title: "Raise max batched tokens", How: "Increase `--max-num-batched-tokens` and rerun."},
						{Title: "Raise max sequences", How: "Increase `--max-num-seqs` after batching."},
					},
					Tradeoff: contracts.RecommendationTradeoff{Summary: "Tail latency can worsen modestly."},
				},
				Caveats: []string{"DCGM was not available."},
			},
			ScenarioOverlays: contracts.ScenarioOverlays{
				Latency: contracts.ScenarioOverlay{
					Target:         "latency",
					Summary:        "Protect latency by staying conservative on batching.",
					Recommendation: &contracts.Recommendation{Title: "Retune for latency"},
					Tradeoff:       contracts.RecommendationTradeoff{Summary: "Lower throughput ceiling."},
				},
				Balanced: contracts.ScenarioOverlay{
					Target:         "balanced",
					Summary:        "Balanced mode keeps moderate batching with bounded latency risk.",
					Recommendation: &contracts.Recommendation{Title: "Keep a moderate scheduler posture"},
					Tradeoff:       contracts.RecommendationTradeoff{Summary: "Some tail risk remains."},
				},
				Throughput: contracts.ScenarioOverlay{
					Target:  "throughput",
					Summary: "Favor higher batching and concurrency for goodput.",
					Frontier: contracts.FrontierBundle{
						ProjectedFrontierAfterPrimaryRecommendation: contracts.FrontierEstimate{
							Value: contracts.EstimateValue{
								Metric:    "requests_per_second",
								Estimate:  &value,
								RangeLow:  &rangeLow,
								RangeHigh: &rangeHigh,
							},
						},
						LikelyGainRange: contracts.GainRange{
							Summary:     "+15% to +35% throughput",
							PercentLow:  &percentLow,
							PercentHigh: &percentHigh,
						},
					},
					Recommendation: &contracts.Recommendation{Title: "Widen scheduler posture for throughput"},
					Tradeoff:       contracts.RecommendationTradeoff{Summary: "Tail latency becomes less strict."},
				},
			},
		},
		DiagnosticCoverage: contracts.DiagnosticCoverage{
			EligibleForRequiredDetectors: true,
			Summary: contracts.DiagnosticCoverageSummary{
				RequiredTotal:  16,
				Attempted:      16,
				Detected:       2,
				NotEvaluable:   1,
				CoverageStatus: "complete_with_partial_evidence",
			},
			ConfidenceImpactSummary: "Richer GPU telemetry was unavailable.",
		},
		Issues: []contracts.Issue{
			{
				Rank:    1,
				Label:   "Useful batching is too conservative",
				Summary: "The deployment queues work before GPU-side saturation.",
			},
		},
		Evidence: contracts.Evidence{
			Highlights: []contracts.EvidenceHighlight{
				{Title: "Queue pressure is high"},
				{Title: "GPU pressure remains moderate"},
			},
		},
		CollectionQuality: contracts.ReportCollectionQuality{
			Summary:                 "The run met the standard telemetry bar.",
			TelemetryMode:           "standard",
			SelectedGPUPath:         "nvml",
			ConfidenceImpactSummary: "Richer GPU telemetry was unavailable.",
		},
		UIHints: contracts.UIHints{DefaultMode: "brief", DefaultTarget: "balanced"},
	}
}
