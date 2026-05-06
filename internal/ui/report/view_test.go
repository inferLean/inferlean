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
		"schema_version": "report-v2",
		"job": map[string]any{
			"run_id": "run_123",
		},
		"entitlement": map[string]any{
			"tier": "paid",
		},
		"diagnosis": map[string]any{
			"base_diagnosis": map[string]any{
				"situation": map[string]any{
					"headline": "GPU memory pressure",
				},
				"current_limiter": map[string]any{
					"label":  "KV cache pressure",
					"family": "kv_pressure",
				},
				"capacity_snapshot": map[string]any{
					"summary":    "Conservative workload-scoped snapshot.",
					"confidence": "medium",
					"pressures": map[string]any{
						"compute":          "medium",
						"memory_bandwidth": "low",
						"kv":               "high",
					},
					"observed": map[string]any{
						"prompt_tokens_per_second":     800.0,
						"generation_tokens_per_second": 200.0,
						"request_throughput":           4.0,
					},
					"current_frontier": map[string]any{
						"generation_tokens_per_second": 240.0,
						"request_throughput":           4.8,
					},
				},
				"recommendation": map[string]any{
					"title":      "Reduce KV footprint",
					"rationale":  "KV pressure is causing preemption, so widening scheduler posture first would be premature.",
					"risk":       "medium",
					"confidence": "high",
					"tradeoff": map[string]any{
						"summary": "May reduce maximum accepted context length for some requests.",
					},
					"expected_effect": map[string]any{
						"summary": "Likely improvement: +5% to +15% throughput under the observed workload.",
					},
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
			},
			"target_overlay": map[string]any{"target": "throughput"},
		},
		"diagnostic_coverage": map[string]any{
			"summary": map[string]any{
				"required_total": 1,
			},
		},
		"diagnostic_lenses":  quantizationLensFixture(),
		"collection_quality": map[string]any{},
		"ui_hints":           map[string]any{},
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
	if !strings.Contains(content, "Capacity Snapshot") || !strings.Contains(content, "prompt_tok/s=800.00") {
		t.Fatalf("formatted report missing capacity snapshot: %s", content)
	}
	if !strings.Contains(content, "Expected Gain Range: Likely improvement: +5% to +15% throughput under the observed workload.") {
		t.Fatalf("formatted report missing gain range: %s", content)
	}
	if !strings.Contains(content, "Current: 8192 (default)") || !strings.Contains(content, "Proposed: 4096") {
		t.Fatalf("formatted report missing action delta: %s", content)
	}
	if !strings.Contains(content, "Follow-up Steps") || !strings.Contains(content, "Rerun at the same offered load") {
		t.Fatalf("formatted report missing follow-up steps: %s", content)
	}
	if !strings.Contains(content, "Quantization Opportunity") || !strings.Contains(content, "Qwen/Qwen3-32B-FP8") {
		t.Fatalf("formatted report missing quantization lens: %s", content)
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
		"schema_version": "report-v2",
		"job": map[string]any{
			"run_id": "run_123",
		},
		"entitlement": map[string]any{},
		"diagnosis": map[string]any{
			"base_diagnosis": map[string]any{
				"situation": map[string]any{
					"headline": "GPU memory pressure",
				},
			},
		},
		"diagnostic_coverage": map[string]any{},
		"diagnostic_lenses":   quantizationLensFixture(),
		"collection_quality":  map[string]any{},
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

func quantizationLensFixture() map[string]any {
	return map[string]any{
		"quantization": map[string]any{
			"id": "lens:quantization",
			"current_posture": map[string]any{
				"model_id":       "Qwen/Qwen3-32B",
				"dtype":          "bfloat16",
				"quantization":   "none",
				"kv_cache_dtype": "auto",
				"gpu_family":     "hopper",
			},
			"selected_candidate": map[string]any{
				"family":     "fp8",
				"repo":       "Qwen/Qwen3-32B-FP8",
				"source":     "verified_allowlist",
				"confidence": "medium",
			},
			"recommendation": map[string]any{
				"decision":   "evaluate_quantized_model_path",
				"title":      "Evaluate Qwen/Qwen3-32B-FP8",
				"rationale":  "Treat quantization as a validation-first opportunity.",
				"confidence": "medium",
				"actions": []map[string]any{{
					"id":             "action:evaluate-quantized-candidate",
					"title":          "Run a side-by-side quantized checkpoint validation",
					"current_value":  "Qwen/Qwen3-32B dtype=bfloat16 quantization=none",
					"proposed_value": "Qwen/Qwen3-32B-FP8",
				}},
				"follow_up_steps": []map[string]any{{
					"id":    "action:validate-quality",
					"title": "Validate answer quality before rollout",
					"how":   "Compare representative prompts against the current checkpoint and keep acceptance criteria explicit.",
				}},
			},
			"target_overlay": map[string]any{
				"target": "throughput",
				"gain_range": map[string]any{
					"percent_low":  8,
					"percent_high": 20,
				},
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
