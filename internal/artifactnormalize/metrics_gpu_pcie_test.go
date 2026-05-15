package artifactnormalize

import (
	"testing"
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
)

func TestNormalizeMetricsUsesNVMLPCIeThroughputWhenDCGMProfilingMissing(t *testing.T) {
	now := time.Unix(25, 0).UTC()
	input := Input{
		Observations: ObservationsInput{
			Prometheus: map[string][]promcollector.Sample{
				"nvml_bridge": {
					gpuPCIeThroughputSample(now, 12_000_000, 34_000_000),
				},
			},
		},
	}

	metrics := normalizeMetrics(input)

	if got, want := *metrics.GPU.PCIeThroughput.RX.Latest, 12_000_000.0; got != want {
		t.Fatalf("PCIe RX throughput = %f, want %f", got, want)
	}
	if got, want := *metrics.GPU.PCIeThroughput.TX.Latest, 34_000_000.0; got != want {
		t.Fatalf("PCIe TX throughput = %f, want %f", got, want)
	}
	if !metrics.GPU.Coverage.HasField("pcie_throughput") {
		t.Fatalf("GPU coverage missing pcie_throughput: %+v", metrics.GPU.Coverage)
	}
}

func gpuPCIeThroughputSample(ts time.Time, rx, tx float64) promcollector.Sample {
	return promcollector.Sample{
		Timestamp: ts,
		Metrics: []promcollector.MetricPoint{
			{Name: "inferlean_nvml_pcie_rx_throughput_bytes_per_second", Value: rx},
			{Name: "inferlean_nvml_pcie_tx_throughput_bytes_per_second", Value: tx},
		},
	}
}
