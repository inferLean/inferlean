package collect

import (
	"testing"

	"github.com/inferLean/inferlean-main/new-cli/internal/types"
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
				PrefixHeavy:     &yes,
				Multimodal:      &yes,
				MultimodalCache: &yes,
			},
			seed: types.UserIntent{WorkloadMode: "latency", WorkloadTarget: "p95"},
			want: true,
		},
		{
			name: "missing mode",
			opts: Options{
				PrefixHeavy:     &yes,
				Multimodal:      &yes,
				MultimodalCache: &yes,
			},
			seed: types.UserIntent{WorkloadTarget: "throughput"},
			want: false,
		},
		{
			name: "missing target",
			opts: Options{
				PrefixHeavy:     &yes,
				Multimodal:      &yes,
				MultimodalCache: &yes,
			},
			seed: types.UserIntent{WorkloadMode: "balanced"},
			want: false,
		},
		{
			name: "missing prefix",
			opts: Options{Multimodal: &yes, MultimodalCache: &yes},
			seed: types.UserIntent{WorkloadMode: "balanced", WorkloadTarget: "latency"},
			want: false,
		},
		{
			name: "missing multimodal",
			opts: Options{PrefixHeavy: &yes, MultimodalCache: &yes},
			seed: types.UserIntent{WorkloadMode: "balanced", WorkloadTarget: "latency"},
			want: false,
		},
		{
			name: "missing multimodal cache",
			opts: Options{PrefixHeavy: &yes, Multimodal: &yes},
			seed: types.UserIntent{WorkloadMode: "balanced", WorkloadTarget: "latency"},
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
