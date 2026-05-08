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
