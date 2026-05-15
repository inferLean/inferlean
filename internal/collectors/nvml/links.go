package nvml

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

type pcieThroughput struct {
	RX float64
	TX float64
}

func pcieCapacity(currentGen, currentWidth, maxGen, maxWidth string) (float64, bool) {
	gen := parseOptional(currentGen)
	width := parseOptional(currentWidth)
	if gen == nil || width == nil || *gen <= 0 || *width <= 0 {
		gen = parseOptional(maxGen)
		width = parseOptional(maxWidth)
	}
	if gen == nil || width == nil {
		return 0, false
	}
	perLaneMB := map[int]float64{
		1: 250,
		2: 500,
		3: 984.6,
		4: 1969,
		5: 3938,
		6: 7563,
	}
	rate, ok := perLaneMB[int(*gen)]
	if !ok || *width <= 0 {
		return 0, false
	}
	return rate * *width * 1000 * 1000, true
}

func queryPCIeThroughput(ctx context.Context) (map[string]pcieThroughput, string) {
	cmd := exec.CommandContext(ctx, "nvidia-smi", "dmon", "-s", "t", "-c", "1", "--format", "csv,noheader,nounit")
	out, err := cmd.CombinedOutput()
	raw := string(out)
	if err != nil {
		return nil, raw
	}
	return parsePCIeThroughput(raw), raw
}

func parsePCIeThroughput(raw string) map[string]pcieThroughput {
	throughput := map[string]pcieThroughput{}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := splitCSV(trimmed)
		if len(parts) < 3 {
			continue
		}
		rx := parseOptional(parts[1])
		tx := parseOptional(parts[2])
		if rx == nil || tx == nil {
			continue
		}
		throughput[parts[0]] = pcieThroughput{
			RX: *rx * 1000 * 1000,
			TX: *tx * 1000 * 1000,
		}
	}
	return throughput
}

func queryNVLinkCapacity(ctx context.Context) (map[string]float64, string) {
	cmd := exec.CommandContext(ctx, "nvidia-smi", "nvlink", "--status")
	out, err := cmd.CombinedOutput()
	raw := string(out)
	if err != nil {
		return nil, raw
	}
	return parseNVLinkCapacity(raw), raw
}

func parseNVLinkCapacity(raw string) map[string]float64 {
	capacity := map[string]float64{}
	gpu := "0"
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "GPU ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				gpu = strings.TrimSuffix(fields[1], ":")
			}
			continue
		}
		if !strings.Contains(trimmed, "GB/s") {
			continue
		}
		fields := strings.Fields(trimmed)
		for idx, field := range fields {
			if field != "GB/s" || idx == 0 {
				continue
			}
			value, err := strconv.ParseFloat(fields[idx-1], 64)
			if err == nil && value > 0 {
				capacity[gpu] += value * 1000 * 1000 * 1000
			}
		}
	}
	return capacity
}
