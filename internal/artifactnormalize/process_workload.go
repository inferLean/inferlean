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
	prefixReuse, multimodal := boolFromIntent(input.UserIntent)
	target := strings.TrimSpace(input.UserIntent.WorkloadTarget)
	mode := strings.TrimSpace(input.UserIntent.WorkloadMode)
	if mode == "" {
		mode = "online"
	}
	if target == "" {
		target = "balanced"
	}
	return contracts.WorkloadObservations{
		Mode:    mode,
		Target:  target,
		Summary: "resolved from collector intent and observed vLLM metrics",
		Hints: map[string]string{
			"prefix_reuse":  prefixReuse,
			"multimodal":    multimodal,
			"dominant_goal": target,
		},
	}
}

func inferExecutable(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
