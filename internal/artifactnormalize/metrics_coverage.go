package artifactnormalize

import "github.com/inferLean/inferlean-main/cli/pkg/contracts"

func hostCoverage(metrics contracts.HostMetrics) contracts.SourceCoverage {
	present := map[string]bool{}
	appendPresent(present, "cpu_utilization", metrics.CPUUtilization.HasData())
	appendPresent(present, "cpu_load", metrics.CPULoad.HasData())
	appendPresent(present, "cpu_load_averages", metrics.CPULoadAverages.HasData())
	appendPresent(present, "memory_used", metrics.MemoryUsed.HasData())
	appendPresent(present, "memory_available", metrics.MemoryAvailable.HasData())
	appendPresent(present, "memory_total", metrics.MemoryTotal.HasData())
	appendPresent(present, "swap_pressure", metrics.SwapPressure.HasData())
	appendPresent(present, "swap_used", metrics.SwapUsed.HasData())
	appendPresent(present, "process_cpu", metrics.ProcessCPU.HasData())
	appendPresent(present, "process_memory", metrics.ProcessMemory.HasData())
	appendPresent(present, "disk_io", metrics.DiskIO.HasData())
	appendPresent(present, "network_rx", metrics.NetworkRX.HasData())
	appendPresent(present, "network_tx", metrics.NetworkTX.HasData())
	appendPresent(present, "kubernetes_cpu_throttling", metrics.KubernetesCPUThrottling.HasData())
	return newCoverage(present, hostRequiredFields())
}

func gpuCoverage(metrics contracts.GPUTelemetry) contracts.SourceCoverage {
	present := map[string]bool{}
	appendPresent(present, "gpu_utilization_or_sm_activity", metrics.GPUUtilizationOrSMActivity.HasData())
	appendPresent(present, "gpu_utilization", metrics.GPUUtilization.HasData())
	appendPresent(present, "sm_active", metrics.SMActive.HasData())
	appendPresent(present, "sm_occupancy", metrics.SMOccupancy.HasData())
	appendPresent(present, "tensor_core_active", metrics.TensorCoreActive.HasData())
	appendPresent(present, "dram_active", metrics.DRAMActive.HasData())
	appendPresent(present, "framebuffer_memory", metrics.FramebufferMemory.HasData())
	appendPresent(present, "memory_bandwidth", metrics.MemoryBandwidth.HasData())
	appendPresent(present, "clocks", metrics.Clocks.HasData())
	appendPresent(present, "power", metrics.Power.HasData())
	appendPresent(present, "temperature", metrics.Temperature.HasData())
	appendPresent(present, "clock_throttle_reasons", metrics.ClockThrottleReasons.HasData())
	appendPresent(present, "pcie_throughput", metrics.PCIeThroughput.HasData())
	appendPresent(present, "nvlink_throughput", metrics.NVLinkThroughput.HasData())
	appendPresent(present, "pcie_bandwidth_capacity", metrics.PCIeBandwidthCapacity.HasData())
	appendPresent(present, "nvlink_bandwidth_capacity", metrics.NVLinkBandwidthCapacity.HasData())
	appendPresent(present, "reliability_errors", metrics.ReliabilityErrors.HasData())
	return newCoverage(present, gpuRequiredFields())
}

func nvidiaCoverage(metrics contracts.NvidiaSMIMetrics) contracts.SourceCoverage {
	present := map[string]bool{}
	appendPresent(present, "gpu_utilization", metrics.GPUUtilization.HasData())
	appendPresent(present, "memory_used", metrics.MemoryUsed.HasData())
	appendPresent(present, "memory_total", metrics.MemoryTotal.HasData())
	appendPresent(present, "power_draw", metrics.PowerDraw.HasData())
	appendPresent(present, "power_limit", metrics.PowerLimit.HasData())
	appendPresent(present, "temperature", metrics.Temperature.HasData())
	appendPresent(present, "sm_clock", metrics.SMClock.HasData())
	appendPresent(present, "mem_clock", metrics.MemClock.HasData())
	appendPresent(present, "process_gpu_memory", metrics.ProcessGPUMemory.HasData())
	appendPresent(present, "performance_state", metrics.PerformanceState != "")
	appendPresent(present, "throttle_reasons", len(metrics.ThrottleReasons) > 0)
	return newCoverage(present, nvidiaRequiredFields())
}

func hostRequiredFields() []string {
	return []string{
		"cpu_utilization",
		"cpu_load",
		"cpu_load_averages",
		"memory_used",
		"memory_available",
		"memory_total",
		"swap_pressure",
		"swap_used",
		"process_cpu",
		"process_memory",
		"disk_io",
		"network_rx",
		"network_tx",
		"kubernetes_cpu_throttling",
	}
}

func gpuRequiredFields() []string {
	return []string{
		"gpu_utilization_or_sm_activity",
		"gpu_utilization",
		"sm_active",
		"sm_occupancy",
		"tensor_core_active",
		"dram_active",
		"framebuffer_memory",
		"memory_bandwidth",
		"clocks",
		"power",
		"temperature",
		"clock_throttle_reasons",
		"pcie_throughput",
		"nvlink_throughput",
		"pcie_bandwidth_capacity",
		"nvlink_bandwidth_capacity",
		"reliability_errors",
	}
}

func nvidiaRequiredFields() []string {
	return []string{
		"gpu_utilization",
		"memory_used",
		"memory_total",
		"power_draw",
		"power_limit",
		"temperature",
		"sm_clock",
		"mem_clock",
		"process_gpu_memory",
		"performance_state",
		"throttle_reasons",
	}
}
