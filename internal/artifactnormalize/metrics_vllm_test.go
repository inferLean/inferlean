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
}
