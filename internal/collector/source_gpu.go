package collector

import (
	"context"
	"os"

	"github.com/inferLean/inferlean/pkg/contracts"
)

var gpuRequiredFields = []string{
	"gpu_utilization_or_sm_activity",
	"framebuffer_memory",
	"memory_bandwidth",
	"clocks",
	"power",
	"temperature",
	"pcie_throughput",
	"nvlink_throughput",
	"reliability_errors",
}

func (r *collectionRun) captureGPUTelemetry(ctx context.Context) error {
	dcgmMetrics, dcgmCoverage, normalized, err := r.captureDCGMMetrics(ctx)
	if err != nil {
		return err
	}

	nvmlMetrics, nvmlCoverage := nvmlMetricsFromSnapshot(r.nvmlSnapshot, r.nvmlCoverage)
	metrics, coverage := mergeGPUCoverage(dcgmMetrics, dcgmCoverage, nvmlMetrics, nvmlCoverage)
	if !coverageHasEntries(coverage) {
		coverage = missingCoverage(gpuRequiredFields, relativeRawArtifact(r.rawPaths.nvmlRaw))
	}
	r.gpuMetrics = metrics
	r.gpuMetrics.Coverage = coverage
	r.gpuCapture = captureFromCoverage(coverage, gpuArtifacts(r), "rich GPU telemetry was incomplete", gpuRequiredFields)
	normalized["merged"] = r.gpuMetrics
	return writeJSONFile(r.rawPaths.gpuNormalized, normalized)
}

func (r *collectionRun) captureDCGMMetrics(ctx context.Context) (contracts.GPUTelemetry, contracts.SourceCoverage, map[string]any, error) {
	if r.dcgmTarget == "" {
		return contracts.GPUTelemetry{}, contracts.SourceCoverage{}, map[string]any{"dcgm": "not detected"}, nil
	}

	rawRef := relativeRawArtifact(r.rawPaths.dcgmRaw)
	coverage := newCoverageBuilder(rawRef)
	metrics := contracts.GPUTelemetry{}
	normalized := map[string]any{}

	r.assignWindow(ctx, &metrics.GPUUtilizationOrSMActivity, coverage, normalized, "gpu_utilization_or_sm_activity", windowSpec("avg(DCGM_FI_PROF_GR_ENGINE_ACTIVE)", "avg(DCGM_FI_DEV_GPU_UTIL)"))
	assignMemoryWindow(ctx, r, &metrics.FramebufferMemory, coverage, normalized, "framebuffer_memory", "sum(DCGM_FI_DEV_FB_USED)", "sum(DCGM_FI_DEV_FB_FREE)")
	r.assignWindow(ctx, &metrics.MemoryBandwidth, coverage, normalized, "memory_bandwidth", windowSpec("avg(DCGM_FI_PROF_DRAM_ACTIVE)"))
	assignClockWindow(ctx, r, &metrics.Clocks, coverage, normalized, "clocks", "avg(DCGM_FI_DEV_SM_CLOCK)", "avg(DCGM_FI_DEV_MEM_CLOCK)")
	r.assignWindow(ctx, &metrics.Power, coverage, normalized, "power", windowSpec("avg(DCGM_FI_DEV_POWER_USAGE)"))
	r.assignWindow(ctx, &metrics.Temperature, coverage, normalized, "temperature", windowSpec("avg(DCGM_FI_DEV_GPU_TEMP)"))
	assignThroughputWindow(ctx, r, &metrics.PCIeThroughput, coverage, normalized, "pcie_throughput", "sum(DCGM_FI_PROF_PCIE_RX_BYTES)", "sum(DCGM_FI_PROF_PCIE_TX_BYTES)")
	assignThroughputWindow(ctx, r, &metrics.NVLinkThroughput, coverage, normalized, "nvlink_throughput", "sum(DCGM_FI_PROF_NVLINK_RX_BYTES)", "sum(DCGM_FI_PROF_NVLINK_TX_BYTES)")
	assignReliabilityWindow(ctx, r, &metrics.ReliabilityErrors, coverage, normalized, "reliability_errors", "sum(DCGM_FI_DEV_XID_ERRORS)", "sum(DCGM_FI_DEV_ECC_DBE_VOL_TOTAL)")

	return metrics, coverage.Build(), normalized, nil
}

func nvmlMetricsFromSnapshot(snapshot *nvmlSnapshot, coverage contracts.SourceCoverage) (contracts.GPUTelemetry, contracts.SourceCoverage) {
	metrics := contracts.GPUTelemetry{}
	if snapshot == nil || len(snapshot.Samples) == 0 {
		return metrics, coverage
	}

	latest := snapshot.Samples[len(snapshot.Samples)-1]
	points := nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.Utilization })
	metrics.GPUUtilizationOrSMActivity = windowFromPoints(points)
	metrics.FramebufferMemory = contracts.MemoryMetrics{
		Used:  windowFromPoints(nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.MemoryUsedBytes })),
		Free:  windowFromPoints(nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.MemoryFreeBytes })),
		Total: windowFromPoints(nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.MemoryTotalBytes })),
	}
	metrics.Clocks = contracts.ClockMetrics{
		SM:     windowFromPoints(nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.SMClockMHz })),
		Memory: windowFromPoints(nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.MemClockMHz })),
	}
	metrics.Power = windowFromPoints(nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.PowerDrawWatts }))
	metrics.Temperature = windowFromPoints(nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.TemperatureC }))
	metrics.PCIeThroughput = contracts.ThroughputMetrics{
		RX: windowFromPoints(nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.PCIeRxKBs })),
		TX: windowFromPoints(nvmlPoints(snapshot, func(gpu nvmlGPU) float64 { return gpu.PCIeTxKBs })),
	}
	metrics.Coverage = coverage
	if latest.Driver != "" {
		metrics.ReliabilityErrors = contracts.ReliabilityMetrics{}
	}
	return metrics, metrics.Coverage
}

func nvmlPoints(snapshot *nvmlSnapshot, value func(nvmlGPU) float64) []metricPoint {
	points := make([]metricPoint, 0, len(snapshot.Samples))
	for _, sample := range snapshot.Samples {
		total := 0.0
		count := 0.0
		for _, gpu := range sample.GPUs {
			total += value(gpu)
			count++
		}
		if count == 0 {
			continue
		}
		points = append(points, metricPoint{Timestamp: sample.Timestamp, Value: total / count})
	}
	return points
}

func mergeGPUCoverage(primary contracts.GPUTelemetry, primaryCoverage contracts.SourceCoverage, secondary contracts.GPUTelemetry, secondaryCoverage contracts.SourceCoverage) (contracts.GPUTelemetry, contracts.SourceCoverage) {
	merged := contracts.GPUTelemetry{
		GPUUtilizationOrSMActivity: preferWindow(primary.GPUUtilizationOrSMActivity, secondary.GPUUtilizationOrSMActivity),
		FramebufferMemory:          preferMemory(primary.FramebufferMemory, secondary.FramebufferMemory),
		MemoryBandwidth:            preferWindow(primary.MemoryBandwidth, secondary.MemoryBandwidth),
		Clocks:                     preferClocks(primary.Clocks, secondary.Clocks),
		Power:                      preferWindow(primary.Power, secondary.Power),
		Temperature:                preferWindow(primary.Temperature, secondary.Temperature),
		PCIeThroughput:             preferThroughput(primary.PCIeThroughput, secondary.PCIeThroughput),
		NVLinkThroughput:           preferThroughput(primary.NVLinkThroughput, secondary.NVLinkThroughput),
		ReliabilityErrors:          preferReliability(primary.ReliabilityErrors, secondary.ReliabilityErrors),
	}
	coverage := mergeCoverage(primaryCoverage, secondaryCoverage)
	return merged, coverage
}

func mergeCoverage(primary, secondary contracts.SourceCoverage) contracts.SourceCoverage {
	coverage := newCoverageBuilder(primary.RawEvidenceRef)
	applyCoverageField(coverage.Present, primary.PresentFields, secondary.PresentFields)
	applyMissingCoverage(coverage, primary.MissingFields, secondary.MissingFields)
	applyUnsupportedCoverage(coverage, primary.UnsupportedFields, secondary.UnsupportedFields)
	for _, name := range append(primary.DerivedFields, secondary.DerivedFields...) {
		coverage.Derived(name)
	}
	if coverage.rawEvidenceRef == "" {
		coverage.rawEvidenceRef = secondary.RawEvidenceRef
	}
	return coverage.Build()
}

func coverageHasEntries(coverage contracts.SourceCoverage) bool {
	return len(coverage.PresentFields)+len(coverage.MissingFields)+len(coverage.UnsupportedFields) > 0
}

func applyCoverageField(mark func(string), groups ...[]string) {
	for _, group := range groups {
		for _, name := range group {
			mark(name)
		}
	}
}

func applyMissingCoverage(coverage *coverageBuilder, groups ...[]string) {
	for _, group := range groups {
		for _, name := range group {
			if !containsCoverageName(coverage.presentNames(), name) {
				coverage.Missing(name)
			}
		}
	}
}

func applyUnsupportedCoverage(coverage *coverageBuilder, groups ...[]string) {
	for _, group := range groups {
		for _, name := range group {
			if !containsCoverageName(coverage.presentNames(), name) {
				coverage.Unsupported(name)
			}
		}
	}
}

func gpuArtifacts(r *collectionRun) []string {
	artifacts := []string{relativeRawArtifact(r.rawPaths.gpuNormalized)}
	for _, path := range []string{r.rawPaths.dcgmRaw, r.rawPaths.nvmlRaw} {
		if path != "" && fileExists(path) {
			artifacts = append(artifacts, relativeRawArtifact(path))
		}
	}
	return artifacts
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func preferWindow(primary, secondary contracts.MetricWindow) contracts.MetricWindow {
	if primary.HasData() {
		return primary
	}
	return secondary
}

func preferMemory(primary, secondary contracts.MemoryMetrics) contracts.MemoryMetrics {
	if primary.HasData() {
		return primary
	}
	return secondary
}

func preferClocks(primary, secondary contracts.ClockMetrics) contracts.ClockMetrics {
	if primary.HasData() {
		return primary
	}
	return secondary
}

func preferThroughput(primary, secondary contracts.ThroughputMetrics) contracts.ThroughputMetrics {
	if primary.HasData() {
		return primary
	}
	return secondary
}

func preferReliability(primary, secondary contracts.ReliabilityMetrics) contracts.ReliabilityMetrics {
	if primary.HasData() {
		return primary
	}
	return secondary
}

func assignMemoryWindow(ctx context.Context, r *collectionRun, target *contracts.MemoryMetrics, coverage *coverageBuilder, normalized map[string]any, field, usedExpr, freeExpr string) {
	used, usedOK, _ := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, windowSpec(usedExpr))
	free, freeOK, _ := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, windowSpec(freeExpr))
	if !usedOK && !freeOK {
		coverage.Missing(field)
		return
	}
	target.Used = used.Window
	target.Free = free.Window
	if usedOK && freeOK && used.Window.Latest != nil && free.Window.Latest != nil {
		target.Total = windowFromPoints([]metricPoint{{Timestamp: r.collectEnded, Value: *used.Window.Latest + *free.Window.Latest}})
	}
	coverage.Present(field)
	normalized[field] = target
}

func assignClockWindow(ctx context.Context, r *collectionRun, target *contracts.ClockMetrics, coverage *coverageBuilder, normalized map[string]any, field, smExpr, memExpr string) {
	sm, smOK, _ := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, windowSpec(smExpr))
	mem, memOK, _ := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, windowSpec(memExpr))
	if !smOK && !memOK {
		coverage.Missing(field)
		return
	}
	target.SM = sm.Window
	target.Memory = mem.Window
	coverage.Present(field)
	normalized[field] = target
}

func assignThroughputWindow(ctx context.Context, r *collectionRun, target *contracts.ThroughputMetrics, coverage *coverageBuilder, normalized map[string]any, field, rxExpr, txExpr string) {
	rx, rxOK, _ := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, windowSpec(rxExpr))
	tx, txOK, _ := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, windowSpec(txExpr))
	if !rxOK && !txOK {
		coverage.Unsupported(field)
		return
	}
	target.RX = rx.Window
	target.TX = tx.Window
	coverage.Present(field)
	normalized[field] = target
}

func assignReliabilityWindow(ctx context.Context, r *collectionRun, target *contracts.ReliabilityMetrics, coverage *coverageBuilder, normalized map[string]any, field, xidExpr, eccExpr string) {
	xid, xidOK, _ := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, windowSpec(xidExpr))
	ecc, eccOK, _ := r.service.firstWindow(ctx, r.promBase, r.collectStarted, r.collectEnded, r.opts.ScrapeEvery, windowSpec(eccExpr))
	if !xidOK && !eccOK {
		coverage.Unsupported(field)
		return
	}
	target.XID = xid.Window
	target.ECC = ecc.Window
	coverage.Present(field)
	normalized[field] = target
}
