package report

import (
	"fmt"
	"strings"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func recommendationActionsSection(actions []contracts.Action) reportSectionViewModel {
	lines := make([]string, 0, len(actions)*2)
	for i, action := range actions {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, fallback(action.Title, action.ID)))
		lines = append(lines, "   "+formatActionDelta(action))
		if why := strings.TrimSpace(action.Why); why != "" {
			lines = append(lines, "   Why: "+why)
		}
		if risk := strings.TrimSpace(action.Risk); risk != "" {
			lines = append(lines, "   Risk: "+risk)
		}
	}
	return reportSectionViewModel{title: "Actions", lines: lines}
}

func recommendationFollowUpsSection(steps []contracts.FollowUpStep) reportSectionViewModel {
	lines := make([]string, 0, len(steps)*2)
	for i, step := range steps {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, fallback(step.Title, step.ID)))
		if why := strings.TrimSpace(step.Why); why != "" {
			lines = append(lines, "   Why: "+why)
		}
		if how := strings.TrimSpace(step.How); how != "" {
			lines = append(lines, "   How: "+how)
		}
	}
	return reportSectionViewModel{title: "Follow-Up Steps", lines: lines}
}

func formatActionDelta(action contracts.Action) string {
	current, proposed := actionChange(action)
	return "Current: " + fallback(current, "-") + " -> Proposed: " + fallback(proposed, "-")
}

func frontierEstimateSummary(estimate contracts.FrontierEstimate) string {
	parts := make([]string, 0, 4)
	if summary := strings.TrimSpace(estimate.EstimateSummary); summary != "" {
		parts = append(parts, summary)
	}
	if value := estimateValueSummary(estimate.Value); value != "" {
		parts = append(parts, value)
	}
	if confidence := strings.TrimSpace(estimate.Confidence); confidence != "" {
		parts = append(parts, "confidence="+confidence)
	}
	if len(parts) == 0 {
		return fallback(estimate.Target, "-")
	}
	return strings.Join(parts, " | ")
}

func estimateValueSummary(value contracts.EstimateValue) string {
	metric := fallback(value.Metric, "metric")
	switch {
	case value.Estimate != nil:
		return fmt.Sprintf("%s=%.2f", metric, *value.Estimate)
	case value.RangeLow != nil && value.RangeHigh != nil:
		return fmt.Sprintf("%s=%.2f..%.2f", metric, *value.RangeLow, *value.RangeHigh)
	case value.RangeLow != nil:
		return fmt.Sprintf("%s>=%.2f", metric, *value.RangeLow)
	case value.RangeHigh != nil:
		return fmt.Sprintf("%s<=%.2f", metric, *value.RangeHigh)
	default:
		return ""
	}
}

func gainRangeSummary(gain contracts.GainRange) string {
	parts := make([]string, 0, 3)
	if summary := strings.TrimSpace(gain.Summary); summary != "" {
		parts = append(parts, summary)
	}
	if percent := formatPercentRange(gain.PercentLow, gain.PercentHigh); percent != "-" {
		parts = append(parts, percent)
	}
	if confidence := strings.TrimSpace(gain.Confidence); confidence != "" {
		parts = append(parts, "confidence="+confidence)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " | ")
}

func scenarioOverlaySummary(overlay contracts.ScenarioOverlay) string {
	parts := []string{fallback(overlay.Target, "unknown")}
	if summary := strings.TrimSpace(overlay.Summary); summary != "" {
		parts = append(parts, summary)
	}
	if confidence := strings.TrimSpace(overlay.Confidence); confidence != "" {
		parts = append(parts, "confidence="+confidence)
	}
	return strings.Join(parts, " | ")
}

func buildTargetSummaryLines(overlay contracts.ScenarioOverlay) []string {
	return []string{
		"Latency: " + crossMetricLine(overlay.CrossMetric.Current.LatencyE2ESeconds, overlay.CrossMetric.Projected.LatencyE2ESeconds, "s"),
		"Throughput: " + throughputProjectionLine(overlay.CrossMetric),
	}
}

func throughputProjectionLine(crossMetric contracts.CrossMetricProjection) string {
	current, currentUnit := throughputValue(crossMetric.Current)
	projected, projectedUnit := throughputValue(crossMetric.Projected)
	unit := currentUnit
	if unit == "" {
		unit = projectedUnit
	}
	return projectionLine(current, projected, unit)
}

func throughputValue(values contracts.CrossMetricValues) (*float64, string) {
	if values.RequestThroughput != nil {
		return values.RequestThroughput, "req/s"
	}
	if values.GenerationTokensPerSecond != nil {
		return values.GenerationTokensPerSecond, "tok/s"
	}
	return nil, ""
}

func crossMetricLine(current, projected *float64, unit string) string {
	return projectionLine(current, projected, unit)
}

func projectionLine(current, projected *float64, unit string) string {
	switch {
	case current == nil && projected == nil:
		return "-"
	case current != nil && projected != nil:
		return fmt.Sprintf("current %.2f%s -> projected %.2f%s", *current, withUnit(unit), *projected, withUnit(unit))
	case current != nil:
		return fmt.Sprintf("current %.2f%s", *current, withUnit(unit))
	default:
		return fmt.Sprintf("projected %.2f%s", *projected, withUnit(unit))
	}
}

func withUnit(unit string) string {
	if strings.TrimSpace(unit) == "" {
		return ""
	}
	return " " + unit
}

func quantizationOverlaySummary(overlay contracts.QuantizationScenarioOverlay) string {
	parts := []string{fallback(overlay.Target, "unknown"), formatPercentRange(overlay.GainRange.PercentLow, overlay.GainRange.PercentHigh)}
	if summary := strings.TrimSpace(overlay.Summary); summary != "" {
		parts = append(parts, summary)
	}
	return strings.Join(parts, " | ")
}

func realLoadSummary(load contracts.RealLoadSummary) string {
	parts := make([]string, 0, 5)
	appendIf := func(label, value string) {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			parts = append(parts, label+"="+trimmed)
		}
	}
	appendIf("compute", load.ComputePressure)
	appendIf("memory_bw", load.MemoryBandwidthPressure)
	appendIf("kv", load.KVPressure)
	appendIf("queue", load.QueuePressure)
	appendIf("host", load.HostPipelinePressure)
	if len(parts) == 0 {
		return strings.TrimSpace(load.Summary)
	}
	if summary := strings.TrimSpace(load.Summary); summary != "" {
		parts = append(parts, summary)
	}
	return strings.Join(parts, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func formatBytes(value int64) string {
	if value <= 0 {
		return "-"
	}
	const gib = 1024 * 1024 * 1024
	if value < gib {
		return fmt.Sprintf("%d B", value)
	}
	return fmt.Sprintf("%.1f GiB", float64(value)/gib)
}
