package report

import (
	"fmt"
	"strings"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func renderQuantizationLens(b *strings.Builder, lens contracts.QuantizationLens, useColor bool) {
	current := lens.CurrentPosture
	candidate := lens.SelectedCandidate
	writeKeyValue(b, "Current", quantizationCurrentSummary(current), useColor)
	writeKeyValue(b, "Selected Candidate", quantizationCandidateSummary(candidate), useColor)
	writeKeyValue(b, "Confidence", fallback(lens.Confidence, candidate.Confidence), useColor)
	if lens.Recommendation != nil {
		writeKeyValue(b, "Recommendation", fallback(lens.Recommendation.Title, lens.Recommendation.Decision), useColor)
		writeKeyValue(b, "Why", fallback(lens.Recommendation.Rationale, "-"), useColor)
		renderQuantizationActions(b, lens.Recommendation.Actions, useColor)
	}
	b.WriteString(colorize(useColor, reportCyan, "Scenario Estimates:") + "\n")
	renderQuantizationOverlay(b, lens.ScenarioOverlays.Latency, useColor)
	renderQuantizationOverlay(b, lens.ScenarioOverlays.Balanced, useColor)
	renderQuantizationOverlay(b, lens.ScenarioOverlays.Throughput, useColor)
	if len(lens.Caveats) > 0 {
		writeKeyValue(b, "Caveats", strings.Join(lens.Caveats, " "), useColor)
	}
}

func renderQuantizationActions(b *strings.Builder, actions []contracts.Action, useColor bool) {
	if len(actions) == 0 {
		return
	}
	b.WriteString(colorize(useColor, reportCyan, "Validation Actions:") + "\n")
	for i, action := range actions {
		item := fmt.Sprintf("  %d. %s", i+1, fallback(action.Title, action.ID))
		b.WriteString(colorize(useColor, reportGreen, item) + "\n")
		if current, proposed := actionChange(action); current != "" || proposed != "" {
			b.WriteString(colorize(useColor, reportDim, "     Current: ") + fallback(current, "-") + "\n")
			b.WriteString(colorize(useColor, reportDim, "     Proposed: ") + fallback(proposed, "-") + "\n")
		}
		if how := strings.TrimSpace(action.How); how != "" {
			b.WriteString(colorize(useColor, reportDim, "     How: ") + how + "\n")
		}
	}
}

func quantizationCurrentSummary(current contracts.QuantizationCurrentPosture) string {
	parts := []string{
		"model=" + fallback(current.ModelID, "-"),
		"dtype=" + fallback(current.DType, "-"),
		"quantization=" + fallback(current.Quantization, "-"),
		"kv_cache_dtype=" + fallback(current.KVCacheDType, "-"),
		"gpu_family=" + fallback(current.GPUFamily, "-"),
	}
	return strings.Join(parts, ", ")
}

func quantizationCandidateSummary(candidate contracts.QuantizationCandidate) string {
	parts := []string{strings.ToUpper(fallback(candidate.Family, "unknown"))}
	if repo := strings.TrimSpace(candidate.Repo); repo != "" {
		parts = append(parts, repo)
	}
	if source := strings.TrimSpace(candidate.Source); source != "" {
		parts = append(parts, "source="+source)
	}
	return strings.Join(parts, " | ")
}

func renderQuantizationOverlay(b *strings.Builder, overlay contracts.QuantizationScenarioOverlay, useColor bool) {
	line := fmt.Sprintf(
		"  - %s: %s",
		fallback(overlay.Target, "unknown"),
		formatPercentRange(overlay.GainRange.PercentLow, overlay.GainRange.PercentHigh),
	)
	b.WriteString(colorize(useColor, reportBold+reportCyan, line) + "\n")
	if kv := strings.TrimSpace(overlay.KVHeadroomEffect); kv != "" {
		b.WriteString(colorize(useColor, reportDim, "      KV: ") + kv + "\n")
	}
	if compute := strings.TrimSpace(overlay.ComputeEffect); compute != "" {
		b.WriteString(colorize(useColor, reportDim, "      Compute: ") + compute + "\n")
	}
	if bandwidth := strings.TrimSpace(overlay.BandwidthEffect); bandwidth != "" {
		b.WriteString(colorize(useColor, reportDim, "      Bandwidth: ") + bandwidth + "\n")
	}
}

func formatPercentRange(low, high *float64) string {
	if low == nil && high == nil {
		return "-"
	}
	if low == nil {
		return fmt.Sprintf("up to %.0f%%", *high)
	}
	if high == nil {
		return fmt.Sprintf("from %.0f%%", *low)
	}
	return fmt.Sprintf("%.0f%% to %.0f%%", *low, *high)
}
