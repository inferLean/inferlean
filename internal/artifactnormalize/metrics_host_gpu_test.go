package artifactnormalize

import (
	"testing"
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/types"
)

func TestNormalizeMetricsUsesDCGMClocksAndUnsupportedProfiling(t *testing.T) {
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
	if !contains(metrics.GPU.Coverage.UnsupportedFields, "memory_bandwidth") {
		t.Fatalf("GPU unsupported fields = %v, want memory_bandwidth", metrics.GPU.Coverage.UnsupportedFields)
	}
	if contains(metrics.GPU.Coverage.MissingFields, "memory_bandwidth") {
		t.Fatalf("GPU missing fields = %v, did not expect memory_bandwidth", metrics.GPU.Coverage.MissingFields)
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
	if !contains(metrics.GPU.Coverage.UnsupportedFields, "memory_bandwidth") {
		t.Fatalf("GPU unsupported fields = %v, want memory_bandwidth", metrics.GPU.Coverage.UnsupportedFields)
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
