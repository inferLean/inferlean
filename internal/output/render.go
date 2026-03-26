package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/inferLean/inferlean/internal/discovery"
)

func RenderDiscovery(w io.Writer, result discovery.Result) {
	if result.Selected == nil {
		return
	}

	target := result.Selected
	fmt.Fprintf(w, "InferLean found a vLLM deployment.\n\n")
	fmt.Fprintf(w, "Selected target\n")
	fmt.Fprintf(w, "  Model: %s\n", valueOrUnknown(target.RuntimeConfig.Model))
	fmt.Fprintf(w, "  PID: %d", target.PrimaryPID)
	if target.ProcessCount > 1 {
		fmt.Fprintf(w, " (%d related processes)", target.ProcessCount)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Entry point: %s\n", target.EntryPoint)
	fmt.Fprintf(w, "  Listen address: %s\n", listenAddress(target.RuntimeConfig))
	fmt.Fprintf(w, "  Why this target: %s\n", result.Reason)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Parsed runtime configuration")
	writeConfigLine(w, "Model", target.RuntimeConfig.Model)
	writeConfigLine(w, "Host", target.RuntimeConfig.Host)
	writeConfigLine(w, "Port", intString(target.RuntimeConfig.Port))
	writeConfigLine(w, "Tensor parallel size", intString(target.RuntimeConfig.TensorParallelSize))
	writeConfigLine(w, "Data parallel size", intString(target.RuntimeConfig.DataParallelSize))
	writeConfigLine(w, "Pipeline parallel size", intString(target.RuntimeConfig.PipelineParallelSize))
	writeConfigLine(w, "Max model len", intString(target.RuntimeConfig.MaxModelLen))
	writeConfigLine(w, "Max batched tokens", intString(target.RuntimeConfig.MaxNumBatchedTokens))
	writeConfigLine(w, "Max sequences", intString(target.RuntimeConfig.MaxNumSeqs))
	writeConfigLine(w, "GPU memory utilization", floatString(target.RuntimeConfig.GPUMemoryUtilization))
	writeConfigLine(w, "KV cache dtype", target.RuntimeConfig.KVCacheDType)
	writeConfigLine(w, "Chunked prefill", boolPointerString(target.RuntimeConfig.ChunkedPrefill))
	writeConfigLine(w, "Prefix caching", boolPointerString(target.RuntimeConfig.PrefixCaching))
	writeConfigLine(w, "Quantization", target.RuntimeConfig.Quantization)
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
		fmt.Fprintf(w, "  - PID %d", candidate.PrimaryPID)
		if candidate.RuntimeConfig.Model != "" {
			fmt.Fprintf(w, " • %s", candidate.RuntimeConfig.Model)
		}
		if candidate.RuntimeConfig.Port > 0 {
			fmt.Fprintf(w, " • port %d", candidate.RuntimeConfig.Port)
		}
		if candidate.ProcessCount > 1 {
			fmt.Fprintf(w, " • %d related processes", candidate.ProcessCount)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Rerun with --pid to select one deployment explicitly.")
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

func valueOrUnknown(value string) string {
	if value == "" {
		return "not detected"
	}
	return value
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
