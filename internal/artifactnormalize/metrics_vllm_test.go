package artifactnormalize

import (
	"testing"
	"time"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
)

func TestNormalizeVLLMMetricsDerivesRequestThroughputFromSuccessCounter(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	samples := []promcollector.Sample{
		{
			Timestamp: now,
			Metrics: []promcollector.MetricPoint{
				{Name: "vllm:request_success_total", Labels: map[string]string{"finished_reason": "length"}, Value: 100},
			},
		},
		{
			Timestamp: now.Add(2 * time.Second),
			Metrics: []promcollector.MetricPoint{
				{Name: "vllm:request_success_total", Labels: map[string]string{"finished_reason": "length"}, Value: 112},
			},
		},
	}

	metrics := normalizeVLLMMetrics(samples)
	if metrics.RequestThroughput.Avg == nil {
		t.Fatal("RequestThroughput.Avg is nil")
	}
	if got, want := *metrics.RequestThroughput.Avg, 6.0; got != want {
		t.Fatalf("RequestThroughput.Avg = %v, want %v", got, want)
	}
	if !metrics.Coverage.HasField("request_throughput") {
		t.Fatalf("coverage missing request_throughput: %+v", metrics.Coverage)
	}
	if metrics.CompletedRequests.Total == nil || *metrics.CompletedRequests.Total != 12 {
		t.Fatalf("CompletedRequests.Total = %v, want 12", metrics.CompletedRequests.Total)
	}
}

func TestNormalizeVLLMMetricsCapturesWindowedCountersAndQueueReasons(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	samples := []promcollector.Sample{
		vllmCounterSample(now, 10, 100, 20, 0),
		vllmCounterSample(now.Add(time.Second), 13, 150, 24, 7),
	}

	metrics := normalizeVLLMMetrics(samples)

	if got, want := *metrics.PromptTokensProcessed.Total, 50.0; got != want {
		t.Fatalf("PromptTokensProcessed.Total = %v, want %v", got, want)
	}
	if got, want := *metrics.GenerationTokensProcessed.Total, 4.0; got != want {
		t.Fatalf("GenerationTokensProcessed.Total = %v, want %v", got, want)
	}
	if got, want := metrics.RequestsWaitingByReason["capacity"].Latest, 7.0; got == nil || *got != want {
		t.Fatalf("RequestsWaitingByReason[capacity].Latest = %v, want %v", got, want)
	}
	if !metrics.Coverage.HasField("requests_waiting_by_reason") {
		t.Fatalf("coverage missing requests_waiting_by_reason: %+v", metrics.Coverage)
	}
}

func TestNormalizeVLLMMetricsCapturesKVSwapAndOffloadSignals(t *testing.T) {
	now := time.Unix(1700000100, 0).UTC()
	samples := []promcollector.Sample{
		vllmKVSample(now, 0.80, 0.12, 3, 64),
		vllmKVSample(now.Add(time.Second), 0.90, 0.25, 5, 64),
	}

	metrics := normalizeVLLMMetrics(samples)

	if got, want := *metrics.KVCacheUsage.Latest, 0.90; got != want {
		t.Fatalf("KVCacheUsage.Latest = %v, want %v", got, want)
	}
	if got, want := *metrics.GPUKVCacheUsage.Latest, 0.90; got != want {
		t.Fatalf("GPUKVCacheUsage.Latest = %v, want %v", got, want)
	}
	if got, want := *metrics.CPUKVCacheUsage.Latest, 0.25; got != want {
		t.Fatalf("CPUKVCacheUsage.Latest = %v, want %v", got, want)
	}
	if got, want := *metrics.CPUKVBlocks.Latest, 64.0; got != want {
		t.Fatalf("CPUKVBlocks.Latest = %v, want %v", got, want)
	}
	if got, want := *metrics.SwappedRequests.Latest, 5.0; got != want {
		t.Fatalf("SwappedRequests.Latest = %v, want %v", got, want)
	}
	if got, want := *metrics.KVOffloadActivity.Latest, 5.0; got != want {
		t.Fatalf("KVOffloadActivity.Latest = %v, want %v", got, want)
	}
	for _, field := range []string{"gpu_kv_cache_usage", "cpu_kv_cache_usage", "cpu_kv_blocks", "swapped_requests", "kv_offload_activity"} {
		if !metrics.Coverage.HasField(field) {
			t.Fatalf("coverage missing %s: %+v", field, metrics.Coverage)
		}
	}
}

func TestNormalizeVLLMMetricsDerivesZeroCPUOffloadWhenCacheConfigSaysNone(t *testing.T) {
	now := time.Unix(1700000200, 0).UTC()
	samples := []promcollector.Sample{
		vllmNoCPUBlocksSample(now, 0.4),
		vllmNoCPUBlocksSample(now.Add(time.Second), 0.5),
	}

	metrics := normalizeVLLMMetrics(samples)

	if got, want := *metrics.GPUKVCacheUsage.Latest, 0.5; got != want {
		t.Fatalf("GPUKVCacheUsage.Latest = %v, want %v", got, want)
	}
	if got, want := *metrics.CPUKVBlocks.Latest, 0.0; got != want {
		t.Fatalf("CPUKVBlocks.Latest = %v, want %v", got, want)
	}
	if got, want := *metrics.CPUKVCacheUsage.Latest, 0.0; got != want {
		t.Fatalf("CPUKVCacheUsage.Latest = %v, want %v", got, want)
	}
	if got, want := *metrics.SwappedRequests.Latest, 0.0; got != want {
		t.Fatalf("SwappedRequests.Latest = %v, want %v", got, want)
	}
	if got, want := *metrics.KVOffloadActivity.Latest, 0.0; got != want {
		t.Fatalf("KVOffloadActivity.Latest = %v, want %v", got, want)
	}
	for _, field := range []string{"gpu_kv_cache_usage", "cpu_kv_blocks", "cpu_kv_cache_usage", "swapped_requests", "kv_offload_activity"} {
		if !contains(metrics.Coverage.DerivedFields, field) {
			t.Fatalf("derived fields = %v, want %s", metrics.Coverage.DerivedFields, field)
		}
		if !metrics.Coverage.HasField(field) {
			t.Fatalf("coverage missing %s: %+v", field, metrics.Coverage)
		}
	}
}

func vllmCounterSample(ts time.Time, requests, prompt, generation, waiting float64) promcollector.Sample {
	return promcollector.Sample{
		Timestamp: ts,
		Metrics: []promcollector.MetricPoint{
			{Name: "vllm:request_success_total", Labels: map[string]string{"finished_reason": "stop"}, Value: requests},
			{Name: "vllm:prompt_tokens_total", Labels: map[string]string{"model_name": "m"}, Value: prompt},
			{Name: "vllm:prompt_tokens_by_source_total", Labels: map[string]string{"source": "local_compute"}, Value: prompt},
			{Name: "vllm:prompt_tokens_cached_total", Labels: map[string]string{"model_name": "m"}, Value: 0},
			{Name: "vllm:generation_tokens_total", Labels: map[string]string{"model_name": "m"}, Value: generation},
			{Name: "vllm:num_requests_waiting_by_reason", Labels: map[string]string{"reason": "capacity"}, Value: waiting},
		},
	}
}

func vllmKVSample(ts time.Time, gpuUsage, cpuUsage, swapped, cpuBlocks float64) promcollector.Sample {
	return promcollector.Sample{
		Timestamp: ts,
		Metrics: []promcollector.MetricPoint{
			{Name: "vllm:gpu_cache_usage_perc", Value: gpuUsage},
			{Name: "vllm:cpu_cache_usage_perc", Value: cpuUsage},
			{Name: "vllm:num_requests_swapped", Value: swapped},
			{Name: "vllm:cache_config_info", Labels: map[string]string{"num_cpu_blocks": "64"}, Value: 1},
		},
	}
}

func vllmNoCPUBlocksSample(ts time.Time, kvUsage float64) promcollector.Sample {
	return promcollector.Sample{
		Timestamp: ts,
		Metrics: []promcollector.MetricPoint{
			{Name: "vllm:kv_cache_usage_perc", Value: kvUsage},
			{Name: "vllm:cache_config_info", Labels: map[string]string{"num_cpu_blocks": "None"}, Value: 1},
		},
	}
}
