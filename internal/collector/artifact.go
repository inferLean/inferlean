package collector

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/inferLean/inferlean/internal/discovery"
	"github.com/inferLean/inferlean/pkg/contracts"
)

func captureProcessInspection(rawPath string, target discovery.CandidateGroup) (sourceCapture, map[string]any) {
	payload := map[string]any{
		"primary_pid":      target.PrimaryPID,
		"related_pids":     target.PIDs,
		"entry_point":      target.EntryPoint,
		"raw_command_line": target.RawCommandLine,
		"parse_warnings":   target.ParseWarnings,
	}
	if target.Executable != "" {
		payload["executable"] = target.Executable
	}
	if !target.StartedAt.IsZero() {
		payload["started_at"] = target.StartedAt.UTC().Format(time.RFC3339)
	}
	if err := writeJSONFile(rawPath, payload); err != nil {
		return degradedCapture(fmt.Sprintf("could not persist process inspection: %v", err)), payload
	}
	return sourceCapture{Status: "ok", Artifacts: []string{relativeRawArtifact(rawPath)}, MetricPayload: map[string]any{"raw_evidence_ref": relativeRawArtifact(rawPath)}}, payload
}

func collectEnvironment(ctx context.Context, snapshot *nvidiaSnapshot) (contracts.Environment, map[string]any) {
	env := contracts.Environment{OS: runtimeOSArch()}
	metrics := map[string]any{}

	if info, err := host.InfoWithContext(ctx); err == nil {
		env.Kernel = info.KernelVersion
	}
	if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
		env.CPUModel = infos[0].ModelName
	}
	if cores, err := cpu.CountsWithContext(ctx, true); err == nil {
		env.CPUCores = cores
	}
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		env.MemoryBytes = int64(vm.Total)
		metrics["memory_used_bytes"] = vm.Used
		metrics["memory_available_bytes"] = vm.Available
	}
	if snapshot != nil && len(snapshot.GPUs) > 0 {
		env.GPUCount = len(snapshot.GPUs)
		env.GPUModel = snapshot.GPUs[0].Name
		env.DriverVersion = snapshot.GPUs[0].DriverVersion
	}
	return env, metrics
}

func runtimeOSArch() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func (r *collectionRun) buildArtifact() (contracts.RunArtifact, bool) {
	states := r.sourceStates()
	minimumEvidenceMet := hasMinimumEvidence(states)
	artifact := contracts.RunArtifact{
		SchemaVersion: contracts.SchemaVersion,
		Job:           r.buildJob(),
		Environment:   r.env,
		RuntimeConfig: toRuntimeConfig(r.opts.Target.RuntimeConfig),
		Metrics:       r.buildMetrics(),
		ProcessInspection: contracts.ProcessInspection{
			TargetProcess: contracts.TargetProcess{
				PID:            r.opts.Target.PrimaryPID,
				Executable:     r.opts.Target.Executable,
				RawCommandLine: r.opts.Target.RawCommandLine,
				EntryPoint:     r.opts.Target.EntryPoint,
				StartedAt:      timePointer(r.opts.Target.StartedAt),
			},
			ParseWarnings: append([]string{}, r.opts.Target.ParseWarnings...),
		},
		WorkloadObservations: r.buildWorkloadObservations(minimumEvidenceMet),
		CollectionQuality:    buildCollectionQuality(states, minimumEvidenceMet),
	}
	return artifact, minimumEvidenceMet
}

func (r *collectionRun) buildJob() contracts.Job {
	job := r.cfg
	job.CollectedAt = time.Now().UTC()
	return job
}

func (r *collectionRun) buildMetrics() contracts.Metrics {
	return contracts.Metrics{
		VLLM:      r.vllmCapture.MetricPayload,
		Host:      mergeMaps(r.hostCapture.MetricPayload, r.envMetrics),
		GPU:       r.gpuCapture.MetricPayload,
		NvidiaSmi: r.nvidiaCapture.MetricPayload,
	}
}

func (r *collectionRun) buildWorkloadObservations(minimumEvidenceMet bool) contracts.WorkloadObservations {
	return contracts.WorkloadObservations{
		Summary: fmt.Sprintf("Collected local evidence for %s over %s", r.opts.Target.DisplayModel(), r.opts.CollectFor),
		Hints:   map[string]string{"target_model": r.opts.Target.DisplayModel()},
		Measurements: map[string]any{
			"collect_for_seconds":           r.opts.CollectFor.Seconds(),
			"scrape_every_seconds":          r.opts.ScrapeEvery.Seconds(),
			"minimum_required_evidence_met": minimumEvidenceMet,
			"process_inspection":            r.processMetrics,
		},
	}
}

func (r *collectionRun) sourceStates() map[string]contracts.SourceState {
	return map[string]contracts.SourceState{
		"vllm_metrics":       toSourceState(r.vllmCapture),
		"host_metrics":       toSourceState(r.hostCapture),
		"gpu_telemetry":      toSourceState(r.gpuCapture),
		"nvidia_smi":         toSourceState(r.nvidiaCapture),
		"process_inspection": toSourceState(r.processCapture),
	}
}

func buildCollectionQuality(states map[string]contracts.SourceState, minimumEvidenceMet bool) contracts.CollectionQuality {
	return contracts.CollectionQuality{
		SourceStates:     states,
		MissingEvidence:  missingEvidence(states),
		DegradedEvidence: degradedEvidence(states),
		Completeness:     computeCompleteness(states),
		Summary:          qualitySummary(states, minimumEvidenceMet),
	}
}

func toSourceState(capture sourceCapture) contracts.SourceState {
	return contracts.SourceState{Status: capture.Status, Reason: capture.Reason, Artifacts: capture.Artifacts}
}

func toRuntimeConfig(cfg discovery.RuntimeConfig) contracts.RuntimeConfig {
	return contracts.RuntimeConfig{
		Model:                cfg.Model,
		ServedModelName:      cfg.ServedModelName,
		Host:                 cfg.Host,
		Port:                 cfg.Port,
		TensorParallelSize:   cfg.TensorParallelSize,
		DataParallelSize:     cfg.DataParallelSize,
		PipelineParallelSize: cfg.PipelineParallelSize,
		MaxModelLen:          cfg.MaxModelLen,
		MaxNumBatchedTokens:  cfg.MaxNumBatchedTokens,
		MaxNumSeqs:           cfg.MaxNumSeqs,
		GPUMemoryUtilization: cfg.GPUMemoryUtilization,
		KVCacheDType:         cfg.KVCacheDType,
		ChunkedPrefill:       cfg.ChunkedPrefill,
		PrefixCaching:        cfg.PrefixCaching,
		Quantization:         cfg.Quantization,
		DType:                cfg.DType,
		GenerationConfig:     cfg.GenerationConfig,
		APIKeyConfigured:     cfg.APIKeyConfigured,
		MultimodalFlags:      append([]string{}, cfg.MultimodalFlags...),
		EnvHints:             cfg.EnvHints,
	}
}
