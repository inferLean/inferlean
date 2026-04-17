package nvml

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Sample struct {
	Timestamp   time.Time `json:"timestamp"`
	GPU         string    `json:"gpu"`
	Utilization float64   `json:"utilization"`
	MemoryUsed  float64   `json:"memory_used"`
	MemoryTotal float64   `json:"memory_total"`
	PowerDraw   float64   `json:"power_draw"`
	Temperature float64   `json:"temperature"`
}

type Result struct {
	RawText     string
	SourceState string
	Samples     []Sample
}

func Collect(ctx context.Context) Result {
	samples, raw, err := querySamples(ctx)
	if err != nil {
		return Result{SourceState: "missing"}
	}
	if len(samples) == 0 {
		return Result{RawText: raw, SourceState: "missing"}
	}
	now := time.Now().UTC()
	for i := range samples {
		samples[i].Timestamp = now
	}
	return Result{RawText: raw, SourceState: "ok", Samples: samples}
}

func BuildPromMetrics(ctx context.Context) (string, string, error) {
	samples, raw, err := querySamples(ctx)
	if err != nil {
		return "", "", err
	}
	lines := make([]string, 0, 8+len(samples)*5)
	lines = append(lines, "# HELP inferlean_nvml_gpu_utilization_percent GPU utilization percentage")
	lines = append(lines, "# TYPE inferlean_nvml_gpu_utilization_percent gauge")
	lines = append(lines, "# HELP inferlean_nvml_memory_used_mb GPU memory used in MB")
	lines = append(lines, "# TYPE inferlean_nvml_memory_used_mb gauge")
	lines = append(lines, "# HELP inferlean_nvml_memory_total_mb GPU memory total in MB")
	lines = append(lines, "# TYPE inferlean_nvml_memory_total_mb gauge")
	lines = append(lines, "# HELP inferlean_nvml_power_draw_watts GPU power draw in watts")
	lines = append(lines, "# TYPE inferlean_nvml_power_draw_watts gauge")
	lines = append(lines, "# HELP inferlean_nvml_temperature_celsius GPU temperature in celsius")
	lines = append(lines, "# TYPE inferlean_nvml_temperature_celsius gauge")
	for _, sample := range samples {
		label := fmt.Sprintf("{gpu=\"%s\"}", sample.GPU)
		lines = append(lines, fmt.Sprintf("inferlean_nvml_gpu_utilization_percent%s %f", label, sample.Utilization))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_memory_used_mb%s %f", label, sample.MemoryUsed))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_memory_total_mb%s %f", label, sample.MemoryTotal))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_power_draw_watts%s %f", label, sample.PowerDraw))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_temperature_celsius%s %f", label, sample.Temperature))
	}
	return strings.Join(lines, "\n"), raw, nil
}

func querySamples(ctx context.Context) ([]Sample, string, error) {
	cmd := exec.CommandContext(
		ctx,
		"nvidia-smi",
		"--query-gpu=index,utilization.gpu,memory.used,memory.total,power.draw,temperature.gpu",
		"--format=csv,noheader,nounits",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, "", err
	}
	raw := string(out)
	line := strings.TrimSpace(raw)
	if line == "" {
		return nil, raw, nil
	}
	records := strings.Split(line, "\n")
	samples := make([]Sample, 0, len(records))
	for _, record := range records {
		sample, ok := parseSample(record)
		if ok {
			samples = append(samples, sample)
		}
	}
	return samples, raw, nil
}

func parseSample(line string) (Sample, bool) {
	parts := splitCSV(line)
	if len(parts) < 6 {
		return Sample{}, false
	}
	return Sample{
		GPU:         strings.TrimSpace(parts[0]),
		Utilization: parse(parts[1]),
		MemoryUsed:  parse(parts[2]),
		MemoryTotal: parse(parts[3]),
		PowerDraw:   parse(parts[4]),
		Temperature: parse(parts[5]),
	}, true
}

func splitCSV(line string) []string {
	parts := strings.Split(line, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func parse(value string) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return parsed
}

func CheckAvailability() error {
	_, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return fmt.Errorf("nvidia-smi not found")
	}
	return nil
}
