package pod

import (
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

func TestAppendPodUsesEnvMetricsPort(t *testing.T) {
	t.Parallel()
	var candidates []shared.Candidate
	appendPod(&candidates, "prod", "vllm-0", []podContainer{{
		Name:    "server",
		Command: []string{"vllm"},
		Args:    []string{"serve", "Qwen/Qwen3.5-0.8B"},
		Env: []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}{
			{Name: "VLLM_PORT", Value: "9100"},
		},
	}})

	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if got, want := candidates[0].MetricsEndpoint, "http://127.0.0.1:9100/metrics"; got != want {
		t.Fatalf("MetricsEndpoint = %q, want %q", got, want)
	}
}
