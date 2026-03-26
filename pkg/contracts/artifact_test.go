package contracts

import (
	"testing"
	"time"
)

func TestRunArtifactValidate(t *testing.T) {
	artifact := RunArtifact{
		SchemaVersion: SchemaVersion,
		Job: Job{
			RunID:            "run-123",
			InstallationID:   "inst-456",
			CollectorVersion: "0.2.0",
			SchemaVersion:    SchemaVersion,
			CollectedAt:      time.Unix(1700000000, 0).UTC(),
		},
		ProcessInspection: ProcessInspection{
			TargetProcess: TargetProcess{
				RawCommandLine: "python -m vllm serve model",
			},
		},
		CollectionQuality: CollectionQuality{
			SourceStates: map[string]SourceState{
				"vllm": {Status: "ok"},
			},
			Completeness: 1,
		},
	}

	if err := artifact.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRunArtifactValidateReportsMissingIdentityFields(t *testing.T) {
	artifact := RunArtifact{
		SchemaVersion: SchemaVersion,
		Job: Job{
			SchemaVersion: SchemaVersion,
		},
		ProcessInspection: ProcessInspection{
			TargetProcess: TargetProcess{
				RawCommandLine: "python -m vllm serve model",
			},
		},
	}

	err := artifact.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing identity fields")
	}

	want := []string{
		"job.run_id is required",
		"job.installation_id is required",
		"job.collector_version is required",
		"job.collected_at is required",
	}
	for _, fragment := range want {
		if !containsError(err.Error(), fragment) {
			t.Fatalf("Validate() error = %v, want fragment %q", err, fragment)
		}
	}
}

func TestRunArtifactValidateRejectsUnsupportedStatuses(t *testing.T) {
	artifact := RunArtifact{
		SchemaVersion: SchemaVersion,
		Job: Job{
			RunID:            "run-123",
			InstallationID:   "inst-456",
			CollectorVersion: "0.2.0",
			SchemaVersion:    SchemaVersion,
			CollectedAt:      time.Unix(1700000000, 0).UTC(),
		},
		ProcessInspection: ProcessInspection{
			TargetProcess: TargetProcess{
				RawCommandLine: "python -m vllm serve model",
			},
		},
		CollectionQuality: CollectionQuality{
			SourceStates: map[string]SourceState{
				"vllm": {Status: "broken"},
			},
			Completeness: 1,
		},
	}

	if err := artifact.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsupported source status")
	}
}

func containsError(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || containsSubstring(haystack, needle))
}

func containsSubstring(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && (indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
