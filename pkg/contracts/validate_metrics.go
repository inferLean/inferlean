package contracts

import "fmt"

func validateMetrics(m Metrics) []error {
	var errs []error

	errs = append(errs, validateCoverage("metrics.vllm", m.VLLM.Coverage, vllmChecks(m.VLLM))...)
	errs = append(errs, validateCoverage("metrics.host", m.Host.Coverage, hostChecks(m.Host))...)
	errs = append(errs, validateCoverage("metrics.gpu", m.GPU.Coverage, gpuChecks(m.GPU))...)
	errs = append(errs, validateCoverage("metrics.nvidia_smi", m.NvidiaSmi.Coverage, nvidiaChecks(m.NvidiaSmi))...)

	return errs
}

func vllmChecks(m VLLMMetrics) map[string]bool {
	return map[string]bool{
		"requests_running":         m.RequestsRunning.HasData(),
		"requests_waiting":         m.RequestsWaiting.HasData(),
		"latency_e2e":              m.LatencyE2E.HasData(),
		"latency_ttft":             m.LatencyTTFT.HasData(),
		"latency_queue":            m.LatencyQueue.HasData(),
		"latency_prefill":          m.LatencyPrefill.HasData(),
		"latency_decode":           m.LatencyDecode.HasData(),
		"prompt_tokens":            m.PromptTokens.HasData(),
		"generation_tokens":        m.GenerationTokens.HasData(),
		"prompt_length":            m.PromptLength.HasData(),
		"generation_length":        m.GenerationLength.HasData(),
		"kv_cache_usage":           m.KVCacheUsage.HasData(),
		"preemptions":              m.Preemptions.HasData(),
		"recomputed_prompt_tokens": m.RecomputedPromptTokens.HasData(),
		"prefix_cache":             m.PrefixCache.HasData(),
		"multimodal_cache":         m.MultimodalCache.HasData(),
	}
}

func hostChecks(m HostMetrics) map[string]bool {
	return map[string]bool{
		"cpu_utilization":  m.CPUUtilization.HasData(),
		"cpu_load":         m.CPULoad.HasData(),
		"memory_used":      m.MemoryUsed.HasData(),
		"memory_available": m.MemoryAvailable.HasData(),
		"swap_pressure":    m.SwapPressure.HasData(),
		"process_cpu":      m.ProcessCPU.HasData(),
		"process_memory":   m.ProcessMemory.HasData(),
		"network_rx":       m.NetworkRX.HasData(),
		"network_tx":       m.NetworkTX.HasData(),
	}
}

func gpuChecks(m GPUTelemetry) map[string]bool {
	return map[string]bool{
		"gpu_utilization_or_sm_activity": m.GPUUtilizationOrSMActivity.HasData(),
		"framebuffer_memory":             m.FramebufferMemory.HasData(),
		"memory_bandwidth":               m.MemoryBandwidth.HasData(),
		"clocks":                         m.Clocks.HasData(),
		"power":                          m.Power.HasData(),
		"temperature":                    m.Temperature.HasData(),
		"pcie_throughput":                m.PCIeThroughput.HasData(),
		"nvlink_throughput":              m.NVLinkThroughput.HasData(),
		"reliability_errors":             m.ReliabilityErrors.HasData(),
	}
}

func nvidiaChecks(m NvidiaSMIMetrics) map[string]bool {
	return map[string]bool{
		"gpu_utilization":    m.GPUUtilization.HasData(),
		"memory_used":        m.MemoryUsed.HasData(),
		"memory_total":       m.MemoryTotal.HasData(),
		"power_draw":         m.PowerDraw.HasData(),
		"temperature":        m.Temperature.HasData(),
		"sm_clock":           m.SMClock.HasData(),
		"mem_clock":          m.MemClock.HasData(),
		"process_gpu_memory": m.ProcessGPUMemory.HasData(),
	}
}

func validateCoverage(source string, coverage SourceCoverage, checks map[string]bool) []error {
	var errs []error

	for field, present := range checks {
		if present || coverage.MarksField(field) {
			continue
		}
		errs = append(errs, fmt.Errorf("%s.%s must be populated or marked missing/unsupported", source, field))
	}

	return errs
}
