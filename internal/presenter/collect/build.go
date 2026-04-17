package collect

import (
	"context"
	"strings"
	"time"

	promcollector "github.com/inferLean/inferlean-main/new-cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/new-cli/internal/types"
	"github.com/inferLean/inferlean-main/new-cli/internal/vllmdiscovery"
)

type buildInput struct {
	RunID            string
	InstallationID   string
	CollectorVersion string
	StartedAt        time.Time
	FinishedAt       time.Time
	Target           vllmdiscovery.Candidate
	Intent           types.UserIntent
	PromResult       promcollector.Result
	NvidiaSMIRaw     string
	ExporterStatus   map[string]string
	CollectFor       time.Duration
	ScrapeEvery      time.Duration
	StaticNvidiaSMI  string
}

func buildArtifact(ctx context.Context, in buildInput) (types.Artifact, error) {
	env := collectConfigEnvironment(ctx, in.Target, in.StaticNvidiaSMI, in.PromResult)
	observations := buildObservations(in)
	rawProcess := buildRawProcess(in)
	quality := buildQuality(in)
	artifact := types.Artifact{
		Job: types.Job{
			RunID:            in.RunID,
			InstallationID:   in.InstallationID,
			SchemaVersion:    schemaVersion,
			CollectorVersion: in.CollectorVersion,
			StartedAt:        in.StartedAt,
			FinishedAt:       in.FinishedAt,
		},
		Target: types.Target{
			PID:             in.Target.PID,
			Executable:      in.Target.Executable,
			RawCommandLine:  in.Target.RawCommandLine,
			MetricsEndpoint: in.Target.MetricsEndpoint,
			ContainerID:     in.Target.ContainerID,
			PodName:         in.Target.PodName,
		},
		Configurations:    env,
		Observations:      observations,
		RawProcessIO:      rawProcess,
		UserIntent:        in.Intent,
		CollectionQuality: quality,
	}
	return artifact, nil
}

func buildObservations(in buildInput) types.Observations {
	return types.Observations{
		"prometheus": in.PromResult.Samples,
	}
}

func buildRawProcess(in buildInput) map[string]any {
	rawProcess := map[string]any{
		"nvidia_smi":      in.NvidiaSMIRaw,
		"exporter_status": in.ExporterStatus,
	}
	if strings.TrimSpace(in.StaticNvidiaSMI) != "" {
		rawProcess["nvidia_smi_static"] = in.StaticNvidiaSMI
	}
	return rawProcess
}

func buildQuality(in buildInput) types.CollectionQuality {
	sourceStatus, mode := sourceStates(in.PromResult)
	fallbacks := []string{}
	if sourceStatus["gpu_telemetry"] != "ok" {
		fallbacks = append(fallbacks, "nvidia-smi")
	}
	if sourceStatus["host_metrics"] != "ok" {
		fallbacks = append(fallbacks, "host-disabled")
	}
	return types.CollectionQuality{
		SourceStatus:      sourceStatus,
		TelemetryMode:     mode,
		FallbacksUsed:     fallbacks,
		MissingSources:    collectMissing(sourceStatus),
		DegradedSources:   collectDegraded(sourceStatus),
		CollectionSeconds: in.CollectFor.Seconds(),
		ScrapeIntervalSec: in.ScrapeEvery.Seconds(),
	}
}

func sourceStates(promRes promcollector.Result) (map[string]string, string) {
	hostState := stateFor(promRes.SourceStatus, "node_exporter")
	if hostState == "missing" {
		hostState = "degraded"
	}
	gpuState := stateFor(promRes.SourceStatus, "dcgm_exporter")
	if gpuState != "ok" {
		gpuState = stateFor(promRes.SourceStatus, "nvml_bridge")
	}
	nvidiaSMIState := stateFor(promRes.SourceStatus, "nvml_bridge")
	states := map[string]string{
		"vllm_metrics":       stateFor(promRes.SourceStatus, "vllm"),
		"host_metrics":       hostState,
		"gpu_telemetry":      gpuState,
		"nvidia_smi":         nvidiaSMIState,
		"process_inspection": "ok",
	}
	mode := "standard"
	if states["gpu_telemetry"] == "ok" {
		mode = "rich"
	}
	return states, mode
}

func stateFor(status map[string]string, key string) string {
	if status == nil {
		return "missing"
	}
	if value, ok := status[key]; ok && strings.TrimSpace(value) != "" {
		return value
	}
	return "missing"
}

func collectMissing(source map[string]string) []string {
	out := []string{}
	for key, state := range source {
		if strings.HasPrefix(state, "missing") {
			out = append(out, key)
		}
	}
	return out
}

func collectDegraded(source map[string]string) []string {
	out := []string{}
	for key, state := range source {
		if strings.HasPrefix(state, "degraded") {
			out = append(out, key)
		}
	}
	return out
}
