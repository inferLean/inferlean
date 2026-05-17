package report

import (
	"time"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func fullReportFixture() contracts.FinalReport {
	reportedAt := time.Unix(1700000100, 0).UTC()
	collectedAt := time.Unix(1700000000, 0).UTC()
	return contracts.FinalReport{
		SchemaVersion: contracts.ReportSchemaVersion,
		Job: contracts.ReportJob{
			RunID:                 "run-123",
			InstallationID:        "inst-123",
			CollectorVersion:      "0.2.0",
			ArtifactSchemaVersion: contracts.SchemaVersion,
			CollectedAt:           collectedAt,
			ReportedAt:            reportedAt,
		},
		Entitlement: contracts.ReportEntitlement{Tier: "paid"},
		Environment: contracts.ReportEnvironment{
			Host:               "gpu-host-1",
			OS:                 "ubuntu",
			Kernel:             "6.8",
			CPUModel:           "AMD EPYC",
			CPUCores:           64,
			MemoryBytes:        256 * 1024 * 1024 * 1024,
			GPUModel:           "H100",
			GPUCount:           8,
			DriverVersion:      "550",
			RuntimeVersion:     "python-3.12",
			VLLMVersion:        "0.8.4",
			TorchVersion:       "2.4",
			CUDARuntimeVersion: "12.4",
			Model:              "Qwen/Qwen3-32B",
			ServedModelName:    "Qwen3-32B",
		},
		Diagnosis: contracts.DiagnosisSection{
			BaseDiagnosis: contracts.BaseDiagnosis{
				WorkloadSummary: contracts.WorkloadSummary{
					DeclaredWorkloadMode:  "throughput",
					ObservedWorkloadShape: "steady multi-user chat",
					ConfiguredPosture:     "conservative",
					Summary:               "Observed batching remains conservative under sustained load.",
				},
				RealLoadSummary: contracts.RealLoadSummary{
					ComputePressure: "medium",
					KVPressure:      "high",
					Summary:         "The service is load-bearing enough to trust throughput guidance.",
				},
				Confidence: "high",
			},
		},
		DiagnosticCoverage: contracts.DiagnosticCoverage{
			DetectorResults: []contracts.DetectorResult{{
				DetectorID:           "kv_pressure_preemption_or_swap",
				Rank:                 1,
				Status:               "detected",
				RequiredEvidenceRefs: []string{"runtime_config.max_model_len"},
			}},
			Summary: contracts.DiagnosticCoverageSummary{CoverageStatus: "complete"},
		},
		Saturation:        saturationReportFixture(),
		Issues:            issueFixtures(),
		Opportunities:     opportunityFixtures(),
		CollectionQuality: collectionQualityFixture(),
	}
}

func issueFixtures() []contracts.Issue {
	return []contracts.Issue{
		{
			ID:           "issue:kv_pressure_preemption_or_swap",
			Rank:         1,
			DetectorID:   "kv_pressure_preemption_or_swap",
			Family:       "kv_footprint_heavy",
			Label:        "KV pressure",
			Confidence:   "high",
			EvidenceRefs: []string{"runtime_config.max_model_len", "metrics.vllm.kv_cache_usage"},
			Recommendation: &contracts.Recommendation{
				Decision:        "reduce_kv_footprint",
				Title:           "Reduce KV footprint",
				Rationale:       "This unlocks safer scheduler headroom before any throughput tuning.",
				Confidence:      "high",
				ProjectedEffect: projectedEffectFixture("Likely improvement: +8% to +15% throughput."),
				Actions: []contracts.Action{{
					ID:            "action:reduce-max-model-len",
					Title:         "Reduce `--max-model-len`",
					CurrentValue:  "8192",
					ProposedValue: "4096",
					Why:           "Free KV headroom before increasing scheduler aggressiveness.",
					Risk:          "Shorter maximum context for some requests.",
				}},
				FollowUpSteps: []contracts.FollowUpStep{{
					ID:    "action:rerun",
					Title: "Rerun under the same load",
					How:   "Keep prompt mix and concurrency stable.",
				}},
			},
		},
		{
			ID:         "issue:underbatching",
			Rank:       2,
			DetectorID: "underbatching_for_throughput_traffic",
			Family:     "scheduler_conservative",
			Label:      "Scheduler posture is conservative",
			Confidence: "medium",
			Recommendation: &contracts.Recommendation{
				Decision:        "increase_batching_posture",
				Title:           "Increase batching posture",
				Rationale:       "Scheduler settings leave throughput headroom after KV pressure is addressed.",
				Confidence:      "medium",
				ProjectedEffect: projectedEffectFixture("Likely improvement: +4% to +8% throughput."),
			},
		},
	}
}

func opportunityFixtures() []contracts.Opportunity {
	return []contracts.Opportunity{
		{
			ID:         "opportunity:prefix-cache",
			Rank:       2,
			DetectorID: "repeated_prefix_reuse_opportunity",
			Category:   "cache_reuse",
			Title:      "Exploit repeated-prefix reuse",
			Confidence: "medium",
			Recommendation: &contracts.Recommendation{
				Decision:        "validate_prefix_caching",
				Title:           "Validate prefix caching",
				Rationale:       "Repeated prefixes can reduce prefill cost.",
				Confidence:      "medium",
				ProjectedEffect: projectedEffectFixture("Likely improvement: +3% to +6% throughput."),
			},
		},
		{
			ID:         "opportunity:quantization",
			Rank:       1,
			DetectorID: "quantized_model_opportunity",
			Category:   "model_optimization",
			Title:      "Evaluate quantization next",
			Confidence: "medium",
			Recommendation: &contracts.Recommendation{
				Decision:        "evaluate_quantization",
				Title:           "Evaluate quantization next",
				Rationale:       "Treat quantization as a ranked opportunity.",
				ProjectedEffect: projectedEffectFixture("Likely improvement: +5% to +10% latency."),
			},
		},
	}
}

func collectionQualityFixture() contracts.ReportCollectionQuality {
	return contracts.ReportCollectionQuality{
		Completeness:            0.93,
		TelemetryMode:           "prometheus",
		SelectedGPUPath:         "nvml_bridge",
		Summary:                 "GPU, host, and vLLM metrics were all collected.",
		ConfidenceImpactSummary: "Low confidence impact.",
		MissingEvidence:         []string{"none"},
		SourceStates: map[string]contracts.SourceState{
			"vllm_metrics": {Status: "ok"},
		},
	}
}

func projectedEffectFixture(summary string) contracts.ProjectedEffect {
	currentLatency := 1.4
	projectedLatency := 1.4
	latencyDelta := 0.0
	latencyPercent := 0.0
	currentRequests := 4.8
	projectedRequests := 5.28
	requestDelta := projectedRequests - currentRequests
	requestPercent := 10.0
	currentOutput := 256.0
	projectedOutput := 281.6
	outputDelta := projectedOutput - currentOutput
	outputPercent := 10.0
	return contracts.ProjectedEffect{
		Summary: summary,
		Latency: contracts.ProjectedMetricEffect{
			Metric:       "latency_e2e_seconds",
			Unit:         "s",
			Current:      &currentLatency,
			Projected:    &projectedLatency,
			Delta:        &latencyDelta,
			PercentDelta: &latencyPercent,
			Direction:    "lower_is_better",
			Confidence:   "medium",
		},
		Throughput: contracts.ProjectedThroughputEffect{
			Requests: contracts.ProjectedMetricEffect{
				Metric:       "request_throughput",
				Unit:         "req/s",
				Current:      &currentRequests,
				Projected:    &projectedRequests,
				Delta:        &requestDelta,
				PercentDelta: &requestPercent,
				Direction:    "higher_is_better",
				Confidence:   "medium",
			},
			OutputTokens: contracts.ProjectedMetricEffect{
				Metric:       "generation_tokens_per_second",
				Unit:         "tok/s",
				Current:      &currentOutput,
				Projected:    &projectedOutput,
				Delta:        &outputDelta,
				PercentDelta: &outputPercent,
				Direction:    "higher_is_better",
				Confidence:   "medium",
			},
		},
	}
}

func saturationReportFixture() contracts.SaturationReport {
	genericScore := 72.0
	genericHeadroom := 28.0
	genericWorstHeadroom := 18.0
	computeScore := 64.0
	computeHeadroom := 36.0
	memoryScore := 0.0
	return contracts.SaturationReport{
		Version: "saturation-v1",
		Generic: contracts.SaturationMetric{
			ID:                           "generic",
			Label:                        "Generic saturation",
			Status:                       "ok",
			Score:                        contracts.MetricWindow{Latest: &genericScore},
			HeadroomPercent:              &genericHeadroom,
			WorstObservedHeadroomPercent: &genericWorstHeadroom,
			Reason:                       "Maximum observed saturation across evaluated dimensions.",
		},
		Dimensions: []contracts.SaturationMetric{
			{
				ID:              "compute",
				Label:           "Compute / SM saturation",
				BottleneckType:  "compute",
				Status:          "ok",
				Score:           contracts.MetricWindow{Latest: &computeScore},
				HeadroomPercent: &computeHeadroom,
			},
			{
				ID:              "memory_bandwidth",
				Label:           "Memory bandwidth",
				BottleneckType:  "memory_bandwidth",
				Status:          "not_evaluable",
				Score:           contracts.MetricWindow{Latest: &memoryScore},
				MissingEvidence: []string{"metrics.gpu.sm_active"},
			},
		},
	}
}
