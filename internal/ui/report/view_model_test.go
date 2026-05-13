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
		"opportunities",
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
	if got := vm.cards[3].sections[0].table; got == nil || len(got.rows) == 0 {
		t.Fatal("issues card should include table rows")
	}
	if got := vm.cards[4].tabs; len(got) != 4 {
		t.Fatalf("evidence tabs = %d, want 4", len(got))
	}
}

func TestBuildReportViewModelOmitsOptionalCardsWhenDataMissing(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	report.Opportunities = nil

	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	for _, card := range vm.cards {
		if card.id == "opportunities" {
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
		"Headline: Top issue: KV pressure",
		"Top Issue: KV pressure",
		"Summary: The service is load-bearing enough to trust throughput guidance.",
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("verdict lines = %v, want %v", lines, want)
	}
}

func TestBuildReportViewModelKeepsPrimaryRecommendationFocused(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	if len(vm.cards[1].sections) != 3 {
		t.Fatalf("primary recommendation sections = %d, want 3", len(vm.cards[1].sections))
	}
	lines := vm.cards[1].sections[0].lines
	want := []string{
		"Title: Reduce KV footprint",
		"Rationale: This unlocks safer scheduler headroom before any throughput tuning.",
		"Projected Effect: Likely improvement: +8% to +15% throughput.",
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("primary recommendation summary lines = %v, want %v", lines, want)
	}
	projectedLines := vm.cards[1].sections[1].lines
	if !slices.Contains(projectedLines, "Request Throughput: request_throughput 4.80 req/s -> 5.28 req/s (+10.0%)") {
		t.Fatalf("projected effect lines = %v, want request throughput projection", projectedLines)
	}
	actionLines := vm.cards[1].sections[2].lines
	if slices.Contains(actionLines, "   How: Keep prompt mix and concurrency stable.") {
		t.Fatalf("action lines should not include how text: %v", actionLines)
	}
	if !slices.Contains(actionLines, "   Risk: Shorter maximum context for some requests.") {
		t.Fatalf("action lines should include risk text: %v", actionLines)
	}
}

func TestBuildReportViewModelUsesRecommendationProjectedEffect(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	lines := vm.cards[1].sections[1].lines
	want := []string{
		"Latency: latency_e2e_seconds 1.40 s -> 1.40 s (+0.0%)",
		"Request Throughput: request_throughput 4.80 req/s -> 5.28 req/s (+10.0%)",
		"Output Token Throughput: generation_tokens_per_second 256.00 tok/s -> 281.60 tok/s (+10.0%)",
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("projected effect lines = %v, want %v", lines, want)
	}
}

func TestBuildReportViewModelKeepsQuantizationAsOpportunity(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	vm := buildReportViewModel(report, reportIdentity{runID: report.Job.RunID, installationID: report.Job.InstallationID}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")

	card := vm.cards[2]
	if got, want := card.id, "opportunities"; got != want {
		t.Fatalf("card id = %q, want %q", got, want)
	}
	if len(card.sections) < 3 {
		t.Fatalf("opportunity sections = %d, want recommendation detail", len(card.sections))
	}
	want := []string{
		"Title: Evaluate quantization next",
		"Rationale: Treat quantization as a ranked opportunity.",
	}
	if !slices.Equal(card.sections[1].lines, want) {
		t.Fatalf("opportunity recommendation lines = %v, want %v", card.sections[1].lines, want)
	}
}

func fullReportFixture() contracts.FinalReport {
	reportedAt := time.Unix(1700000100, 0).UTC()
	collectedAt := time.Unix(1700000000, 0).UTC()
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
				RealLoadSummary: contracts.RealLoadSummary{
					ComputePressure: "medium",
					KVPressure:      "high",
					Summary:         "The service is load-bearing enough to trust throughput guidance.",
				},
				Confidence: "high",
			},
		},
		DiagnosticCoverage: contracts.DiagnosticCoverage{
			Summary: contracts.DiagnosticCoverageSummary{
				CoverageStatus: "complete",
			},
		},
		Issues: []contracts.Issue{{
			ID:         "issue:kv_pressure_preemption_or_swap",
			Rank:       1,
			DetectorID: "kv_pressure_preemption_or_swap",
			Family:     "kv_footprint_heavy",
			Label:      "KV pressure",
			Confidence: "high",
			Recommendation: &contracts.Recommendation{
				Decision:        "reduce_kv_footprint",
				Title:           "Reduce KV footprint",
				Rationale:       "This unlocks safer scheduler headroom before any throughput tuning.",
				Confidence:      "high",
				ProjectedEffect: projectedEffectFixture("Likely improvement: +8% to +15% throughput."),
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
		}},
		Opportunities: []contracts.Opportunity{{
			ID:         "opportunity:quantization",
			Rank:       1,
			DetectorID: "quantized_model_opportunity",
			Category:   "model_optimization",
			Title:      "Evaluate quantization next",
			Recommendation: &contracts.Recommendation{
				Decision:        "evaluate_quantization",
				Title:           "Evaluate quantization next",
				Rationale:       "Treat quantization as a ranked opportunity.",
				ProjectedEffect: projectedEffectFixture("Likely improvement: +5% to +10% latency."),
			},
		}},
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
	}
}

func projectedEffectFixture(summary string) contracts.ProjectedEffect {
	currentLatency := 1.4
	projectedLatency := 1.4
	latencyDelta := 0.0
	latencyPercent := 0.0
	currentRequests := 4.8
	projectedRequests := 5.28
	requestDelta := projectedRequests - currentRequests
	requestPercent := 10.0
	currentOutput := 256.0
	projectedOutput := 281.6
	outputDelta := projectedOutput - currentOutput
	outputPercent := 10.0
	return contracts.ProjectedEffect{
		Summary: summary,
		Latency: contracts.ProjectedMetricEffect{
			Metric:       "latency_e2e_seconds",
			Unit:         "s",
			Current:      &currentLatency,
			Projected:    &projectedLatency,
			Delta:        &latencyDelta,
			PercentDelta: &latencyPercent,
			Direction:    "lower_is_better",
			Confidence:   "medium",
		},
		Throughput: contracts.ProjectedThroughputEffect{
			Requests: contracts.ProjectedMetricEffect{
				Metric:       "request_throughput",
				Unit:         "req/s",
				Current:      &currentRequests,
				Projected:    &projectedRequests,
				Delta:        &requestDelta,
				PercentDelta: &requestPercent,
				Direction:    "higher_is_better",
				Confidence:   "medium",
			},
			OutputTokens: contracts.ProjectedMetricEffect{
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
