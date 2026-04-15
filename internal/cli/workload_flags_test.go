package cli

import "testing"

func TestNormalizeWorkloadInputsUsesDefaultSemanticValues(t *testing.T) {
	t.Parallel()

	inputs, err := normalizeWorkloadInputs(workloadFlagValues{
		mode:           "mixed",
		target:         "balanced",
		repeatedPrefix: false,
	})
	if err != nil {
		t.Fatalf("normalizeWorkloadInputs() error = %v", err)
	}
	if inputs.mode != "mixed" {
		t.Fatalf("mode = %q, want %q", inputs.mode, "mixed")
	}
	if inputs.target != "balanced" {
		t.Fatalf("target = %q, want %q", inputs.target, "balanced")
	}
	if inputs.repeatedPrefix == nil || *inputs.repeatedPrefix {
		t.Fatalf("repeatedPrefix = %v, want false", inputs.repeatedPrefix)
	}
}
