package artifactnormalize

import (
	"strconv"
	"strings"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func normalizeMetrics(input Input) contracts.Metrics {
	prom := input.Observations.Prometheus
	vllm := prom["vllm"]
	node := prom["node_exporter"]
	nvml := prom["nvml_bridge"]
	dcgm := prom["dcgm_exporter"]
	return contracts.Metrics{
		VLLM:      normalizeVLLMMetrics(vllm),
		Host:      normalizeHostMetrics(node, vllm),
		GPU:       normalizeGPUMetrics(nvml, dcgm),
		NvidiaSmi: normalizeNvidiaSMIMetrics(nvml, dcgm, input.Configurations.NvidiaSMIStaticText),
	}
}

func normalizeHostMetrics(node, vllm []promcollector.Sample) contracts.HostMetrics {
	cpuUtil := nodeCPUUtilization(node)
	memoryUsed, memoryAvail := nodeMemoryWindows(node)
	swap := nodeSwapPressure(node)
	host := contracts.HostMetrics{
		CPUUtilization:  cpuUtil,
		CPULoad:         windowFromMetric(node, "node_load1"),
		MemoryUsed:      memoryUsed,
		MemoryAvailable: memoryAvail,
		SwapPressure:    swap,
		ProcessCPU:      deltaRateWindow(vllm, "process_cpu_seconds_total", 100),
		ProcessMemory:   windowFromMetric(vllm, "process_resident_memory_bytes"),
		NetworkRX:       deltaRateWindow(node, "node_network_receive_bytes_total", 1),
		NetworkTX:       deltaRateWindow(node, "node_network_transmit_bytes_total", 1),
	}
	host.Coverage = hostCoverage(host)
	return host
}

func normalizeGPUMetrics(nvml, dcgm []promcollector.Sample) contracts.GPUTelemetry {
	gpuUtil := firstWindow(
		windowFromMetric(dcgm, "DCGM_FI_PROF_GR_ENGINE_ACTIVE"),
		windowFromMetric(dcgm, "DCGM_FI_DEV_GPU_UTIL"),
		windowFromMetric(nvml, "inferlean_nvml_gpu_utilization_percent"),
	)
	memoryUsed := firstWindow(
		mbToBytesWindow(windowFromMetric(nvml, "inferlean_nvml_memory_used_mb")),
		mbToBytesWindow(windowFromMetric(dcgm, "DCGM_FI_DEV_FB_USED")),
	)
	memoryTotal := mbToBytesWindow(windowFromMetric(nvml, "inferlean_nvml_memory_total_mb"))
	gpu := contracts.GPUTelemetry{
		GPUUtilizationOrSMActivity: gpuUtil,
		FramebufferMemory:          memoryWindows(memoryUsed, memoryTotal),
		MemoryBandwidth:            windowFromMetric(dcgm, "DCGM_FI_PROF_DRAM_ACTIVE"),
		Clocks: contracts.ClockMetrics{
			SM:     windowFromMetric(dcgm, "DCGM_FI_DEV_SM_CLOCK"),
			Memory: windowFromMetric(dcgm, "DCGM_FI_DEV_MEM_CLOCK"),
		},
		Power: firstWindow(
			windowFromMetric(dcgm, "DCGM_FI_DEV_POWER_USAGE"),
			windowFromMetric(nvml, "inferlean_nvml_power_draw_watts"),
		),
		Temperature: firstWindow(
			windowFromMetric(dcgm, "DCGM_FI_DEV_GPU_TEMP"),
			windowFromMetric(nvml, "inferlean_nvml_temperature_celsius"),
		),
		PCIeThroughput: contracts.ThroughputMetrics{
			RX: deltaRateWindow(dcgm, "DCGM_FI_PROF_PCIE_RX_BYTES", 1),
			TX: deltaRateWindow(dcgm, "DCGM_FI_PROF_PCIE_TX_BYTES", 1),
		},
		NVLinkThroughput: contracts.ThroughputMetrics{
			RX: deltaRateWindow(dcgm, "DCGM_FI_PROF_NVLINK_RX_BYTES", 1),
			TX: deltaRateWindow(dcgm, "DCGM_FI_PROF_NVLINK_TX_BYTES", 1),
		},
		ReliabilityErrors: contracts.ReliabilityMetrics{
			XID: windowFromMetric(dcgm, "DCGM_FI_DEV_XID_ERRORS"),
			ECC: firstWindow(
				windowFromMetric(dcgm, "DCGM_FI_DEV_ECC_DBE_VOL_TOTAL"),
				windowFromMetric(dcgm, "DCGM_FI_DEV_ECC_SBE_VOL_TOTAL"),
			),
		},
	}
	gpu.Coverage = gpuCoverage(gpu, len(dcgm) > 0)
	return gpu
}

func normalizeNvidiaSMIMetrics(nvml, dcgm []promcollector.Sample, staticSMI string) contracts.NvidiaSMIMetrics {
	metrics := contracts.NvidiaSMIMetrics{
		GPUUtilization: windowFromMetric(nvml, "inferlean_nvml_gpu_utilization_percent"),
		MemoryUsed:     mbToBytesWindow(windowFromMetric(nvml, "inferlean_nvml_memory_used_mb")),
		MemoryTotal:    mbToBytesWindow(windowFromMetric(nvml, "inferlean_nvml_memory_total_mb")),
		PowerDraw:      windowFromMetric(nvml, "inferlean_nvml_power_draw_watts"),
		Temperature:    windowFromMetric(nvml, "inferlean_nvml_temperature_celsius"),
		SMClock: firstWindow(
			windowFromMetric(nvml, "inferlean_nvml_sm_clock_mhz"),
			windowFromMetric(dcgm, "DCGM_FI_DEV_SM_CLOCK"),
		),
		MemClock: firstWindow(
			windowFromMetric(nvml, "inferlean_nvml_memory_clock_mhz"),
			windowFromMetric(dcgm, "DCGM_FI_DEV_MEM_CLOCK"),
		),
		ProcessGPUMemory: processGPUMemoryFromNvidiaSMI(staticSMI),
	}
	metrics.Coverage = nvidiaCoverage(metrics)
	return metrics
}

func firstWindow(windows ...contracts.MetricWindow) contracts.MetricWindow {
	for _, window := range windows {
		if window.HasData() {
			return window
		}
	}
	return contracts.MetricWindow{}
}

func scalarWindow(value float64) contracts.MetricWindow {
	return contracts.MetricWindow{
		Latest: floatPtr(value),
		Min:    floatPtr(value),
		Max:    floatPtr(value),
		Avg:    floatPtr(value),
	}
}

func processGPUMemoryFromNvidiaSMI(raw string) contracts.MetricWindow {
	totalMiB := 0.0
	for _, line := range strings.Split(raw, "\n") {
		normalized := strings.ToLower(line)
		if !strings.Contains(line, "MiB") || !strings.Contains(normalized, "vllm") {
			continue
		}
		fields := strings.Fields(strings.ReplaceAll(line, "|", " "))
		for _, field := range fields {
			if !strings.HasSuffix(field, "MiB") {
				continue
			}
			value := strings.TrimSuffix(field, "MiB")
			parsed, err := strconv.ParseFloat(value, 64)
			if err == nil {
				totalMiB += parsed
			}
		}
	}
	if totalMiB <= 0 {
		return contracts.MetricWindow{}
	}
	return scalarWindow(totalMiB * 1024 * 1024)
}

func nodeCPUUtilization(samples []promcollector.Sample) contracts.MetricWindow {
	points := make([]contracts.MetricSample, 0, len(samples))
	for idx := 1; idx < len(samples); idx++ {
		idleCurrent, okIdleCurrent := metricValueWithLabel(samples[idx].Metrics, "node_cpu_seconds_total", "mode", "idle")
		idlePrev, okIdlePrev := metricValueWithLabel(samples[idx-1].Metrics, "node_cpu_seconds_total", "mode", "idle")
		totalCurrent, okTotalCurrent := metricValue(samples[idx].Metrics, "node_cpu_seconds_total")
		totalPrev, okTotalPrev := metricValue(samples[idx-1].Metrics, "node_cpu_seconds_total")
		if !(okIdleCurrent && okIdlePrev && okTotalCurrent && okTotalPrev) {
			continue
		}
		deltaTotal := totalCurrent - totalPrev
		deltaIdle := idleCurrent - idlePrev
		if deltaTotal <= 0 {
			continue
		}
		utilization := 100 * (1 - (deltaIdle / deltaTotal))
		if utilization < 0 {
			utilization = 0
		}
		if utilization > 100 {
			utilization = 100
		}
		points = append(points, contracts.MetricSample{Timestamp: samples[idx].Timestamp, Value: utilization})
	}
	return withSamples(points)
}

func nodeMemoryWindows(samples []promcollector.Sample) (contracts.MetricWindow, contracts.MetricWindow) {
	used := make([]contracts.MetricSample, 0, len(samples))
	available := make([]contracts.MetricSample, 0, len(samples))
	for _, sample := range samples {
		total, okTotal := metricValue(sample.Metrics, "node_memory_MemTotal_bytes")
		avail, okAvail := metricValue(sample.Metrics, "node_memory_MemAvailable_bytes")
		if !okTotal || !okAvail {
			continue
		}
		used = append(used, contracts.MetricSample{Timestamp: sample.Timestamp, Value: total - avail})
		available = append(available, contracts.MetricSample{Timestamp: sample.Timestamp, Value: avail})
	}
	return withSamples(used), withSamples(available)
}

func nodeSwapPressure(samples []promcollector.Sample) contracts.MetricWindow {
	points := make([]contracts.MetricSample, 0, len(samples))
	for _, sample := range samples {
		total, okTotal := metricValue(sample.Metrics, "node_memory_SwapTotal_bytes")
		free, okFree := metricValue(sample.Metrics, "node_memory_SwapFree_bytes")
		if !okTotal || !okFree || total <= 0 {
			continue
		}
		usedPerc := ((total - free) / total) * 100
		points = append(points, contracts.MetricSample{Timestamp: sample.Timestamp, Value: usedPerc})
	}
	return withSamples(points)
}

func mbToBytesWindow(window contracts.MetricWindow) contracts.MetricWindow {
	samples := make([]contracts.MetricSample, 0, len(window.Samples))
	for _, sample := range window.Samples {
		samples = append(samples, contracts.MetricSample{Timestamp: sample.Timestamp, Value: sample.Value * 1024 * 1024})
	}
	return withSamples(samples)
}

func hostCoverage(metrics contracts.HostMetrics) contracts.SourceCoverage {
	present := map[string]bool{}
	appendPresent(present, "cpu_utilization", metrics.CPUUtilization.HasData())
	appendPresent(present, "cpu_load", metrics.CPULoad.HasData())
	appendPresent(present, "memory_used", metrics.MemoryUsed.HasData())
	appendPresent(present, "memory_available", metrics.MemoryAvailable.HasData())
	appendPresent(present, "swap_pressure", metrics.SwapPressure.HasData())
	appendPresent(present, "process_cpu", metrics.ProcessCPU.HasData())
	appendPresent(present, "process_memory", metrics.ProcessMemory.HasData())
	appendPresent(present, "network_rx", metrics.NetworkRX.HasData())
	appendPresent(present, "network_tx", metrics.NetworkTX.HasData())
	return newCoverage(present, hostRequiredFields())
}

func gpuCoverage(metrics contracts.GPUTelemetry, dcgmAvailable bool) contracts.SourceCoverage {
	present := map[string]bool{}
	appendPresent(present, "gpu_utilization_or_sm_activity", metrics.GPUUtilizationOrSMActivity.HasData())
	appendPresent(present, "framebuffer_memory", metrics.FramebufferMemory.HasData())
	appendPresent(present, "memory_bandwidth", metrics.MemoryBandwidth.HasData())
	appendPresent(present, "clocks", metrics.Clocks.HasData())
	appendPresent(present, "power", metrics.Power.HasData())
	appendPresent(present, "temperature", metrics.Temperature.HasData())
	appendPresent(present, "pcie_throughput", metrics.PCIeThroughput.HasData())
	appendPresent(present, "nvlink_throughput", metrics.NVLinkThroughput.HasData())
	appendPresent(present, "reliability_errors", metrics.ReliabilityErrors.HasData())
	coverage := newCoverage(present, gpuRequiredFields())
	if dcgmAvailable {
		coverage = markUnsupported(
			coverage,
			"memory_bandwidth",
			"pcie_throughput",
			"nvlink_throughput",
			"reliability_errors",
		)
	}
	return coverage
}

func nvidiaCoverage(metrics contracts.NvidiaSMIMetrics) contracts.SourceCoverage {
	present := map[string]bool{}
	appendPresent(present, "gpu_utilization", metrics.GPUUtilization.HasData())
	appendPresent(present, "memory_used", metrics.MemoryUsed.HasData())
	appendPresent(present, "memory_total", metrics.MemoryTotal.HasData())
	appendPresent(present, "power_draw", metrics.PowerDraw.HasData())
	appendPresent(present, "temperature", metrics.Temperature.HasData())
	appendPresent(present, "sm_clock", metrics.SMClock.HasData())
	appendPresent(present, "mem_clock", metrics.MemClock.HasData())
	appendPresent(present, "process_gpu_memory", metrics.ProcessGPUMemory.HasData())
	return newCoverage(present, nvidiaRequiredFields())
}

func hostRequiredFields() []string {
	return []string{
		"cpu_utilization",
		"cpu_load",
		"memory_used",
		"memory_available",
		"swap_pressure",
		"process_cpu",
		"process_memory",
		"network_rx",
		"network_tx",
	}
}

func gpuRequiredFields() []string {
	return []string{
		"gpu_utilization_or_sm_activity",
		"framebuffer_memory",
		"memory_bandwidth",
		"clocks",
		"power",
		"temperature",
		"pcie_throughput",
		"nvlink_throughput",
		"reliability_errors",
	}
}

func nvidiaRequiredFields() []string {
	return []string{
		"gpu_utilization",
		"memory_used",
		"memory_total",
		"power_draw",
		"temperature",
		"sm_clock",
		"mem_clock",
		"process_gpu_memory",
	}
}
