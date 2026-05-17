package report

import (
	"time"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func fullArtifactFixture() contracts.RunArtifact {
	collectedAt := time.Unix(1700000000, 0).UTC()
	startedAt := collectedAt.Add(-5 * time.Minute)
	kvLatest := 0.91
	asyncScheduling := true
	prefixCaching := true
	chunkedPrefill := true
	return contracts.RunArtifact{
		SchemaVersion: contracts.SchemaVersion,
		Job: contracts.Job{
			RunID:            "run-123",
			InstallationID:   "inst-123",
			CollectorVersion: "0.2.0",
			SchemaVersion:    contracts.SchemaVersion,
			CollectedAt:      collectedAt,
		},
		Environment: contracts.Environment{
			OS:             "ubuntu",
			Kernel:         "6.8",
			CPUModel:       "AMD EPYC",
			CPUCores:       64,
			MemoryBytes:    256 * 1024 * 1024 * 1024,
			GPUModel:       "H100",
			GPUCount:       8,
			DriverVersion:  "550",
			RuntimeVersion: "python-3.12",
		},
		RuntimeConfig: contracts.RuntimeConfig{
			Model:               "Qwen/Qwen3-32B",
			ServedModelName:     "Qwen3-32B",
			Host:                "127.0.0.1",
			Port:                8000,
			MaxModelLen:         8192,
			MaxNumBatchedTokens: 4096,
			MaxNumSeqs:          256,
			Scheduler: contracts.SchedulerConfig{
				AsyncScheduling:           &asyncScheduling,
				Policy:                    "fcfs",
				MaxNumPartialPrefills:     1,
				MaxLongPartialPrefills:    1,
				LongPrefillTokenThreshold: 512,
			},
			GPUMemoryUtilization: 0.9,
			KVCacheDType:         "auto",
			ChunkedPrefill:       &chunkedPrefill,
			PrefixCaching:        &prefixCaching,
			VLLMVersion:          "0.8.4",
			TorchVersion:         "2.4",
			CUDARuntimeVersion:   "12.4",
		},
		Metrics: contracts.Metrics{
			VLLM: contracts.VLLMMetrics{
				KVCacheUsage: contracts.MetricWindow{Latest: &kvLatest},
			},
		},
		ProcessInspection: contracts.ProcessInspection{
			TargetProcess: contracts.TargetProcess{
				PID:            1234,
				InternalPID:    1234,
				Executable:     "/usr/bin/python3",
				RawCommandLine: "python -m vllm.entrypoints.openai.api_server",
				StartedAt:      &startedAt,
			},
			RelatedProcesses: []contracts.ObservedProcess{{
				PID:            1235,
				Executable:     "VLLM::EngineCore",
				RawCommandLine: "VLLM::EngineCore",
				StartedAt:      &startedAt,
			}},
		},
		CollectionQuality: contracts.CollectionQuality{
			SourceStates: map[string]contracts.SourceState{
				"process_inspection": {Status: "ok"},
			},
			Completeness:  0.91,
			TelemetryMode: "prometheus",
			Summary:       "Artifact evidence collected.",
		},
	}
}
