package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

const (
	reportReset  = "\x1b[0m"
	reportBold   = "\x1b[1m"
	reportDim    = "\x1b[2m"
	reportCyan   = "\x1b[36m"
	reportGreen  = "\x1b[32m"
	reportYellow = "\x1b[33m"
	reportRed    = "\x1b[31m"
)

func formatReportForDisplay(input map[string]any, useColor bool) (string, string, error) {
	report, err := decodeFinalReport(input)
	if err != nil {
		raw, rawErr := formatRawJSON(input)
		if rawErr != nil {
			return "", "", rawErr
		}
		return raw, "Raw JSON (schema decode failed)", nil
	}
	content := renderStructuredReport(report, useColor)
	summary := summaryLine(report)
	if err := report.Validate(); err != nil {
		warn := "Schema validation warning: " + err.Error()
		content = colorize(useColor, reportRed, warn) + "\n\n" + content
		summary += " | schema-warning"
	}
	return content, summary, nil
}

func decodeFinalReport(input map[string]any) (contracts.FinalReport, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return contracts.FinalReport{}, fmt.Errorf("encode report payload: %w", err)
	}
	var report contracts.FinalReport
	if err := json.Unmarshal(data, &report); err != nil {
		return contracts.FinalReport{}, fmt.Errorf("decode report schema: %w", err)
	}
	return report, nil
}

func formatRawJSON(input map[string]any) (string, error) {
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func summaryLine(report contracts.FinalReport) string {
	parts := make([]string, 0, 4)
	if runID := strings.TrimSpace(report.Job.RunID); runID != "" {
		parts = append(parts, "run="+runID)
	}
	if tier := strings.TrimSpace(report.Entitlement.Tier); tier != "" {
		parts = append(parts, "tier="+tier)
	}
	if issue := primaryIssue(report); issue != nil {
		if label := strings.TrimSpace(firstNonEmpty(issue.Label, issue.DetectorID)); label != "" {
			parts = append(parts, "top_issue="+label)
		}
	}
	if confidence := strings.TrimSpace(report.Diagnosis.BaseDiagnosis.Confidence); confidence != "" {
		parts = append(parts, "confidence="+confidence)
	}
	if len(parts) == 0 {
		return "Parsed report"
	}
	return strings.Join(parts, " | ")
}

func renderStructuredReport(report contracts.FinalReport, useColor bool) string {
	var b strings.Builder
	writeSection(&b, "Overview", useColor)
	writeKeyValue(&b, "Schema", fallback(report.SchemaVersion, "unknown"), useColor)
	writeKeyValue(&b, "Run ID", fallback(report.Job.RunID, "-"), useColor)
	writeKeyValue(&b, "Collector Version", fallback(report.Job.CollectorVersion, "-"), useColor)
	writeKeyValue(&b, "Reported At", formatTime(report.Job.ReportedAt), useColor)
	writeKeyValue(&b, "Tier", fallback(report.Entitlement.Tier, "-"), useColor)
	writeKeyValue(&b, "Capabilities", joinOrDash(report.Entitlement.Capabilities), useColor)

	writeSection(&b, "Environment", useColor)
	writeKeyValue(&b, "Host", fallback(report.Environment.Host, "-"), useColor)
	writeKeyValue(&b, "Model", fallback(report.Environment.Model, "-"), useColor)
	writeKeyValue(&b, "GPU", environmentGPU(report.Environment), useColor)
	writeKeyValue(&b, "CPU", environmentCPU(report.Environment), useColor)
	writeKeyValue(&b, "Runtime", fallback(report.Environment.RuntimeVersion, "-"), useColor)
	writeKeyValue(&b, "vLLM", fallback(report.Environment.VLLMVersion, "-"), useColor)

	writeSection(&b, "Diagnosis", useColor)
	base := report.Diagnosis.BaseDiagnosis
	writeKeyValue(&b, "Headline", diagnosisHeadline(report), useColor)
	writeKeyValue(&b, "Top Issue", issueLabel(primaryIssue(report)), useColor)
	writeKeyValue(&b, "Confidence", fallback(base.Confidence, "-"), useColor)
	writeKeyValue(&b, "Summary", diagnosisSummary(report), useColor)
	writeKeyValue(&b, "Workload", workloadSummary(base.WorkloadSummary), useColor)

	if recommendation := primaryRecommendation(report); recommendation != nil {
		writeSection(&b, "Primary Recommendation", useColor)
		writeKeyValue(&b, "Primary Recommendation", fallback(recommendation.Title, "-"), useColor)
		writeKeyValue(&b, "Why this is next", fallback(recommendation.Rationale, "-"), useColor)
		writeKeyValue(&b, "Projected Effect", fallback(recommendation.ProjectedEffect.Summary, "-"), useColor)
		renderProjectedEffect(&b, recommendation.ProjectedEffect, useColor)
		writeKeyValue(&b, "Confidence", fallback(recommendation.Confidence, "-"), useColor)
		if len(recommendation.Actions) == 0 && len(recommendation.FollowUpSteps) == 0 {
			writeKeyValue(&b, "Actions", "-", useColor)
		} else {
			renderRecommendationActions(&b, recommendation.Actions, useColor)
			renderFollowUpSteps(&b, recommendation.FollowUpSteps, useColor)
		}
	} else {
		writeSection(&b, "Primary Recommendation", useColor)
		writeKeyValue(&b, "Decision", "No safe recommendation", useColor)
		writeKeyValue(&b, "Reason", fallback(base.NoSafeRecommendationReason, "-"), useColor)
	}

	writeSection(&b, "Opportunities", useColor)
	if len(report.Opportunities) == 0 {
		writeKeyValue(&b, "Top Opportunities", "-", useColor)
	} else {
		for _, opportunity := range report.Opportunities {
			line := fmt.Sprintf("  [%d] %s", opportunity.Rank, fallback(opportunity.Title, opportunity.ID))
			b.WriteString(colorize(useColor, reportBold+reportYellow, line) + "\n")
		}
	}

	writeSection(&b, "Issues", useColor)
	if len(report.Issues) == 0 {
		writeKeyValue(&b, "Top Issues", "-", useColor)
	} else {
		for _, issue := range report.Issues {
			line := fmt.Sprintf("  [%d] %s", issue.Rank, fallback(issue.Label, issue.ID))
			b.WriteString(colorize(useColor, reportBold+reportYellow, line) + "\n")
			if issue.Recommendation != nil {
				recommendation := fallback(issue.Recommendation.Title, issue.Recommendation.Decision)
				b.WriteString(colorize(useColor, reportDim, "      recommendation: ") + recommendation + "\n")
			}
		}
	}

	writeSection(&b, "Coverage", useColor)
	coverage := report.DiagnosticCoverage
	writeKeyValue(&b, "Status", fallback(coverage.Summary.CoverageStatus, "-"), useColor)
	writeKeyValue(&b, "Required Detectors", fmt.Sprintf("%d", coverage.Summary.RequiredTotal), useColor)
	writeKeyValue(&b, "Attempted", fmt.Sprintf("%d", coverage.Summary.Attempted), useColor)
	writeKeyValue(&b, "Detected", fmt.Sprintf("%d", coverage.Summary.Detected), useColor)
	writeKeyValue(&b, "Ruled Out", fmt.Sprintf("%d", coverage.Summary.RuledOut), useColor)
	writeKeyValue(&b, "Not Evaluable", fmt.Sprintf("%d", coverage.Summary.NotEvaluable), useColor)
	writeKeyValue(&b, "Confidence Impact", fallback(coverage.ConfidenceImpactSummary, "-"), useColor)
	if len(coverage.DetectorResults) > 0 {
		b.WriteString(colorize(useColor, reportCyan, "Detectors:") + "\n")
		for _, detector := range coverage.DetectorResults {
			b.WriteString(colorize(useColor, reportDim, fmt.Sprintf("  - [%d] %s: %s", detector.Rank, detector.DetectorID, detector.Status)))
			if reason := strings.TrimSpace(detector.Reason); reason != "" {
				b.WriteString(" (" + reason + ")")
			}
			b.WriteString("\n")
		}
	}

	writeSection(&b, "Collection Quality", useColor)
	quality := report.CollectionQuality
	writeKeyValue(&b, "Completeness", fmt.Sprintf("%.2f", quality.Completeness), useColor)
	writeKeyValue(&b, "Telemetry Mode", fallback(quality.TelemetryMode, "-"), useColor)
	writeKeyValue(&b, "Summary", fallback(quality.Summary, "-"), useColor)
	writeKeyValue(&b, "Missing Evidence", joinOrDash(quality.MissingEvidence), useColor)
	writeKeyValue(&b, "Degraded Evidence", joinOrDash(quality.DegradedEvidence), useColor)

	return strings.TrimSpace(b.String())
}

func writeSection(b *strings.Builder, title string, useColor bool) {
	if b.Len() > 0 {
		divider := strings.Repeat("─", 72)
		b.WriteString("\n" + colorize(useColor, reportDim, divider) + "\n\n")
	}
	sectionTitle := colorize(useColor, reportBold+reportCyan, title)
	underline := colorize(useColor, reportCyan, strings.Repeat("=", max(10, len(title)+6)))
	b.WriteString(sectionTitle + "\n")
	b.WriteString(underline + "\n")
}

func writeKeyValue(b *strings.Builder, key, value string, useColor bool) {
	label := colorize(useColor, reportCyan, key+":")
	rendered := colorize(useColor, reportBold, value)
	b.WriteString(fmt.Sprintf("%s %s\n", label, rendered))
}

func fallback(value, fallbackValue string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallbackValue
	}
	return trimmed
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

func environmentGPU(env contracts.ReportEnvironment) string {
	model := fallback(env.GPUModel, "unknown")
	if env.GPUCount <= 0 {
		return model
	}
	return fmt.Sprintf("%s (%d)", model, env.GPUCount)
}

func environmentCPU(env contracts.ReportEnvironment) string {
	model := fallback(env.CPUModel, "unknown")
	if env.CPUCores <= 0 {
		return model
	}
	return fmt.Sprintf("%s (%d cores)", model, env.CPUCores)
}

func diagnosisHeadline(report contracts.FinalReport) string {
	if issue := primaryIssue(report); issue != nil {
		if label := strings.TrimSpace(issue.Label); label != "" {
			return "Top issue: " + label
		}
	}
	if recommendation := primaryRecommendation(report); recommendation != nil {
		return fallback(recommendation.Title, recommendation.Decision)
	}
	return "No safe recommendation identified"
}

func diagnosisSummary(report contracts.FinalReport) string {
	base := report.Diagnosis.BaseDiagnosis
	if summary := strings.TrimSpace(base.RealLoadSummary.Summary); summary != "" {
		return summary
	}
	return "-"
}

func workloadSummary(workload contracts.WorkloadSummary) string {
	parts := make([]string, 0, 3)
	if mode := strings.TrimSpace(workload.DeclaredWorkloadMode); mode != "" {
		parts = append(parts, "Declared Mode="+mode)
	}
	if shape := strings.TrimSpace(workload.ObservedWorkloadShape); shape != "" {
		parts = append(parts, "Observed Shape="+shape)
	}
	if posture := strings.TrimSpace(workload.ConfiguredPosture); posture != "" {
		parts = append(parts, "Configured Posture="+posture)
	}
	if workload.Multimodal {
		parts = append(parts, "Multimodal=true")
	}
	if len(parts) == 0 {
		return fallback(workload.Summary, "-")
	}
	if summary := strings.TrimSpace(workload.Summary); summary != "" {
		return strings.Join(parts, ", ") + " | " + summary
	}
	return strings.Join(parts, ", ")
}

func actionChange(action contracts.Action) (string, string) {
	return strings.TrimSpace(action.CurrentValue), strings.TrimSpace(action.ProposedValue)
}

func renderProjectedEffect(b *strings.Builder, effect contracts.ProjectedEffect, useColor bool) {
	for _, line := range projectedEffectLines(effect) {
		b.WriteString(colorize(useColor, reportDim, "  - ") + line + "\n")
	}
}

func renderRecommendationActions(b *strings.Builder, actions []contracts.Action, useColor bool) {
	if len(actions) == 0 {
		return
	}
	b.WriteString(colorize(useColor, reportCyan, "Actions:") + "\n")
	for i, action := range actions {
		item := fmt.Sprintf("  %d. %s", i+1, fallback(action.Title, action.ID))
		b.WriteString(colorize(useColor, reportGreen, item) + "\n")
		if why := strings.TrimSpace(action.Why); why != "" {
			b.WriteString(colorize(useColor, reportDim, "     Why: ") + why + "\n")
		}
		current, proposed := actionChange(action)
		b.WriteString(colorize(useColor, reportDim, "     Current: ") + fallback(current, "-") + "\n")
		b.WriteString(colorize(useColor, reportDim, "     Proposed: ") + fallback(proposed, "-") + "\n")
		if how := strings.TrimSpace(action.How); how != "" {
			b.WriteString(colorize(useColor, reportDim, "     How: ") + how + "\n")
		}
	}
}

func renderFollowUpSteps(b *strings.Builder, steps []contracts.FollowUpStep, useColor bool) {
	if len(steps) == 0 {
		return
	}
	b.WriteString(colorize(useColor, reportCyan, "Follow-up Steps:") + "\n")
	for i, step := range steps {
		item := fmt.Sprintf("  %d. %s", i+1, fallback(step.Title, step.ID))
		b.WriteString(colorize(useColor, reportGreen, item) + "\n")
		if why := strings.TrimSpace(step.Why); why != "" {
			b.WriteString(colorize(useColor, reportDim, "     Why: ") + why + "\n")
		}
		if how := strings.TrimSpace(step.How); how != "" {
			b.WriteString(colorize(useColor, reportDim, "     How: ") + how + "\n")
		}
	}
}

func colorize(useColor bool, colorCode, text string) string {
	if !useColor || strings.TrimSpace(text) == "" {
		return text
	}
	return colorCode + text + reportReset
}
