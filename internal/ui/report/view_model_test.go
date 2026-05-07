package report

import (
	"slices"
	"testing"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func TestBuildReportViewModelIncludesDashboardParityCards(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	var ids []string
	for _, card := range vm.cards {
		ids = append(ids, card.id)
	}
	want := []string{
		"verdict",
		"primary-recommendation",
		"frontier",
		"quantization",
		"secondary-opportunity",
		"issues",
		"evidence",
		"collection-quality",
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("card order = %v, want %v", ids, want)
	}
	if vm.browserURL == "" {
		t.Fatal("expected browser URL when identity is complete")
	}
	if got := vm.cards[5].sections[0].table; got == nil || len(got.rows) == 0 {
		t.Fatal("issues card should include table rows")
	}
	if got := vm.cards[6].tabs; len(got) != 4 {
		t.Fatalf("evidence tabs = %d, want 4", len(got))
	}
}

func TestBuildReportViewModelOmitsOptionalCardsWhenDataMissing(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	report.DiagnosticLenses.Quantization = nil
	report.UIHints.SecondaryOpportunity = nil

	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	for _, card := range vm.cards {
		if card.id == "quantization" || card.id == "secondary-opportunity" {
			t.Fatalf("unexpected optional card present: %s", card.id)
		}
	}
	if vm.browserURL != "" {
		t.Fatalf("browser URL should be absent when installation identity is incomplete, got %q", vm.browserURL)
	}
}

func TestBuildReportViewModelCarriesValidationWarning(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(
		report,
		reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID},
		defaults.AppBaseURL,
		time.Unix(1700000200, 0).UTC(),
		"Schema validation warning: invalid detector status",
	)
	if vm.validationWarning == "" {
		t.Fatal("expected validation warning to be preserved in the view model")
	}
}

func TestBuildReportViewModelKeepsVerdictCardConcise(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	lines := vm.cards[0].sections[0].lines
	want := []string{
		"Headline: KV pressure is limiting throughput.",
		"Limiter: KV cache pressure [kv_pressure]",
		"Summary: Queue depth is stable but KV usage constrains headroom.",
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("verdict lines = %v, want %v", lines, want)
	}
}

func TestBuildReportViewModelKeepsPrimaryRecommendationFocused(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	if len(vm.cards[1].sections) != 2 {
		t.Fatalf("primary recommendation sections = %d, want 2", len(vm.cards[1].sections))
	}
	lines := vm.cards[1].sections[0].lines
	want := []string{
		"Title: Reduce KV footprint",
		"Rationale: This unlocks safer scheduler headroom before any throughput tuning.",
		"Expected Gain: Likely improvement: +8% to +15% throughput.",
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("primary recommendation summary lines = %v, want %v", lines, want)
	}
	actionLines := vm.cards[1].sections[1].lines
	if slices.Contains(actionLines, "   How: Keep prompt mix and concurrency stable.") {
		t.Fatalf("action lines should not include how text: %v", actionLines)
	}
	if !slices.Contains(actionLines, "   Risk: Shorter maximum context for some requests.") {
		t.Fatalf("action lines should include risk text: %v", actionLines)
	}
}

func TestBuildReportViewModelKeepsFrontierCardToTwoRows(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	lines := vm.cards[2].sections[0].lines
	want := []string{
		"Latency: current 0.82 s -> projected 0.71 s",
		"Throughput: current 4.80 req/s -> projected 5.60 req/s",
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("frontier lines = %v, want %v", lines, want)
	}
}

func fullReportFixture() contracts.FinalReport {
	reportedAt := time.Unix(1700000100, 0).UTC()
	collectedAt := time.Unix(1700000000, 0).UTC()
	reqPerSec := 4.8
	projReqPerSec := 5.6
	latencySec := 0.82
	projectedLatencySec := 0.71
	low := 8.0
	high := 15.0
	return contracts.FinalReport{
		SchemaVersion: contracts.ReportSchemaVersion,
		Job: contracts.ReportJob{
			RunID:                 "run-123",
			InstallationID:        "inst-123",
			CollectorVersion:      "0.2.0",
			ArtifactSchemaVersion: contracts.SchemaVersion,
			CollectedAt:           collectedAt,
			ReportedAt:            reportedAt,
		},
		Entitlement: contracts.ReportEntitlement{Tier: "paid"},
		Environment: contracts.ReportEnvironment{
			Host:               "gpu-host-1",
			OS:                 "ubuntu",
			Kernel:             "6.8",
			CPUModel:           "AMD EPYC",
			CPUCores:           64,
			MemoryBytes:        256 * 1024 * 1024 * 1024,
			GPUModel:           "H100",
			GPUCount:           8,
			DriverVersion:      "550",
			RuntimeVersion:     "python-3.12",
			VLLMVersion:        "0.8.4",
			TorchVersion:       "2.4",
			CUDARuntimeVersion: "12.4",
			Model:              "Qwen/Qwen3-32B",
			ServedModelName:    "Qwen3-32B",
		},
		Diagnosis: contracts.DiagnosisSection{
			BaseDiagnosis: contracts.BaseDiagnosis{
				WorkloadSummary: contracts.WorkloadSummary{
					DeclaredWorkloadMode:  "throughput",
					ObservedWorkloadShape: "steady multi-user chat",
					ConfiguredPosture:     "conservative",
					Summary:               "Observed batching remains conservative under sustained load.",
				},
				CurrentLimiter: contracts.CurrentLimiter{
					Family:  "kv_pressure",
					Label:   "KV cache pressure",
					Summary: "KV pressure remains the dominant limiter.",
				},
				RealLoadSummary: contracts.RealLoadSummary{
					ComputePressure: "medium",
					KVPressure:      "high",
					Summary:         "The service is load-bearing enough to trust throughput guidance.",
				},
				CapacitySnapshot: &contracts.CapacitySnapshot{
					Summary:    "Snapshot gathered under representative load.",
					Confidence: "medium",
					Observed: contracts.CapacityRates{
						RequestThroughput: &reqPerSec,
					},
				},
				Situation: contracts.Situation{
					Headline:    "KV pressure is limiting throughput.",
					Summary:     "Queue depth is stable but KV usage constrains headroom.",
					KeyTradeoff: "Lower max context can free KV headroom.",
				},
				Frontier: contracts.FrontierBundle{
					CurrentPracticalFrontier: contracts.FrontierEstimate{
						EstimateSummary: "Current throughput frontier",
						Value:           contracts.EstimateValue{Metric: "req/s", Estimate: &reqPerSec},
						Confidence:      "medium",
					},
					ProjectedFrontierAfterPrimaryRecommendation: contracts.FrontierEstimate{
						EstimateSummary: "Projected frontier after KV reduction",
						Value:           contracts.EstimateValue{Metric: "req/s", Estimate: &projReqPerSec},
						Confidence:      "medium",
					},
					LikelyGainRange: contracts.GainRange{
						Summary:     "Likely improvement under the observed workload.",
						PercentLow:  &low,
						PercentHigh: &high,
						Confidence:  "medium",
					},
				},
				Recommendation: &contracts.Recommendation{
					Decision:      "reduce_kv_footprint",
					Title:         "Reduce KV footprint",
					Rationale:     "This unlocks safer scheduler headroom before any throughput tuning.",
					Effort:        "low",
					Risk:          "medium",
					Reversibility: "high",
					Confidence:    "high",
					ExpectedEffect: contracts.RecommendationEffect{
						Summary: "Likely improvement: +8% to +15% throughput.",
					},
					Tradeoff: contracts.RecommendationTradeoff{
						Summary: "Some requests may need a shorter maximum context.",
					},
					Actions: []contracts.Action{{
						ID:            "action:reduce-max-model-len",
						Title:         "Reduce `--max-model-len`",
						CurrentValue:  "8192",
						ProposedValue: "4096",
						Why:           "Free KV headroom before increasing scheduler aggressiveness.",
						Risk:          "Shorter maximum context for some requests.",
					}},
					FollowUpSteps: []contracts.FollowUpStep{{
						ID:    "action:rerun",
						Title: "Rerun under the same load",
						How:   "Keep prompt mix and concurrency stable.",
					}},
				},
				Confidence: "high",
			},
			TargetOverlay: contracts.ScenarioOverlay{
				Target:     "throughput",
				Summary:    "A modest throughput win is plausible if KV pressure is reduced first.",
				Confidence: "medium",
				CrossMetric: contracts.CrossMetricProjection{
					Current: contracts.CrossMetricValues{
						LatencyE2ESeconds: &latencySec,
						RequestThroughput: &reqPerSec,
					},
					Projected: contracts.CrossMetricValues{
						LatencyE2ESeconds: &projectedLatencySec,
						RequestThroughput: &projReqPerSec,
					},
				},
			},
		},
		DiagnosticLenses: contracts.DiagnosticLenses{
			Quantization: &contracts.QuantizationLens{
				CurrentPosture: contracts.QuantizationCurrentPosture{
					ModelID:      "Qwen/Qwen3-32B",
					DType:        "bfloat16",
					Quantization: "none",
					KVCacheDType: "auto",
					GPUFamily:    "hopper",
				},
				SelectedCandidate: contracts.QuantizationCandidate{
					Family:     "fp8",
					Repo:       "Qwen/Qwen3-32B-FP8",
					Confidence: "medium",
				},
				Recommendation: &contracts.Recommendation{
					Decision: "evaluate_quantization",
					Title:    "Evaluate the FP8 candidate",
				},
				TargetOverlay: contracts.QuantizationScenarioOverlay{
					Target: "throughput",
					GainRange: contracts.GainRange{
						PercentLow:  &low,
						PercentHigh: &high,
					},
				},
			},
		},
		DiagnosticCoverage: contracts.DiagnosticCoverage{
			Summary: contracts.DiagnosticCoverageSummary{
				CoverageStatus: "complete",
			},
		},
		Issues: []contracts.Issue{{
			ID:         "issue:kv_pressure",
			Rank:       1,
			Label:      "KV pressure",
			Summary:    "KV usage remains the top constraint.",
			Confidence: "high",
		}},
		Evidence: contracts.Evidence{
			Highlights: []contracts.EvidenceHighlight{{
				ID:      "highlight:kv",
				Title:   "KV cache utilization",
				Summary: "The evidence points to sustained KV pressure.",
			}},
		},
		CollectionQuality: contracts.ReportCollectionQuality{
			Completeness:            0.93,
			TelemetryMode:           "prometheus",
			SelectedGPUPath:         "nvml_bridge",
			Summary:                 "GPU, host, and vLLM metrics were all collected.",
			ConfidenceImpactSummary: "Low confidence impact.",
			MissingEvidence:         []string{"none"},
			SourceStates: map[string]contracts.SourceState{
				"vllm_metrics": {Status: "ok"},
			},
		},
		UIHints: contracts.UIHints{
			SecondaryOpportunity: &contracts.SecondaryOpportunity{
				IssueID:      "issue:quantization",
				PriorityNote: "Lower priority than the KV reduction.",
				Recommendation: &contracts.Recommendation{
					Decision: "evaluate_quantization",
					Title:    "Evaluate quantization next",
				},
			},
		},
	}
}
