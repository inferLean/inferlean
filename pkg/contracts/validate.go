package contracts

import (
	"errors"
	"fmt"
	"strings"
)

func (a RunArtifact) Validate() error {
	var errs []error

	errs = append(errs, validateSchema(a)...)
	errs = append(errs, validateIdentity(a.Job)...)
	errs = append(errs, validateWorkloadObservations(a.WorkloadObservations)...)
	errs = append(errs, a.RuntimeConfig.validate()...)
	errs = append(errs, a.ProcessInspection.validate()...)
	errs = append(errs, a.CollectionQuality.validate()...)
	errs = append(errs, validateMetrics(a.Metrics)...)

	return errors.Join(errs...)
}

func validateSchema(a RunArtifact) []error {
	var errs []error

	if strings.TrimSpace(a.SchemaVersion) == "" {
		errs = append(errs, errors.New("schema_version is required"))
	} else if a.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("schema_version must be %s", SchemaVersion))
	}
	if a.Job.SchemaVersion != "" && a.Job.SchemaVersion != a.SchemaVersion {
		errs = append(errs, errors.New("job.schema_version must match schema_version"))
	}

	return errs
}

func validateIdentity(j Job) []error {
	var errs []error

	if strings.TrimSpace(j.RunID) == "" {
		errs = append(errs, errors.New("job.run_id is required"))
	}
	if strings.TrimSpace(j.InstallationID) == "" {
		errs = append(errs, errors.New("job.installation_id is required"))
	}
	if strings.TrimSpace(j.CollectorVersion) == "" {
		errs = append(errs, errors.New("job.collector_version is required"))
	}
	if strings.TrimSpace(j.SchemaVersion) == "" {
		errs = append(errs, errors.New("job.schema_version is required"))
	}
	if j.CollectedAt.IsZero() {
		errs = append(errs, errors.New("job.collected_at is required"))
	}

	return errs
}

func validateWorkloadObservations(w WorkloadObservations) []error {
	var errs []error

	errs = append(errs, validateEnum("workload_observations.mode", w.Mode, "realtime_chat", "batch_processing", "mixed", "unknown"))
	errs = append(errs, validateEnum("workload_observations.target", w.Target, "latency", "throughput", "balanced", "unknown"))
	errs = append(errs, validateEnum("workload_observations.prefix_reuse", w.PrefixReuse, "high", "low", "unknown"))
	errs = append(errs, validateEnum("workload_observations.multimodal", w.Multimodal, "present", "absent", "unknown"))
	errs = append(errs, validateEnum("workload_observations.repeated_multimodal_media", w.RepeatedMultimodalMedia, "high", "low", "unknown"))

	return errs
}

func validateEnum(field, value string, allowed ...string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return fmt.Errorf("%s must be one of %s", field, strings.Join(allowed, ", "))
}

func (r RuntimeConfig) validate() []error {
	checks := map[string]bool{
		"max_model_len":            r.MaxModelLen != 0,
		"max_num_batched_tokens":   r.MaxNumBatchedTokens != 0,
		"max_num_seqs":             r.MaxNumSeqs != 0,
		"gpu_memory_utilization":   r.GPUMemoryUtilization != 0,
		"parallelism_settings":     hasParallelism(r),
		"quantization_mode":        strings.TrimSpace(r.Quantization) != "",
		"prefix_caching_state":     r.PrefixCaching != nil,
		"chunked_prefill_state":    r.ChunkedPrefill != nil,
		"multimodal_runtime_hints": hasStrings(r.MultimodalFlags),
		"vllm_version":             strings.TrimSpace(r.VLLMVersion) != "",
		"torch_version":            strings.TrimSpace(r.TorchVersion) != "",
		"cuda_runtime_version":     strings.TrimSpace(r.CUDARuntimeVersion) != "",
		"nvidia_driver_version":    strings.TrimSpace(r.NvidiaDriverVersion) != "",
		"attention_backend":        strings.TrimSpace(r.AttentionBackend) != "",
		"flashinfer_presence":      r.FlashinferPresent != nil,
		"flash_attention_presence": r.FlashAttentionPresent != nil,
		"image_processor":          strings.TrimSpace(r.ImageProcessor) != "",
	}

	return validateCoverage("runtime_config", r.Coverage, checks)
}

func hasParallelism(r RuntimeConfig) bool {
	return r.TensorParallelSize != 0 || r.DataParallelSize != 0 || r.PipelineParallelSize != 0
}

func hasStrings(values []string) bool {
	return len(values) > 0
}

func (p ProcessInspection) validate() []error {
	checks := map[string]bool{
		"raw_command_line":           strings.TrimSpace(p.TargetProcess.RawCommandLine) != "",
		"target_pid":                 p.TargetProcess.PID != 0,
		"executable_identity":        strings.TrimSpace(p.TargetProcess.Executable) != "",
		"related_process_identities": len(p.RelatedProcesses) > 0,
	}

	return validateCoverage("process_inspection", p.Coverage, checks)
}

func (q CollectionQuality) validate() []error {
	var errs []error

	for _, name := range requiredSourceStates() {
		source, ok := q.SourceStates[name]
		if !ok {
			errs = append(errs, fmt.Errorf("collection_quality.source_states[%s] is required", name))
			continue
		}
		if err := validateSourceState(name, source); err != nil {
			errs = append(errs, err)
		}
	}
	if q.Completeness < 0 || q.Completeness > 1 {
		errs = append(errs, errors.New("collection_quality.completeness must be between 0 and 1"))
	}

	return errs
}

func requiredSourceStates() []string {
	return []string{
		"vllm_metrics",
		"host_metrics",
		"gpu_telemetry",
		"nvidia_smi",
		"process_inspection",
	}
}

func validateSourceState(name string, source SourceState) error {
	if strings.TrimSpace(source.Status) == "" {
		return fmt.Errorf("collection_quality.source_states[%s].status is required", name)
	}
	switch source.Status {
	case "ok", "degraded", "missing":
		return nil
	default:
		return fmt.Errorf("collection_quality.source_states[%s].status must be ok, degraded, or missing", name)
	}
}
