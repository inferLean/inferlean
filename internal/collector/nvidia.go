package collector

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/inferLean/inferlean/internal/debug"
	"github.com/inferLean/inferlean/pkg/contracts"
)

var nvidiaRequiredFields = []string{
	"gpu_utilization",
	"memory_used",
	"memory_total",
	"power_draw",
	"temperature",
	"sm_clock",
	"mem_clock",
	"process_gpu_memory",
}

func captureNvidiaSMIMetrics(ctx context.Context, rawPath string) (sourceCapture, contracts.NvidiaSMIMetrics, *nvidiaSnapshot) {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return sourceCapture{Status: "missing", Reason: "nvidia-smi was not found in PATH"}, contracts.NvidiaSMIMetrics{
			Coverage: missingCoverage(nvidiaRequiredFields, ""),
		}, nil
	}

	gpuOutput, err := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name,driver_version,utilization.gpu,memory.used,memory.total,power.draw,temperature.gpu,clocks.sm,clocks.mem", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return sourceCapture{Status: "missing", Reason: fmt.Sprintf("nvidia-smi gpu query failed: %v", err)}, contracts.NvidiaSMIMetrics{
			Coverage: missingCoverage(nvidiaRequiredFields, ""),
		}, nil
	}

	processOutput, processErr := exec.CommandContext(ctx, "nvidia-smi", "--query-compute-apps=pid,process_name,used_gpu_memory,gpu_uuid", "--format=csv,noheader,nounits").Output()
	if processErr != nil {
		debug.Debugf("nvidia-smi process query failed: %v", processErr)
	}
	if err := os.WriteFile(rawPath, joinedNvidiaOutput(gpuOutput, processOutput), 0o600); err != nil {
		return sourceCapture{Status: "degraded", Reason: fmt.Sprintf("write nvidia-smi raw output: %v", err)}, contracts.NvidiaSMIMetrics{
			Coverage: missingCoverage(nvidiaRequiredFields, ""),
		}, nil
	}

	snapshot := parseNvidiaSMIOutput(gpuOutput, processOutput)
	metrics := nvidiaMetricsFromSnapshot(snapshot, processErr == nil, relativeRawArtifact(rawPath))
	capture := captureFromCoverage(metrics.Coverage, []string{relativeRawArtifact(rawPath)}, "nvidia-smi metrics were incomplete", nvidiaRequiredFields)
	if processErr != nil && capture.Status == "ok" {
		capture.Status = "degraded"
		capture.Reason = "gpu metrics were captured but per-process GPU memory was unavailable"
	}
	return capture, metrics, snapshot
}

func joinedNvidiaOutput(gpuOutput, processOutput []byte) []byte {
	if len(processOutput) == 0 {
		return gpuOutput
	}
	return []byte(string(gpuOutput) + "\n# compute_apps\n" + string(processOutput))
}

func nvidiaMetricsFromSnapshot(snapshot *nvidiaSnapshot, hasProcesses bool, rawRef string) contracts.NvidiaSMIMetrics {
	coverage := newCoverageBuilder(rawRef)
	metrics := contracts.NvidiaSMIMetrics{}
	if snapshot == nil || len(snapshot.GPUs) == 0 {
		metrics.Coverage = missingCoverage(nvidiaRequiredFields, rawRef)
		return metrics
	}

	value, ok := nvidiaAverage(snapshot.GPUs, func(gpu nvidiaGPU) (float64, bool) { return parseFloatField(gpu.Utilization) })
	recordWindow(&metrics.GPUUtilization, coverage, "gpu_utilization", value, ok)
	value, ok = nvidiaSum(snapshot.GPUs, func(gpu nvidiaGPU) (float64, bool) { return parseFloatField(gpu.MemoryUsedMiB) })
	recordWindow(&metrics.MemoryUsed, coverage, "memory_used", value, ok)
	value, ok = nvidiaSum(snapshot.GPUs, func(gpu nvidiaGPU) (float64, bool) { return parseFloatField(gpu.MemoryTotalMiB) })
	recordWindow(&metrics.MemoryTotal, coverage, "memory_total", value, ok)
	value, ok = nvidiaAverage(snapshot.GPUs, func(gpu nvidiaGPU) (float64, bool) { return parseFloatField(gpu.PowerDrawW) })
	recordWindow(&metrics.PowerDraw, coverage, "power_draw", value, ok)
	value, ok = nvidiaAverage(snapshot.GPUs, func(gpu nvidiaGPU) (float64, bool) { return parseFloatField(gpu.TemperatureC) })
	recordWindow(&metrics.Temperature, coverage, "temperature", value, ok)
	value, ok = nvidiaAverage(snapshot.GPUs, func(gpu nvidiaGPU) (float64, bool) { return parseFloatField(gpu.SMClockMHz) })
	recordWindow(&metrics.SMClock, coverage, "sm_clock", value, ok)
	value, ok = nvidiaAverage(snapshot.GPUs, func(gpu nvidiaGPU) (float64, bool) { return parseFloatField(gpu.MemClockMHz) })
	recordWindow(&metrics.MemClock, coverage, "mem_clock", value, ok)
	if hasProcesses {
		value, ok = nvidiaProcessMemory(snapshot.Processes)
		recordWindow(&metrics.ProcessGPUMemory, coverage, "process_gpu_memory", value, ok)
	} else {
		coverage.Missing("process_gpu_memory")
	}

	metrics.Coverage = coverage.Build()
	return metrics
}

func recordWindow(target *contracts.MetricWindow, coverage *coverageBuilder, field string, value float64, ok bool) {
	if !ok {
		coverage.Missing(field)
		return
	}
	point := metricPoint{Timestamp: time.Now().UTC(), Value: value}
	*target = windowFromPoints([]metricPoint{point})
	coverage.Present(field)
}

func nvidiaAverage(gpus []nvidiaGPU, value func(nvidiaGPU) (float64, bool)) (float64, bool) {
	total := 0.0
	count := 0.0
	for _, gpu := range gpus {
		current, ok := value(gpu)
		if !ok {
			continue
		}
		total += current
		count++
	}
	if count == 0 {
		return 0, false
	}
	return total / count, true
}

func nvidiaSum(gpus []nvidiaGPU, value func(nvidiaGPU) (float64, bool)) (float64, bool) {
	total := 0.0
	count := 0
	for _, gpu := range gpus {
		current, ok := value(gpu)
		if !ok {
			continue
		}
		total += current
		count++
	}
	return total, count > 0
}

func nvidiaProcessMemory(processes []nvidiaProcess) (float64, bool) {
	total := 0.0
	count := 0
	for _, proc := range processes {
		current, ok := parseFloatField(proc.GPUMemoryMiB)
		if !ok {
			continue
		}
		total += current
		count++
	}
	return total, count > 0
}

func parseNvidiaSMIOutput(gpuOutput, processOutput []byte) *nvidiaSnapshot {
	snapshot := &nvidiaSnapshot{}
	snapshot.GPUs = parseNvidiaGPUs(gpuOutput)
	snapshot.Processes = parseNvidiaProcesses(processOutput)
	if len(snapshot.GPUs) > 0 {
		snapshot.DriverVersion = snapshot.GPUs[0].DriverVersion
	}
	return snapshot
}

func parseNvidiaGPUs(output []byte) []nvidiaGPU {
	var gpus []nvidiaGPU
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := splitCSVLine(line)
		gpus = append(gpus, nvidiaGPU{
			Name:           field(parts, 0),
			DriverVersion:  field(parts, 1),
			Utilization:    field(parts, 2),
			MemoryUsedMiB:  field(parts, 3),
			MemoryTotalMiB: field(parts, 4),
			PowerDrawW:     field(parts, 5),
			TemperatureC:   field(parts, 6),
			SMClockMHz:     field(parts, 7),
			MemClockMHz:    field(parts, 8),
		})
	}
	return gpus
}

func parseNvidiaProcesses(output []byte) []nvidiaProcess {
	var processes []nvidiaProcess
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.Contains(strings.ToLower(trimmed), "no running processes found") {
			continue
		}
		parts := splitCSVLine(trimmed)
		processes = append(processes, nvidiaProcess{
			PID:          field(parts, 0),
			ProcessName:  field(parts, 1),
			GPUMemoryMiB: field(parts, 2),
			GPUUUID:      field(parts, 3),
		})
	}
	return processes
}

func splitCSVLine(line string) []string {
	parts := strings.Split(line, ",")
	for idx := range parts {
		parts[idx] = strings.TrimSpace(parts[idx])
	}
	return parts
}

func field(parts []string, index int) string {
	if index >= len(parts) {
		return ""
	}
	return parts[index]
}

func parseFloatField(raw string) (float64, bool) {
	if strings.TrimSpace(raw) == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
