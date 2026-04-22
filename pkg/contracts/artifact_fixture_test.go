package contracts

import "time"

func validArtifact() RunArtifact {
	now := time.Unix(1700000000, 0).UTC()
	return RunArtifact{
		SchemaVersion:     SchemaVersion,
		Job:               validJob(now),
		Environment:       validEnvironment(),
		RuntimeConfig:     validRuntimeConfig(),
		Metrics:           validMetrics(now),
		ProcessInspection: validProcessInspection(now),
		WorkloadObservations: WorkloadObservations{
			Mode:                    "mixed",
			Target:                  "balanced",
			PrefixReuse:             "high",
			Multimodal:              "present",
			RepeatedMultimodalMedia: "high",
		},
		CollectionQuality: validCollectionQuality(),
	}
}

func validJob(now time.Time) Job {
	return Job{
		RunID:            "run-123",
		InstallationID:   "inst-456",
		CollectorVersion: "0.2.0",
		SchemaVersion:    SchemaVersion,
		CollectedAt:      now,
	}
}

func validEnvironment() Environment {
	return Environment{
		OS:            "linux/amd64",
		DriverVersion: "550.54.15",
	}
}

func validRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		Model:                 "meta-llama/Llama-3.1-8B-Instruct",
		Host:                  "127.0.0.1",
		Port:                  8000,
		TensorParallelSize:    1,
		MaxModelLen:           8192,
		MaxNumBatchedTokens:   4096,
		MaxNumSeqs:            256,
		GPUMemoryUtilization:  0.9,
		Quantization:          "none",
		ChunkedPrefill:        boolPointer(true),
		PrefixCaching:         boolPointer(true),
		MultimodalFlags:       []string{"image-inputs"},
		VLLMVersion:           "0.8.4",
		TorchVersion:          "2.6.0",
		CUDARuntimeVersion:    "12.4",
		NvidiaDriverVersion:   "550.54.15",
		AttentionBackend:      "flash-attn",
		FlashinferPresent:     boolPointer(false),
		FlashAttentionPresent: boolPointer(true),
		ImageProcessor:        "qwen-vl",
		Coverage:              coverage(runtimeRequiredFields()...),
	}
}

func validMetrics(now time.Time) Metrics {
	return Metrics{
		VLLM:      validVLLMMetrics(now),
		Host:      validHostMetrics(now),
		GPU:       validGPUTelemetry(now),
		NvidiaSmi: validNvidiaSMIMetrics(now),
	}
}

func validVLLMMetrics(now time.Time) VLLMMetrics {
	window := metricWindow(now, 1)
	return VLLMMetrics{
		RequestsRunning:        window,
		RequestsWaiting:        window,
		LatencyE2E:             window,
		LatencyTTFT:            window,
		LatencyQueue:           window,
		LatencyPrefill:         window,
		LatencyDecode:          window,
		PromptTokens:           window,
		GenerationTokens:       window,
		PromptLength:           histogram(),
		GenerationLength:       histogram(),
		KVCacheUsage:           window,
		Preemptions:            window,
		RecomputedPromptTokens: window,
		PrefixCache:            cacheSnapshot(),
		MultimodalCache:        cacheSnapshot(),
		Coverage:               coverage(vllmRequiredFields()...),
	}
}

func validHostMetrics(now time.Time) HostMetrics {
	window := metricWindow(now, 1)
	return HostMetrics{
		CPUUtilization:  window,
		CPULoad:         window,
		MemoryUsed:      window,
		MemoryAvailable: window,
		SwapPressure:    window,
		ProcessCPU:      window,
		ProcessMemory:   window,
		NetworkRX:       window,
		NetworkTX:       window,
		Coverage:        coverage(hostRequiredFields()...),
	}
}

func validGPUTelemetry(now time.Time) GPUTelemetry {
	window := metricWindow(now, 1)
	return GPUTelemetry{
		GPUUtilizationOrSMActivity: window,
		FramebufferMemory:          memoryMetrics(now),
		MemoryBandwidth:            window,
		Clocks:                     clockMetrics(now),
		Power:                      window,
		Temperature:                window,
		PCIeThroughput:             throughputMetrics(now),
		NVLinkThroughput:           throughputMetrics(now),
		ReliabilityErrors:          reliabilityMetrics(now),
		Coverage:                   coverage(gpuRequiredFields()...),
	}
}

func validNvidiaSMIMetrics(now time.Time) NvidiaSMIMetrics {
	window := metricWindow(now, 1)
	return NvidiaSMIMetrics{
		GPUUtilization:   window,
		MemoryUsed:       window,
		MemoryTotal:      window,
		PowerDraw:        window,
		Temperature:      window,
		SMClock:          window,
		MemClock:         window,
		ProcessGPUMemory: window,
		Coverage:         coverage(nvidiaRequiredFields()...),
	}
}

func validProcessInspection(now time.Time) ProcessInspection {
	startedAt := timePointer(now.Add(-5 * time.Minute))
	return ProcessInspection{
		TargetProcess: TargetProcess{
			PID:            1234,
			Executable:     "/usr/bin/python3",
			RawCommandLine: "python -m vllm.entrypoints.openai.api_server",
			StartedAt:      startedAt,
		},
		RelatedProcesses: []ObservedProcess{{
			PID:            1234,
			Executable:     "/usr/bin/python3",
			RawCommandLine: "python -m vllm.entrypoints.openai.api_server",
			StartedAt:      startedAt,
		}},
		Coverage: coverage(
			"raw_command_line",
			"target_pid",
			"executable_identity",
			"related_process_identities",
		),
	}
}

func validCollectionQuality() CollectionQuality {
	return CollectionQuality{
		SourceStates: map[string]SourceState{
			"vllm_metrics":       {Status: "ok"},
			"host_metrics":       {Status: "ok"},
			"gpu_telemetry":      {Status: "ok"},
			"nvidia_smi":         {Status: "ok"},
			"process_inspection": {Status: "ok"},
		},
		Completeness: 1,
	}
}

func metricWindow(ts time.Time, value float64) MetricWindow {
	return MetricWindow{
		Latest:  floatPointer(value),
		Min:     floatPointer(value),
		Max:     floatPointer(value),
		Avg:     floatPointer(value),
		Samples: []MetricSample{{Timestamp: ts, Value: value}},
	}
}

func histogram() DistributionSnapshot {
	return DistributionSnapshot{
		Count: floatPointer(10),
		Sum:   floatPointer(20),
		Buckets: []DistributionBucket{
			{UpperBound: "1", Value: 4},
			{UpperBound: "+Inf", Value: 10},
		},
	}
}

func cacheSnapshot() CacheSnapshot {
	return CacheSnapshot{
		Hits:    floatPointer(8),
		Queries: floatPointer(10),
		HitRate: floatPointer(0.8),
	}
}

func memoryMetrics(ts time.Time) MemoryMetrics {
	return MemoryMetrics{
		Used:  metricWindow(ts, 2),
		Free:  metricWindow(ts, 1),
		Total: metricWindow(ts, 3),
	}
}

func clockMetrics(ts time.Time) ClockMetrics {
	return ClockMetrics{
		SM:     metricWindow(ts, 1200),
		Memory: metricWindow(ts, 1800),
	}
}

func throughputMetrics(ts time.Time) ThroughputMetrics {
	return ThroughputMetrics{
		RX: metricWindow(ts, 100),
		TX: metricWindow(ts, 150),
	}
}

func reliabilityMetrics(ts time.Time) ReliabilityMetrics {
	return ReliabilityMetrics{
		XID: metricWindow(ts, 0),
		ECC: metricWindow(ts, 0),
	}
}

func coverage(fields ...string) SourceCoverage {
	return SourceCoverage{
		PresentFields:  append([]string{}, fields...),
		RawEvidenceRef: "raw/test.json",
	}
}
