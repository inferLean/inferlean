package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/inferLean/inferlean/pkg/contracts"
)

var hostRequiredFields = []string{
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

func (r *collectionRun) captureHostMetrics(ctx context.Context) error {
	rawRef := relativeRawArtifact(r.rawPaths.hostRaw)
	coverage := newCoverageBuilder(rawRef)
	metrics := contracts.HostMetrics{}
	normalized := map[string]any{}
	lookback := vllmLookback(r.opts.ScrapeEvery)
	artifacts := []string{rawRef, relativeRawArtifact(r.rawPaths.hostNormalized)}
	if len(r.processSamples) > 0 {
		artifacts = append(artifacts, relativeRawArtifact(r.rawPaths.processSamples))
	}

	r.assignWindow(ctx, &metrics.CPUUtilization, coverage, normalized, "cpu_utilization", windowSpec(hostCPUUtilizationExpr(lookback)))
	r.assignWindow(ctx, &metrics.CPULoad, coverage, normalized, "cpu_load", windowSpec("avg(node_load1)"))
	r.assignWindow(ctx, &metrics.MemoryUsed, coverage, normalized, "memory_used", windowSpec("max(node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes)"))
	r.assignWindow(ctx, &metrics.MemoryAvailable, coverage, normalized, "memory_available", windowSpec("max(node_memory_MemAvailable_bytes)"))
	r.assignSwapPressure(ctx, &metrics.SwapPressure, coverage, normalized)
	r.assignWindow(ctx, &metrics.NetworkRX, coverage, normalized, "network_rx", windowSpec(hostNetworkExpr("receive", lookback)))
	r.assignWindow(ctx, &metrics.NetworkTX, coverage, normalized, "network_tx", windowSpec(hostNetworkExpr("transmit", lookback)))
	assignProcessWindow(&metrics.ProcessCPU, coverage, normalized, "process_cpu", r.processSamples, func(sample processSample) float64 {
		return sample.CPUPercent
	})
	assignProcessWindow(&metrics.ProcessMemory, coverage, normalized, "process_memory", r.processSamples, func(sample processSample) float64 {
		return sample.RSSBytes
	})

	metrics.Coverage = coverage.Build()
	r.hostMetrics = metrics
	r.hostCapture = captureFromCoverage(metrics.Coverage, artifacts, "host metrics were incomplete", hostRequiredFields)
	normalized["coverage"] = metrics.Coverage
	return writeJSONFile(r.rawPaths.hostNormalized, normalized)
}

func (r *collectionRun) assignSwapPressure(ctx context.Context, target *contracts.MetricWindow, coverage *coverageBuilder, normalized map[string]any) {
	expr := fmt.Sprintf("(max(node_memory_SwapTotal_bytes - node_memory_SwapFree_bytes)) / clamp_min(max(node_memory_SwapTotal_bytes), 1)")
	result, ok, err := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, windowSpec(expr))
	if err != nil {
		coverage.Missing("swap_pressure")
		normalized["swap_pressure"] = map[string]any{"error": err.Error()}
		return
	}
	if !ok {
		coverage.Unsupported("swap_pressure")
		return
	}
	*target = result.Window
	coverage.Present("swap_pressure")
	coverage.Derived("swap_pressure")
	normalized["swap_pressure"] = result
}

func assignProcessWindow(target *contracts.MetricWindow, coverage *coverageBuilder, normalized map[string]any, field string, samples []processSample, value func(processSample) float64) {
	points := make([]metricPoint, 0, len(samples))
	for _, sample := range samples {
		points = append(points, metricPoint{Timestamp: sample.Timestamp, Value: value(sample)})
	}
	if len(points) == 0 {
		coverage.Missing(field)
		return
	}
	*target = windowFromPoints(points)
	coverage.Present(field)
	normalized[field] = target
}

func hostCPUUtilizationExpr(lookback time.Duration) string {
	return fmt.Sprintf("100 * (1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[%s])))", lookback)
}

func hostNetworkExpr(direction string, lookback time.Duration) string {
	pattern := "device!~\"lo|docker.*|veth.*|br.*|virbr.*\""
	return fmt.Sprintf("sum(rate(node_network_%s_bytes_total{%s}[%s]))", direction, pattern, lookback)
}
