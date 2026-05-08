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
							{Name: "inferlean_nvml_temperature_celsius", Value: 60},
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
