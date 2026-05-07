package collect

import (
	"context"
	"strings"
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

func TestStartVLLMMetricsUsesProcessEndpointFromTarget(t *testing.T) {
	t.Parallel()
	endpoint, session, err := startVLLMMetrics(context.Background(), vllmdiscovery.Candidate{
		Source:          "process",
		MetricsEndpoint: "http://127.0.0.1:9100/metrics",
	})
	if err != nil {
		t.Fatalf("startVLLMMetrics returned error: %v", err)
	}
	if session != nil {
		t.Fatal("process target should not start a helper session")
	}
	if endpoint != "http://127.0.0.1:9100/metrics" {
		t.Fatalf("endpoint = %q, want target endpoint", endpoint)
	}
}

func TestStartVLLMMetricsInfersProcessEndpoint(t *testing.T) {
	t.Parallel()
	endpoint, session, err := startVLLMMetrics(context.Background(), vllmdiscovery.Candidate{
		Source:         "process",
		RawCommandLine: "vllm serve Qwen/Qwen3.5-0.8B --port 9101",
	})
	if err != nil {
		t.Fatalf("startVLLMMetrics returned error: %v", err)
	}
	if session != nil {
		t.Fatal("process target should not start a helper session")
	}
	if endpoint != "http://127.0.0.1:9101/metrics" {
		t.Fatalf("endpoint = %q, want inferred endpoint", endpoint)
	}
}

func TestStartVLLMMetricsRejectsDockerWithoutPublishedEndpoint(t *testing.T) {
	t.Parallel()
	_, _, err := startVLLMMetrics(context.Background(), vllmdiscovery.Candidate{
		Source:         "docker",
		RawCommandLine: "vllm serve Qwen/Qwen3.5-0.8B --port 9000",
	})
	if err == nil {
		t.Fatal("expected missing docker published port error")
	}
	if !strings.Contains(err.Error(), "docker -p") {
		t.Fatalf("error = %q, want docker publish guidance", err)
	}
}

func TestEndpointPort(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"http://127.0.0.1:9100/metrics": 9100,
		"http://[::1]:9200/metrics":     9200,
		"http://127.0.0.1/metrics":      0,
		"":                              0,
	}
	for input, want := range cases {
		if got := endpointPort(input); got != want {
			t.Fatalf("endpointPort(%q) = %d, want %d", input, got, want)
		}
	}
}
