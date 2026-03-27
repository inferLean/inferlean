package collector

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/inferLean/inferlean/internal/debug"
)

func captureNvidiaSMI(ctx context.Context, rawPath string) (sourceCapture, *nvidiaSnapshot) {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return missingCapture("nvidia-smi was not found in PATH"), nil
	}

	gpuOutput, err := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name,driver_version,utilization.gpu,memory.used,memory.total,power.draw,temperature.gpu,clocks.sm,clocks.mem", "--format=csv,noheader,nounits").Output()
	if err != nil {
		return degradedCapture(fmt.Sprintf("nvidia-smi gpu query failed: %v", err)), nil
	}

	processOutput, processErr := exec.CommandContext(ctx, "nvidia-smi", "--query-compute-apps=pid,process_name,used_gpu_memory,gpu_uuid", "--format=csv,noheader,nounits").Output()
	if processErr != nil {
		debug.Debugf("nvidia-smi process query failed: %v", processErr)
	}
	if err := os.WriteFile(rawPath, joinedNvidiaOutput(gpuOutput, processOutput), 0o600); err != nil {
		return degradedCapture(fmt.Sprintf("write nvidia-smi raw output: %v", err)), nil
	}

	snapshot := parseNvidiaSMIOutput(gpuOutput, processOutput)
	capture := sourceCapture{Status: "ok", Artifacts: []string{relativeRawArtifact(rawPath)}, MetricPayload: buildNvidiaPayload(rawPath, snapshot)}
	if processErr != nil {
		capture.Status = "degraded"
		capture.Reason = "gpu metrics were captured but per-process GPU memory was unavailable"
	}
	return capture, snapshot
}

func joinedNvidiaOutput(gpuOutput, processOutput []byte) []byte {
	if len(processOutput) == 0 {
		return gpuOutput
	}
	return []byte(string(gpuOutput) + "\n# compute_apps\n" + string(processOutput))
}

func buildNvidiaPayload(rawPath string, snapshot *nvidiaSnapshot) map[string]any {
	payload := map[string]any{"raw_evidence_ref": relativeRawArtifact(rawPath)}
	if snapshot == nil {
		return payload
	}
	payload["gpus"] = snapshot.GPUs
	payload["processes"] = snapshot.Processes
	payload["gpu_count"] = len(snapshot.GPUs)
	if snapshot.DriverVersion != "" {
		payload["driver_version"] = snapshot.DriverVersion
	}
	return payload
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
