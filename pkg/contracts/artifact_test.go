package contracts

import (
	"strings"
	"testing"
)

func TestRunArtifactValidate(t *testing.T) {
	artifact := validArtifact()
	if err := artifact.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRunArtifactValidateRejectsBalancedTarget(t *testing.T) {
	artifact := validArtifact()
	artifact.WorkloadObservations.DeclaredWorkloadTarget = "balanced"

	err := artifact.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid target failure")
	}
	if !strings.Contains(err.Error(), "workload_observations.declared_workload_target must be one of extreme_latency, latency, throughput, extreme_throughput, unknown") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRunArtifactValidateRejectsMissingCanonicalMetricWithoutCoverage(t *testing.T) {
	artifact := validArtifact()
	artifact.Metrics.Host.CPULoad = MetricWindow{}
	artifact.Metrics.Host.Coverage.PresentFields = removeField(artifact.Metrics.Host.Coverage.PresentFields, "cpu_load")

	err := artifact.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing canonical metric failure")
	}
	if !strings.Contains(err.Error(), "metrics.host.cpu_load must be populated or marked missing/unsupported") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRunArtifactValidateReportsMissingIdentityFields(t *testing.T) {
	artifact := validArtifact()
	artifact.Job = Job{SchemaVersion: SchemaVersion}

	err := artifact.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want missing identity fields")
	}
	for _, fragment := range []string{
		"job.run_id is required",
		"job.installation_id is required",
		"job.collector_version is required",
		"job.collected_at is required",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("Validate() error = %v, want fragment %q", err, fragment)
		}
	}
}

func TestRunArtifactValidateRejectsUnsupportedStatuses(t *testing.T) {
	artifact := validArtifact()
	artifact.CollectionQuality.SourceStates["vllm_metrics"] = SourceState{Status: "broken"}

	err := artifact.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want unsupported source status")
	}
	if !strings.Contains(err.Error(), "collection_quality.source_states[vllm_metrics].status must be ok, degraded, or missing") {
		t.Fatalf("Validate() error = %v", err)
	}
}
