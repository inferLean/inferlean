package evidencegate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type Failure struct {
	BlockedSources []string
	Reason         string
	Hint           string
}

func Check(artifact contracts.RunArtifact) (Failure, bool) {
	blocked := make([]string, 0, 4)
	addIfBlocked := func(source string) {
		if !sourceReady(artifact, source) {
			blocked = append(blocked, source+"="+sourceStatus(artifact, source))
		}
	}

	addIfBlocked("vllm_metrics")
	addIfBlocked("host_metrics")
	addIfBlocked("process_inspection")
	if !sourceReady(artifact, "gpu_telemetry") && !sourceReady(artifact, "nvidia_smi") {
		blocked = append(blocked, "gpu_telemetry="+sourceStatus(artifact, "gpu_telemetry"))
		blocked = append(blocked, "nvidia_smi="+sourceStatus(artifact, "nvidia_smi"))
	}

	if len(blocked) == 0 {
		return Failure{}, true
	}

	blocked = unique(blocked)
	reason := "insufficient evidence: " + strings.Join(blocked, ", ")
	hint := "collect a longer window until vLLM and host metrics are parseable"
	if containsSource(blocked, "gpu_telemetry=") || containsSource(blocked, "nvidia_smi=") {
		hint = hint + " and keep at least one GPU telemetry source healthy"
	}
	return Failure{
		BlockedSources: blocked,
		Reason:         reason,
		Hint:           hint,
	}, false
}

func sourceReady(artifact contracts.RunArtifact, source string) bool {
	state, ok := artifact.CollectionQuality.SourceStates[source]
	if !ok {
		return false
	}
	return strings.TrimSpace(state.Status) == "ok"
}

func sourceStatus(artifact contracts.RunArtifact, source string) string {
	state, ok := artifact.CollectionQuality.SourceStates[source]
	if !ok {
		return "missing"
	}
	status := strings.TrimSpace(state.Status)
	if status == "" {
		return "missing"
	}
	return status
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func containsSource(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func (f Failure) String() string {
	if f.Reason != "" {
		return f.Reason
	}
	return fmt.Sprintf("blocked sources: %s", strings.Join(f.BlockedSources, ", "))
}
