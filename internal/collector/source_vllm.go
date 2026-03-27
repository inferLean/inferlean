package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/inferLean/inferlean/pkg/contracts"
)

var vllmRequiredFields = []string{
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
	"multimodal_preprocessing",
}

func (r *collectionRun) captureVLLMMetrics(ctx context.Context) error {
	if r.vllmTarget == "" {
		r.vllmMetrics = contracts.VLLMMetrics{Coverage: missingCoverage(vllmRequiredFields, "")}
		r.vllmCapture = sourceCapture{Status: "missing", Reason: "vLLM metrics endpoint was not configured"}
		return nil
	}

	rawRef := relativeRawArtifact(r.rawPaths.vllmRaw)
	coverage := newCoverageBuilder(rawRef)
	metrics := contracts.VLLMMetrics{}
	lookback := vllmLookback(r.opts.ScrapeEvery)
	normalized := map[string]any{}
	artifacts := []string{rawRef, relativeRawArtifact(r.rawPaths.vllmNormalized)}

	r.assignWindow(ctx, &metrics.RequestsRunning, coverage, normalized, "requests_running", windowSpec("sum(vllm:num_requests_running)", "sum(vllm_num_requests_running)"))
	r.assignWindow(ctx, &metrics.RequestsWaiting, coverage, normalized, "requests_waiting", windowSpec("sum(vllm:num_requests_waiting)", "sum(vllm_num_requests_waiting)"))
	r.assignLatency(ctx, &metrics.LatencyE2E, coverage, normalized, "latency_e2e", "vllm:e2e_request_latency_seconds", lookback)
	r.assignLatency(ctx, &metrics.LatencyTTFT, coverage, normalized, "latency_ttft", "vllm:time_to_first_token_seconds", lookback)
	r.assignLatency(ctx, &metrics.LatencyQueue, coverage, normalized, "latency_queue", "vllm:request_queue_time_seconds", lookback)
	r.assignLatency(ctx, &metrics.LatencyPrefill, coverage, normalized, "latency_prefill", "vllm:request_prefill_time_seconds", lookback)
	r.assignLatency(ctx, &metrics.LatencyDecode, coverage, normalized, "latency_decode", "vllm:request_decode_time_seconds", lookback)
	r.assignWindow(ctx, &metrics.PromptTokens, coverage, normalized, "prompt_tokens", sumMetricSpec("vllm:prompt_tokens_total", "vllm_prompt_tokens_total", "vllm:prompt_tokens", "vllm_prompt_tokens"))
	r.assignWindow(ctx, &metrics.GenerationTokens, coverage, normalized, "generation_tokens", sumMetricSpec("vllm:generation_tokens_total", "vllm_generation_tokens_total", "vllm:generation_tokens", "vllm_generation_tokens"))
	r.assignHistogram(ctx, &metrics.PromptLength, coverage, normalized, "prompt_length", "vllm:request_prompt_tokens")
	r.assignHistogram(ctx, &metrics.GenerationLength, coverage, normalized, "generation_length", "vllm:request_generation_tokens")
	r.assignWindow(ctx, &metrics.KVCacheUsage, coverage, normalized, "kv_cache_usage", windowSpec("avg(vllm:kv_cache_usage_perc)", "avg(vllm_kv_cache_usage_perc)"))
	r.assignWindow(ctx, &metrics.Preemptions, coverage, normalized, "preemptions", sumMetricSpec("vllm:num_preemptions_total", "vllm_num_preemptions_total", "vllm:num_preemptions", "vllm_num_preemptions"))
	r.assignWindow(ctx, &metrics.RecomputedPromptTokens, coverage, normalized, "recomputed_prompt_tokens", sumMetricSpec("vllm:prompt_tokens_recomputed_total", "vllm_prompt_tokens_recomputed_total", "vllm:prompt_tokens_recomputed", "vllm_prompt_tokens_recomputed"))
	metrics.PrefixCache = r.captureCache(ctx, coverage, normalized, "prefix_cache", sumMetricSpec("vllm:prefix_cache_hits_total", "vllm_prefix_cache_hits_total", "vllm:prefix_cache_hits", "vllm_prefix_cache_hits"), sumMetricSpec("vllm:prefix_cache_queries_total", "vllm_prefix_cache_queries_total", "vllm:prefix_cache_queries", "vllm_prefix_cache_queries"))
	metrics.MultimodalCache = r.captureCache(ctx, coverage, normalized, "multimodal_cache", sumMetricSpec("vllm:mm_cache_hits_total", "vllm_mm_cache_hits_total", "vllm:mm_cache_hits", "vllm_mm_cache_hits"), sumMetricSpec("vllm:mm_cache_queries_total", "vllm_mm_cache_queries_total", "vllm:mm_cache_queries", "vllm_mm_cache_queries"))
	r.assignLatency(ctx, &metrics.MultimodalPreprocessing, coverage, normalized, "multimodal_preprocessing", "vllm:request_inference_time_seconds", lookback)

	metrics.Coverage = coverage.Build()
	r.vllmMetrics = metrics
	r.vllmCapture = captureFromCoverage(metrics.Coverage, artifacts, "vLLM metrics were incomplete", vllmRequiredFields)

	normalized["coverage"] = metrics.Coverage
	return writeJSONFile(r.rawPaths.vllmNormalized, normalized)
}

func (r *collectionRun) assignWindow(ctx context.Context, target *contracts.MetricWindow, coverage *coverageBuilder, normalized map[string]any, field string, exprs []string) {
	result, ok, err := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, exprs)
	if err != nil {
		coverage.Missing(field)
		normalized[field] = map[string]any{"error": err.Error()}
		return
	}
	if !ok {
		coverage.Missing(field)
		return
	}
	*target = result.Window
	coverage.Present(field)
	normalized[field] = result
}

func (r *collectionRun) assignLatency(ctx context.Context, target *contracts.MetricWindow, coverage *coverageBuilder, normalized map[string]any, field, base string, lookback time.Duration) {
	expr := fmt.Sprintf("sum(rate(%s_sum[%s])) / sum(rate(%s_count[%s]))", base, lookback, base, lookback)
	r.assignWindow(ctx, target, coverage, normalized, field, windowSpec(expr))
	coverage.Derived(field)
}

func (r *collectionRun) assignHistogram(ctx context.Context, target *contracts.DistributionSnapshot, coverage *coverageBuilder, normalized map[string]any, field string, bases ...string) {
	result, ok, err := r.service.firstHistogram(ctx, r.promBase, r.collectEnded, bases)
	if err != nil {
		coverage.Missing(field)
		normalized[field] = map[string]any{"error": err.Error()}
		return
	}
	if !ok {
		coverage.Missing(field)
		return
	}
	*target = result.Distribution
	coverage.Present(field)
	normalized[field] = result
}

func (r *collectionRun) captureCache(ctx context.Context, coverage *coverageBuilder, normalized map[string]any, field string, hitsExprs, queriesExprs []string) contracts.CacheSnapshot {
	hits, hitsOK, hitsErr := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, hitsExprs)
	queries, queriesOK, queriesErr := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, queriesExprs)
	if hitsErr != nil || queriesErr != nil {
		coverage.Missing(field)
		normalized[field] = map[string]any{"error": firstError(hitsErr, queriesErr)}
		return contracts.CacheSnapshot{}
	}
	if !hitsOK && !queriesOK {
		coverage.Missing(field)
		return contracts.CacheSnapshot{}
	}

	cache := cacheSnapshot(hits.Window, queries.Window)
	if cache.HasData() {
		coverage.Present(field)
		coverage.Derived(field)
	}
	normalized[field] = map[string]any{"hits": hits, "queries": queries, "cache": cache}
	return cache
}

func missingCoverage(required []string, rawRef string) contracts.SourceCoverage {
	coverage := newCoverageBuilder(rawRef)
	for _, field := range required {
		coverage.Missing(field)
	}
	return coverage.Build()
}

func windowSpec(exprs ...string) []string {
	return exprs
}

func sumMetricSpec(names ...string) []string {
	exprs := make([]string, 0, len(names))
	for _, name := range names {
		exprs = append(exprs, "sum("+name+")")
	}
	return exprs
}

func vllmLookback(step time.Duration) time.Duration {
	if step*2 >= 10*time.Second {
		return step * 2
	}
	return 10 * time.Second
}

func firstError(errs ...error) string {
	for _, err := range errs {
		if err != nil {
			return err.Error()
		}
	}
	return ""
}
