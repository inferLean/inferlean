package runbrowser

import (
	"encoding/json"
	"testing"
)

func TestRenderArtifactTree(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"job": map[string]any{
			"run_id": "run-123",
		},
		"metrics": map[string]any{
			"gpu_utilization": 0.8,
		},
		"issues": []any{"missing-prometheus", "short-window"},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	got, err := RenderArtifactTree(raw)
	if err != nil {
		t.Fatalf("RenderArtifactTree() error = %v", err)
	}

	for _, want := range []string{
		"`- artifact",
		"   |- issues [2]",
		"   |  |- [0]: \"missing-prometheus\"",
		"   |  `- [1]: \"short-window\"",
		"   |- job",
		"   `- metrics",
	} {
		if !containsLine(got, want) {
			t.Fatalf("tree missing line %q\n%s", want, got)
		}
	}
}

func containsLine(text, want string) bool {
	for _, line := range splitLines(text) {
		if line == want {
			return true
		}
	}
	return false
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for idx := range text {
		if text[idx] == '\n' {
			lines = append(lines, text[start:idx])
			start = idx + 1
		}
	}
	lines = append(lines, text[start:])
	return lines
}
