package run

import (
	"context"
	"testing"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func TestHandleUploadFailsBeforeUploadWhenEvidenceIsInsufficient(t *testing.T) {
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

	result, err := p.handleUpload(context.Background(), Options{}, Result{
		ArtifactPath: "artifact.json",
		RunID:        "run-1",
	}, artifact)
	if err != nil {
		t.Fatalf("handleUpload() error = %v, want nil", err)
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
