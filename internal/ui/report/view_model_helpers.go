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

func projectedEffectLines(effect contracts.ProjectedEffect) []string {
	return []string{
		projectedMetricSummary("Latency", effect.Latency),
		projectedMetricSummary("Request Throughput", effect.Throughput.Requests),
		projectedMetricSummary("Output Token Throughput", effect.Throughput.OutputTokens),
	}
}

func projectedMetricSummary(label string, metric contracts.ProjectedMetricEffect) string {
	name := fallback(metric.Metric, "metric")
	unit := strings.TrimSpace(metric.Unit)
	if metric.Current == nil || metric.Projected == nil || metric.PercentDelta == nil {
		reason := fallback(metric.Reason, "estimate unavailable")
		return fmt.Sprintf("%s: %s unavailable (%s)", label, name, reason)
	}
	current := formatProjectedValue(*metric.Current, unit)
	projected := formatProjectedValue(*metric.Projected, unit)
	return fmt.Sprintf("%s: %s %s -> %s (%+.1f%%)", label, name, current, projected, *metric.PercentDelta)
}

func formatProjectedValue(value float64, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%.2f", value)
	}
	return fmt.Sprintf("%.2f %s", value, unit)
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
