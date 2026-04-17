package contracts

import "time"

func runtimeRequiredFields() []string {
	return []string{
		"max_model_len",
		"max_num_batched_tokens",
		"max_num_seqs",
		"gpu_memory_utilization",
		"parallelism_settings",
		"quantization_mode",
		"prefix_caching_state",
		"chunked_prefill_state",
		"multimodal_runtime_hints",
		"vllm_version",
		"torch_version",
		"cuda_runtime_version",
		"nvidia_driver_version",
		"attention_backend",
		"flashinfer_presence",
		"flash_attention_presence",
		"image_processor",
		"multimodal_cache_hints",
	}
}

func vllmRequiredFields() []string {
	return []string{
		"requests_running",
		"requests_waiting",
		"latency_e2e",
		"latency_ttft",
		"latency_queue",
		"latency_prefill",
		"latency_decode",
		"prompt_tokens",
		"generation_tokens",
		"prompt_length",
		"generation_length",
		"kv_cache_usage",
		"preemptions",
		"recomputed_prompt_tokens",
		"prefix_cache",
		"multimodal_cache",
	}
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

func removeField(fields []string, target string) []string {
	filtered := make([]string, 0, len(fields))
	for _, field := range fields {
		if field != target {
			filtered = append(filtered, field)
		}
	}
	return filtered
}

func floatPointer(value float64) *float64 {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}

func timePointer(value time.Time) *time.Time {
	return &value
}
