package collector

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func collectWarnings(collectFor, scrapeEvery time.Duration) []string {
	if collectFor/scrapeEvery >= 2 {
		return nil
	}
	return []string{"collection window may produce fewer than two scrapes; results may be less stable"}
}

func emitStep(stepf func(StepUpdate), step Step, message string, collectFor time.Duration) {
	if stepf != nil {
		stepf(StepUpdate{Step: step, Message: message, CollectFor: collectFor})
	}
}

func missingCapture(reason string) sourceCapture {
	return sourceCapture{Status: "missing", Reason: reason}
}

func degradedCapture(reason string) sourceCapture {
	return sourceCapture{Status: "degraded", Reason: reason}
}

func computeCompleteness(states map[string]contracts.SourceState) float64 {
	weights := map[string]float64{
		"vllm_metrics":       0.30,
		"host_metrics":       0.20,
		"gpu_telemetry":      0.20,
		"nvidia_smi":         0.10,
		"process_inspection": 0.20,
	}
	score := 0.0
	for name, weight := range weights {
		switch states[name].Status {
		case "ok":
			score += weight
		case "degraded":
			score += weight * 0.5
		}
	}
	return score
}

func hasMinimumEvidence(states map[string]contracts.SourceState) bool {
	for _, key := range []string{"vllm_metrics", "host_metrics", "gpu_telemetry", "nvidia_smi", "process_inspection"} {
		if states[key].Status != "ok" {
			return false
		}
	}
	return true
}

func missingEvidence(states map[string]contracts.SourceState) []string {
	return collectByStatus(states, "missing")
}

func degradedEvidence(states map[string]contracts.SourceState) []string {
	return collectByStatus(states, "degraded")
}

func collectByStatus(states map[string]contracts.SourceState, status string) []string {
	var values []string
	for name, state := range states {
		if state.Status == status {
			values = append(values, name)
		}
	}
	sort.Strings(values)
	return values
}

func qualitySummary(states map[string]contracts.SourceState, minimumEvidenceMet bool) string {
	if minimumEvidenceMet {
		return "all required evidence sources were captured successfully"
	}
	var parts []string
	for name, state := range states {
		if state.Status == "ok" {
			continue
		}
		if state.Reason != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", name, state.Reason))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %s", name, state.Status))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}
