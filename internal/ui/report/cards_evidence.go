package report

import (
	"fmt"
	"sort"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func buildEvidenceCard(report contracts.FinalReport) reportCardViewModel {
	return reportCardViewModel{
		id:              "evidence",
		title:           "Evidence Browser",
		summary:         "Metrics, config, environment, and quality details",
		defaultExpanded: true,
		tabs: []reportTabViewModel{
			buildMetricsTab(report),
			buildConfigTab(report),
			buildEnvironmentTab(report),
			buildQualityTab(report),
		},
	}
}

func buildCollectionQualityCard(report contracts.FinalReport) reportCardViewModel {
	quality := report.CollectionQuality
	lines := []string{
		"Completeness: " + fmt.Sprintf("%.2f", quality.Completeness),
		"Telemetry Mode: " + fallback(quality.TelemetryMode, "-"),
		"GPU Path: " + fallback(quality.SelectedGPUPath, "-"),
		"Summary: " + fallback(quality.Summary, "-"),
		"Confidence Impact: " + fallback(quality.ConfidenceImpactSummary, "-"),
		"Missing Evidence: " + joinOrDash(quality.MissingEvidence),
		"Degraded Evidence: " + joinOrDash(quality.DegradedEvidence),
	}
	return reportCardViewModel{
		id:              "collection-quality",
		title:           "Collection Quality",
		summary:         fallback(quality.Summary, fallback(quality.TelemetryMode, "Collection quality")),
		defaultExpanded: true,
		sections:        []reportSectionViewModel{{lines: lines}},
	}
}

func buildMetricsTab(report contracts.FinalReport) reportTabViewModel {
	base := report.Diagnosis.BaseDiagnosis
	lines := []string{
		"Current Frontier: " + frontierEstimateSummary(base.Frontier.CurrentPracticalFrontier),
		"Safe Headroom: " + frontierEstimateSummary(base.Frontier.SafeHeadroom),
		"Projected Frontier: " + frontierEstimateSummary(base.Frontier.ProjectedFrontierAfterPrimaryRecommendation),
		"Likely Gain: " + gainRangeSummary(base.Frontier.LikelyGainRange),
		"Target Overlay: " + scenarioOverlaySummary(report.Diagnosis.TargetOverlay),
	}
	if base.CapacitySnapshot != nil && base.CapacitySnapshot.HasData() {
		lines = append(lines, "Observed Rates: "+rateSummary(base.CapacitySnapshot.Observed))
	}
	return reportTabViewModel{
		title:    "metrics",
		summary:  "Frontier and throughput evidence",
		sections: []reportSectionViewModel{{lines: lines}},
	}
}

func buildConfigTab(report contracts.FinalReport) reportTabViewModel {
	rows := [][]string{
		{"runtime", fallback(report.Environment.RuntimeVersion, "-"), ""},
		{"vllm", fallback(report.Environment.VLLMVersion, "-"), ""},
		{"torch", fallback(report.Environment.TorchVersion, "-"), ""},
		{"cuda", fallback(report.Environment.CUDARuntimeVersion, "-"), ""},
	}
	if rec := report.Diagnosis.BaseDiagnosis.Recommendation; rec != nil {
		for _, action := range rec.Actions {
			rows = append(rows, []string{fallback(action.Title, action.ID), formatActionDelta(action), "recommended"})
		}
	}
	if lens := report.DiagnosticLenses.Quantization; lens != nil {
		rows = append(rows,
			[]string{"quantization.model", fallback(lens.CurrentPosture.ModelID, "-"), "current"},
			[]string{"quantization.dtype", fallback(lens.CurrentPosture.DType, "-"), "current"},
			[]string{"quantization.candidate", fallback(firstNonEmpty(lens.SelectedCandidate.Repo, lens.SelectedCandidate.Family), "-"), "candidate"},
		)
	}
	return reportTabViewModel{
		title:   "config",
		summary: "Runtime posture and actionable config",
		sections: []reportSectionViewModel{{
			table: &reportTableViewModel{
				headers: []string{"Key", "Value", "Note"},
				rows:    rows,
			},
		}},
	}
}

func buildEnvironmentTab(report contracts.FinalReport) reportTabViewModel {
	rows := [][]string{
		{"host", fallback(report.Environment.Host, "-")},
		{"os", fallback(report.Environment.OS, "-")},
		{"kernel", fallback(report.Environment.Kernel, "-")},
		{"cpu", environmentCPU(report.Environment)},
		{"memory", formatBytes(report.Environment.MemoryBytes)},
		{"gpu", environmentGPU(report.Environment)},
		{"driver", fallback(report.Environment.DriverVersion, "-")},
		{"model", fallback(firstNonEmpty(report.Environment.ServedModelName, report.Environment.Model), "-")},
	}
	return reportTabViewModel{
		title:   "environment",
		summary: "Host and runtime context",
		sections: []reportSectionViewModel{{
			table: &reportTableViewModel{
				headers: []string{"Field", "Value"},
				rows:    rows,
			},
		}},
	}
}

func buildQualityTab(report contracts.FinalReport) reportTabViewModel {
	rows := make([][]string, 0, len(report.CollectionQuality.SourceStates))
	keys := make([]string, 0, len(report.CollectionQuality.SourceStates))
	for key := range report.CollectionQuality.SourceStates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		state := report.CollectionQuality.SourceStates[key]
		rows = append(rows, []string{key, fallback(state.Status, "-"), fallback(state.Reason, "-")})
	}
	lines := []string{
		"Coverage: " + fallback(report.DiagnosticCoverage.Summary.CoverageStatus, "-"),
		"Missing Evidence: " + joinOrDash(report.CollectionQuality.MissingEvidence),
		"Degraded Evidence: " + joinOrDash(report.CollectionQuality.DegradedEvidence),
	}
	for _, highlight := range report.Evidence.Highlights {
		lines = append(lines, "Highlight: "+fallback(firstNonEmpty(highlight.Title, highlight.ID), "-")+" | "+fallback(highlight.Summary, "-"))
	}
	sections := []reportSectionViewModel{{lines: lines}}
	if len(rows) > 0 {
		sections = append(sections, reportSectionViewModel{
			table: &reportTableViewModel{
				headers: []string{"Source", "Status", "Reason"},
				rows:    rows,
			},
		})
	}
	return reportTabViewModel{
		title:    "quality",
		summary:  "Coverage and collection details",
		sections: sections,
	}
}
