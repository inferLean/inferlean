package collector

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	gopsprocess "github.com/shirou/gopsutil/v4/process"

	"github.com/inferLean/inferlean/internal/discovery"
	"github.com/inferLean/inferlean/pkg/contracts"
)

func captureProcessInspection(ctx context.Context, rawPath string, target discovery.CandidateGroup) (sourceCapture, contracts.ProcessInspection) {
	inspection := contracts.ProcessInspection{
		TargetProcess: contracts.TargetProcess{
			PID:            target.PrimaryPID,
			Executable:     target.Executable,
			RawCommandLine: target.RawCommandLine,
			EntryPoint:     target.EntryPoint,
			StartedAt:      timePointer(target.StartedAt),
		},
		ParseWarnings:    append([]string{}, target.ParseWarnings...),
		RelatedProcesses: observeProcesses(ctx, target.PIDs),
	}
	inspection.Coverage = processCoverage(inspection, relativeRawArtifact(rawPath))
	if err := writeJSONFile(rawPath, inspection); err != nil {
		return sourceCapture{Status: "degraded", Reason: fmt.Sprintf("could not persist process inspection: %v", err)}, inspection
	}
	capture := captureFromCoverage(inspection.Coverage, []string{relativeRawArtifact(rawPath)}, "process inspection was incomplete", []string{
		"raw_command_line",
		"target_pid",
		"executable_identity",
		"related_process_identities",
	})
	return capture, inspection
}

func observeProcesses(ctx context.Context, pids []int32) []contracts.ObservedProcess {
	processes := make([]contracts.ObservedProcess, 0, len(pids))
	for _, pid := range pids {
		proc, err := gopsprocess.NewProcess(pid)
		if err != nil {
			continue
		}
		processes = append(processes, observeProcess(ctx, proc))
	}
	return processes
}

func observeProcess(ctx context.Context, proc *gopsprocess.Process) contracts.ObservedProcess {
	raw, _ := proc.CmdlineWithContext(ctx)
	exe, _ := proc.ExeWithContext(ctx)
	ppid, _ := proc.PpidWithContext(ctx)
	startedAt := time.Time{}
	if createdMS, err := proc.CreateTimeWithContext(ctx); err == nil {
		startedAt = time.UnixMilli(createdMS)
	}
	return contracts.ObservedProcess{
		PID:            proc.Pid,
		PPID:           ppid,
		Executable:     exe,
		RawCommandLine: raw,
		StartedAt:      timePointer(startedAt),
	}
}

func processCoverage(inspection contracts.ProcessInspection, rawRef string) contracts.SourceCoverage {
	coverage := newCoverageBuilder(rawRef)
	if inspection.TargetProcess.RawCommandLine != "" {
		coverage.Present("raw_command_line")
	} else {
		coverage.Missing("raw_command_line")
	}
	if inspection.TargetProcess.PID != 0 {
		coverage.Present("target_pid")
	} else {
		coverage.Missing("target_pid")
	}
	if inspection.TargetProcess.Executable != "" {
		coverage.Present("executable_identity")
	} else {
		coverage.Missing("executable_identity")
	}
	if len(inspection.RelatedProcesses) > 0 {
		coverage.Present("related_process_identities")
	} else {
		coverage.Missing("related_process_identities")
	}
	return coverage.Build()
}

func collectEnvironment(ctx context.Context, nvidia *nvidiaSnapshot, nvml *nvmlSnapshot) contracts.Environment {
	env := contracts.Environment{OS: runtime.GOOS + "/" + runtime.GOARCH}
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
	}
	switch {
	case nvml != nil && len(nvml.Samples) > 0 && len(nvml.Samples[len(nvml.Samples)-1].GPUs) > 0:
		env.GPUCount = len(nvml.Samples[len(nvml.Samples)-1].GPUs)
		env.GPUModel = nvml.Samples[len(nvml.Samples)-1].GPUs[0].Name
		env.DriverVersion = nvml.DriverVersion
	case nvidia != nil && len(nvidia.GPUs) > 0:
		env.GPUCount = len(nvidia.GPUs)
		env.GPUModel = nvidia.GPUs[0].Name
		env.DriverVersion = nvidia.DriverVersion
	}
	return env
}

func (r *collectionRun) buildArtifact() (contracts.RunArtifact, bool) {
	states := r.sourceStates()
	minimumEvidenceMet := hasMinimumEvidence(states)
	return contracts.RunArtifact{
		SchemaVersion:        contracts.SchemaVersion,
		Job:                  r.buildJob(),
		Environment:          r.env,
		RuntimeConfig:        r.runtimeConfig,
		Metrics:              r.buildMetrics(),
		ProcessInspection:    r.processInspection,
		WorkloadObservations: r.buildWorkloadObservations(minimumEvidenceMet),
		CollectionQuality:    buildCollectionQuality(states, minimumEvidenceMet),
	}, minimumEvidenceMet
}

func (r *collectionRun) buildJob() contracts.Job {
	job := r.cfg
	job.CollectedAt = time.Now().UTC()
	return job
}

func (r *collectionRun) buildMetrics() contracts.Metrics {
	return contracts.Metrics{
		VLLM:      r.vllmMetrics,
		Host:      r.hostMetrics,
		GPU:       r.gpuMetrics,
		NvidiaSmi: r.nvidiaMetrics,
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
			"process_samples":               len(r.processSamples),
			"nvml_samples":                  nvmlSampleCount(r.nvmlSnapshot),
		},
	}
}

func nvmlSampleCount(snapshot *nvmlSnapshot) int {
	if snapshot == nil {
		return 0
	}
	return len(snapshot.Samples)
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

func baseRuntimeConfig(cfg discovery.RuntimeConfig) contracts.RuntimeConfig {
	return contracts.RuntimeConfig{
		Model:                 cfg.Model,
		ServedModelName:       cfg.ServedModelName,
		Host:                  cfg.Host,
		Port:                  cfg.Port,
		TensorParallelSize:    cfg.TensorParallelSize,
		DataParallelSize:      cfg.DataParallelSize,
		PipelineParallelSize:  cfg.PipelineParallelSize,
		MaxModelLen:           cfg.MaxModelLen,
		MaxNumBatchedTokens:   cfg.MaxNumBatchedTokens,
		MaxNumSeqs:            cfg.MaxNumSeqs,
		GPUMemoryUtilization:  cfg.GPUMemoryUtilization,
		KVCacheDType:          cfg.KVCacheDType,
		ChunkedPrefill:        cfg.ChunkedPrefill,
		PrefixCaching:         cfg.PrefixCaching,
		Quantization:          cfg.Quantization,
		DType:                 cfg.DType,
		GenerationConfig:      cfg.GenerationConfig,
		APIKeyConfigured:      cfg.APIKeyConfigured,
		MultimodalFlags:       append([]string{}, cfg.MultimodalFlags...),
		AttentionBackend:      cfg.AttentionBackend,
		FlashinferPresent:     cfg.FlashinferPresent,
		FlashAttentionPresent: cfg.FlashAttentionPresent,
		ImageProcessor:        cfg.ImageProcessor,
		MultimodalCacheHints:  append([]string{}, cfg.MultimodalCacheHints...),
		EnvHints:              cfg.EnvHints,
	}
}
