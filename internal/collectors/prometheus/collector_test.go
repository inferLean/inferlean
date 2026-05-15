package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestCollectTargets(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("test_metric 1\n"))
	}))
	defer server.Close()
	collector := NewCollector()
	result := collector.CollectTargets(context.Background(), []Target{
		{Name: "good", Endpoint: server.URL, Required: true},
		{Name: "missing", Endpoint: "http://127.0.0.1:1/metrics"},
	}, 250*time.Millisecond, 100*time.Millisecond)
	if result.SourceStatus["good"] != "ok" {
		t.Fatalf("good source status = %q", result.SourceStatus["good"])
	}
	if result.SourceStatus["missing"] != "missing" {
		t.Fatalf("missing source status = %q", result.SourceStatus["missing"])
	}
	if len(result.Samples["good"]) == 0 {
		t.Fatal("expected good source samples")
	}
	if result.Samples["good"][0].Timestamp.IsZero() {
		t.Fatal("expected timestamped samples")
	}
	if len(result.Samples["good"][0].Metrics) == 0 {
		t.Fatal("expected parsed metrics in first sample")
	}
	if !strings.Contains(result.RawText, "target=good") {
		t.Fatal("expected raw text to include good target")
	}
	if !strings.Contains(result.RawByTarget["good"], "test_metric") {
		t.Fatal("expected per-target raw data")
	}
	if result.SourceStatus["prometheus_runtime"] == "" {
		t.Fatal("expected prometheus runtime status")
	}
}

func TestScrapeTargetsOnce(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("test_metric 1\n"))
	}))
	defer server.Close()

	result := NewCollector().ScrapeTargetsOnce(context.Background(), []Target{
		{Name: "good", Endpoint: server.URL, Required: true},
		{Name: "missing", Endpoint: "http://127.0.0.1:1/metrics"},
	})

	if result.SourceStatus["good"] != "ok" {
		t.Fatalf("good source status = %q", result.SourceStatus["good"])
	}
	if result.SourceStatus["missing"] != "missing" {
		t.Fatalf("missing source status = %q", result.SourceStatus["missing"])
	}
	if len(result.Samples["good"]) != 1 {
		t.Fatalf("good sample count = %d, want 1", len(result.Samples["good"]))
	}
	if result.StartedAt.IsZero() || result.FinishedAt.IsZero() {
		t.Fatalf("expected scrape timestamps, got started=%s finished=%s", result.StartedAt, result.FinishedAt)
	}
	if _, ok := result.SourceStatus["prometheus_runtime"]; ok {
		t.Fatalf("one-shot scrape should not start prometheus runtime: %+v", result.SourceStatus)
	}
}

func TestParseMetricsUsesPrometheusTextParser(t *testing.T) {
	t.Parallel()
	points, err := parseMetrics("test_metric{label=\"hello world\",escaped=\"a\\\"b\"} 42 1710000000000\n")
	if err != nil {
		t.Fatalf("parseMetrics() error = %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("len(points) = %d, want 1", len(points))
	}
	point := points[0]
	if point.Name != "test_metric" {
		t.Fatalf("Name = %q", point.Name)
	}
	if point.Value != 42 {
		t.Fatalf("Value = %f, want 42", point.Value)
	}
	if point.Labels["label"] != "hello world" {
		t.Fatalf("label = %q", point.Labels["label"])
	}
	if point.Labels["escaped"] != `a"b` {
		t.Fatalf("escaped = %q", point.Labels["escaped"])
	}
}

func TestParseMetricsFlattensHistograms(t *testing.T) {
	t.Parallel()
	points, err := parseMetrics(`
# HELP vllm:e2e_request_latency_seconds Histogram of e2e request latency in seconds.
# TYPE vllm:e2e_request_latency_seconds histogram
vllm:e2e_request_latency_seconds_bucket{engine="0",le="0.5",model_name="model"} 1
vllm:e2e_request_latency_seconds_bucket{engine="0",le="+Inf",model_name="model"} 3
vllm:e2e_request_latency_seconds_count{engine="0",model_name="model"} 3
vllm:e2e_request_latency_seconds_sum{engine="0",model_name="model"} 12
`)
	if err != nil {
		t.Fatalf("parseMetrics() error = %v", err)
	}
	names := metricNames(points)
	for _, want := range []string{
		"vllm:e2e_request_latency_seconds_bucket",
		"vllm:e2e_request_latency_seconds_count",
		"vllm:e2e_request_latency_seconds_sum",
	} {
		if !containsName(names, want) {
			t.Fatalf("names = %v, missing %s", names, want)
		}
	}
	if containsName(names, "vllm:e2e_request_latency_seconds") {
		t.Fatalf("histogram base metric should not be emitted: %v", names)
	}
	count := findPoint(t, points, "vllm:e2e_request_latency_seconds_count", "")
	if count.Value != 3 {
		t.Fatalf("count value = %v, want 3", count.Value)
	}
	sum := findPoint(t, points, "vllm:e2e_request_latency_seconds_sum", "")
	if sum.Value != 12 {
		t.Fatalf("sum value = %v, want 12", sum.Value)
	}
	bucket := findPoint(t, points, "vllm:e2e_request_latency_seconds_bucket", "+Inf")
	if bucket.Value != 3 {
		t.Fatalf("+Inf bucket value = %v, want 3", bucket.Value)
	}
	if bucket.Labels["engine"] != "0" || bucket.Labels["model_name"] != "model" {
		t.Fatalf("bucket labels = %+v", bucket.Labels)
	}
}

func TestParseMetricsFlattensSummaries(t *testing.T) {
	t.Parallel()
	points, err := parseMetrics(`
# HELP request_duration_seconds request duration.
# TYPE request_duration_seconds summary
request_duration_seconds{quantile="0.5",route="/v1"} 4
request_duration_seconds{quantile="0.9",route="/v1"} 8
request_duration_seconds_sum{route="/v1"} 12
request_duration_seconds_count{route="/v1"} 3
`)
	if err != nil {
		t.Fatalf("parseMetrics() error = %v", err)
	}
	if point := findPoint(t, points, "request_duration_seconds", "0.9"); point.Value != 8 {
		t.Fatalf("quantile value = %v, want 8", point.Value)
	}
	if point := findPoint(t, points, "request_duration_seconds_count", ""); point.Value != 3 {
		t.Fatalf("count value = %v, want 3", point.Value)
	}
	if point := findPoint(t, points, "request_duration_seconds_sum", ""); point.Value != 12 {
		t.Fatalf("sum value = %v, want 12", point.Value)
	}
}

func metricNames(points []MetricPoint) []string {
	names := make([]string, 0, len(points))
	for _, point := range points {
		names = append(names, point.Name)
	}
	sort.Strings(names)
	return names
}

func containsName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func findPoint(t *testing.T, points []MetricPoint, name, leOrQuantile string) MetricPoint {
	t.Helper()
	for _, point := range points {
		if point.Name != name {
			continue
		}
		if leOrQuantile == "" {
			return point
		}
		if point.Labels["le"] == leOrQuantile || point.Labels["quantile"] == leOrQuantile {
			return point
		}
	}
	t.Fatalf("point %q with discriminator %q not found in %+v", name, leOrQuantile, points)
	return MetricPoint{}
}
