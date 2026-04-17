package collect

import (
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"

	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/types"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdefaults"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

var (
	driverVersionPattern = regexp.MustCompile(`Driver Version:\s*([0-9.]+)`)
	cudaVersionPattern   = regexp.MustCompile(`CUDA Version:\s*([0-9.]+)`)
	pinnedVLLMPattern    = regexp.MustCompile(`(?i)vllm(?:==|~=|>=|<=|>|<)\s*v?(\d+\.\d+(?:\.\d+)?(?:rc\d+|post\d+)?)`)
	imageVLLMPattern     = regexp.MustCompile(`(?i)(?:^|[/:_-])vllm(?:[-_a-z]*)?:v?(\d+\.\d+(?:\.\d+)?(?:rc\d+|post\d+)?)`)
)

type gpuSnapshot struct {
	Model        string
	Count        int
	Driver       string
	MaxMemoryMiB float64
}

func collectConfigEnvironment(
	ctx context.Context,
	target vllmdiscovery.Candidate,
	staticNvidiaSMI string,
	promRes promcollector.Result,
) types.Configurations {
	rawCommandLine := target.RawCommandLine
	cfg := types.Configurations{
		OS:                  runtime.GOOS + "/" + runtime.GOARCH,
		ParsedArgs:          parseVLLMArgs(rawCommandLine),
		NvidiaSMIStaticText: strings.TrimSpace(staticNvidiaSMI),
	}
	fillHostConfig(ctx, &cfg)
	gpu := collectGPUSnapshot(ctx, promRes, staticNvidiaSMI)
	applyGPUSnapshot(&cfg, gpu, staticNvidiaSMI)
	applyVLLMDefaults(&cfg, rawCommandLine, inferVLLMVersionHint(ctx, target), gpu)
	return cfg
}

func fillHostConfig(ctx context.Context, cfg *types.Configurations) {
	if info, err := host.InfoWithContext(ctx); err == nil {
		cfg.Kernel = info.KernelVersion
	}
	if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
		cfg.CPUModel = infos[0].ModelName
	}
	if cores, err := cpu.CountsWithContext(ctx, true); err == nil {
		cfg.CPUCores = cores
	}
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		cfg.RAMBytes = vm.Total
	}
}

func collectGPUSnapshot(ctx context.Context, promRes promcollector.Result, staticNvidiaSMI string) gpuSnapshot {
	snapshot := gpuSnapshot{MaxMemoryMiB: maxNVMLMemoryTotal(promRes)}
	rows, err := queryNvidiaSMIGPUs(ctx)
	if err == nil && len(rows) > 0 {
		snapshot.Count = len(rows)
		snapshot.Model = rows[0].name
		snapshot.Driver = rows[0].driverVersion
		for _, row := range rows {
			if row.memoryMiB > snapshot.MaxMemoryMiB {
				snapshot.MaxMemoryMiB = row.memoryMiB
			}
		}
	}
	return withSMIVersions(snapshot, staticNvidiaSMI)
}

func applyGPUSnapshot(cfg *types.Configurations, snapshot gpuSnapshot, staticNvidiaSMI string) {
	if snapshot.Model != "" {
		cfg.GPUModel = snapshot.Model
	}
	if snapshot.Count > 0 {
		cfg.GPUCount = snapshot.Count
	}
	if snapshot.Driver != "" {
		cfg.DriverVersion = snapshot.Driver
	}
	if cfg.CUDARuntimeVersion == "" {
		cfg.CUDARuntimeVersion = extractSMICUDAVersion(staticNvidiaSMI)
	}
}

func withSMIVersions(snapshot gpuSnapshot, staticNvidiaSMI string) gpuSnapshot {
	if snapshot.Driver == "" {
		snapshot.Driver = extractSMIDriverVersion(staticNvidiaSMI)
	}
	return snapshot
}

func applyVLLMDefaults(cfg *types.Configurations, rawCommandLine, versionHint string, snapshot gpuSnapshot) {
	resolved, err := vllmdefaults.Resolve(vllmdefaults.Input{
		RawCommandLine: rawCommandLine,
		ExplicitArgs:   cfg.ParsedArgs,
		VLLMVersion:    versionHint,
		GPUModel:       snapshot.Model,
		GPUMemoryMiB:   snapshot.MaxMemoryMiB,
	})
	if err != nil {
		cfg.EnvironmentHints = withHint(cfg.EnvironmentHints, "vllm_defaults_error", err.Error())
		return
	}
	cfg.ParsedArgs = resolved.Args
	hints := cfg.EnvironmentHints
	hints = withHint(hints, "vllm_defaults_tag", resolved.SelectedTag)
	hints = withHint(hints, "vllm_defaults_profile", resolved.SelectedProfile)
	hints = withHint(hints, "vllm_defaults_applied", strconv.Itoa(resolved.AppliedDefaults))
	hints = withHint(hints, "vllm_defaults_dir", resolved.DefaultsDir)
	if strings.TrimSpace(resolved.RequestedVersion) != "" {
		hints = withHint(hints, "vllm_defaults_requested_version", resolved.RequestedVersion)
	}
	if strings.TrimSpace(versionHint) != "" {
		hints = withHint(hints, "vllm_version_hint", versionHint)
	}
	if strings.TrimSpace(resolved.SelectedModel) != "" {
		hints = withHint(hints, "vllm_model", resolved.SelectedModel)
	}
	cfg.EnvironmentHints = hints
}

func inferVLLMVersionHint(ctx context.Context, target vllmdiscovery.Candidate) string {
	if version := parseVLLMVersionText(target.RawCommandLine); version != "" {
		return version
	}
	containerID := strings.TrimSpace(target.ContainerID)
	if containerID == "" {
		return ""
	}
	image, err := inspectDockerImage(ctx, containerID)
	if err != nil {
		return ""
	}
	return parseVLLMVersionText(image)
}

func inspectDockerImage(ctx context.Context, containerID string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Config.Image}}", containerID)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	image := strings.TrimSpace(string(out))
	if image == "" {
		return "", fmt.Errorf("empty docker image for container %s", containerID)
	}
	return image, nil
}

func parseVLLMVersionText(text string) string {
	if matches := pinnedVLLMPattern.FindStringSubmatch(text); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	if matches := imageVLLMPattern.FindStringSubmatch(text); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func withHint(hints map[string]string, key, value string) map[string]string {
	trimmedKey := strings.TrimSpace(key)
	trimmedValue := strings.TrimSpace(value)
	if trimmedKey == "" || trimmedValue == "" {
		return hints
	}
	if hints == nil {
		hints = map[string]string{}
	}
	hints[trimmedKey] = trimmedValue
	return hints
}

func maxNVMLMemoryTotal(promRes promcollector.Result) float64 {
	samples := promRes.Samples["nvml_bridge"]
	max := 0.0
	for _, sample := range samples {
		for _, metric := range sample.Metrics {
			if metric.Name != "inferlean_nvml_memory_total_mb" {
				continue
			}
			if metric.Value > max {
				max = metric.Value
			}
		}
	}
	return max
}

type smiRow struct {
	name          string
	memoryMiB     float64
	driverVersion string
}

func queryNvidiaSMIGPUs(ctx context.Context) ([]smiRow, error) {
	cmd := exec.CommandContext(
		ctx,
		"nvidia-smi",
		"--query-gpu=name,memory.total,driver_version",
		"--format=csv,noheader,nounits",
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(string(output)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	rows := make([]smiRow, 0, len(records))
	for _, record := range records {
		if len(record) < 3 {
			continue
		}
		memoryMiB, _ := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
		rows = append(rows, smiRow{
			name:          strings.TrimSpace(record[0]),
			memoryMiB:     memoryMiB,
			driverVersion: strings.TrimSpace(record[2]),
		})
	}
	return rows, nil
}

func extractSMIDriverVersion(raw string) string {
	matches := driverVersionPattern.FindStringSubmatch(raw)
	if len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func extractSMICUDAVersion(raw string) string {
	matches := cudaVersionPattern.FindStringSubmatch(raw)
	if len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}
