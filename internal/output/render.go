package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/inferLean/inferlean/internal/collector"
	"github.com/inferLean/inferlean/internal/discovery"
	"github.com/inferLean/inferlean/pkg/contracts"
)

func RenderDiscovery(w io.Writer, result discovery.Result) {
	if result.Selected == nil {
		return
	}

	target := result.Selected
	fmt.Fprintf(w, "InferLean found a vLLM deployment.\n\n")
	fmt.Fprintf(w, "Selected target\n")
	fmt.Fprintf(w, "  Model: %s\n", valueOrUnknown(displayModelName(target.RuntimeConfig)))
	fmt.Fprintf(w, "  Target: %s\n", target.LocationLabel())
	if target.PrimaryPID > 0 {
		fmt.Fprintf(w, "  Host PID: %d", target.PrimaryPID)
		if target.ProcessCount > 1 {
			fmt.Fprintf(w, " (%d related processes)", target.ProcessCount)
		}
		fmt.Fprintln(w)
	} else if target.Target.IsHost() {
		fmt.Fprintln(w, "  Host PID: not detected")
	}
	fmt.Fprintf(w, "  Entry point: %s\n", target.EntryPoint)
	fmt.Fprintf(w, "  Listen address: %s\n", listenAddress(target.RuntimeConfig))
	fmt.Fprintf(w, "  Why this target: %s\n", result.Reason)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Parsed runtime configuration")
	writeConfigLine(w, "Model", target.RuntimeConfig.Model)
	writeConfigLine(w, "Served model name", target.RuntimeConfig.ServedModelName)
	writeConfigLine(w, "Host", target.RuntimeConfig.Host)
	writeConfigLine(w, "Port", intString(target.RuntimeConfig.Port))
	writeConfigLine(w, "Tensor parallel size", intString(target.RuntimeConfig.TensorParallelSize))
	writeConfigLine(w, "Data parallel size", intString(target.RuntimeConfig.DataParallelSize))
	writeConfigLine(w, "Pipeline parallel size", intString(target.RuntimeConfig.PipelineParallelSize))
	writeConfigLine(w, "Max model len", maxModelLenString(target.RuntimeConfig.MaxModelLen))
	writeConfigLine(w, "Max batched tokens", intString(target.RuntimeConfig.MaxNumBatchedTokens))
	writeConfigLine(w, "Max sequences", intString(target.RuntimeConfig.MaxNumSeqs))
	writeConfigLine(w, "GPU memory utilization", floatString(target.RuntimeConfig.GPUMemoryUtilization))
	writeConfigLine(w, "KV cache dtype", target.RuntimeConfig.KVCacheDType)
	writeConfigLine(w, "Chunked prefill", boolPointerString(target.RuntimeConfig.ChunkedPrefill))
	writeConfigLine(w, "Prefix caching", boolPointerString(target.RuntimeConfig.PrefixCaching))
	writeConfigLine(w, "Quantization", target.RuntimeConfig.Quantization)
	writeConfigLine(w, "DType", target.RuntimeConfig.DType)
	writeConfigLine(w, "Generation config", target.RuntimeConfig.GenerationConfig)
	writeConfigLine(w, "API key", configuredString(target.RuntimeConfig.APIKeyConfigured))
	writeConfigLine(w, "Multimodal flags", strings.Join(target.RuntimeConfig.MultimodalFlags, ", "))
	writeConfigLine(w, "Environment hints", formatHints(target.RuntimeConfig.EnvHints))

	if len(target.ParseWarnings) > 0 || len(result.Warnings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Warnings")
		for _, warning := range append(target.ParseWarnings, result.Warnings...) {
			fmt.Fprintf(w, "  - %s\n", warning)
		}
	}
}

func RenderAmbiguity(w io.Writer, result discovery.Result) {
	fmt.Fprintln(w, "InferLean found multiple vLLM deployments.")
	fmt.Fprintln(w)
	for _, candidate := range result.Candidates {
		fmt.Fprintf(w, "  - %s", candidate.IdentityLabel())
		if model := displayModelName(candidate.RuntimeConfig); model != "" {
			fmt.Fprintf(w, " • %s", model)
		}
		if candidate.RuntimeConfig.Port > 0 {
			fmt.Fprintf(w, " • port %d", candidate.RuntimeConfig.Port)
		}
		if candidate.ProcessCount > 1 {
			fmt.Fprintf(w, " • %d related processes", candidate.ProcessCount)
		}
		if !candidate.Target.IsHost() && candidate.PrimaryPID > 0 {
			fmt.Fprintf(w, " • host pid %d", candidate.PrimaryPID)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Rerun with --pid, --container, or --pod to select one deployment explicitly.")
}

func RenderCollection(w io.Writer, target discovery.Result, result collector.Result) {
	selected := target.Selected
	if selected == nil {
		return
	}

	fmt.Fprintln(w, "InferLean collected a local run artifact.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Selected target")
	fmt.Fprintf(w, "  Model: %s\n", valueOrUnknown(displayModelName(selected.RuntimeConfig)))
	fmt.Fprintf(w, "  Target: %s\n", selected.LocationLabel())
	if selected.PrimaryPID > 0 {
		fmt.Fprintf(w, "  Host PID: %d\n", selected.PrimaryPID)
	}
	fmt.Fprintf(w, "  Why this target: %s\n", target.Reason)
	fmt.Fprintf(w, "  Artifact: %s\n", result.ArtifactPath)
	if result.Artifact.WorkloadObservations.Mode != "" {
		fmt.Fprintf(w, "  Workload mode: %s\n", result.Artifact.WorkloadObservations.Mode)
	}
	if result.Artifact.WorkloadObservations.Target != "" {
		fmt.Fprintf(w, "  Workload target: %s\n", result.Artifact.WorkloadObservations.Target)
	}
	if result.MinimumEvidenceMet {
		fmt.Fprintln(w, "  Minimum evidence set: fully met")
	} else {
		fmt.Fprintln(w, "  Minimum evidence set: not fully met")
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Collection quality")
	sourceNames := make([]string, 0, len(result.Artifact.CollectionQuality.SourceStates))
	for name := range result.Artifact.CollectionQuality.SourceStates {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)
	for _, name := range sourceNames {
		source := result.Artifact.CollectionQuality.SourceStates[name]
		line := fmt.Sprintf("  %s: %s", name, source.Status)
		if source.Reason != "" {
			line += " (" + source.Reason + ")"
		}
		fmt.Fprintln(w, line)
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Warnings")
		for _, warning := range result.Warnings {
			fmt.Fprintf(w, "  - %s\n", warning)
		}
	}
}

func RenderPublication(w io.Writer, ack contracts.ArtifactUploadAck) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Backend acknowledgement")
	fmt.Fprintf(w, "  Upload ID: %s\n", ack.UploadID)
	fmt.Fprintf(w, "  Run ID: %s\n", ack.RunID)
	fmt.Fprintf(w, "  Installation ID: %s\n", ack.InstallationID)
	fmt.Fprintf(w, "  Status: %s\n", ack.Status)
	fmt.Fprintf(w, "  Received at: %s\n", ack.ReceivedAt.Format(time.RFC3339))
	if ack.StatusURL != "" {
		fmt.Fprintf(w, "  Trace: %s\n", ack.StatusURL)
	}
	if ack.ReportURL != "" {
		fmt.Fprintf(w, "  Report: %s\n", ack.ReportURL)
	}
}

func RenderReportSummary(w io.Writer, report contracts.FinalReport) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Report summary")
	fmt.Fprintf(w, "  Tier: %s\n", valueOrUnknown(report.Entitlement.Tier))
	fmt.Fprintf(w, "  Dominant limiter: %s\n", valueOrUnknown(report.Diagnosis.BaseDiagnosis.CurrentLimiter.Label))
	if recommendation := report.Diagnosis.BaseDiagnosis.Recommendation; recommendation != nil {
		fmt.Fprintf(w, "  Primary recommendation: %s\n", recommendation.Title)
		if recommendation.Tradeoff.Summary != "" {
			fmt.Fprintf(w, "  Likely tradeoff: %s\n", recommendation.Tradeoff.Summary)
		}
	} else {
		fmt.Fprintf(w, "  Primary recommendation: %s\n", valueOrUnknown(report.Diagnosis.BaseDiagnosis.NoSafeRecommendationReason))
	}
	fmt.Fprintf(w, "  Confidence: %s\n", valueOrUnknown(report.Diagnosis.BaseDiagnosis.Confidence))
}

func RenderSummaryPreview(w io.Writer, preview *contracts.SummaryPreview) {
	if preview == nil {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Report preview")
	fmt.Fprintf(w, "  Headline: %s\n", valueOrUnknown(preview.Headline))
	if preview.CurrentLimiterLabel != "" {
		fmt.Fprintf(w, "  Dominant limiter: %s\n", preview.CurrentLimiterLabel)
	}
	if preview.PrimaryRecommendation != "" {
		fmt.Fprintf(w, "  Primary recommendation: %s\n", preview.PrimaryRecommendation)
	}
	if preview.KeyTradeoff != "" {
		fmt.Fprintf(w, "  Likely tradeoff: %s\n", preview.KeyTradeoff)
	}
	if preview.Confidence != "" {
		fmt.Fprintf(w, "  Confidence: %s\n", preview.Confidence)
	}
	fmt.Fprintln(w, "  Full report: run inferlean login, then open the claimed run with inferlean runs.")
}

func writeConfigLine(w io.Writer, label, value string) {
	fmt.Fprintf(w, "  %s: %s\n", label, valueOrUnknown(value))
}

func intString(v int) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%d", v)
}

func maxModelLenString(v int) string {
	if v == 0 {
		return ""
	}
	if v == -1 {
		return "auto"
	}
	return fmt.Sprintf("%d", v)
}

func floatString(v float64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", v)
}

func boolPointerString(v *bool) string {
	if v == nil {
		return ""
	}
	if *v {
		return "enabled"
	}
	return "disabled"
}

func formatHints(hints map[string]string) string {
	if len(hints) == 0 {
		return ""
	}

	keys := make([]string, 0, len(hints))
	for key := range hints {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, hints[key]))
	}

	return strings.Join(parts, ", ")
}

func configuredString(v bool) string {
	if !v {
		return ""
	}
	return "configured"
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "not detected"
	}
	return value
}

func displayModelName(cfg discovery.RuntimeConfig) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	return cfg.ServedModelName
}

func listenAddress(cfg discovery.RuntimeConfig) string {
	host := cfg.Host
	if host == "" {
		host = "not detected"
	}
	if cfg.Port == 0 {
		return host
	}

	return fmt.Sprintf("%s:%d", host, cfg.Port)
}
