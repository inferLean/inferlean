package artifactnormalize

import (
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

func TestNormalizeWorkloadUsesCLIWorkloadModeValues(t *testing.T) {
	workload := normalizeWorkload(Input{
		UserIntent: types.UserIntent{
			WorkloadMode:   "realtime_chat",
			WorkloadTarget: "latency",
		},
	})

	if got, want := workload.Mode, "realtime_chat"; got != want {
		t.Fatalf("Mode = %q, want %q", got, want)
	}
	if got, want := workload.Target, "latency"; got != want {
		t.Fatalf("Target = %q, want %q", got, want)
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

	if got, want := workload.Mode, "unknown"; got != want {
		t.Fatalf("Mode = %q, want %q", got, want)
	}
	if got, want := workload.Target, "unknown"; got != want {
		t.Fatalf("Target = %q, want %q", got, want)
	}
}
