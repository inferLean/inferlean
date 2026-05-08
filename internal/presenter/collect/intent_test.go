package collect

import (
	"strings"
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/types"
)

func TestHasCompleteIntent(t *testing.T) {
	t.Parallel()
	yes := true
	cases := []struct {
		name string
		opts Options
		seed types.UserIntent
		want bool
	}{
		{
			name: "complete",
			opts: Options{
				PrefixHeavy:             &yes,
				Multimodal:              &yes,
				RepeatedMultimodalMedia: &yes,
			},
			seed: types.UserIntent{DeclaredWorkloadMode: "mixed", DeclaredWorkloadTarget: "latency"},
			want: true,
		},
		{
			name: "missing mode",
			opts: Options{
				PrefixHeavy:             &yes,
				Multimodal:              &yes,
				RepeatedMultimodalMedia: &yes,
			},
			seed: types.UserIntent{DeclaredWorkloadTarget: "throughput"},
			want: false,
		},
		{
			name: "missing target",
			opts: Options{
				PrefixHeavy:             &yes,
				Multimodal:              &yes,
				RepeatedMultimodalMedia: &yes,
			},
			seed: types.UserIntent{DeclaredWorkloadMode: "mixed"},
			want: false,
		},
		{
			name: "missing prefix",
			opts: Options{Multimodal: &yes, RepeatedMultimodalMedia: &yes},
			seed: types.UserIntent{DeclaredWorkloadMode: "mixed", DeclaredWorkloadTarget: "latency"},
			want: false,
		},
		{
			name: "missing multimodal",
			opts: Options{PrefixHeavy: &yes, RepeatedMultimodalMedia: &yes},
			seed: types.UserIntent{DeclaredWorkloadMode: "mixed", DeclaredWorkloadTarget: "latency"},
			want: false,
		},
		{
			name: "missing repeated multimodal media",
			opts: Options{PrefixHeavy: &yes, Multimodal: &yes},
			seed: types.UserIntent{DeclaredWorkloadMode: "mixed", DeclaredWorkloadTarget: "latency"},
			want: false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := hasCompleteIntent(tc.opts, tc.seed)
			if got != tc.want {
				t.Fatalf("hasCompleteIntent() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRequireCompleteIntentReportsMissingFlags(t *testing.T) {
	t.Parallel()
	err := requireCompleteIntent(Options{}, types.UserIntent{})
	if err == nil {
		t.Fatal("expected missing intent error")
	}
	message := err.Error()
	for _, want := range []string{
		"--workload-mode",
		"--workload-target",
		"--prefix-heavy",
		"--multimodal",
		"--repeated-multimodal-media",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("missing error %q does not mention %s", message, want)
		}
	}
}

func TestRequireCompleteIntentAllowsExplicitFalseValues(t *testing.T) {
	t.Parallel()
	no := false
	err := requireCompleteIntent(Options{
		PrefixHeavy:             &no,
		Multimodal:              &no,
		RepeatedMultimodalMedia: &no,
	}, types.UserIntent{
		DeclaredWorkloadMode:   "mixed",
		DeclaredWorkloadTarget: "latency",
	})
	if err != nil {
		t.Fatalf("expected explicit false intent to be complete, got %v", err)
	}
}
