package collect

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/collectors/nvml"
	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/exporters/nodeexporter"
	"github.com/inferLean/inferlean-main/cli/internal/types"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
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

func TestBuildQualityCarriesCollectionMetadataAndFallbacks(t *testing.T) {
	started := time.Unix(1700000000, 0).UTC()
	quality := buildQuality(buildInput{
		StartedAt:  started,
		FinishedAt: started.Add(30 * time.Second),
		PromResult: promcollector.Result{
			StartedAt:  started.Add(5 * time.Second),
			FinishedAt: started.Add(20 * time.Second),
			SourceStatus: map[string]string{
				"vllm":               "ok",
				"nvml_bridge":        "ok",
				"prometheus_runtime": "degraded: prometheus not found",
			},
			ScrapeInterval: time.Second,
		},
		Sources: collectionSources{
			vllmEndpoint: "http://127.0.0.1:8000/metrics",
			node:         nodeexporter.StartResult{Available: false, Reason: "node missing"},
			nvml:         nvml.BridgeResult{Available: true, Endpoint: "http://127.0.0.1:9999/metrics"},
		},
	})

	if got, want := quality.CollectionDuration, 15*time.Second; got != want {
		t.Fatalf("CollectionDuration = %v, want %v", got, want)
	}
	if got, want := quality.ScrapeInterval, time.Second; got != want {
		t.Fatalf("ScrapeInterval = %v, want %v", got, want)
	}
	if len(quality.Fallbacks) == 0 {
		t.Fatal("Fallbacks is empty, want degraded source fallbacks")
	}
	if got, want := quality.SourceMetadata["gpu_telemetry"].Transport, "nvml_bridge"; got != want {
		t.Fatalf("gpu transport = %q, want %q", got, want)
	}
}

func TestBuildArtifactReportsPostCollectionSteps(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	var steps []string

	_, err := buildArtifact(context.Background(), buildInput{
		RunID:            "run-1",
		InstallationID:   "install-1",
		CollectorVersion: "test",
		StartedAt:        now,
		FinishedAt:       now.Add(time.Second),
		Target: vllmdiscovery.Candidate{
			RawCommandLine: "vllm serve test-model",
		},
		Intent: types.UserIntent{
			DeclaredWorkloadMode:   "unknown",
			DeclaredWorkloadTarget: "unknown",
		},
		PromResult: promcollector.Result{
			SourceStatus: map[string]string{
				"vllm":          "ok",
				"node_exporter": "missing",
				"nvml_bridge":   "missing",
			},
			ScrapeInterval: time.Second,
		},
		ShowStep: func(step string) {
			steps = append(steps, step)
		},
	})
	if err != nil {
		t.Fatalf("buildArtifact() error = %v", err)
	}

	want := []string{
		"collecting runtime configuration",
		"resolving vLLM defaults",
		"building artifact",
	}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %#v, want %#v", steps, want)
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
