package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func buildEvidenceCard(report contracts.FinalReport, artifact *contracts.RunArtifact) reportCardViewModel {
	return reportCardViewModel{
		id:              "evidence",
		title:           "Evidence Context",
		summary:         "Runtime config, environment, and process evidence",
		defaultExpanded: true,
		tabs: []reportTabViewModel{
			buildConfigTab(report, artifact),
			buildEnvironmentTab(report, artifact),
			buildProcessTab(artifact),
		},
	}
}

func buildCollectionQualityCard(report contracts.FinalReport, artifact *contracts.RunArtifact) reportCardViewModel {
	quality := effectiveQuality(report, artifact)
	lines := []string{
		"Completeness: " + fmt.Sprintf("%.2f", quality.completeness),
		"Summary: " + fallback(quality.summary, "-"),
		"Telemetry Mode: " + fallback(quality.telemetryMode, "-"),
		"GPU Path: " + fallback(quality.selectedGPUPath, "-"),
		"Missing Evidence: " + joinOrDash(quality.missingEvidence),
		"Degraded Evidence: " + joinOrDash(quality.degradedEvidence),
		"Fallbacks: " + joinOrDash(quality.fallbacks),
		"Confidence Impact: " + fallback(quality.confidenceImpact, "-"),
	}
	sections := []reportSectionViewModel{{lines: lines}}
	if len(quality.sourceStates) > 0 {
		sections = append(sections, reportSectionViewModel{
			title: "Source States",
			table: &reportTableViewModel{
				headers: []string{"Source", "Status", "Reason", "Transport", "Fallback"},
				rows:    sourceStateRows(quality.sourceStates),
			},
		})
	}
	return reportCardViewModel{
		id:              "collection-quality",
		title:           "Collection Quality",
		summary:         fallback(quality.summary, fallback(quality.telemetryMode, "Collection quality")),
		defaultExpanded: true,
		sections:        sections,
	}
}

func buildConfigTab(report contracts.FinalReport, artifact *contracts.RunArtifact) reportTabViewModel {
	if artifact == nil {
		return reportTabViewModel{
			title:   "config",
			summary: "Runtime config artifact unavailable",
			sections: []reportSectionViewModel{{lines: []string{
				"Runtime config evidence is unavailable in this report-only view.",
			}}},
		}
	}
	return reportTabViewModel{
		title:   "config",
		summary: "Runtime config values; cited keys are referenced by report findings",
		sections: []reportSectionViewModel{{
			table: &reportTableViewModel{
				headers: []string{"Key", "Value", "Note"},
				rows:    runtimeConfigRows(artifact.RuntimeConfig, citedRuntimeConfigKeys(report)),
			},
		}},
	}
}

func buildEnvironmentTab(report contracts.FinalReport, artifact *contracts.RunArtifact) reportTabViewModel {
	rows := environmentRows(report.Environment, artifact)
	return reportTabViewModel{
		title:   "environment",
		summary: "Host, GPU, and runtime context",
		sections: []reportSectionViewModel{{
			table: &reportTableViewModel{
				headers: []string{"Field", "Value"},
				rows:    rows,
			},
		}},
	}
}

func buildProcessTab(artifact *contracts.RunArtifact) reportTabViewModel {
	if artifact == nil {
		return reportTabViewModel{
			title:   "process",
			summary: "Process inspection artifact unavailable",
			sections: []reportSectionViewModel{{lines: []string{
				"Process inspection evidence is unavailable in this report-only view.",
			}}},
		}
	}
	process := artifact.ProcessInspection
	lines := []string{
		"PID: " + int32String(process.TargetProcess.PID),
		"Internal PID: " + int32String(process.TargetProcess.InternalPID),
		"Executable: " + fallback(process.TargetProcess.Executable, "-"),
		"Started At: " + formatTimePtr(process.TargetProcess.StartedAt),
		"Command Line: " + fallback(process.TargetProcess.RawCommandLine, "-"),
		"Parse Warnings: " + joinOrDash(process.ParseWarnings),
		"Probe Warnings: " + joinOrDash(process.ProbeWarnings),
	}
	sections := []reportSectionViewModel{{title: "Target Process", lines: lines}}
	if len(process.RelatedProcesses) > 0 {
		sections = append(sections, reportSectionViewModel{
			title: "Related Processes",
			table: &reportTableViewModel{
				headers: []string{"PID", "Executable", "Started", "Command"},
				rows:    relatedProcessRows(process.RelatedProcesses),
			},
		})
	}
	return reportTabViewModel{
		title:    "process",
		summary:  "Target process and related process inspection",
		sections: sections,
	}
}

func runtimeConfigRows(config contracts.RuntimeConfig, cited map[string]bool) [][]string {
	values := mapFromJSON(config)
	rows := make([][]string, 0, len(values))
	flattenConfigRows("", values, cited, &rows)
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	return rows
}

func flattenConfigRows(prefix string, values map[string]any, cited map[string]bool, rows *[][]string) {
	for key, value := range values {
		if key == "coverage" || key == "env_hints" {
			continue
		}
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		if nested, ok := value.(map[string]any); ok {
			flattenConfigRows(fullKey, nested, cited, rows)
			continue
		}
		note := ""
		if isCitedConfigKey(fullKey, cited) {
			note = "cited"
		}
		*rows = append(*rows, []string{fullKey, formatEvidenceValue(value), note})
	}
}

func citedRuntimeConfigKeys(report contracts.FinalReport) map[string]bool {
	cited := map[string]bool{}
	add := func(ref string) {
		key, ok := strings.CutPrefix(strings.TrimSpace(ref), "runtime_config.")
		if ok && key != "" {
			cited[key] = true
		}
	}
	for _, detector := range report.DiagnosticCoverage.DetectorResults {
		for _, ref := range detector.RequiredEvidenceRefs {
			add(ref)
		}
	}
	for _, issue := range report.Issues {
		for _, ref := range issue.EvidenceRefs {
			add(ref)
		}
	}
	for _, opportunity := range report.Opportunities {
		for _, ref := range opportunity.EvidenceRefs {
			add(ref)
		}
	}
	return cited
}

func isCitedConfigKey(key string, cited map[string]bool) bool {
	if cited[key] {
		return true
	}
	for citedKey := range cited {
		if strings.HasPrefix(citedKey, key+".") || strings.HasPrefix(key, citedKey+".") {
			return true
		}
	}
	return false
}

func environmentRows(env contracts.ReportEnvironment, artifact *contracts.RunArtifact) [][]string {
	if artifact != nil {
		return [][]string{
			{"os", fallback(artifact.Environment.OS, "-")},
			{"kernel", fallback(artifact.Environment.Kernel, "-")},
			{"cpu_model", fallback(artifact.Environment.CPUModel, "-")},
			{"cpu_cores", formatInt(artifact.Environment.CPUCores)},
			{"memory", formatBytes(artifact.Environment.MemoryBytes)},
			{"gpu_model", fallback(artifact.Environment.GPUModel, "-")},
			{"gpu_count", formatInt(artifact.Environment.GPUCount)},
			{"driver_version", fallback(artifact.Environment.DriverVersion, "-")},
			{"runtime_version", fallback(artifact.Environment.RuntimeVersion, "-")},
			{"model", fallback(artifact.RuntimeConfig.Model, "-")},
			{"served_model_name", fallback(artifact.RuntimeConfig.ServedModelName, "-")},
			{"vllm_version", fallback(artifact.RuntimeConfig.VLLMVersion, "-")},
			{"torch_version", fallback(artifact.RuntimeConfig.TorchVersion, "-")},
			{"cuda_runtime_version", fallback(artifact.RuntimeConfig.CUDARuntimeVersion, "-")},
		}
	}
	return [][]string{
		{"host", fallback(env.Host, "-")},
		{"os", fallback(env.OS, "-")},
		{"kernel", fallback(env.Kernel, "-")},
		{"cpu", environmentCPU(env)},
		{"memory", formatBytes(env.MemoryBytes)},
		{"gpu", environmentGPU(env)},
		{"driver", fallback(env.DriverVersion, "-")},
		{"runtime", fallback(env.RuntimeVersion, "-")},
		{"model", fallback(firstNonEmpty(env.ServedModelName, env.Model), "-")},
		{"vllm_version", fallback(env.VLLMVersion, "-")},
		{"torch_version", fallback(env.TorchVersion, "-")},
		{"cuda_runtime_version", fallback(env.CUDARuntimeVersion, "-")},
	}
}

func relatedProcessRows(processes []contracts.ObservedProcess) [][]string {
	rows := make([][]string, 0, len(processes))
	for _, process := range processes {
		rows = append(rows, []string{
			int32String(process.PID),
			fallback(process.Executable, "-"),
			formatTimePtr(process.StartedAt),
			fallback(process.RawCommandLine, "-"),
		})
	}
	return rows
}

func sourceStateRows(states map[string]contracts.SourceState) [][]string {
	keys := make([]string, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, key := range keys {
		state := states[key]
		fallbackUsed := "-"
		if state.Fallback {
			fallbackUsed = "yes"
		}
		rows = append(rows, []string{
			key,
			fallback(state.Status, "-"),
			fallback(state.Reason, "-"),
			fallback(state.Transport, "-"),
			fallbackUsed,
		})
	}
	return rows
}
