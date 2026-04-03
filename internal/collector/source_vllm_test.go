package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestCaptureVLLMMetricsUsesTotalMetricFallbacks(t *testing.T) {
	t.Parallel()

	start := time.Unix(1700000000, 0).UTC()
	end := start.Add(25 * time.Second)
	points := [][]any{
		{float64(start.Unix()), "1"},
		{float64(end.Unix()), "2"},
	}

	rangeResults := map[string][]promRangeSeries{
		"sum(vllm:num_requests_running)": matrixResult(points, nil),
		"sum(vllm:num_requests_waiting)": matrixResult(points, nil),
		"sum(rate(vllm:e2e_request_latency_seconds_sum[10s])) / sum(rate(vllm:e2e_request_latency_seconds_count[10s]))":   matrixResult(points, nil),
		"sum(rate(vllm:time_to_first_token_seconds_sum[10s])) / sum(rate(vllm:time_to_first_token_seconds_count[10s]))":   matrixResult(points, nil),
		"sum(rate(vllm:request_queue_time_seconds_sum[10s])) / sum(rate(vllm:request_queue_time_seconds_count[10s]))":     matrixResult(points, nil),
		"sum(rate(vllm:request_prefill_time_seconds_sum[10s])) / sum(rate(vllm:request_prefill_time_seconds_count[10s]))": matrixResult(points, nil),
		"sum(rate(vllm:request_decode_time_seconds_sum[10s])) / sum(rate(vllm:request_decode_time_seconds_count[10s]))":   matrixResult(points, nil),
		"sum(vllm:prompt_tokens_total)":            matrixResult(points, nil),
		"sum(vllm:generation_tokens_total)":        matrixResult(points, nil),
		"avg(vllm:kv_cache_usage_perc)":            matrixResult(points, nil),
		"sum(vllm:num_preemptions_total)":          matrixResult(points, nil),
		"sum(vllm:prompt_tokens_recomputed_total)": matrixResult(points, nil),
		"sum(vllm:prefix_cache_hits_total)":        matrixResult(points, nil),
		"sum(vllm:prefix_cache_queries_total)":     matrixResult(points, nil),
		"sum(vllm:mm_cache_hits_total)":            matrixResult(points, nil),
		"sum(vllm:mm_cache_queries_total)":         matrixResult(points, nil),
	}

	vectorResults := map[string][]struct {
		metric map[string]string
		value  []any
	}{
		"sum(vllm:request_prompt_tokens_bucket) by (le)": {
			{metric: map[string]string{"le": "10"}, value: []any{float64(end.Unix()), "1"}},
			{metric: map[string]string{"le": "+Inf"}, value: []any{float64(end.Unix()), "2"}},
		},
		"sum(vllm:request_prompt_tokens_count)": {
			{metric: map[string]string{}, value: []any{float64(end.Unix()), "2"}},
		},
		"sum(vllm:request_prompt_tokens_sum)": {
			{metric: map[string]string{}, value: []any{float64(end.Unix()), "100"}},
		},
		"sum(vllm:request_generation_tokens_bucket) by (le)": {
			{metric: map[string]string{"le": "10"}, value: []any{float64(end.Unix()), "1"}},
			{metric: map[string]string{"le": "+Inf"}, value: []any{float64(end.Unix()), "2"}},
		},
		"sum(vllm:request_generation_tokens_count)": {
			{metric: map[string]string{}, value: []any{float64(end.Unix()), "2"}},
		},
		"sum(vllm:request_generation_tokens_sum)": {
			{metric: map[string]string{}, value: []any{float64(end.Unix()), "100"}},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/query_range":
			expr := mustQueryParam(t, r.URL, "query")
			_ = json.NewEncoder(w).Encode(promRangeResponse{
				Status: "success",
				Data: struct {
					Result []promRangeSeries `json:"result"`
				}{
					Result: rangeResults[expr],
				},
			})
		case "/api/v1/query":
			expr := mustQueryParam(t, r.URL, "query")
			results := vectorResults[expr]
			body := promVectorResponse{Status: "success"}
			body.Data.Result = make([]struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
			}, 0, len(results))
			for _, result := range results {
				body.Data.Result = append(body.Data.Result, struct {
					Metric map[string]string `json:"metric"`
					Value  []any             `json:"value"`
				}{
					Metric: result.metric,
					Value:  result.value,
				})
			}
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	runDir := t.TempDir()
	artifacts := buildRuntimeArtifacts(runDir)
	run := &collectionRun{
		service: Service{client: server.Client()},
		opts: Options{
			ScrapeEvery: 5 * time.Second,
		},
		vllmTarget:     "127.0.0.1:8000",
		promBase:       server.URL,
		collectStarted: start,
		collectEnded:   end,
		rawPaths:       artifacts,
	}

	if err := run.captureVLLMMetrics(context.Background()); err != nil {
		t.Fatalf("capture vllm metrics: %v", err)
	}
	if run.vllmCapture.Status != "ok" {
		t.Fatalf("expected vllm capture to be ok, got %q with coverage %+v", run.vllmCapture.Status, run.vllmMetrics.Coverage)
	}
	for _, field := range []string{"prompt_tokens", "generation_tokens", "preemptions", "recomputed_prompt_tokens", "prefix_cache", "multimodal_cache"} {
		if !containsCoverageName(run.vllmMetrics.Coverage.PresentFields, field) {
			t.Fatalf("expected %s to be present in coverage, got %+v", field, run.vllmMetrics.Coverage)
		}
		if containsCoverageName(run.vllmMetrics.Coverage.MissingFields, field) {
			t.Fatalf("expected %s not to be missing, got %+v", field, run.vllmMetrics.Coverage)
		}
	}
}

func TestCaptureVLLMMetricsDropsNonFiniteLatencySamples(t *testing.T) {
	t.Parallel()

	start := time.Unix(1700000000, 0).UTC()
	end := start.Add(25 * time.Second)
	points := [][]any{
		{float64(start.Unix()), "1"},
		{float64(end.Unix()), "2"},
	}
	nonFinite := [][]any{
		{float64(start.Unix()), "NaN"},
		{float64(end.Unix()), "NaN"},
	}

	rangeResults := map[string][]promRangeSeries{
		"sum(vllm:num_requests_running)": matrixResult(points, nil),
		"sum(vllm:num_requests_waiting)": matrixResult(points, nil),
		"sum(rate(vllm:e2e_request_latency_seconds_sum[10s])) / sum(rate(vllm:e2e_request_latency_seconds_count[10s]))":   matrixResult(nonFinite, nil),
		"sum(rate(vllm:time_to_first_token_seconds_sum[10s])) / sum(rate(vllm:time_to_first_token_seconds_count[10s]))":   matrixResult(points, nil),
		"sum(rate(vllm:request_queue_time_seconds_sum[10s])) / sum(rate(vllm:request_queue_time_seconds_count[10s]))":     matrixResult(points, nil),
		"sum(rate(vllm:request_prefill_time_seconds_sum[10s])) / sum(rate(vllm:request_prefill_time_seconds_count[10s]))": matrixResult(points, nil),
		"sum(rate(vllm:request_decode_time_seconds_sum[10s])) / sum(rate(vllm:request_decode_time_seconds_count[10s]))":   matrixResult(points, nil),
		"sum(vllm:prompt_tokens_total)":            matrixResult(points, nil),
		"sum(vllm:generation_tokens_total)":        matrixResult(points, nil),
		"avg(vllm:kv_cache_usage_perc)":            matrixResult(points, nil),
		"sum(vllm:num_preemptions_total)":          matrixResult(points, nil),
		"sum(vllm:prompt_tokens_recomputed_total)": matrixResult(points, nil),
		"sum(vllm:prefix_cache_hits_total)":        matrixResult(points, nil),
		"sum(vllm:prefix_cache_queries_total)":     matrixResult(points, nil),
		"sum(vllm:mm_cache_hits_total)":            matrixResult(points, nil),
		"sum(vllm:mm_cache_queries_total)":         matrixResult(points, nil),
	}

	vectorResults := map[string][]struct {
		metric map[string]string
		value  []any
	}{
		"sum(vllm:request_prompt_tokens_bucket) by (le)": {
			{metric: map[string]string{"le": "10"}, value: []any{float64(end.Unix()), "1"}},
			{metric: map[string]string{"le": "+Inf"}, value: []any{float64(end.Unix()), "2"}},
		},
		"sum(vllm:request_prompt_tokens_count)": {
			{metric: map[string]string{}, value: []any{float64(end.Unix()), "2"}},
		},
		"sum(vllm:request_prompt_tokens_sum)": {
			{metric: map[string]string{}, value: []any{float64(end.Unix()), "100"}},
		},
		"sum(vllm:request_generation_tokens_bucket) by (le)": {
			{metric: map[string]string{"le": "10"}, value: []any{float64(end.Unix()), "1"}},
			{metric: map[string]string{"le": "+Inf"}, value: []any{float64(end.Unix()), "2"}},
		},
		"sum(vllm:request_generation_tokens_count)": {
			{metric: map[string]string{}, value: []any{float64(end.Unix()), "2"}},
		},
		"sum(vllm:request_generation_tokens_sum)": {
			{metric: map[string]string{}, value: []any{float64(end.Unix()), "100"}},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/query_range":
			expr := mustQueryParam(t, r.URL, "query")
			_ = json.NewEncoder(w).Encode(promRangeResponse{
				Status: "success",
				Data: struct {
					Result []promRangeSeries `json:"result"`
				}{
					Result: rangeResults[expr],
				},
			})
		case "/api/v1/query":
			expr := mustQueryParam(t, r.URL, "query")
			results := vectorResults[expr]
			body := promVectorResponse{Status: "success"}
			body.Data.Result = make([]struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
			}, 0, len(results))
			for _, result := range results {
				body.Data.Result = append(body.Data.Result, struct {
					Metric map[string]string `json:"metric"`
					Value  []any             `json:"value"`
				}{
					Metric: result.metric,
					Value:  result.value,
				})
			}
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	runDir := t.TempDir()
	artifacts := buildRuntimeArtifacts(runDir)
	run := &collectionRun{
		service: Service{client: server.Client()},
		opts: Options{
			ScrapeEvery: 5 * time.Second,
		},
		vllmTarget:     "127.0.0.1:8000",
		promBase:       server.URL,
		collectStarted: start,
		collectEnded:   end,
		rawPaths:       artifacts,
	}

	if err := run.captureVLLMMetrics(context.Background()); err != nil {
		t.Fatalf("capture vllm metrics with non-finite latency values: %v", err)
	}
	if containsCoverageName(run.vllmMetrics.Coverage.PresentFields, "latency_e2e") {
		t.Fatalf("expected latency_e2e to be absent after dropping non-finite samples, got %+v", run.vllmMetrics.Coverage)
	}
	if !containsCoverageName(run.vllmMetrics.Coverage.MissingFields, "latency_e2e") {
		t.Fatalf("expected latency_e2e to be marked missing after dropping non-finite samples, got %+v", run.vllmMetrics.Coverage)
	}
	if run.vllmMetrics.LatencyE2E.HasData() {
		t.Fatalf("expected latency_e2e window to stay empty, got %+v", run.vllmMetrics.LatencyE2E)
	}
}

func matrixResult(points [][]any, labels map[string]string) []promRangeSeries {
	if labels == nil {
		labels = map[string]string{}
	}
	return []promRangeSeries{{
		Metric: labels,
		Values: points,
	}}
}

func mustQueryParam(t *testing.T, u *url.URL, key string) string {
	t.Helper()

	value := u.Query().Get(key)
	if value == "" {
		t.Fatalf("missing query parameter %q", key)
	}
	return value
}
