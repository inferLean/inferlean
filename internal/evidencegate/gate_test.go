package evidencegate

import (
	"strings"
	"testing"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func TestCheckBlocksDegradedHostMetrics(t *testing.T) {
	artifact := contracts.RunArtifact{
		CollectionQuality: contracts.CollectionQuality{
			SourceStates: map[string]contracts.SourceState{
				"vllm_metrics":       {Status: "ok"},
				"host_metrics":       {Status: "degraded"},
				"process_inspection": {Status: "ok"},
				"gpu_telemetry":      {Status: "ok"},
				"nvidia_smi":         {Status: "ok"},
			},
		},
	}

	failure, ok := Check(artifact)
	if ok {
		t.Fatal("ok = true, want false")
	}
	if !strings.Contains(failure.Reason, "host_metrics=degraded") {
		t.Fatalf("reason = %q, want host_metrics fragment", failure.Reason)
	}
	if failure.Hint == "" {
		t.Fatal("hint is empty")
	}
}
