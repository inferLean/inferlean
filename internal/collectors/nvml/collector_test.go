package nvml

import (
	"strings"
	"testing"
)

func TestParseSampleIncludesClocks(t *testing.T) {
	sample, ok := parseSample("0, 12, 4749, 6144, 80.5, 55, 1400, 7001, 90, P0, Not Active, 4, 16, 5, 16")
	if !ok {
		t.Fatal("parseSample() ok = false")
	}
	if sample.SMClock != 1400 {
		t.Fatalf("SMClock = %f, want 1400", sample.SMClock)
	}
	if sample.MemoryClock != 7001 {
		t.Fatalf("MemoryClock = %f, want 7001", sample.MemoryClock)
	}
	if sample.PowerLimit == nil || *sample.PowerLimit != 90 {
		t.Fatalf("PowerLimit = %v, want 90", sample.PowerLimit)
	}
	if sample.PState != "P0" {
		t.Fatalf("PState = %q, want P0", sample.PState)
	}
	if got := throttleReasons(sample.Throttle); len(got) != 1 || got[0] != "none" {
		t.Fatalf("throttleReasons = %v, want [none]", got)
	}
	if sample.PCIeRXCap == nil || *sample.PCIeRXCap <= 31_000_000_000 {
		t.Fatalf("PCIeRXCap = %v, want gen4 x16 capacity", sample.PCIeRXCap)
	}
}

func TestParsePCIeThroughputConvertsDmonMBToBytes(t *testing.T) {
	raw := `#gpu, rxpci, txpci
#Idx, MB/s, MB/s
 0, 12, 34
 1, -, N/A`

	got := parsePCIeThroughput(raw)
	if got["0"].RX != 12_000_000 {
		t.Fatalf("GPU 0 RX throughput = %f, want 12000000", got["0"].RX)
	}
	if got["0"].TX != 34_000_000 {
		t.Fatalf("GPU 0 TX throughput = %f, want 34000000", got["0"].TX)
	}
	if _, ok := got["1"]; ok {
		t.Fatalf("GPU 1 throughput = %+v, want omitted unsupported row", got["1"])
	}
}

func TestAppendPromSampleIncludesPCIeThroughput(t *testing.T) {
	rx, tx := 12_000_000.0, 34_000_000.0
	lines := appendPromSample(nil, Sample{
		GPU:    "0",
		PCIeRX: &rx,
		PCIeTX: &tx,
	}, 0)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "inferlean_nvml_pcie_rx_throughput_bytes_per_second{gpu=\"0\"} 12000000.000000") {
		t.Fatalf("prometheus output missing PCIe RX throughput: %s", joined)
	}
	if !strings.Contains(joined, "inferlean_nvml_pcie_tx_throughput_bytes_per_second{gpu=\"0\"} 34000000.000000") {
		t.Fatalf("prometheus output missing PCIe TX throughput: %s", joined)
	}
}

func TestThrottleReasonsDecodesIdleAndPowerMasks(t *testing.T) {
	if got := throttleReasons("0x0000000000000001"); len(got) != 1 || got[0] != "gpu_idle" {
		t.Fatalf("idle throttle reasons = %v, want [gpu_idle]", got)
	}
	if got := throttleReasons("0x0000000000000025"); len(got) != 3 || got[0] != "gpu_idle" || got[1] != "sw_power_cap" || got[2] != "sw_thermal_slowdown" {
		t.Fatalf("combined throttle reasons = %v, want decoded idle/power/thermal reasons", got)
	}
	if got := throttleReasons("0x0000000000000000"); len(got) != 1 || got[0] != "none" {
		t.Fatalf("zero throttle reasons = %v, want [none]", got)
	}
}

func TestParseNVLinkCapacitySumsActiveLinksByGPU(t *testing.T) {
	raw := `GPU 0: NVIDIA H100 (UUID: GPU-0)
	 Link 0: 25.781 GB/s
	 Link 1: 25.781 GB/s
GPU 1: NVIDIA H100 (UUID: GPU-1)
	 Link 0: Disabled
	 Link 1: 25 GB/s`

	got := parseNVLinkCapacity(raw)
	if got["0"] <= 51_000_000_000 || got["0"] >= 52_000_000_000 {
		t.Fatalf("GPU 0 capacity = %f, want summed active links", got["0"])
	}
	if got["1"] != 25_000_000_000 {
		t.Fatalf("GPU 1 capacity = %f, want 25GB/s", got["1"])
	}
}
