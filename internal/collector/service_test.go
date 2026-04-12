package collector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inferLean/inferlean/internal/discovery"
	"github.com/inferLean/inferlean/pkg/contracts"
)

func TestValidateDurations(t *testing.T) {
	t.Parallel()

	if err := ValidateDurations(30*time.Second, 5*time.Second); err != nil {
		t.Fatalf("validate durations: %v", err)
	}
	if err := ValidateDurations(0, 5*time.Second); err == nil {
		t.Fatal("expected collect duration validation error")
	}
	if err := ValidateDurations(10*time.Second, 11*time.Second); err == nil {
		t.Fatal("expected scrape interval validation error")
	}
}

func TestCollectRejectsRemoteKubernetesPod(t *testing.T) {
	t.Parallel()

	_, err := NewService().Collect(context.Background(), Options{
		Version:     "test",
		CollectFor:  30 * time.Second,
		ScrapeEvery: 5 * time.Second,
		Target: discovery.CandidateGroup{
			Target: discovery.TargetRef{
				Kind:                discovery.TargetKindKubernetes,
				KubernetesNamespace: "nortal",
				KubernetesPodName:   "vllm-llm-0",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "collection for remote Kubernetes pods is not supported yet") {
		t.Fatalf("err = %v, want remote kubernetes collection rejection", err)
	}
}

func TestWritePrometheusConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prometheus.yml")
	if err := writePrometheusConfig(path, 5*time.Second, "127.0.0.1:8000", "127.0.0.1:9100", "127.0.0.1:9400"); err != nil {
		t.Fatalf("write config: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)
	for _, needle := range []string{"job_name: \"node_exporter\"", "job_name: \"vllm\"", "job_name: \"dcgm\""} {
		if !strings.Contains(text, needle) {
			t.Fatalf("config missing %q", needle)
		}
	}
}

func TestComputeCompleteness(t *testing.T) {
	t.Parallel()

	states := map[string]contracts.SourceState{
		"vllm_metrics":       {Status: "ok"},
		"host_metrics":       {Status: "ok"},
		"gpu_telemetry":      {Status: "degraded"},
		"nvidia_smi":         {Status: "ok"},
		"process_inspection": {Status: "ok"},
	}
	score := computeCompleteness(states)
	if score <= 0.8 || score >= 1 {
		t.Fatalf("unexpected completeness score: %f", score)
	}
}
