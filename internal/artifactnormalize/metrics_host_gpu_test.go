package artifactnormalize

import (
	"testing"
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/types"
)

func TestNormalizeMetricsUsesDCGMClocksAndMarksMissingProfiling(t *testing.T) {
	input := Input{
		Configurations: testStaticNvidiaSMIConfig(),
		Observations: ObservationsInput{
			Prometheus: map[string][]promcollector.Sample{
				"dcgm_exporter": {
					{
						Timestamp: time.Unix(10, 0).UTC(),
						Metrics: []promcollector.MetricPoint{
							{Name: "DCGM_FI_DEV_SM_CLOCK", Value: 210},
							{Name: "DCGM_FI_DEV_MEM_CLOCK", Value: 405},
							{Name: "DCGM_FI_DEV_FB_FREE", Value: 70},
							{Name: "DCGM_FI_DEV_FB_RESERVED", Value: 5},
						},
					},
				},
				"nvml_bridge": {
					{
						Timestamp: time.Unix(10, 0).UTC(),
						Metrics: []promcollector.MetricPoint{
							{Name: "inferlean_nvml_gpu_utilization_percent", Value: 1},
							{Name: "inferlean_nvml_memory_used_mb", Value: 100},
							{Name: "inferlean_nvml_memory_total_mb", Value: 200},
							{Name: "inferlean_nvml_power_draw_watts", Value: 50},
							{Name: "inferlean_nvml_power_limit_watts", Value: 80},
							{Name: "inferlean_nvml_temperature_celsius", Value: 60},
							{Name: "inferlean_nvml_performance_state_info", Labels: map[string]string{"pstate": "P0"}, Value: 1},
							{Name: "inferlean_nvml_throttle_reason_active", Labels: map[string]string{"reason": "none"}, Value: 1},
						},
					},
				},
			},
		},
	}

	metrics := normalizeMetrics(input)

	if got, want := *metrics.GPU.Clocks.SM.Latest, 210.0; got != want {
		t.Fatalf("GPU SM clock = %f, want %f", got, want)
	}
	if got, want := *metrics.NvidiaSmi.MemClock.Latest, 405.0; got != want {
		t.Fatalf("nvidia_smi memory clock = %f, want %f", got, want)
	}
	if !contains(metrics.GPU.Coverage.MissingFields, "memory_bandwidth") {
		t.Fatalf("GPU missing fields = %v, want memory_bandwidth", metrics.GPU.Coverage.MissingFields)
	}
	if contains(metrics.GPU.Coverage.UnsupportedFields, "memory_bandwidth") {
		t.Fatalf("GPU unsupported fields = %v, did not expect memory_bandwidth", metrics.GPU.Coverage.UnsupportedFields)
	}
	if got, want := *metrics.NvidiaSmi.ProcessGPUMemory.Latest, 4726.0*1024*1024; got != want {
		t.Fatalf("process GPU memory = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.FramebufferMemory.Free.Latest, 70.0*1024*1024; got != want {
		t.Fatalf("GPU free memory = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.FramebufferMemory.Reserved.Latest, 5.0*1024*1024; got != want {
		t.Fatalf("GPU reserved memory = %f, want %f", got, want)
	}
	if got, want := *metrics.NvidiaSmi.PowerLimit.Latest, 80.0; got != want {
		t.Fatalf("power limit = %f, want %f", got, want)
	}
	if got, want := metrics.NvidiaSmi.PerformanceState, "P0"; got != want {
		t.Fatalf("performance state = %q, want %q", got, want)
	}
	if got, want := metrics.NvidiaSmi.ThrottleReasons[0], "none"; got != want {
		t.Fatalf("throttle reason = %q, want %q", got, want)
	}
}

func TestNormalizeMetricsFallsBackToNVMLWhenDCGMUnavailable(t *testing.T) {
	input := Input{
		Observations: ObservationsInput{
			Prometheus: map[string][]promcollector.Sample{
				"nvml_bridge": {
					{
						Timestamp: time.Unix(10, 0).UTC(),
						Metrics: []promcollector.MetricPoint{
							{Name: "inferlean_nvml_gpu_utilization_percent", Value: 91},
							{Name: "inferlean_nvml_memory_used_mb", Value: 120},
							{Name: "inferlean_nvml_memory_total_mb", Value: 200},
							{Name: "inferlean_nvml_power_draw_watts", Value: 50},
							{Name: "inferlean_nvml_temperature_celsius", Value: 60},
						},
					},
				},
			},
		},
	}

	metrics := normalizeMetrics(input)

	if got, want := *metrics.GPU.GPUUtilizationOrSMActivity.Latest, 91.0; got != want {
		t.Fatalf("GPU utilization = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.FramebufferMemory.Used.Latest, 120.0*1024*1024; got != want {
		t.Fatalf("GPU used memory = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.FramebufferMemory.Free.Latest, 80.0*1024*1024; got != want {
		t.Fatalf("GPU derived free memory = %f, want %f", got, want)
	}
	if !contains(metrics.GPU.Coverage.MissingFields, "memory_bandwidth") {
		t.Fatalf("GPU missing fields = %v, want memory_bandwidth", metrics.GPU.Coverage.MissingFields)
	}
}

func TestNormalizeMetricsCapturesCriticalHostAndGPUMetrics(t *testing.T) {
	now := time.Unix(20, 0).UTC()
	input := Input{
		Observations: ObservationsInput{
			Prometheus: map[string][]promcollector.Sample{
				"node_exporter": {
					hostMetricSample(now, 100, 1000, 500, 1000),
					hostMetricSample(now.Add(2*time.Second), 101, 1200, 900, 1003),
				},
				"dcgm_exporter": {
					gpuMetricSample(now, 100, 200, 300, 400),
					gpuMetricSample(now.Add(2*time.Second), 120, 230, 360, 480),
				},
				"nvml_bridge": {
					gpuCapacitySample(now, 32_000_000_000, 50_000_000_000),
					gpuCapacitySample(now.Add(2*time.Second), 32_000_000_000, 50_000_000_000),
				},
			},
		},
	}

	metrics := normalizeMetrics(input)

	if got, want := *metrics.Host.CPULoadAverages.Load5.Latest, 2.0; got != want {
		t.Fatalf("host load5 = %f, want %f", got, want)
	}
	if got, want := *metrics.Host.MemoryTotal.Latest, 16.0; got != want {
		t.Fatalf("host memory total = %f, want %f", got, want)
	}
	if got, want := *metrics.Host.SwapUsed.Latest, 4.0; got != want {
		t.Fatalf("host swap used = %f, want %f", got, want)
	}
	if got, want := *metrics.Host.DiskIO.ReadBytes.Avg, 100.0; got != want {
		t.Fatalf("host disk read bytes/sec = %f, want %f", got, want)
	}
	if got, want := *metrics.Host.KubernetesCPUThrottling.Avg, 1.5; got != want {
		t.Fatalf("kubernetes cpu throttling seconds/sec = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.GPUUtilization.Latest, 82.0; got != want {
		t.Fatalf("GPU utilization = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.SMActive.Latest, 80.0; got != want {
		t.Fatalf("SM active = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.TensorCoreActive.Latest, 40.0; got != want {
		t.Fatalf("tensor active = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.DRAMActive.Latest, 70.0; got != want {
		t.Fatalf("DRAM active = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.PCIeThroughput.RX.Avg, 10.0; got != want {
		t.Fatalf("PCIe RX bytes/sec = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.NVLinkThroughput.TX.Avg, 40.0; got != want {
		t.Fatalf("NVLink TX bytes/sec = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.PCIeBandwidthCapacity.RX.Latest, 32_000_000_000.0; got != want {
		t.Fatalf("PCIe RX capacity = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.NVLinkBandwidthCapacity.TX.Latest, 50_000_000_000.0; got != want {
		t.Fatalf("NVLink TX capacity = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.FramebufferMemory.Total.Latest, 170.0*1024*1024; got != want {
		t.Fatalf("GPU derived total memory = %f, want %f", got, want)
	}
	if !metrics.GPU.Coverage.HasField("tensor_core_active") {
		t.Fatalf("GPU coverage missing tensor_core_active: %+v", metrics.GPU.Coverage)
	}
}

func TestNormalizeHostMetricsDerivesZeroSwapWhenNoSwapConfigured(t *testing.T) {
	now := time.Unix(30, 0).UTC()
	metrics := normalizeHostMetrics([]promcollector.Sample{
		{
			Timestamp: now,
			Metrics: []promcollector.MetricPoint{
				{Name: "node_memory_SwapTotal_bytes", Value: 0},
				{Name: "node_memory_SwapFree_bytes", Value: 0},
			},
		},
	}, nil)

	if got, want := *metrics.SwapPressure.Latest, 0.0; got != want {
		t.Fatalf("swap pressure = %f, want %f", got, want)
	}
	if got, want := *metrics.SwapUsed.Latest, 0.0; got != want {
		t.Fatalf("swap used = %f, want %f", got, want)
	}
	if !metrics.Coverage.HasField("swap_used") {
		t.Fatalf("coverage missing swap_used: %+v", metrics.Coverage)
	}
}

func hostMetricSample(ts time.Time, idleCPU, diskRead, networkRX, throttled float64) promcollector.Sample {
	return promcollector.Sample{
		Timestamp: ts,
		Metrics: []promcollector.MetricPoint{
			{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "idle"}, Value: idleCPU},
			{Name: "node_cpu_seconds_total", Labels: map[string]string{"cpu": "0", "mode": "user"}, Value: idleCPU / 2},
			{Name: "node_load1", Value: 1},
			{Name: "node_load5", Value: 2},
			{Name: "node_load15", Value: 3},
			{Name: "node_memory_MemTotal_bytes", Value: 16},
			{Name: "node_memory_MemAvailable_bytes", Value: 10},
			{Name: "node_memory_SwapTotal_bytes", Value: 8},
			{Name: "node_memory_SwapFree_bytes", Value: 4},
			{Name: "node_disk_read_bytes_total", Labels: map[string]string{"device": "nvme0n1"}, Value: diskRead},
			{Name: "node_disk_written_bytes_total", Labels: map[string]string{"device": "nvme0n1"}, Value: diskRead + 100},
			{Name: "node_network_receive_bytes_total", Labels: map[string]string{"device": "eth0"}, Value: networkRX},
			{Name: "node_network_transmit_bytes_total", Labels: map[string]string{"device": "eth0"}, Value: networkRX + 100},
			{Name: "container_cpu_cfs_throttled_seconds_total", Labels: map[string]string{"pod": "vllm"}, Value: throttled},
		},
	}
}

func gpuMetricSample(ts time.Time, pcieRX, pcieTX, nvlinkRX, nvlinkTX float64) promcollector.Sample {
	return promcollector.Sample{
		Timestamp: ts,
		Metrics: []promcollector.MetricPoint{
			{Name: "DCGM_FI_DEV_GPU_UTIL", Value: 82},
			{Name: "DCGM_FI_DEV_FB_USED", Value: 100},
			{Name: "DCGM_FI_DEV_FB_FREE", Value: 70},
			{Name: "DCGM_FI_PROF_SM_ACTIVE", Value: 0.8},
			{Name: "DCGM_FI_PROF_SM_OCCUPANCY", Value: 0.6},
			{Name: "DCGM_FI_PROF_PIPE_TENSOR_ACTIVE", Value: 0.4},
			{Name: "DCGM_FI_PROF_DRAM_ACTIVE", Value: 0.7},
			{Name: "DCGM_FI_PROF_PCIE_RX_BYTES", Value: pcieRX},
			{Name: "DCGM_FI_PROF_PCIE_TX_BYTES", Value: pcieTX},
			{Name: "DCGM_FI_PROF_NVLINK_RX_BYTES", Value: nvlinkRX},
			{Name: "DCGM_FI_PROF_NVLINK_TX_BYTES", Value: nvlinkTX},
			{Name: "DCGM_FI_DEV_POWER_USAGE", Value: 250},
			{Name: "DCGM_FI_DEV_GPU_TEMP", Value: 65},
			{Name: "DCGM_FI_DEV_SM_CLOCK", Value: 1200},
			{Name: "DCGM_FI_DEV_MEM_CLOCK", Value: 1800},
			{Name: "DCGM_FI_DEV_CLOCK_THROTTLE_REASONS", Value: 4},
		},
	}
}

func gpuCapacitySample(ts time.Time, pcie, nvlink float64) promcollector.Sample {
	return promcollector.Sample{
		Timestamp: ts,
		Metrics: []promcollector.MetricPoint{
			{Name: "inferlean_nvml_pcie_rx_capacity_bytes_per_second", Value: pcie},
			{Name: "inferlean_nvml_pcie_tx_capacity_bytes_per_second", Value: pcie},
			{Name: "inferlean_nvml_nvlink_rx_capacity_bytes_per_second", Value: nvlink},
			{Name: "inferlean_nvml_nvlink_tx_capacity_bytes_per_second", Value: nvlink},
		},
	}
}

func testStaticNvidiaSMIConfig() types.Configurations {
	return types.Configurations{
		NvidiaSMIStaticText: `+-----------------------------------------------------------------------------------------+
| Processes:                                                                              |
|  GPU   GI   CI              PID   Type   Process name                        GPU Memory |
|=========================================================================================|
|    0   N/A  N/A          530596      C   VLLM::EngineCore                       4726MiB |
+-----------------------------------------------------------------------------------------+`,
	}
}
