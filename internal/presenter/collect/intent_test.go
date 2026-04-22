package collect

import (
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
			seed: types.UserIntent{WorkloadMode: "mixed", WorkloadTarget: "latency"},
			want: true,
		},
		{
			name: "missing mode",
			opts: Options{
				PrefixHeavy:             &yes,
				Multimodal:              &yes,
				RepeatedMultimodalMedia: &yes,
			},
			seed: types.UserIntent{WorkloadTarget: "throughput"},
			want: false,
		},
		{
			name: "missing target",
			opts: Options{
				PrefixHeavy:             &yes,
				Multimodal:              &yes,
				RepeatedMultimodalMedia: &yes,
			},
			seed: types.UserIntent{WorkloadMode: "mixed"},
			want: false,
		},
		{
			name: "missing prefix",
			opts: Options{Multimodal: &yes, RepeatedMultimodalMedia: &yes},
			seed: types.UserIntent{WorkloadMode: "mixed", WorkloadTarget: "latency"},
			want: false,
		},
		{
			name: "missing multimodal",
			opts: Options{PrefixHeavy: &yes, RepeatedMultimodalMedia: &yes},
			seed: types.UserIntent{WorkloadMode: "mixed", WorkloadTarget: "latency"},
			want: false,
		},
		{
			name: "missing repeated multimodal media",
			opts: Options{PrefixHeavy: &yes, Multimodal: &yes},
			seed: types.UserIntent{WorkloadMode: "mixed", WorkloadTarget: "latency"},
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
