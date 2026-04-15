package collector

import (
	"strings"
	"testing"
	"time"

	"github.com/inferLean/inferlean/internal/discovery"
)

func TestNormalizeWorkloadMode(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "underscore", input: "realtime_chat", want: "realtime_chat"},
		{name: "hyphenated", input: "batch-processing", want: "batch_processing"},
		{name: "spaced", input: " mixed ", want: "mixed"},
	} {
		got, err := NormalizeWorkloadMode(tc.input)
		if err != nil {
			t.Fatalf("%s: NormalizeWorkloadMode() error = %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: NormalizeWorkloadMode() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNormalizeWorkloadModeRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	_, err := NormalizeWorkloadMode("chat")
	if err == nil {
		t.Fatal("NormalizeWorkloadMode() error = nil, want validation failure")
	}
	if !strings.Contains(err.Error(), "workload mode must be one of") {
		t.Fatalf("NormalizeWorkloadMode() error = %v, want workload mode guidance", err)
	}
}

func TestNormalizeWorkloadTarget(t *testing.T) {
	t.Parallel()

	got, err := NormalizeWorkloadTarget(" throughput ")
	if err != nil {
		t.Fatalf("NormalizeWorkloadTarget() error = %v", err)
	}
	if got != "throughput" {
		t.Fatalf("NormalizeWorkloadTarget() = %q, want %q", got, "throughput")
	}
}

func TestBuildWorkloadObservationsIncludesModeAndTarget(t *testing.T) {
	t.Parallel()

	run := collectionRun{
		opts: Options{
			Target: discovery.CandidateGroup{
				RuntimeConfig: discovery.RuntimeConfig{Model: "Qwen/Qwen3.5-2B"},
			},
			CollectFor:     30 * time.Second,
			ScrapeEvery:    5 * time.Second,
			WorkloadMode:   "batch_processing",
			WorkloadTarget: "throughput",
			RepeatedPrefix: boolPointer(true),
		},
		processSamples: []processSample{{}},
		nvmlSnapshot:   &nvmlSnapshot{Samples: []nvmlSample{{}}},
	}

	workload := run.buildWorkloadObservations(true)
	if workload.Mode != "batch_processing" {
		t.Fatalf("Mode = %q, want %q", workload.Mode, "batch_processing")
	}
	if workload.Target != "throughput" {
		t.Fatalf("Target = %q, want %q", workload.Target, "throughput")
	}
	if workload.Summary == "" {
		t.Fatal("Summary = empty, want collector-generated summary")
	}
	if workload.Hints["target_model"] != "Qwen/Qwen3.5-2B" {
		t.Fatalf("Hints[target_model] = %q, want target model", workload.Hints["target_model"])
	}
	value, ok := workload.Measurements["repeated_prefix_present"].(bool)
	if !ok || !value {
		t.Fatalf("Measurements[repeated_prefix_present] = %v, want true", workload.Measurements["repeated_prefix_present"])
	}
}
