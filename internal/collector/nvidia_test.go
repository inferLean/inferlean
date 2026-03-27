package collector

import "testing"

func TestNvidiaMetricsFromSnapshotMarksMissingProcessMemoryWhenUnavailable(t *testing.T) {
	snapshot := &nvidiaSnapshot{
		GPUs: []nvidiaGPU{{
			Utilization:    "75",
			MemoryUsedMiB:  "40000",
			MemoryTotalMiB: "80000",
			PowerDrawW:     "250",
			TemperatureC:   "70",
			SMClockMHz:     "1200",
			MemClockMHz:    "1800",
		}},
	}

	metrics := nvidiaMetricsFromSnapshot(snapshot, false, "raw/nvidia.txt")

	if !metrics.GPUUtilization.HasData() || !metrics.MemoryUsed.HasData() {
		t.Fatalf("expected nvidia-smi metrics to populate GPU fields: %+v", metrics)
	}
	if !containsCoverageName(metrics.Coverage.MissingFields, "process_gpu_memory") {
		t.Fatalf("expected missing per-process GPU memory coverage, got %+v", metrics.Coverage)
	}
}

func TestNvidiaMetricsFromSnapshotAggregatesProcessMemoryWhenAvailable(t *testing.T) {
	snapshot := &nvidiaSnapshot{
		GPUs: []nvidiaGPU{{
			Utilization:    "75",
			MemoryUsedMiB:  "40000",
			MemoryTotalMiB: "80000",
			PowerDrawW:     "250",
			TemperatureC:   "70",
			SMClockMHz:     "1200",
			MemClockMHz:    "1800",
		}},
		Processes: []nvidiaProcess{
			{GPUMemoryMiB: "1000"},
			{GPUMemoryMiB: "2000"},
		},
	}

	metrics := nvidiaMetricsFromSnapshot(snapshot, true, "raw/nvidia.txt")

	if !metrics.ProcessGPUMemory.HasData() {
		t.Fatalf("expected process GPU memory window, got %+v", metrics)
	}
	if containsCoverageName(metrics.Coverage.MissingFields, "process_gpu_memory") {
		t.Fatalf("did not expect process_gpu_memory to be marked missing: %+v", metrics.Coverage)
	}
}
