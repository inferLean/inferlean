package artifactnormalize

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func parseInt(values map[string]string, keys []string, fallback int) int {
	for _, key := range keys {
		raw := strings.TrimSpace(values[key])
		if raw == "" {
			continue
		}
		parsed, err := strconv.Atoi(raw)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func parseFloat(values map[string]string, keys []string, fallback float64) float64 {
	for _, key := range keys {
		raw := strings.TrimSpace(values[key])
		if raw == "" {
			continue
		}
		parsed, err := strconv.ParseFloat(raw, 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func parseBool(values map[string]string, keys []string) (*bool, bool) {
	for _, key := range keys {
		raw := strings.TrimSpace(strings.ToLower(values[key]))
		if raw == "" {
			continue
		}
		switch raw {
		case "1", "true", "yes", "on", "enabled":
			value := true
			return &value, true
		case "0", "false", "no", "off", "disabled":
			value := false
			return &value, true
		}
	}
	return nil, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseHostPort(endpoint string) (string, int) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", 0
	}
	host := parsed.Hostname()
	if host == "" {
		return "", 0
	}
	portText := parsed.Port()
	if portText == "" {
		switch strings.ToLower(parsed.Scheme) {
		case "https":
			return host, 443
		default:
			return host, 80
		}
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return host, 0
	}
	return host, port
}

func toSourceStates(rawStates map[string]string) map[string]contracts.SourceState {
	states := map[string]contracts.SourceState{}
	for key, raw := range rawStates {
		status, reason := normalizeSourceStatus(raw)
		states[key] = contracts.SourceState{Status: status, Reason: reason}
	}
	for _, key := range requiredSourceNames() {
		if _, ok := states[key]; ok {
			continue
		}
		states[key] = contracts.SourceState{Status: "missing", Reason: "not reported by collector"}
	}
	return states
}

func normalizeSourceStatus(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "missing", "empty source status"
	}
	parts := strings.SplitN(raw, ":", 2)
	status := strings.TrimSpace(strings.ToLower(parts[0]))
	reason := ""
	if len(parts) == 2 {
		reason = strings.TrimSpace(parts[1])
	}
	switch status {
	case "ok", "degraded", "missing":
		return status, reason
	default:
		return "missing", raw
	}
}

func requiredSourceNames() []string {
	return []string{"vllm_metrics", "host_metrics", "gpu_telemetry", "nvidia_smi", "process_inspection"}
}

func appendMissing(source map[string]contracts.SourceState, target *[]string, wantStatus string) {
	for _, name := range requiredSourceNames() {
		state, ok := source[name]
		if !ok || state.Status != wantStatus {
			continue
		}
		*target = append(*target, name)
	}
}

func completeness(states map[string]contracts.SourceState) float64 {
	if len(states) == 0 {
		return 0
	}
	okCount := 0
	for _, state := range states {
		if state.Status == "ok" {
			okCount++
		}
	}
	return float64(okCount) / float64(len(states))
}

func newCoverage(present map[string]bool, required []string) contracts.SourceCoverage {
	coverage := contracts.SourceCoverage{}
	for _, field := range required {
		if present[field] {
			coverage.PresentFields = append(coverage.PresentFields, field)
			continue
		}
		coverage.MissingFields = append(coverage.MissingFields, field)
	}
	return coverage
}

func appendPresent(present map[string]bool, field string, enabled bool) {
	if enabled {
		present[field] = true
	}
}

func withSamples(values []contracts.MetricSample) contracts.MetricWindow {
	window := contracts.MetricWindow{Samples: values}
	if len(values) == 0 {
		return window
	}
	min, max, sum := values[0].Value, values[0].Value, 0.0
	for _, sample := range values {
		if sample.Value < min {
			min = sample.Value
		}
		if sample.Value > max {
			max = sample.Value
		}
		sum += sample.Value
	}
	latest := values[len(values)-1].Value
	avg := sum / float64(len(values))
	window.Latest = floatPtr(latest)
	window.Min = floatPtr(min)
	window.Max = floatPtr(max)
	window.Avg = floatPtr(avg)
	return window
}

func floatPtr(value float64) *float64 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
