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
	if limiter := strings.TrimSpace(report.Diagnosis.BaseDiagnosis.CurrentLimiter.Label); limiter != "" {
		parts = append(parts, "limiter="+limiter)
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
	writeKeyValue(&b, "Headline", fallback(base.Situation.Headline, "-"), useColor)
	writeKeyValue(&b, "Limiter", limiterLabel(base.CurrentLimiter), useColor)
	writeKeyValue(&b, "Confidence", fallback(base.Confidence, "-"), useColor)
	writeKeyValue(&b, "Summary", fallback(base.Situation.Summary, "-"), useColor)
	writeKeyValue(&b, "Workload", workloadSummary(base.WorkloadSummary), useColor)
	if base.CapacitySnapshot != nil && base.CapacitySnapshot.HasData() {
		writeSection(&b, "Capacity Snapshot", useColor)
		renderCapacitySnapshot(&b, *base.CapacitySnapshot, useColor)
	}

	if base.Recommendation != nil {
		writeSection(&b, "Primary Recommendation", useColor)
		writeKeyValue(&b, "Primary Recommendation", fallback(base.Recommendation.Title, "-"), useColor)
		writeKeyValue(&b, "Why this is next", fallback(base.Recommendation.Rationale, "-"), useColor)
		writeKeyValue(&b, "Expected Gain Range", fallback(base.Recommendation.ExpectedEffect.Summary, "-"), useColor)
		writeKeyValue(&b, "Risk", fallback(base.Recommendation.Risk, "-"), useColor)
		writeKeyValue(&b, "Confidence", fallback(base.Recommendation.Confidence, "-"), useColor)
		writeKeyValue(&b, "Tradeoff", fallback(base.Recommendation.Tradeoff.Summary, "-"), useColor)
		if len(base.Recommendation.Actions) == 0 {
			writeKeyValue(&b, "Actions", "-", useColor)
		} else {
			b.WriteString(colorize(useColor, reportCyan, "Actions:") + "\n")
			for i, action := range base.Recommendation.Actions {
				item := fmt.Sprintf("  %d. %s", i+1, fallback(action.Title, action.ID))
				b.WriteString(colorize(useColor, reportGreen, item) + "\n")
				if why := strings.TrimSpace(action.Why); why != "" {
					b.WriteString(colorize(useColor, reportDim, "     Why: ") + why + "\n")
				}
				if current, proposed := actionChange(action); current != "" || proposed != "" {
					b.WriteString(colorize(useColor, reportDim, "     Current: ") + fallback(current, "-") + "\n")
					b.WriteString(colorize(useColor, reportDim, "     Proposed: ") + fallback(proposed, "-") + "\n")
				}
				if how := strings.TrimSpace(action.How); how != "" {
					b.WriteString(colorize(useColor, reportDim, "     How: ") + how + "\n")
				}
			}
		}
	} else {
		writeSection(&b, "Primary Recommendation", useColor)
		writeKeyValue(&b, "Decision", "No safe recommendation", useColor)
		writeKeyValue(&b, "Reason", fallback(base.NoSafeRecommendationReason, "-"), useColor)
	}

	writeSection(&b, "Issues", useColor)
	if len(report.Issues) == 0 {
		writeKeyValue(&b, "Top Issues", "-", useColor)
	} else {
		for _, issue := range report.Issues {
			line := fmt.Sprintf("  [%d] %s", issue.Rank, fallback(issue.Label, issue.ID))
			b.WriteString(colorize(useColor, reportBold+reportYellow, line) + "\n")
			if summary := strings.TrimSpace(issue.Summary); summary != "" {
				b.WriteString(colorize(useColor, reportDim, "      ") + summary + "\n")
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
			b.WriteString(colorize(useColor, reportDim, fmt.Sprintf("  - %s: %s", detector.DetectorID, detector.Status)))
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

	writeSection(&b, "Evidence Highlights", useColor)
	if len(report.Evidence.Highlights) == 0 {
		writeKeyValue(&b, "Highlights", "-", useColor)
	} else {
		for i, highlight := range report.Evidence.Highlights {
			b.WriteString(colorize(useColor, reportBold+reportYellow, fmt.Sprintf("  %d. %s", i+1, fallback(highlight.Title, highlight.ID))) + "\n")
			if summary := strings.TrimSpace(highlight.Summary); summary != "" {
				b.WriteString(colorize(useColor, reportDim, "     ") + summary + "\n")
			}
		}
	}

	writeSection(&b, "Scenario Overlays", useColor)
	renderOverlay(&b, report.Diagnosis.ScenarioOverlays.Latency, useColor)
	renderOverlay(&b, report.Diagnosis.ScenarioOverlays.Balanced, useColor)
	renderOverlay(&b, report.Diagnosis.ScenarioOverlays.Throughput, useColor)

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

func limiterLabel(limiter contracts.CurrentLimiter) string {
	label := strings.TrimSpace(limiter.Label)
	family := strings.TrimSpace(limiter.Family)
	if label == "" && family == "" {
		return "-"
	}
	if label == "" {
		return family
	}
	if family == "" {
		return label
	}
	return label + " [" + family + "]"
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

func renderCapacitySnapshot(b *strings.Builder, snapshot contracts.CapacitySnapshot, useColor bool) {
	writeKeyValue(b, "Summary", fallback(snapshot.Summary, "-"), useColor)
	writeKeyValue(b, "Confidence", fallback(snapshot.Confidence, "-"), useColor)
	if snapshot.Pressures.HasData() {
		writeKeyValue(b, "Pressures", pressureSummary(snapshot.Pressures), useColor)
	}
	if snapshot.Observed.HasData() {
		writeKeyValue(b, "Observed Rates", rateSummary(snapshot.Observed), useColor)
	}
	if snapshot.CurrentFrontier.HasData() {
		writeKeyValue(b, "Current Frontier", rateSummary(snapshot.CurrentFrontier), useColor)
	}
	if len(snapshot.Notes) > 0 {
		writeKeyValue(b, "Notes", strings.Join(snapshot.Notes, " "), useColor)
	}
}

func pressureSummary(pressures contracts.CapacityPressures) string {
	parts := make([]string, 0, 5)
	appendPressure := func(label, value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, label+"="+value)
		}
	}
	appendPressure("compute", pressures.Compute)
	appendPressure("memory_bw", pressures.MemoryBandwidth)
	appendPressure("kv", pressures.KV)
	appendPressure("queue", pressures.Queue)
	appendPressure("host", pressures.Host)
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func rateSummary(rates contracts.CapacityRates) string {
	parts := make([]string, 0, 3)
	appendRate := func(label string, value *float64) {
		if value != nil {
			parts = append(parts, fmt.Sprintf("%s=%.2f", label, *value))
		}
	}
	appendRate("prompt_tok/s", rates.PromptTokensPerSecond)
	appendRate("generation_tok/s", rates.GenerationTokensPerSecond)
	appendRate("req/s", rates.RequestThroughput)
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func renderOverlay(b *strings.Builder, overlay contracts.ScenarioOverlay, useColor bool) {
	target := fallback(overlay.Target, "unknown")
	recommendation := "-"
	if overlay.Recommendation != nil {
		recommendation = fallback(overlay.Recommendation.Title, overlay.Recommendation.Decision)
	}
	line := fmt.Sprintf("  - %s: %s", target, fallback(overlay.Summary, "-"))
	b.WriteString(colorize(useColor, reportBold+reportCyan, line) + "\n")
	b.WriteString(colorize(useColor, reportDim, "      recommendation: ") + recommendation + "\n")
	b.WriteString(colorize(useColor, reportDim, "      confidence: ") + fallback(overlay.Confidence, "-") + "\n")
}

func actionChange(action contracts.Action) (string, string) {
	return strings.TrimSpace(action.CurrentValue), strings.TrimSpace(action.ProposedValue)
}

func colorize(useColor bool, colorCode, text string) string {
	if !useColor || strings.TrimSpace(text) == "" {
		return text
	}
	return colorCode + text + reportReset
}
