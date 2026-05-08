package artifactnormalize

import (
	"strconv"
	"strings"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func normalizeProcessInspection(input Input) contracts.ProcessInspection {
	target := contracts.TargetProcess{
		PID:            input.Target.PID,
		InternalPID:    input.Target.InternalPID,
		Executable:     strings.TrimSpace(input.Target.Executable),
		RawCommandLine: strings.TrimSpace(input.Target.RawCommandLine),
	}
	related := relatedProcessesFromNvidiaSMI(input.Configurations.NvidiaSMIStaticText)
	present := map[string]bool{}
	appendPresent(present, "raw_command_line", target.RawCommandLine != "")
	appendPresent(present, "target_pid", target.PID > 0)
	appendPresent(present, "internal_pid", target.InternalPID > 0)
	appendPresent(present, "executable_identity", target.Executable != "")
	appendPresent(present, "related_process_identities", len(related) > 0)
	return contracts.ProcessInspection{
		TargetProcess:    target,
		RelatedProcesses: related,
		Coverage:         newCoverage(present, processInspectionRequiredFields()),
	}
}

func relatedProcessesFromNvidiaSMI(raw string) []contracts.ObservedProcess {
	processes := []contracts.ObservedProcess{}
	for _, line := range strings.Split(raw, "\n") {
		normalized := strings.ToLower(line)
		if !strings.Contains(normalized, "vllm") || !strings.Contains(line, "MiB") {
			continue
		}
		fields := strings.Fields(strings.ReplaceAll(line, "|", " "))
		for idx, field := range fields {
			if field != "C" && field != "G" {
				continue
			}
			if idx == 0 || idx+1 >= len(fields) {
				continue
			}
			pid, err := strconv.ParseInt(fields[idx-1], 10, 32)
			if err != nil || pid <= 0 {
				continue
			}
			executable := strings.Join(fields[idx+1:len(fields)-1], " ")
			processes = append(processes, contracts.ObservedProcess{
				PID:            int32(pid),
				Executable:     strings.TrimSpace(executable),
				RawCommandLine: strings.TrimSpace(executable),
			})
			break
		}
	}
	return processes
}

func processInspectionRequiredFields() []string {
	return []string{
		"raw_command_line",
		"target_pid",
		"internal_pid",
		"executable_identity",
		"related_process_identities",
	}
}

func normalizeWorkload(input Input) contracts.WorkloadObservations {
	mode := canonicalDeclaredWorkloadMode(input.UserIntent.DeclaredWorkloadMode)
	target := canonicalDeclaredWorkloadTarget(input.UserIntent.DeclaredWorkloadTarget)
	return contracts.WorkloadObservations{
		DeclaredWorkloadMode:    mode,
		DeclaredWorkloadTarget:  target,
		PrefixReuse:             prefixReuseState(input.UserIntent.PrefixHeavy),
		Multimodal:              multimodalState(input.UserIntent.Multimodal),
		RepeatedMultimodalMedia: repeatedMultimodalMediaState(input.UserIntent.RepeatedMultimodalMedia),
	}
}

func canonicalDeclaredWorkloadMode(value string) string {
	switch strings.TrimSpace(value) {
	case "chat", "batch", "mixed":
		return strings.TrimSpace(value)
	default:
		return "unknown"
	}
}

func canonicalDeclaredWorkloadTarget(value string) string {
	switch strings.TrimSpace(value) {
	case "latency", "throughput":
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

func repeatedMultimodalMediaState(value bool) string {
	if value {
		return "high"
	}
	return "low"
}
