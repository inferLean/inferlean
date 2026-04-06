package reportview

import (
	"strings"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func Render(report contracts.FinalReport, mode, target string) string {
	sections := []string{
		renderVerdictSection(report, mode),
		renderOverlaySection(report, mode, target),
		renderScenarioSection(report),
		renderEvidenceSection(report, mode),
		renderCoverageSection(report, mode),
	}

	if mode == string(modeFull) {
		sections = append(sections, renderFullSections(report)...)
	}

	return strings.TrimSpace(strings.Join(compactStrings(sections), "\n\n"))
}

func overlayForTarget(report contracts.FinalReport, target string) contracts.ScenarioOverlay {
	switch target {
	case "latency":
		return report.Diagnosis.ScenarioOverlays.Latency
	case "throughput":
		return report.Diagnosis.ScenarioOverlays.Throughput
	default:
		return report.Diagnosis.ScenarioOverlays.Balanced
	}
}

func compactStrings(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
