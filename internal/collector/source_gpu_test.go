package collector

import (
	"testing"
	"time"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func TestMergeGPUCoveragePrefersPrimaryAndFillsFromSecondary(t *testing.T) {
	primary := contracts.GPUTelemetry{
		Power: testWindow(150),
	}
	secondary := contracts.GPUTelemetry{
		Temperature: testWindow(72),
	}

	merged, coverage := mergeGPUCoverage(
		primary,
		contracts.SourceCoverage{PresentFields: []string{"power"}, RawEvidenceRef: "raw/dcgm.metrics"},
		secondary,
		contracts.SourceCoverage{PresentFields: []string{"temperature"}, RawEvidenceRef: "raw/nvml.json"},
	)

	if !merged.Power.HasData() || !merged.Temperature.HasData() {
		t.Fatalf("expected merged GPU telemetry to include both primary and secondary fields: %+v", merged)
	}
	if coverage.RawEvidenceRef != "raw/dcgm.metrics" {
		t.Fatalf("expected primary raw evidence ref, got %q", coverage.RawEvidenceRef)
	}
	if !containsCoverageName(coverage.PresentFields, "power") || !containsCoverageName(coverage.PresentFields, "temperature") {
		t.Fatalf("expected merged coverage to include both fields, got %+v", coverage)
	}
}

func TestNVMLMetricsFromSnapshotUsesProvidedCoverage(t *testing.T) {
	snapshot := &nvmlSnapshot{
		DriverVersion: "550.54.15",
		Samples: []nvmlSample{{
			Timestamp: time.Unix(1700000000, 0).UTC(),
			Driver:    "550.54.15",
			GPUs: []nvmlGPU{{
				Name:             "NVIDIA H100",
				Utilization:      73,
				MemoryUsedBytes:  100,
				MemoryFreeBytes:  200,
				MemoryTotalBytes: 300,
				SMClockMHz:       1200,
				MemClockMHz:      1800,
				PowerDrawWatts:   250,
				TemperatureC:     70,
				PCIeRxKBs:        10,
				PCIeTxKBs:        12,
			}},
		}},
	}
	coverage := contracts.SourceCoverage{
		PresentFields:     []string{"gpu_utilization_or_sm_activity", "framebuffer_memory", "clocks", "power", "temperature", "pcie_throughput"},
		UnsupportedFields: []string{"memory_bandwidth", "nvlink_throughput", "reliability_errors"},
		RawEvidenceRef:    "raw/nvml.json",
	}

	metrics, gotCoverage := nvmlMetricsFromSnapshot(snapshot, coverage)

	if !metrics.GPUUtilizationOrSMActivity.HasData() || !metrics.FramebufferMemory.HasData() {
		t.Fatalf("expected NVML-derived metrics to populate canonical fields: %+v", metrics)
	}
	if gotCoverage.RawEvidenceRef != "raw/nvml.json" {
		t.Fatalf("expected coverage raw ref to be preserved, got %q", gotCoverage.RawEvidenceRef)
	}
	if !containsCoverageName(gotCoverage.UnsupportedFields, "memory_bandwidth") {
		t.Fatalf("expected unsupported NVML fields to remain marked, got %+v", gotCoverage)
	}
}

func testWindow(value float64) contracts.MetricWindow {
	point := metricPoint{Timestamp: time.Unix(1700000000, 0).UTC(), Value: value}
	return windowFromPoints([]metricPoint{point})
}
