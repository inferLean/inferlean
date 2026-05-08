package artifactnormalize

import (
	"testing"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

func TestNormalizeProcessInspectionDoesNotSynthesizeEvidence(t *testing.T) {
	startedAt := time.Unix(1700000000, 0).UTC()
	process := normalizeProcessInspection(Input{
		Job: JobInput{StartedAt: startedAt},
		Target: TargetInput{
			RawCommandLine: "python -m vllm.entrypoints.openai.api_server",
		},
	})

	if process.TargetProcess.StartedAt != nil {
		t.Fatalf("StartedAt = %v, want nil when process start time was not observed", process.TargetProcess.StartedAt)
	}
	if process.TargetProcess.Executable != "" {
		t.Fatalf("Executable = %q, want empty when executable was not observed", process.TargetProcess.Executable)
	}
	if len(process.RelatedProcesses) != 0 {
		t.Fatalf("RelatedProcesses = %+v, want none without related process evidence", process.RelatedProcesses)
	}
	if !process.Coverage.HasField("raw_command_line") {
		t.Fatalf("coverage should mark raw command line present: %+v", process.Coverage)
	}
	for _, missing := range []string{"target_pid", "executable_identity", "related_process_identities"} {
		if !process.Coverage.MarksField(missing) {
			t.Fatalf("coverage should mark %s missing: %+v", missing, process.Coverage)
		}
	}
}

func TestNormalizeProcessInspectionMarksOnlyObservedFieldsPresent(t *testing.T) {
	process := normalizeProcessInspection(Input{
		Target: TargetInput{
			PID:            123,
			Executable:     "python",
			RawCommandLine: "python -m vllm.entrypoints.openai.api_server",
		},
	})

	for _, present := range []string{"raw_command_line", "target_pid", "executable_identity"} {
		if !process.Coverage.HasField(present) {
			t.Fatalf("coverage should mark %s present: %+v", present, process.Coverage)
		}
	}
	if !process.Coverage.MarksField("related_process_identities") {
		t.Fatalf("coverage should mark related process identities missing: %+v", process.Coverage)
	}
}

func TestNormalizeProcessInspectionUsesNvidiaSMIProcessEvidence(t *testing.T) {
	process := normalizeProcessInspection(Input{
		Configurations: types.Configurations{
			NvidiaSMIStaticText: `|    0   N/A  N/A          530596      C   VLLM::EngineCore                       4726MiB |`,
		},
	})

	if len(process.RelatedProcesses) != 1 {
		t.Fatalf("related process count = %d, want 1", len(process.RelatedProcesses))
	}
	if got, want := process.RelatedProcesses[0].PID, int32(530596); got != want {
		t.Fatalf("related process pid = %d, want %d", got, want)
	}
	if got, want := process.RelatedProcesses[0].Executable, "VLLM::EngineCore"; got != want {
		t.Fatalf("related process executable = %q, want %q", got, want)
	}
	if !process.Coverage.HasField("related_process_identities") {
		t.Fatalf("coverage should mark related process identities present: %+v", process.Coverage)
	}
}

func TestNormalizeWorkloadUsesCLIWorkloadModeValues(t *testing.T) {
	workload := normalizeWorkload(Input{
		UserIntent: types.UserIntent{
			DeclaredWorkloadMode:   "chat",
			DeclaredWorkloadTarget: "latency",
		},
	})

	if got, want := workload.DeclaredWorkloadMode, "chat"; got != want {
		t.Fatalf("DeclaredWorkloadMode = %q, want %q", got, want)
	}
	if got, want := workload.DeclaredWorkloadTarget, "latency"; got != want {
		t.Fatalf("DeclaredWorkloadTarget = %q, want %q", got, want)
	}
	if got, want := workload.PrefixReuse, "low"; got != want {
		t.Fatalf("PrefixReuse = %q, want %q", got, want)
	}
	if got, want := workload.Multimodal, "absent"; got != want {
		t.Fatalf("Multimodal = %q, want %q", got, want)
	}
	if got, want := workload.RepeatedMultimodalMedia, "low"; got != want {
		t.Fatalf("RepeatedMultimodalMedia = %q, want %q", got, want)
	}
}

func TestNormalizeWorkloadDefaultsUnknownForMissingMode(t *testing.T) {
	workload := normalizeWorkload(Input{UserIntent: types.UserIntent{}})

	if got, want := workload.DeclaredWorkloadMode, "unknown"; got != want {
		t.Fatalf("DeclaredWorkloadMode = %q, want %q", got, want)
	}
	if got, want := workload.DeclaredWorkloadTarget, "unknown"; got != want {
		t.Fatalf("DeclaredWorkloadTarget = %q, want %q", got, want)
	}
}
