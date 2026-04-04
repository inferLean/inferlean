package collector

import (
	"fmt"
	"sort"
	"strings"
)

var (
	validWorkloadModes = map[string]struct{}{
		"realtime_chat":    {},
		"batch_processing": {},
		"mixed":            {},
	}
	validWorkloadTargets = map[string]struct{}{
		"latency":    {},
		"balanced":   {},
		"throughput": {},
	}
)

func NormalizeWorkloadMode(value string) (string, error) {
	return normalizeWorkloadValue(value, validWorkloadModes, "workload mode")
}

func NormalizeWorkloadTarget(value string) (string, error) {
	return normalizeWorkloadValue(value, validWorkloadTargets, "workload target")
}

func normalizeWorkloadValue(value string, allowed map[string]struct{}, label string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if normalized == "" {
		return "", nil
	}
	if _, ok := allowed[normalized]; ok {
		return normalized, nil
	}

	values := make([]string, 0, len(allowed))
	for value := range allowed {
		values = append(values, value)
	}
	sort.Strings(values)
	return "", fmt.Errorf("%s must be one of %s", label, strings.Join(values, ", "))
}
