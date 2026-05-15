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
	memoryUsed, memoryAvail, memoryTotal := nodeMemoryWindows(node)
	swapPressure, swapUsed := nodeSwapWindows(node)
	host := contracts.HostMetrics{
		CPUUtilization:  cpuUtil,
		CPULoad:         windowFromMetric(node, "node_load1"),
		CPULoadAverages: loadWindows(node),
		MemoryUsed:      memoryUsed,
		MemoryAvailable: memoryAvail,
		MemoryTotal:     memoryTotal,
		SwapPressure:    swapPressure,
		SwapUsed:        swapUsed,
		ProcessCPU:      deltaRateWindow(vllm, "process_cpu_seconds_total", 100),
		ProcessMemory:   windowFromMetric(vllm, "process_resident_memory_bytes"),
		DiskIO: contracts.DiskIOMetrics{
			ReadBytes:  deltaRateWindow(node, "node_disk_read_bytes_total", 1),
			WriteBytes: deltaRateWindow(node, "node_disk_written_bytes_total", 1),
		},
		NetworkRX:               deltaRateWindow(node, "node_network_receive_bytes_total", 1),
		NetworkTX:               deltaRateWindow(node, "node_network_transmit_bytes_total", 1),
		KubernetesCPUThrottling: deltaRateWindow(node, "container_cpu_cfs_throttled_seconds_total", 1),
	}
	host.Coverage = hostCoverage(host)
	return host
}

func normalizeGPUMetrics(nvml, dcgm []promcollector.Sample) contracts.GPUTelemetry {
	gpuUtil := firstWindow(
		windowFromMetric(dcgm, "DCGM_FI_DEV_GPU_UTIL"),
		windowFromMetric(nvml, "inferlean_nvml_gpu_utilization_percent"),
	)
	smActive := ratioToPercentWindow(windowFromMetric(dcgm, "DCGM_FI_PROF_SM_ACTIVE"))
	grEngineActive := ratioToPercentWindow(windowFromMetric(dcgm, "DCGM_FI_PROF_GR_ENGINE_ACTIVE"))
	dramActive := ratioToPercentWindow(windowFromMetric(dcgm, "DCGM_FI_PROF_DRAM_ACTIVE"))
	dcgmMemoryUsed := mbToBytesWindow(windowFromMetric(dcgm, "DCGM_FI_DEV_FB_USED"))
	dcgmMemoryFree := mbToBytesWindow(windowFromMetric(dcgm, "DCGM_FI_DEV_FB_FREE"))
	dcgmMemoryReserved := mbToBytesWindow(windowFromMetric(dcgm, "DCGM_FI_DEV_FB_RESERVED"))
	memoryUsed := firstWindow(
		dcgmMemoryUsed,
		mbToBytesWindow(windowFromMetric(nvml, "inferlean_nvml_memory_used_mb")),
	)
	memoryFree := dcgmMemoryFree
	memoryReserved := dcgmMemoryReserved
	memoryTotal := firstWindow(
		mbToBytesWindow(windowFromMetric(dcgm, "DCGM_FI_DEV_FB_TOTAL")),
		derivedTotalMemoryWindow(dcgmMemoryUsed.Samples, dcgmMemoryFree.Samples, dcgmMemoryReserved.Samples),
		mbToBytesWindow(windowFromMetric(nvml, "inferlean_nvml_memory_total_mb")),
	)
	gpu := contracts.GPUTelemetry{
		GPUUtilizationOrSMActivity: firstWindow(gpuUtil, smActive, grEngineActive),
		GPUUtilization:             gpuUtil,
		SMActive:                   smActive,
		SMOccupancy:                ratioToPercentWindow(windowFromMetric(dcgm, "DCGM_FI_PROF_SM_OCCUPANCY")),
		TensorCoreActive:           ratioToPercentWindow(windowFromMetric(dcgm, "DCGM_FI_PROF_PIPE_TENSOR_ACTIVE")),
		DRAMActive:                 dramActive,
		FramebufferMemory:          memoryWindows(memoryUsed, memoryFree, memoryReserved, memoryTotal),
		MemoryBandwidth:            dramActive,
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
		ClockThrottleReasons: windowFromMetric(dcgm, "DCGM_FI_DEV_CLOCK_THROTTLE_REASONS"),
		PCIeThroughput: contracts.ThroughputMetrics{
			RX: firstWindow(
				deltaRateWindow(dcgm, "DCGM_FI_PROF_PCIE_RX_BYTES", 1),
				windowFromMetric(nvml, "inferlean_nvml_pcie_rx_throughput_bytes_per_second"),
			),
			TX: firstWindow(
				deltaRateWindow(dcgm, "DCGM_FI_PROF_PCIE_TX_BYTES", 1),
				windowFromMetric(nvml, "inferlean_nvml_pcie_tx_throughput_bytes_per_second"),
			),
		},
		NVLinkThroughput: contracts.ThroughputMetrics{
			RX: deltaRateWindow(dcgm, "DCGM_FI_PROF_NVLINK_RX_BYTES", 1),
			TX: deltaRateWindow(dcgm, "DCGM_FI_PROF_NVLINK_TX_BYTES", 1),
		},
		PCIeBandwidthCapacity: contracts.ThroughputMetrics{
			RX: windowFromMetric(nvml, "inferlean_nvml_pcie_rx_capacity_bytes_per_second"),
			TX: windowFromMetric(nvml, "inferlean_nvml_pcie_tx_capacity_bytes_per_second"),
		},
		NVLinkBandwidthCapacity: contracts.ThroughputMetrics{
			RX: windowFromMetric(nvml, "inferlean_nvml_nvlink_rx_capacity_bytes_per_second"),
			TX: windowFromMetric(nvml, "inferlean_nvml_nvlink_tx_capacity_bytes_per_second"),
		},
		ReliabilityErrors: contracts.ReliabilityMetrics{
			XID: windowFromMetric(dcgm, "DCGM_FI_DEV_XID_ERRORS"),
			ECC: firstWindow(
				windowFromMetric(dcgm, "DCGM_FI_DEV_ECC_DBE_VOL_TOTAL"),
				windowFromMetric(dcgm, "DCGM_FI_DEV_ECC_SBE_VOL_TOTAL"),
			),
		},
	}
	gpu.Coverage = gpuCoverage(gpu)
	return gpu
}

func normalizeNvidiaSMIMetrics(nvml, dcgm []promcollector.Sample, staticSMI string) contracts.NvidiaSMIMetrics {
	metrics := contracts.NvidiaSMIMetrics{
		GPUUtilization: windowFromMetric(nvml, "inferlean_nvml_gpu_utilization_percent"),
		MemoryUsed:     mbToBytesWindow(windowFromMetric(nvml, "inferlean_nvml_memory_used_mb")),
		MemoryTotal:    mbToBytesWindow(windowFromMetric(nvml, "inferlean_nvml_memory_total_mb")),
		PowerDraw:      windowFromMetric(nvml, "inferlean_nvml_power_draw_watts"),
		PowerLimit:     windowFromMetric(nvml, "inferlean_nvml_power_limit_watts"),
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
		PerformanceState: latestMetricLabelValue(nvml, "inferlean_nvml_performance_state_info", "pstate"),
		ThrottleReasons:  latestMetricLabelValues(nvml, "inferlean_nvml_throttle_reason_active", "reason"),
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

func loadWindows(samples []promcollector.Sample) contracts.LoadMetrics {
	return contracts.LoadMetrics{
		Load1:  windowFromMetric(samples, "node_load1"),
		Load5:  windowFromMetric(samples, "node_load5"),
		Load15: windowFromMetric(samples, "node_load15"),
	}
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

func nodeMemoryWindows(samples []promcollector.Sample) (contracts.MetricWindow, contracts.MetricWindow, contracts.MetricWindow) {
	used := make([]contracts.MetricSample, 0, len(samples))
	available := make([]contracts.MetricSample, 0, len(samples))
	totalPoints := make([]contracts.MetricSample, 0, len(samples))
	for _, sample := range samples {
		total, okTotal := metricValue(sample.Metrics, "node_memory_MemTotal_bytes")
		avail, okAvail := metricValue(sample.Metrics, "node_memory_MemAvailable_bytes")
		if !okTotal || !okAvail {
			continue
		}
		used = append(used, contracts.MetricSample{Timestamp: sample.Timestamp, Value: total - avail})
		available = append(available, contracts.MetricSample{Timestamp: sample.Timestamp, Value: avail})
		totalPoints = append(totalPoints, contracts.MetricSample{Timestamp: sample.Timestamp, Value: total})
	}
	return withSamples(used), withSamples(available), withSamples(totalPoints)
}

func nodeSwapWindows(samples []promcollector.Sample) (contracts.MetricWindow, contracts.MetricWindow) {
	pressure := make([]contracts.MetricSample, 0, len(samples))
	used := make([]contracts.MetricSample, 0, len(samples))
	for _, sample := range samples {
		total, okTotal := metricValue(sample.Metrics, "node_memory_SwapTotal_bytes")
		free, okFree := metricValue(sample.Metrics, "node_memory_SwapFree_bytes")
		if !okTotal || !okFree {
			continue
		}
		if total <= 0 {
			pressure = append(pressure, contracts.MetricSample{Timestamp: sample.Timestamp, Value: 0})
			used = append(used, contracts.MetricSample{Timestamp: sample.Timestamp, Value: 0})
			continue
		}
		usedBytes := total - free
		usedPerc := (usedBytes / total) * 100
		pressure = append(pressure, contracts.MetricSample{Timestamp: sample.Timestamp, Value: usedPerc})
		used = append(used, contracts.MetricSample{Timestamp: sample.Timestamp, Value: usedBytes})
	}
	return withSamples(pressure), withSamples(used)
}

func mbToBytesWindow(window contracts.MetricWindow) contracts.MetricWindow {
	samples := make([]contracts.MetricSample, 0, len(window.Samples))
	for _, sample := range window.Samples {
		samples = append(samples, contracts.MetricSample{Timestamp: sample.Timestamp, Value: sample.Value * 1024 * 1024})
	}
	return withSamples(samples)
}

func ratioToPercentWindow(window contracts.MetricWindow) contracts.MetricWindow {
	samples := make([]contracts.MetricSample, 0, len(window.Samples))
	for _, sample := range window.Samples {
		value := sample.Value
		if value >= 0 && value <= 1 {
			value *= 100
		}
		samples = append(samples, contracts.MetricSample{Timestamp: sample.Timestamp, Value: value})
	}
	return withSamples(samples)
}
