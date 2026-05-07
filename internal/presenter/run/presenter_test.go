package run

import (
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/evidencegate"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func TestEvidenceFailureResultMarksRunFailed(t *testing.T) {
	failure, ok := evidencegate.Check(contracts.RunArtifact{
		CollectionQuality: contracts.CollectionQuality{
			SourceStates: map[string]contracts.SourceState{
				"vllm_metrics":       {Status: "ok"},
				"host_metrics":       {Status: "degraded"},
				"process_inspection": {Status: "ok"},
				"gpu_telemetry":      {Status: "ok"},
				"nvidia_smi":         {Status: "ok"},
			},
		},
	})
	if ok {
		t.Fatal("evidencegate.Check() ok = true, want false")
	}

	result, mapped := evidenceFailureResult(Result{
		ArtifactPath: "artifact.json",
		RunID:        "run-1",
	}, evidencegate.Error{Failure: failure})
	if !mapped {
		t.Fatal("mapped = false, want true")
	}
	if !result.Failed {
		t.Fatal("result.Failed = false, want true")
	}
	if result.FailureReason == "" {
		t.Fatal("result.FailureReason is empty")
	}
}
