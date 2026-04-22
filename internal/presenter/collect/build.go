package collect

import (
	"context"
	"strings"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/artifactnormalize"
	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/types"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
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
	StaticNvidiaSMI  string
	ProcessIODir     string
}

func buildArtifact(ctx context.Context, in buildInput) (contracts.RunArtifact, error) {
	env := collectConfigEnvironment(ctx, in.Target, in.ProcessIODir, in.StaticNvidiaSMI, in.PromResult)
	quality := buildQuality(in)
	return artifactnormalize.Build(artifactnormalize.Input{
		Job: artifactnormalize.JobInput{
			RunID:            in.RunID,
			InstallationID:   in.InstallationID,
			CollectorVersion: in.CollectorVersion,
			StartedAt:        in.StartedAt,
			FinishedAt:       in.FinishedAt,
		},
		Target: artifactnormalize.TargetInput{
			PID:             in.Target.PID,
			Executable:      in.Target.Executable,
			RawCommandLine:  in.Target.RawCommandLine,
			MetricsEndpoint: in.Target.MetricsEndpoint,
			ContainerID:     in.Target.ContainerID,
			PodName:         in.Target.PodName,
		},
		Configurations: env,
		Observations: artifactnormalize.ObservationsInput{
			Prometheus: in.PromResult.Samples,
		},
		UserIntent:        in.Intent,
		CollectionQuality: quality,
	})
}

func buildQuality(in buildInput) types.CollectionQuality {
	sourceStatus, mode := sourceStates(in.PromResult)
	return types.CollectionQuality{
		SourceStatus:  sourceStatus,
		TelemetryMode: mode,
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
