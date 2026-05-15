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
	PCIeRXCap   *float64  `json:"pcie_rx_capacity_bytes_per_second,omitempty"`
	PCIeTXCap   *float64  `json:"pcie_tx_capacity_bytes_per_second,omitempty"`
	PCIeRX      *float64  `json:"pcie_rx_bytes_per_second,omitempty"`
	PCIeTX      *float64  `json:"pcie_tx_bytes_per_second,omitempty"`
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

func querySamples(ctx context.Context) ([]Sample, string, error) {
	samples, raw, err := querySamplesWithFields(ctx, "index,utilization.gpu,memory.used,memory.total,power.draw,temperature.gpu,clocks.sm,clocks.mem,power.limit,pstate,clocks_throttle_reasons.active,pcie.link.gen.current,pcie.link.width.current,pcie.link.gen.max,pcie.link.width.max")
	if err != nil {
		samples, raw, err = querySamplesWithFields(ctx, "index,utilization.gpu,memory.used,memory.total,power.draw,temperature.gpu,clocks.sm,clocks.mem")
	}
	if err != nil {
		return samples, raw, err
	}
	throughput, throughputRaw := queryPCIeThroughput(ctx)
	if strings.TrimSpace(throughputRaw) != "" {
		raw = strings.TrimRight(raw, "\n") + "\n" + throughputRaw
	}
	for idx := range samples {
		if metric, ok := throughput[samples[idx].GPU]; ok {
			samples[idx].PCIeRX = floatPtr(metric.RX)
			samples[idx].PCIeTX = floatPtr(metric.TX)
		}
	}
	return samples, raw, nil
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
	if len(parts) >= 15 {
		if capacity, ok := pcieCapacity(parts[11], parts[12], parts[13], parts[14]); ok {
			sample.PCIeRXCap = &capacity
			sample.PCIeTXCap = &capacity
		}
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
	if trimmed == "" || trimmed == "-" || strings.EqualFold(trimmed, "N/A") || strings.EqualFold(trimmed, "not supported") {
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

func floatPtr(value float64) *float64 {
	return &value
}

func throttleReasons(value string) []string {
	trimmed := cleanText(value)
	if trimmed == "" {
		return nil
	}
	if reasons, ok := throttleMaskReasons(trimmed); ok {
		return reasons
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

func throttleMaskReasons(value string) ([]string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "0x") {
		return nil, false
	}
	mask, err := strconv.ParseUint(value, 0, 64)
	if err != nil {
		return nil, false
	}
	if mask == 0 {
		return []string{"none"}, true
	}
	reasons := []struct {
		bit  uint64
		name string
	}{
		{0x1, "gpu_idle"},
		{0x2, "applications_clocks_setting"},
		{0x4, "sw_power_cap"},
		{0x8, "hw_slowdown"},
		{0x10, "sync_boost"},
		{0x20, "sw_thermal_slowdown"},
		{0x40, "hw_thermal_slowdown"},
		{0x80, "hw_power_brake"},
		{0x100, "display_clock_setting"},
	}
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if mask&reason.bit != 0 {
			out = append(out, reason.name)
		}
	}
	if len(out) == 0 {
		return []string{strings.ToLower(value)}, true
	}
	return out, true
}

func CheckAvailability() error {
	_, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return fmt.Errorf("nvidia-smi not found")
	}
	return nil
}
