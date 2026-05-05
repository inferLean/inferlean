package collect

import (
	"testing"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/exporters/nodeexporter"
)

func TestSourceStatesExplainUnavailableHostMetrics(t *testing.T) {
	states, _ := sourceStates(promcollector.Result{
		SourceStatus: map[string]string{
			"vllm":        "ok",
			"nvml_bridge": "ok",
		},
	}, collectionSources{
		node: nodeexporter.StartResult{
			Available: false,
			Reason:    "node_exporter not found in InferLean tool directories or PATH",
		},
	})

	want := "degraded: node_exporter unavailable: node_exporter not found in InferLean tool directories or PATH"
	if states["host_metrics"] != want {
		t.Fatalf("host_metrics = %q, want %q", states["host_metrics"], want)
	}
}

func TestSourceStatesExplainHostScrapeMiss(t *testing.T) {
	states, _ := sourceStates(promcollector.Result{
		SourceStatus: map[string]string{
			"vllm":          "ok",
			"node_exporter": "missing",
			"nvml_bridge":   "ok",
		},
	}, collectionSources{
		node: nodeexporter.StartResult{Available: true},
	})

	want := "degraded: node_exporter did not produce scrape samples"
	if states["host_metrics"] != want {
		t.Fatalf("host_metrics = %q, want %q", states["host_metrics"], want)
	}
}
