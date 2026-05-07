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

func TestAppendPodUsesImageFallback(t *testing.T) {
	t.Parallel()
	var candidates []shared.Candidate
	appendPod(&candidates, "prod", "vllm-0", []podContainer{{
		Name:  "server",
		Image: "vllm/vllm-openai:latest",
	}})

	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if got := candidates[0].Namespace; got != "prod" {
		t.Fatalf("Namespace = %q, want prod", got)
	}
	if got, want := candidates[0].MetricsEndpoint, "http://127.0.0.1:8000/metrics"; got != want {
		t.Fatalf("MetricsEndpoint = %q, want %q", got, want)
	}
}

func TestKubectlGetArgsUsesContextNamespaceWhenUnset(t *testing.T) {
	t.Parallel()
	got := kubectlGetArgs("vllm-0", "")
	want := []string{"get", "pod", "vllm-0", "-o", "json"}
	if len(got) != len(want) {
		t.Fatalf("len(kubectlGetArgs()) = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kubectlGetArgs()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPodNamespacePrefersObservedNamespace(t *testing.T) {
	t.Parallel()
	if got := podNamespace("default", "prod"); got != "prod" {
		t.Fatalf("podNamespace() = %q, want prod", got)
	}
	if got := podNamespace("staging", ""); got != "staging" {
		t.Fatalf("podNamespace() = %q, want staging", got)
	}
}
