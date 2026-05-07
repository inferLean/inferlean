package run

import (
	"context"
	"testing"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func TestUploadAndFinishFailsBeforeUploadWhenEvidenceIsInsufficient(t *testing.T) {
	p := Presenter{}
	artifact := contracts.RunArtifact{
		CollectionQuality: contracts.CollectionQuality{
			SourceStates: map[string]contracts.SourceState{
				"vllm_metrics":  {Status: "ok"},
				"host_metrics":  {Status: "degraded"},
				"gpu_telemetry": {Status: "ok"},
				"nvidia_smi":    {Status: "ok"},
			},
		},
	}

	result, err := p.uploadAndFinish(context.Background(), Options{}, Result{
		ArtifactPath: "artifact.json",
		RunID:        "run-1",
	}, artifact)
	if err == nil {
		t.Fatal("uploadAndFinish() error = nil, want failure")
	}
	if !result.Failed {
		t.Fatal("result.Failed = false, want true")
	}
	if result.Uploaded {
		t.Fatal("result.Uploaded = true, want false")
	}
	if result.FailureReason == "" {
		t.Fatal("result.FailureReason is empty")
	}
}
