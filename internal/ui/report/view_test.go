package report

import (
	"strings"
	"testing"
)

func TestFormatReportForDisplayStructured(t *testing.T) {
	t.Parallel()
	content, summary, err := formatReportForDisplay(map[string]any{
		"schema_version": "report-v1",
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
			},
			"scenario_overlays": map[string]any{
				"latency":    map[string]any{"target": "latency"},
				"balanced":   map[string]any{"target": "balanced"},
				"throughput": map[string]any{"target": "throughput"},
			},
		},
		"diagnostic_coverage": map[string]any{
			"summary": map[string]any{
				"required_total": 1,
			},
		},
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
