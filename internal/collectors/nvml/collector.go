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
	PowerLimit  *float64  `json:"power_limit,omitempty"`
	Temperature float64   `json:"temperature"`
	SMClock     float64   `json:"sm_clock"`
	MemoryClock float64   `json:"memory_clock"`
	PState      string    `json:"pstate,omitempty"`
	Throttle    string    `json:"throttle,omitempty"`
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
	lines = append(lines, "# HELP inferlean_nvml_power_limit_watts GPU enforced power limit in watts")
	lines = append(lines, "# TYPE inferlean_nvml_power_limit_watts gauge")
	lines = append(lines, "# HELP inferlean_nvml_temperature_celsius GPU temperature in celsius")
	lines = append(lines, "# TYPE inferlean_nvml_temperature_celsius gauge")
	lines = append(lines, "# HELP inferlean_nvml_sm_clock_mhz GPU SM clock in MHz")
	lines = append(lines, "# TYPE inferlean_nvml_sm_clock_mhz gauge")
	lines = append(lines, "# HELP inferlean_nvml_memory_clock_mhz GPU memory clock in MHz")
	lines = append(lines, "# TYPE inferlean_nvml_memory_clock_mhz gauge")
	lines = append(lines, "# HELP inferlean_nvml_performance_state_info GPU performance state label")
	lines = append(lines, "# TYPE inferlean_nvml_performance_state_info gauge")
	lines = append(lines, "# HELP inferlean_nvml_throttle_reason_active Active GPU clocks throttle reason label")
	lines = append(lines, "# TYPE inferlean_nvml_throttle_reason_active gauge")
	for _, sample := range samples {
		label := fmt.Sprintf("{gpu=\"%s\"}", promLabelValue(sample.GPU))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_gpu_utilization_percent%s %f", label, sample.Utilization))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_memory_used_mb%s %f", label, sample.MemoryUsed))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_memory_total_mb%s %f", label, sample.MemoryTotal))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_power_draw_watts%s %f", label, sample.PowerDraw))
		if sample.PowerLimit != nil {
			lines = append(lines, fmt.Sprintf("inferlean_nvml_power_limit_watts%s %f", label, *sample.PowerLimit))
		}
		lines = append(lines, fmt.Sprintf("inferlean_nvml_temperature_celsius%s %f", label, sample.Temperature))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_sm_clock_mhz%s %f", label, sample.SMClock))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_memory_clock_mhz%s %f", label, sample.MemoryClock))
		if strings.TrimSpace(sample.PState) != "" {
			lines = append(lines, fmt.Sprintf(
				"inferlean_nvml_performance_state_info{gpu=\"%s\",pstate=\"%s\"} 1",
				promLabelValue(sample.GPU),
				promLabelValue(sample.PState),
			))
		}
		for _, reason := range throttleReasons(sample.Throttle) {
			lines = append(lines, fmt.Sprintf(
				"inferlean_nvml_throttle_reason_active{gpu=\"%s\",reason=\"%s\"} 1",
				promLabelValue(sample.GPU),
				promLabelValue(reason),
			))
		}
	}
	return strings.Join(lines, "\n"), raw, nil
}

func querySamples(ctx context.Context) ([]Sample, string, error) {
	samples, raw, err := querySamplesWithFields(ctx, "index,utilization.gpu,memory.used,memory.total,power.draw,temperature.gpu,clocks.sm,clocks.mem,power.limit,pstate,clocks_throttle_reasons.active")
	if err == nil {
		return samples, raw, nil
	}
	return querySamplesWithFields(ctx, "index,utilization.gpu,memory.used,memory.total,power.draw,temperature.gpu,clocks.sm,clocks.mem")
}

func querySamplesWithFields(ctx context.Context, fields string) ([]Sample, string, error) {
	cmd := exec.CommandContext(
		ctx,
		"nvidia-smi",
		"--query-gpu="+fields,
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
	sample := Sample{
		GPU:         strings.TrimSpace(parts[0]),
		Utilization: parse(parts[1]),
		MemoryUsed:  parse(parts[2]),
		MemoryTotal: parse(parts[3]),
		PowerDraw:   parse(parts[4]),
		Temperature: parse(parts[5]),
	}
	if len(parts) >= 8 {
		sample.SMClock = parse(parts[6])
		sample.MemoryClock = parse(parts[7])
	}
	if len(parts) >= 9 {
		sample.PowerLimit = parseOptional(parts[8])
	}
	if len(parts) >= 10 {
		sample.PState = cleanText(parts[9])
	}
	if len(parts) >= 11 {
		sample.Throttle = cleanText(parts[10])
	}
	return sample, true
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

func parseOptional(value string) *float64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "N/A") || strings.EqualFold(trimmed, "not supported") {
		return nil
	}
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func cleanText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "N/A") {
		return ""
	}
	return trimmed
}

func throttleReasons(value string) []string {
	trimmed := cleanText(value)
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)
	if lower == "not active" || lower == "none" {
		return []string{"none"}
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == ';'
	})
	reasons := make([]string, 0, len(parts))
	for _, part := range parts {
		if reason := cleanText(part); reason != "" {
			reasons = append(reasons, reason)
		}
	}
	return reasons
}

func promLabelValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}

func CheckAvailability() error {
	_, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return fmt.Errorf("nvidia-smi not found")
	}
	return nil
}
