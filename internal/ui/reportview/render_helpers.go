package reportview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func actionSummary(action contracts.Action) string {
	parts := []string{firstNonEmpty(action.Title, action.ID)}
	if action.Why != "" {
		parts = append(parts, action.Why)
	}
	if action.How != "" {
		parts = append(parts, "How: "+action.How)
	}
	return strings.Join(parts, " - ")
}

func frontierSummary(estimate contracts.FrontierEstimate) string {
	if summary := strings.TrimSpace(estimate.EstimateSummary); summary != "" && estimate.Value.Metric == "" {
		return summary
	}
	if estimate.Value.Metric == "" {
		return firstNonEmpty(estimate.EstimateSummary, "not available")
	}

	value := firstNonEmpty(formatFloat(estimate.Value.Estimate), "n/a")
	rangeSummary := formatRange(estimate.Value.RangeLow, estimate.Value.RangeHigh)
	parts := []string{estimate.Value.Metric + ": " + value}
	if rangeSummary != "" {
		parts = append(parts, "range "+rangeSummary)
	}
	if estimate.EstimateSummary != "" {
		parts = append(parts, estimate.EstimateSummary)
	}
	return strings.Join(parts, " • ")
}

func gainSummary(gain contracts.GainRange) string {
	if gain.Summary != "" {
		return gain.Summary
	}

	parts := make([]string, 0, 2)
	if gain.Metric != "" && gain.Estimate != nil {
		parts = append(parts, fmt.Sprintf("%s: %s", gain.Metric, formatFloat(gain.Estimate)))
	}
	if percent := formatPercentRange(gain.PercentLow, gain.PercentHigh); percent != "" {
		parts = append(parts, percent)
	}
	if len(parts) == 0 {
		return "not available"
	}
	return strings.Join(parts, " • ")
}

func recommendationTradeoff(recommendation *contracts.Recommendation) string {
	if recommendation == nil {
		return ""
	}
	return recommendation.Tradeoff.Summary
}

func formatRange(low, high *float64) string {
	if low == nil && high == nil {
		return ""
	}
	return fmt.Sprintf("%s to %s", firstNonEmpty(formatFloat(low), "n/a"), firstNonEmpty(formatFloat(high), "n/a"))
}

func formatPercentRange(low, high *float64) string {
	if low == nil && high == nil {
		return ""
	}
	return fmt.Sprintf("%s%% to %s%%", firstNonEmpty(formatFloat(low), "n/a"), firstNonEmpty(formatFloat(high), "n/a"))
}

func formatFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', 1, 64)
}

func nonZeroInt(value int) string {
	if value == 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func trimList(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func section(title string) string {
	return title
}

func listSection(title string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	lines := []string{title}
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func line(label, value string) string {
	return fmt.Sprintf("%s: %s", label, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func titleCaseTarget(target string) string {
	if target == "" {
		return "Unknown"
	}
	return strings.ToUpper(target[:1]) + target[1:]
}
