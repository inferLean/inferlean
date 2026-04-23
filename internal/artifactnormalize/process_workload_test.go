package artifactnormalize

import (
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

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
