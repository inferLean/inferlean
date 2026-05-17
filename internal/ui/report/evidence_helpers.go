package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type qualityView struct {
	sourceStates     map[string]contracts.SourceState
	missingEvidence  []string
	degradedEvidence []string
	fallbacks        []string
	completeness     float64
	summary          string
	selectedGPUPath  string
	telemetryMode    string
	confidenceImpact string
}

func effectiveQuality(report contracts.FinalReport, artifact *contracts.RunArtifact) qualityView {
	quality := qualityView{
		sourceStates:     report.CollectionQuality.SourceStates,
		missingEvidence:  report.CollectionQuality.MissingEvidence,
		degradedEvidence: report.CollectionQuality.DegradedEvidence,
		fallbacks:        report.CollectionQuality.Fallbacks,
		completeness:     report.CollectionQuality.Completeness,
		summary:          report.CollectionQuality.Summary,
		selectedGPUPath:  report.CollectionQuality.SelectedGPUPath,
		telemetryMode:    report.CollectionQuality.TelemetryMode,
		confidenceImpact: firstNonEmpty(
			report.CollectionQuality.ConfidenceImpactSummary,
			report.DiagnosticCoverage.ConfidenceImpactSummary,
		),
	}
	if artifact == nil {
		return quality
	}
	if len(quality.sourceStates) == 0 {
		quality.sourceStates = artifact.CollectionQuality.SourceStates
	}
	if len(quality.missingEvidence) == 0 {
		quality.missingEvidence = artifact.CollectionQuality.MissingEvidence
	}
	if len(quality.degradedEvidence) == 0 {
		quality.degradedEvidence = artifact.CollectionQuality.DegradedEvidence
	}
	if len(quality.fallbacks) == 0 {
		quality.fallbacks = artifact.CollectionQuality.Fallbacks
	}
	if quality.completeness == 0 {
		quality.completeness = artifact.CollectionQuality.Completeness
	}
	if strings.TrimSpace(quality.summary) == "" {
		quality.summary = artifact.CollectionQuality.Summary
	}
	if strings.TrimSpace(quality.telemetryMode) == "" {
		quality.telemetryMode = artifact.CollectionQuality.TelemetryMode
	}
	return quality
}

func evidenceReferenceLines(refs []string, artifact *contracts.RunArtifact) []string {
	if len(refs) == 0 {
		return []string{"Evidence Refs: -"}
	}
	lines := []string{"Evidence Refs:"}
	evidence := map[string]any(nil)
	if artifact != nil {
		evidence = mapFromJSON(*artifact)
	}
	for _, ref := range refs {
		line := "  - " + ref
		if value, ok := valueByPath(evidence, ref); ok {
			line += " = " + summarizeEvidenceValue(ref, value)
		}
		lines = append(lines, line)
	}
	return lines
}

func mapFromJSON(value any) map[string]any {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func valueByPath(root map[string]any, path string) (any, bool) {
	if root == nil {
		return nil, false
	}
	var current any = root
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func summarizeEvidenceValue(ref string, value any) string {
	if strings.HasPrefix(ref, "metrics.") {
		return summarizeMetricEvidence(value)
	}
	return formatEvidenceValue(value)
}

func summarizeMetricEvidence(value any) string {
	object, ok := value.(map[string]any)
	if !ok {
		return formatEvidenceValue(value)
	}
	for _, key := range []string{"latest", "value", "avg", "count", "total"} {
		if v, ok := object[key]; ok {
			return key + "=" + formatEvidenceValue(v)
		}
	}
	return "collected"
}

func formatEvidenceValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "-"
	case string:
		return fallback(typed, "-")
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprintf("%.4g", typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, formatEvidenceValue(item))
		}
		return strings.Join(parts, ", ")
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		rendered := string(data)
		if len(rendered) > 80 {
			return rendered[:77] + "..."
		}
		return rendered
	}
}

func formatInt(value int) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", value)
}

func int32String(value int32) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", value)
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return formatTime(*value)
}
