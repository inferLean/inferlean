package collect

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/collectors/nvml"
	promcollector "github.com/inferLean/inferlean-main/cli/internal/collectors/prometheus"
	"github.com/inferLean/inferlean-main/cli/internal/exporters/dcgm"
	"github.com/inferLean/inferlean-main/cli/internal/exporters/nodeexporter"
	runstore "github.com/inferLean/inferlean-main/cli/internal/storage/run"
)

type collectionSources struct {
	node nodeexporter.StartResult
	dcgm dcgm.StartResult
	nvml nvml.BridgeResult
}

func startSources(ctx context.Context) collectionSources {
	return collectionSources{
		node: nodeexporter.Start(ctx),
		dcgm: dcgm.Start(ctx),
		nvml: nvml.StartBridge(),
	}
}

func buildPromTargets(opts Options, sources collectionSources) []promcollector.Target {
	targets := []promcollector.Target{{Name: "vllm", Endpoint: opts.Target.MetricsEndpoint, Required: true}}
	if sources.node.Available {
		targets = append(targets, promcollector.Target{Name: "node_exporter", Endpoint: sources.node.Endpoint})
	}
	if sources.dcgm.Available {
		targets = append(targets, promcollector.Target{Name: "dcgm_exporter", Endpoint: sources.dcgm.Endpoint})
	}
	if sources.nvml.Available {
		targets = append(targets, promcollector.Target{Name: "nvml_bridge", Endpoint: sources.nvml.Endpoint, Required: true})
	}
	return targets
}

func savePrometheusObservations(p Presenter, paths runstore.Paths, promRes promcollector.Result) {
	if strings.TrimSpace(promRes.RawText) != "" {
		_, _ = p.obsStore.SaveRaw(paths.Observations, "prometheus.metrics", []byte(promRes.RawText))
	}
	if raw, ok := promRes.RawByTarget["vllm"]; ok && strings.TrimSpace(raw) != "" {
		_, _ = p.obsStore.SaveRaw(paths.Observations, "vllm.metrics", []byte(raw))
	}
	if raw, ok := promRes.RawByTarget["node_exporter"]; ok && strings.TrimSpace(raw) != "" {
		_, _ = p.obsStore.SaveRaw(paths.Observations, "node_exporter.metrics", []byte(raw))
	}
	if raw, ok := promRes.RawByTarget["dcgm_exporter"]; ok && strings.TrimSpace(raw) != "" {
		_, _ = p.obsStore.SaveRaw(paths.Observations, "dcgm_exporter.metrics", []byte(raw))
	}
	if len(promRes.Samples) > 0 {
		if payload, err := json.MarshalIndent(promRes.Samples, "", "  "); err == nil {
			payload = append(payload, '\n')
			_, _ = p.obsStore.SaveRaw(paths.Observations, "prometheus.samples.json", payload)
		}
	}
}

func stopSources(ctx context.Context, p Presenter, paths runstore.Paths, sources collectionSources) string {
	bridgeRaw := ""
	if sources.nvml.Bridge != nil {
		_ = sources.nvml.Bridge.Stop(ctx)
		if raw := strings.TrimSpace(sources.nvml.Bridge.LastRaw()); raw != "" {
			bridgeRaw = raw
			_, _ = p.pioStore.Save(paths.ProcessIO, "nvidia-smi-bridge.txt", []byte(raw))
		}
	}
	stopExporterSession(ctx, p, paths, "node-exporter", sources.node.Session)
	stopExporterSession(ctx, p, paths, "dcgm-exporter", sources.dcgm.Session)
	return bridgeRaw
}

func stopExporterSession(ctx context.Context, p Presenter, paths runstore.Paths, name string, session interface {
	Stop(context.Context) error
	Stdout() string
	Stderr() string
}) {
	if session == nil {
		return
	}
	_ = session.Stop(ctx)
	if out := strings.TrimSpace(session.Stdout()); out != "" {
		_, _ = p.pioStore.Save(paths.ProcessIO, name+".stdout.log", []byte(out))
	}
	if errText := strings.TrimSpace(session.Stderr()); errText != "" {
		_, _ = p.pioStore.Save(paths.ProcessIO, name+".stderr.log", []byte(errText))
	}
}
