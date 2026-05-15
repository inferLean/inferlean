package nvml

import (
	"context"
	"fmt"
	"strings"
)

var promHeaders = []string{
	"# HELP inferlean_nvml_gpu_utilization_percent GPU utilization percentage",
	"# TYPE inferlean_nvml_gpu_utilization_percent gauge",
	"# HELP inferlean_nvml_memory_used_mb GPU memory used in MB",
	"# TYPE inferlean_nvml_memory_used_mb gauge",
	"# HELP inferlean_nvml_memory_total_mb GPU memory total in MB",
	"# TYPE inferlean_nvml_memory_total_mb gauge",
	"# HELP inferlean_nvml_power_draw_watts GPU power draw in watts",
	"# TYPE inferlean_nvml_power_draw_watts gauge",
	"# HELP inferlean_nvml_power_limit_watts GPU enforced power limit in watts",
	"# TYPE inferlean_nvml_power_limit_watts gauge",
	"# HELP inferlean_nvml_temperature_celsius GPU temperature in celsius",
	"# TYPE inferlean_nvml_temperature_celsius gauge",
	"# HELP inferlean_nvml_sm_clock_mhz GPU SM clock in MHz",
	"# TYPE inferlean_nvml_sm_clock_mhz gauge",
	"# HELP inferlean_nvml_memory_clock_mhz GPU memory clock in MHz",
	"# TYPE inferlean_nvml_memory_clock_mhz gauge",
	"# HELP inferlean_nvml_performance_state_info GPU performance state label",
	"# TYPE inferlean_nvml_performance_state_info gauge",
	"# HELP inferlean_nvml_throttle_reason_active Active GPU clocks throttle reason label",
	"# TYPE inferlean_nvml_throttle_reason_active gauge",
	"# HELP inferlean_nvml_pcie_rx_throughput_bytes_per_second PCIe receive throughput in bytes per second",
	"# TYPE inferlean_nvml_pcie_rx_throughput_bytes_per_second gauge",
	"# HELP inferlean_nvml_pcie_tx_throughput_bytes_per_second PCIe transmit throughput in bytes per second",
	"# TYPE inferlean_nvml_pcie_tx_throughput_bytes_per_second gauge",
	"# HELP inferlean_nvml_pcie_rx_capacity_bytes_per_second Estimated PCIe receive link capacity in bytes per second",
	"# TYPE inferlean_nvml_pcie_rx_capacity_bytes_per_second gauge",
	"# HELP inferlean_nvml_pcie_tx_capacity_bytes_per_second Estimated PCIe transmit link capacity in bytes per second",
	"# TYPE inferlean_nvml_pcie_tx_capacity_bytes_per_second gauge",
	"# HELP inferlean_nvml_nvlink_rx_capacity_bytes_per_second Estimated NVLink receive link capacity in bytes per second",
	"# TYPE inferlean_nvml_nvlink_rx_capacity_bytes_per_second gauge",
	"# HELP inferlean_nvml_nvlink_tx_capacity_bytes_per_second Estimated NVLink transmit link capacity in bytes per second",
	"# TYPE inferlean_nvml_nvlink_tx_capacity_bytes_per_second gauge",
}

func BuildPromMetrics(ctx context.Context) (string, string, error) {
	samples, raw, err := querySamples(ctx)
	if err != nil {
		return "", "", err
	}
	nvlinkCaps, nvlinkRaw := queryNVLinkCapacity(ctx)
	if strings.TrimSpace(nvlinkRaw) != "" {
		raw += "\n" + nvlinkRaw
	}

	lines := make([]string, 0, len(promHeaders)+len(samples)*16)
	lines = append(lines, promHeaders...)
	for _, sample := range samples {
		lines = appendPromSample(lines, sample, nvlinkCaps[sample.GPU])
	}
	return strings.Join(lines, "\n"), raw, nil
}

func appendPromSample(lines []string, sample Sample, nvlinkCap float64) []string {
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
	lines = appendPromLabels(lines, sample)
	if sample.PCIeRX != nil {
		lines = append(lines, fmt.Sprintf("inferlean_nvml_pcie_rx_throughput_bytes_per_second%s %f", label, *sample.PCIeRX))
	}
	if sample.PCIeTX != nil {
		lines = append(lines, fmt.Sprintf("inferlean_nvml_pcie_tx_throughput_bytes_per_second%s %f", label, *sample.PCIeTX))
	}
	if sample.PCIeRXCap != nil {
		lines = append(lines, fmt.Sprintf("inferlean_nvml_pcie_rx_capacity_bytes_per_second%s %f", label, *sample.PCIeRXCap))
	}
	if sample.PCIeTXCap != nil {
		lines = append(lines, fmt.Sprintf("inferlean_nvml_pcie_tx_capacity_bytes_per_second%s %f", label, *sample.PCIeTXCap))
	}
	if nvlinkCap > 0 {
		lines = append(lines, fmt.Sprintf("inferlean_nvml_nvlink_rx_capacity_bytes_per_second%s %f", label, nvlinkCap))
		lines = append(lines, fmt.Sprintf("inferlean_nvml_nvlink_tx_capacity_bytes_per_second%s %f", label, nvlinkCap))
	}
	return lines
}

func appendPromLabels(lines []string, sample Sample) []string {
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
	return lines
}

func promLabelValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}
