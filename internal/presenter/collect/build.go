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
	Sources          collectionSources
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
			Source:          in.Target.Source,
			PID:             in.Target.PID,
			InternalPID:     in.Target.InternalPID,
			Executable:      in.Target.Executable,
			RawCommandLine:  in.Target.RawCommandLine,
			MetricsEndpoint: in.Target.MetricsEndpoint,
			ContainerID:     in.Target.ContainerID,
			PodName:         in.Target.PodName,
			Namespace:       in.Target.Namespace,
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
	sourceStatus, mode := sourceStates(in.PromResult, in.Sources)
	return types.CollectionQuality{
		SourceStatus:       sourceStatus,
		SourceMetadata:     sourceMetadata(in),
		TelemetryMode:      mode,
		Fallbacks:          sourceFallbacks(in),
		CollectionDuration: collectionDuration(in),
		ScrapeInterval:     in.PromResult.ScrapeInterval,
	}
}

func collectionDuration(in buildInput) time.Duration {
	if !in.PromResult.StartedAt.IsZero() && in.PromResult.FinishedAt.After(in.PromResult.StartedAt) {
		return in.PromResult.FinishedAt.Sub(in.PromResult.StartedAt)
	}
	if in.FinishedAt.After(in.StartedAt) {
		return in.FinishedAt.Sub(in.StartedAt)
	}
	return 0
}

func sourceMetadata(in buildInput) map[string]types.SourceMetadata {
	return map[string]types.SourceMetadata{
		"vllm_metrics": {
			Endpoint:  in.Sources.vllmEndpoint,
			Transport: vllmTransport(in.Target, in.Sources),
			Artifacts: []string{"observations/vllm.metrics"},
		},
		"host_metrics": {
			Endpoint:  in.Sources.node.Endpoint,
			Transport: "node_exporter",
			Reason:    in.Sources.node.Reason,
			Artifacts: []string{"observations/node_exporter.metrics"},
		},
		"gpu_telemetry": {
			Endpoint:  gpuTelemetryEndpoint(in.Sources),
			Transport: gpuTelemetryTransport(in.Sources),
			Fallback:  !in.Sources.dcgm.Available && in.Sources.nvml.Available,
			Reason:    in.Sources.dcgm.Reason,
			Artifacts: gpuTelemetryArtifacts(in.Sources),
		},
		"nvidia_smi": {
			Endpoint:  in.Sources.nvml.Endpoint,
			Transport: "nvml_bridge",
			Reason:    in.Sources.nvml.Reason,
			Artifacts: []string{"observations/nvml_bridge.metrics", "observations/nvidia-smi.csv", "process-io/nvidia-smi-static.txt"},
		},
		"process_inspection": {
			Transport: "process_io",
			Artifacts: []string{"process-io/nvidia-smi-static.txt", "process-io/vllm-defaults-runtime.json"},
		},
		"prometheus_runtime": {
			Transport: "local_prometheus",
			Fallback:  stateFor(in.PromResult.SourceStatus, "prometheus_runtime") != "ok",
			Reason:    sourceStateReason(in.PromResult.SourceStatus, "prometheus_runtime"),
		},
	}
}

func vllmTransport(target vllmdiscovery.Candidate, sources collectionSources) string {
	switch strings.ToLower(strings.TrimSpace(target.Source)) {
	case "pod", "kubernetes":
		if sources.vllmSession != nil {
			return "kubectl_port_forward"
		}
		return "kubernetes_service"
	case "docker":
		return "docker_published_port"
	default:
		return "direct_http"
	}
}

func gpuTelemetryEndpoint(sources collectionSources) string {
	if sources.dcgm.Available {
		return sources.dcgm.Endpoint
	}
	return sources.nvml.Endpoint
}

func gpuTelemetryTransport(sources collectionSources) string {
	if sources.dcgm.Available {
		return "dcgm_exporter"
	}
	if sources.nvml.Available {
		return "nvml_bridge"
	}
	return ""
}

func gpuTelemetryArtifacts(sources collectionSources) []string {
	if sources.dcgm.Available {
		return []string{"observations/dcgm_exporter.metrics"}
	}
	if sources.nvml.Available {
		return []string{"observations/nvml_bridge.metrics", "observations/nvidia-smi.csv"}
	}
	return nil
}

func sourceFallbacks(in buildInput) []string {
	fallbacks := []string{}
	if !in.Sources.dcgm.Available && in.Sources.nvml.Available {
		fallbacks = append(fallbacks, fallbackText("gpu_telemetry", "dcgm_exporter unavailable; using nvml_bridge", in.Sources.dcgm.Reason))
	}
	if !in.Sources.node.Available {
		fallbacks = append(fallbacks, fallbackText("host_metrics", "node_exporter unavailable", in.Sources.node.Reason))
	}
	if state := stateFor(in.PromResult.SourceStatus, "prometheus_runtime"); state != "ok" && state != "missing" {
		fallbacks = append(fallbacks, fallbackText("prometheus_runtime", "local Prometheus unavailable; using direct HTTP scrapes", sourceStateReason(in.PromResult.SourceStatus, "prometheus_runtime")))
	}
	return fallbacks
}

func fallbackText(source, summary, reason string) string {
	if strings.TrimSpace(reason) == "" {
		return source + ": " + summary
	}
	return source + ": " + summary + ": " + strings.TrimSpace(reason)
}

func sourceStateReason(status map[string]string, key string) string {
	value := stateFor(status, key)
	_, reason := splitSourceState(value)
	return reason
}

func splitSourceState(raw string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(raw), ":", 2)
	if len(parts) < 2 {
		return strings.TrimSpace(raw), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func sourceStates(promRes promcollector.Result, sources collectionSources) (map[string]string, string) {
	hostState := stateFor(promRes.SourceStatus, "node_exporter")
	if hostState == "missing" {
		hostState = degradedHostReason(sources)
	} else if hostState == "degraded" {
		hostState = "degraded: node_exporter scrape returned no parseable metrics"
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
	if runtimeState := stateFor(promRes.SourceStatus, "prometheus_runtime"); runtimeState != "missing" {
		states["prometheus_runtime"] = runtimeState
	}
	mode := "standard"
	if states["gpu_telemetry"] == "ok" {
		mode = "rich"
	}
	return states, mode
}

func degradedHostReason(sources collectionSources) string {
	if !sources.node.Available {
		if reason := strings.TrimSpace(sources.node.Reason); reason != "" {
			return "degraded: node_exporter unavailable: " + reason
		}
		return "degraded: node_exporter unavailable"
	}
	return "degraded: node_exporter did not produce scrape samples"
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
