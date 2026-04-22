package artifactnormalize

import (
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func normalizeProcessInspection(input Input) contracts.ProcessInspection {
	startedAt := input.Job.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	target := contracts.TargetProcess{
		PID:            input.Target.PID,
		Executable:     firstNonEmpty(input.Target.Executable, inferExecutable(input.Target.RawCommandLine)),
		RawCommandLine: strings.TrimSpace(input.Target.RawCommandLine),
		StartedAt:      &startedAt,
	}
	observed := contracts.ObservedProcess{
		PID:            target.PID,
		Executable:     target.Executable,
		RawCommandLine: target.RawCommandLine,
		StartedAt:      target.StartedAt,
	}
	return contracts.ProcessInspection{
		TargetProcess:    target,
		RelatedProcesses: []contracts.ObservedProcess{observed},
		Coverage: contracts.SourceCoverage{PresentFields: []string{
			"raw_command_line",
			"target_pid",
			"executable_identity",
			"related_process_identities",
		}},
	}
}

func normalizeWorkload(input Input) contracts.WorkloadObservations {
	mode := canonicalWorkloadMode(input.UserIntent.WorkloadMode)
	target := canonicalWorkloadTarget(input.UserIntent.WorkloadTarget)
	return contracts.WorkloadObservations{
		Mode:            mode,
		Target:          target,
		PrefixReuse:     prefixReuseState(input.UserIntent.PrefixHeavy),
		Multimodal:      multimodalState(input.UserIntent.Multimodal),
		MultimodalCache: multimodalCacheState(input.UserIntent.MultimodalCache),
	}
}

func canonicalWorkloadMode(value string) string {
	switch strings.TrimSpace(value) {
	case "realtime_chat", "batch_processing", "mixed":
		return strings.TrimSpace(value)
	default:
		return "unknown"
	}
}

func canonicalWorkloadTarget(value string) string {
	switch strings.TrimSpace(value) {
	case "latency", "throughput", "balanced":
		return strings.TrimSpace(value)
	default:
		return "unknown"
	}
}

func prefixReuseState(value bool) string {
	if value {
		return "high"
	}
	return "low"
}

func multimodalState(value bool) string {
	if value {
		return "present"
	}
	return "absent"
}

func multimodalCacheState(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}

func inferExecutable(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
