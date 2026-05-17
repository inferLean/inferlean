package report

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func buildSaturationCard(report contracts.FinalReport) (reportCardViewModel, bool) {
	saturation := report.Saturation
	if isEmptySaturationMetric(saturation.Generic) && len(saturation.Dimensions) == 0 {
		return reportCardViewModel{}, false
	}
	card := reportCardViewModel{
		id:              "saturation",
		title:           "Deployment Saturation",
		summary:         fallback(saturation.Generic.Reason, "Backend-shaped saturation summary"),
		defaultExpanded: true,
		sections: []reportSectionViewModel{{
			title: "Generic Saturation",
			lines: saturationMetricLines(saturation.Generic),
		}},
	}
	if len(saturation.Dimensions) > 0 {
		card.sections = append(card.sections, reportSectionViewModel{
			title: "Dimensions",
			table: &reportTableViewModel{
				headers: []string{"Dimension", "Score", "Headroom", "Status", "Missing Evidence"},
				rows:    saturationDimensionRows(saturation.Dimensions),
			},
		})
	}
	return card, true
}

func saturationMetricLines(metric contracts.SaturationMetric) []string {
	return []string{
		"Label: " + fallback(metric.Label, metric.ID),
		"Status: " + fallback(metric.Status, "unknown"),
		"Score: " + formatMetricWindow(metric.Score),
		"Headroom: " + formatPercentPtr(metric.HeadroomPercent),
		"Worst Headroom: " + formatPercentPtr(metric.WorstObservedHeadroomPercent),
		"Reason: " + fallback(metric.Reason, "-"),
		"Missing Evidence: " + joinOrDash(metric.MissingEvidence),
	}
}

func saturationDimensionRows(dimensions []contracts.SaturationMetric) [][]string {
	rows := make([][]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		rows = append(rows, []string{
			fallback(dimension.Label, dimension.ID),
			formatMetricWindow(dimension.Score),
			formatPercentPtr(dimension.HeadroomPercent),
			fallback(dimension.Status, "unknown"),
			joinOrDash(dimension.MissingEvidence),
		})
	}
	return rows
}

func isEmptySaturationMetric(metric contracts.SaturationMetric) bool {
	return metric.ID == "" &&
		metric.Label == "" &&
		metric.Status == "" &&
		metric.Score.Latest == nil &&
		metric.Score.Avg == nil &&
		metric.HeadroomPercent == nil &&
		metric.WorstObservedHeadroomPercent == nil &&
		len(metric.MissingEvidence) == 0
}

func formatMetricWindow(window contracts.MetricWindow) string {
	switch {
	case window.Latest != nil:
		return fmt.Sprintf("%.1f%% latest", *window.Latest)
	case window.Avg != nil:
		return fmt.Sprintf("%.1f%% avg", *window.Avg)
	case window.Max != nil:
		return fmt.Sprintf("%.1f%% max", *window.Max)
	default:
		return "-"
	}
}

func formatPercentPtr(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", *value)
}
