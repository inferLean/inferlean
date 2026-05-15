package report

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
)

func TestFormatReportForDisplayStructured(t *testing.T) {
	t.Parallel()
	content, summary, err := formatReportForDisplay(map[string]any{
		"schema_version": "report-v5",
		"job": map[string]any{
			"run_id": "run_123",
		},
		"entitlement": map[string]any{
			"tier": "paid",
		},
		"diagnosis": map[string]any{
			"base_diagnosis": map[string]any{},
		},
		"diagnostic_coverage": map[string]any{
			"summary": map[string]any{
				"required_total": 1,
			},
		},
		"saturation": saturationPayload(),
		"opportunities": []map[string]any{{
			"id":          "opportunity:quantized_model_opportunity",
			"rank":        1,
			"detector_id": "quantized_model_opportunity",
			"title":       "Evaluate Qwen/Qwen3-32B-FP8",
			"recommendation": map[string]any{
				"decision":         "evaluate_quantized_model_path",
				"title":            "Evaluate Qwen/Qwen3-32B-FP8",
				"projected_effect": projectedEffectPayload(),
				"actions": []map[string]any{{
					"id":    "action:evaluate-quantized-model",
					"title": "Test the quantized model under the same workload",
				}},
			},
		}},
		"issues": []map[string]any{{
			"id":          "issue:kv_pressure_preemption_or_swap",
			"rank":        1,
			"detector_id": "kv_pressure_preemption_or_swap",
			"label":       "KV cache pressure",
			"recommendation": map[string]any{
				"title":            "Reduce KV footprint",
				"rationale":        "KV pressure is causing preemption, so widening scheduler posture first would be premature.",
				"confidence":       "high",
				"projected_effect": projectedEffectPayload(),
				"actions": []map[string]any{{
					"id":             "action:reduce-max-model-len",
					"title":          "Reduce `--max-model-len`",
					"current_value":  "8192 (default)",
					"proposed_value": "4096",
					"value_kind":     "number",
				}},
				"follow_up_steps": []map[string]any{{
					"id":    "action:rerun-under-same-load",
					"title": "Rerun at the same offered load",
					"how":   "Keep prompt/output mix and client concurrency stable for the comparison run.",
				}},
			},
		}},
		"collection_quality": map[string]any{},
	}, false)
	if err != nil {
		t.Fatalf("formatReportForDisplay() error = %v", err)
	}
	if !strings.Contains(content, "Overview") || !strings.Contains(content, "Diagnosis") {
		t.Fatalf("formatted report missing expected sections: %s", content)
	}
	if !strings.Contains(content, "KV cache pressure") {
		t.Fatalf("formatted report missing parsed limiter label: %s", content)
	}
	if strings.Contains(content, "Capacity Snapshot") {
		t.Fatalf("formatted report includes removed capacity snapshot: %s", content)
	}
	if !strings.Contains(content, "Projected Effect: Likely improvement: +5% to +15% throughput under the observed workload.") {
		t.Fatalf("formatted report missing projected effect: %s", content)
	}
	if !strings.Contains(content, "Request Throughput: request_throughput 4.00 req/s -> 4.40 req/s (+10.0%)") {
		t.Fatalf("formatted report missing throughput projection: %s", content)
	}
	if !strings.Contains(content, "Current: 8192 (default)") || !strings.Contains(content, "Proposed: 4096") {
		t.Fatalf("formatted report missing action delta: %s", content)
	}
	if !strings.Contains(content, "Follow-up Steps") || !strings.Contains(content, "Rerun at the same offered load") {
		t.Fatalf("formatted report missing follow-up steps: %s", content)
	}
	if !strings.Contains(content, "Opportunities") || !strings.Contains(content, "Qwen/Qwen3-32B-FP8") {
		t.Fatalf("formatted report missing quantization opportunity: %s", content)
	}
	if !strings.Contains(summary, "run=run_123") {
		t.Fatalf("summary missing run id: %s", summary)
	}
}

func TestFormatReportForDisplayFallback(t *testing.T) {
	t.Parallel()
	content, summary, err := formatReportForDisplay(map[string]any{
		"schema_version": "unexpected",
		"blob":           map[string]any{"a": 1},
	}, false)
	if err != nil {
		t.Fatalf("formatReportForDisplay() error = %v", err)
	}
	if !strings.Contains(content, "Schema validation warning:") {
		t.Fatalf("expected schema warning, got: %s", content)
	}
	if !strings.Contains(summary, "schema-warning") {
		t.Fatalf("summary should include schema warning marker: %s", summary)
	}
}

func TestRenderNonInteractivePrintsPlainReport(t *testing.T) {
	payload := map[string]any{
		"schema_version": "report-v5",
		"job": map[string]any{
			"run_id": "run_123",
		},
		"entitlement": map[string]any{},
		"diagnosis": map[string]any{
			"base_diagnosis": map[string]any{
				"confidence": "medium",
			},
		},
		"diagnostic_coverage": map[string]any{},
		"saturation":          saturationPayload(),
		"issues": []map[string]any{{
			"id":          "issue:gpu_memory_pressure",
			"rank":        1,
			"detector_id": "gpu_memory_pressure",
			"label":       "GPU memory pressure",
			"recommendation": map[string]any{
				"title":            "Reduce GPU memory pressure",
				"projected_effect": projectedEffectPayload(),
				"actions": []map[string]any{{
					"id":    "action:test",
					"title": "Test under the same load",
				}},
			},
		}},
		"collection_quality": map[string]any{},
	}

	output := captureStdout(t, func() {
		NewView().Render(payload, RenderOptions{NonInteractive: true, BackendURL: defaults.AppBaseURL})
	})

	if !strings.Contains(output, "[report] parsed report") {
		t.Fatalf("rendered output missing report header: %s", output)
	}
	if !strings.Contains(output, "GPU memory pressure") {
		t.Fatalf("rendered output missing full report content: %s", output)
	}
	if strings.Contains(output, "\x1b[?1049h") || strings.Contains(output, "\x1b[?1049l") {
		t.Fatalf("non-interactive output must not use the alternate screen: %q", output)
	}
}

func saturationPayload() map[string]any {
	return map[string]any{
		"version": "saturation-v1",
		"generic": map[string]any{
			"id":     "generic",
			"label":  "Generic saturation",
			"status": "ok",
			"score":  map[string]any{"latest": 75, "avg": 75},
			"reason": "Maximum observed saturation across evaluated dimensions.",
		},
		"dimensions": []map[string]any{{
			"id":              "compute",
			"label":           "Compute / SM saturation",
			"bottleneck_type": "compute",
			"status":          "ok",
			"score":           map[string]any{"latest": 75, "avg": 75},
			"reason":          "compute saturation estimated from collected metrics.",
		}},
	}
}

func projectedEffectPayload() map[string]any {
	return map[string]any{
		"summary": "Likely improvement: +5% to +15% throughput under the observed workload.",
		"latency": map[string]any{
			"metric":        "latency_e2e_seconds",
			"unit":          "s",
			"current":       1.2,
			"projected":     1.2,
			"delta":         0.0,
			"percent_delta": 0.0,
			"direction":     "lower_is_better",
			"confidence":    "medium",
		},
		"throughput": map[string]any{
			"requests": map[string]any{
				"metric":        "request_throughput",
				"unit":          "req/s",
				"current":       4.0,
				"projected":     4.4,
				"delta":         0.4,
				"percent_delta": 10.0,
				"direction":     "higher_is_better",
				"confidence":    "medium",
			},
			"output_tokens": map[string]any{
				"metric":        "generation_tokens_per_second",
				"unit":          "tok/s",
				"current":       200.0,
				"projected":     220.0,
				"delta":         20.0,
				"percent_delta": 10.0,
				"direction":     "higher_is_better",
				"confidence":    "medium",
			},
		},
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return string(data)
}
