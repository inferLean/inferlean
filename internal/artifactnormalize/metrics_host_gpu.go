package artifactnormalize

import (
	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func normalizeMetrics(input Input) contracts.Metrics {
	prom := input.Observations.Prometheus
	vllm := prom["vllm"]
	node := prom["node_exporter"]
	nvml := prom["nvml_bridge"]
	return contracts.Metrics{
		VLLM:      normalizeVLLMMetrics(vllm),
		Host:      normalizeHostMetrics(node, vllm),
		GPU:       normalizeGPUMetrics(nvml),
		NvidiaSmi: normalizeNvidiaSMIMetrics(nvml),
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

func normalizeGPUMetrics(samples []promcollector.Sample) contracts.GPUTelemetry {
	gpuUtil := windowFromMetric(samples, "inferlean_nvml_gpu_utilization_percent")
	memoryUsed := mbToBytesWindow(windowFromMetric(samples, "inferlean_nvml_memory_used_mb"))
	memoryTotal := mbToBytesWindow(windowFromMetric(samples, "inferlean_nvml_memory_total_mb"))
	gpu := contracts.GPUTelemetry{
		GPUUtilizationOrSMActivity: gpuUtil,
		FramebufferMemory:          memoryWindows(memoryUsed, memoryTotal),
		MemoryBandwidth:            contracts.MetricWindow{},
		Clocks:                     contracts.ClockMetrics{},
		Power:                      windowFromMetric(samples, "inferlean_nvml_power_draw_watts"),
		Temperature:                windowFromMetric(samples, "inferlean_nvml_temperature_celsius"),
		PCIeThroughput:             contracts.ThroughputMetrics{},
		NVLinkThroughput:           contracts.ThroughputMetrics{},
		ReliabilityErrors:          contracts.ReliabilityMetrics{},
	}
	gpu.Coverage = gpuCoverage(gpu)
	return gpu
}

func normalizeNvidiaSMIMetrics(samples []promcollector.Sample) contracts.NvidiaSMIMetrics {
	metrics := contracts.NvidiaSMIMetrics{
		GPUUtilization:   windowFromMetric(samples, "inferlean_nvml_gpu_utilization_percent"),
		MemoryUsed:       mbToBytesWindow(windowFromMetric(samples, "inferlean_nvml_memory_used_mb")),
		MemoryTotal:      mbToBytesWindow(windowFromMetric(samples, "inferlean_nvml_memory_total_mb")),
		PowerDraw:        windowFromMetric(samples, "inferlean_nvml_power_draw_watts"),
		Temperature:      windowFromMetric(samples, "inferlean_nvml_temperature_celsius"),
		SMClock:          contracts.MetricWindow{},
		MemClock:         contracts.MetricWindow{},
		ProcessGPUMemory: contracts.MetricWindow{},
	}
	metrics.Coverage = nvidiaCoverage(metrics)
	return metrics
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

func gpuCoverage(metrics contracts.GPUTelemetry) contracts.SourceCoverage {
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
	return newCoverage(present, gpuRequiredFields())
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
