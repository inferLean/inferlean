package reportview

import (
	"fmt"
	"strings"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func renderVerdictSection(report contracts.FinalReport, mode string) string {
	base := report.Diagnosis.BaseDiagnosis
	lines := []string{
		section("Verdict"),
		line("Headline", firstNonEmpty(base.Situation.Headline, base.Situation.Summary, "InferLean prepared a degraded report.")),
		line("Workload", firstNonEmpty(base.WorkloadSummary.Summary, "not detected")),
		line("Dominant limiter", firstNonEmpty(base.CurrentLimiter.Label, "insufficient evidence")),
		line("Confidence", firstNonEmpty(base.Confidence, "unknown")),
		line("Current practical frontier", frontierSummary(base.Frontier.CurrentPracticalFrontier)),
		line("Safe headroom", frontierSummary(base.Frontier.SafeHeadroom)),
		renderRecommendationLines(base, mode),
		line("Key tradeoff", firstNonEmpty(base.Situation.KeyTradeoff, recommendationTradeoff(base.Recommendation), "not provided")),
	}

	actionLimit := 3
	if mode == string(modeFull) {
		actionLimit = 0
	}
	if actions := actionsForMode(base.Recommendation, actionLimit); len(actions) > 0 {
		lines = append(lines, listSection("Recommended actions", actions))
	}
	return strings.Join(compactStrings(lines), "\n")
}

func renderOverlaySection(report contracts.FinalReport, mode, target string) string {
	overlay := overlayForTarget(report, target)
	lines := []string{
		section(fmt.Sprintf("Target Overlay: %s", titleCaseTarget(target))),
		line("Summary", firstNonEmpty(overlay.Summary, "not available")),
		line("Projected frontier", frontierSummary(overlay.Frontier.ProjectedFrontierAfterPrimaryRecommendation)),
		line("Likely gain", gainSummary(overlay.Frontier.LikelyGainRange)),
		renderOverlayRecommendationLines(overlay, mode),
		line("Overlay tradeoff", firstNonEmpty(overlay.Tradeoff.Summary, "not provided")),
	}
	if caveats := trimList(overlay.Caveats); len(caveats) > 0 && mode == string(modeFull) {
		lines = append(lines, listSection("Overlay caveats", caveats))
	}
	return strings.Join(compactStrings(lines), "\n")
}

func renderScenarioSection(report contracts.FinalReport) string {
	items := scenarioSummaries(report)
	if len(items) == 0 {
		return ""
	}

	lines := []string{section("Scenario At A Glance")}
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func renderEvidenceSection(report contracts.FinalReport, mode string) string {
	highlights := evidenceHighlights(report, mode)
	if len(highlights) == 0 {
		return section("Evidence") + "\n- No evidence highlights were returned in this entitled report."
	}

	lines := []string{section("Evidence")}
	for _, item := range highlights {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func renderCoverageSection(report contracts.FinalReport, mode string) string {
	summary := coverageSummary(report.DiagnosticCoverage, mode)
	if summary == "" {
		return ""
	}
	return section("Coverage") + "\n" + summary
}

func renderFullSections(report contracts.FinalReport) []string {
	sections := []string{
		renderIssuesSection(report.Issues),
		renderCollectionQualitySection(report.CollectionQuality),
		renderEnvironmentSection(report.Environment),
		renderBaseCaveatsSection(report.Diagnosis.BaseDiagnosis),
	}
	return compactStrings(sections)
}

func renderIssuesSection(issues []contracts.Issue) string {
	items := issueSummaries(issues)
	if len(items) == 0 {
		return ""
	}

	lines := []string{section("Issues")}
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func renderCollectionQualitySection(quality contracts.ReportCollectionQuality) string {
	items := collectionQualitySummary(quality)
	if len(items) == 0 {
		return ""
	}
	return listSection("Collection Quality", items)
}

func renderEnvironmentSection(environment contracts.ReportEnvironment) string {
	items := environmentSummary(environment)
	if len(items) == 0 {
		return ""
	}
	return listSection("Environment", items)
}

func renderBaseCaveatsSection(base contracts.BaseDiagnosis) string {
	if caveats := trimList(base.Caveats); len(caveats) > 0 {
		return listSection("Base caveats", caveats)
	}
	return ""
}

func renderRecommendationLines(base contracts.BaseDiagnosis, mode string) string {
	if base.Recommendation == nil {
		return line("Primary recommendation", firstNonEmpty(base.NoSafeRecommendationReason, "no safe recommendation available"))
	}

	lines := []string{line("Primary recommendation", base.Recommendation.Title)}
	if mode == string(modeFull) {
		lines = append(lines,
			line("Rationale", firstNonEmpty(base.Recommendation.Rationale, "not provided")),
			line("Mechanism", firstNonEmpty(base.Recommendation.Mechanism, "not provided")),
		)
	}
	return strings.Join(lines, "\n")
}

func renderOverlayRecommendationLines(overlay contracts.ScenarioOverlay, mode string) string {
	if overlay.Recommendation == nil {
		return line("Overlay recommendation", firstNonEmpty(overlay.Tradeoff.Summary, "not available"))
	}

	lines := []string{line("Overlay recommendation", overlay.Recommendation.Title)}
	if mode == string(modeFull) {
		lines = append(lines, line("Overlay rationale", firstNonEmpty(overlay.Recommendation.Rationale, "not provided")))
	}
	return strings.Join(lines, "\n")
}

func scenarioSummaries(report contracts.FinalReport) []string {
	overlays := []contracts.ScenarioOverlay{
		report.Diagnosis.ScenarioOverlays.Latency,
		report.Diagnosis.ScenarioOverlays.Balanced,
		report.Diagnosis.ScenarioOverlays.Throughput,
	}
	items := make([]string, 0, len(overlays))
	for _, overlay := range overlays {
		target := firstNonEmpty(overlay.Target, "unknown")
		items = append(items, fmt.Sprintf("%s: %s", target, firstNonEmpty(overlay.Summary, overlay.Tradeoff.Summary, "not available")))
	}
	return items
}

func actionsForMode(recommendation *contracts.Recommendation, limit int) []string {
	if recommendation == nil || len(recommendation.Actions) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(recommendation.Actions) {
		limit = len(recommendation.Actions)
	}
	items := make([]string, 0, limit)
	for _, action := range recommendation.Actions[:limit] {
		items = append(items, actionSummary(action))
	}
	return items
}

func evidenceHighlights(report contracts.FinalReport, mode string) []string {
	limit := 3
	if mode == string(modeFull) {
		limit = len(report.Evidence.Highlights)
	}
	if limit > len(report.Evidence.Highlights) {
		limit = len(report.Evidence.Highlights)
	}

	items := make([]string, 0, limit)
	for _, highlight := range report.Evidence.Highlights[:limit] {
		items = append(items, firstNonEmpty(highlight.Title, highlight.Summary, highlight.ID))
	}
	return items
}

func coverageSummary(coverage contracts.DiagnosticCoverage, mode string) string {
	if !coverage.EligibleForRequiredDetectors {
		return line("Status", firstNonEmpty(coverage.IneligibleReason, "run was not eligible for the required detector pack"))
	}

	lines := []string{
		line("Status", firstNonEmpty(coverage.Summary.CoverageStatus, "unknown")),
		line("Attempted", fmt.Sprintf("%d / %d", coverage.Summary.Attempted, coverage.Summary.RequiredTotal)),
		line("Detected", fmt.Sprintf("%d", coverage.Summary.Detected)),
		line("Not evaluable", fmt.Sprintf("%d", coverage.Summary.NotEvaluable)),
	}
	if mode == string(modeFull) && coverage.ConfidenceImpactSummary != "" {
		lines = append(lines, line("Confidence impact", coverage.ConfidenceImpactSummary))
	}
	return strings.Join(lines, "\n")
}

func issueSummaries(issues []contracts.Issue) []string {
	items := make([]string, 0, len(issues))
	for _, issue := range issues {
		label := firstNonEmpty(issue.Label, issue.Family, issue.ID)
		summary := firstNonEmpty(issue.Summary, issue.Impact.Summary, "no summary")
		items = append(items, fmt.Sprintf("#%d %s: %s", issue.Rank, label, summary))
	}
	return items
}

func collectionQualitySummary(quality contracts.ReportCollectionQuality) []string {
	var items []string
	if quality.Summary != "" {
		items = append(items, quality.Summary)
	}
	if quality.TelemetryMode != "" {
		items = append(items, "Telemetry mode: "+quality.TelemetryMode)
	}
	if quality.SelectedGPUPath != "" {
		items = append(items, "GPU path: "+quality.SelectedGPUPath)
	}
	if quality.ConfidenceImpactSummary != "" {
		items = append(items, "Confidence impact: "+quality.ConfidenceImpactSummary)
	}
	for _, item := range trimList(quality.MissingEvidence) {
		items = append(items, "Missing evidence: "+item)
	}
	for _, item := range trimList(quality.DegradedEvidence) {
		items = append(items, "Degraded evidence: "+item)
	}
	return items
}

func environmentSummary(environment contracts.ReportEnvironment) []string {
	type pair struct {
		label string
		value string
	}
	values := []pair{
		{"OS", environment.OS},
		{"Kernel", environment.Kernel},
		{"CPU", environment.CPUModel},
		{"GPU", environment.GPUModel},
		{"GPU count", nonZeroInt(environment.GPUCount)},
		{"Model", firstNonEmpty(environment.Model, environment.ServedModelName)},
		{"vLLM", environment.VLLMVersion},
		{"PyTorch", environment.TorchVersion},
		{"CUDA runtime", environment.CUDARuntimeVersion},
		{"Driver", environment.DriverVersion},
	}

	items := make([]string, 0, len(values))
	for _, item := range values {
		if strings.TrimSpace(item.value) == "" {
			continue
		}
		items = append(items, item.label+": "+item.value)
	}
	return items
}
