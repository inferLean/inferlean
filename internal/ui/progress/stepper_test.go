package progress

import "testing"

func TestStepperModelUpdateActiveReplacesCurrentStepLabel(t *testing.T) {
	t.Parallel()
	model := newStepperModel("collect", false)
	updated, _ := model.Update(stepMsg{label: "collecting metrics (30s remaining)"})
	modelAfterStep, ok := updated.(stepperModel)
	if !ok {
		t.Fatalf("unexpected model type after step update")
	}
	updated, _ = modelAfterStep.Update(updateActiveMsg{label: "collecting metrics (29s remaining)"})
	modelAfterCountdown, ok := updated.(stepperModel)
	if !ok {
		t.Fatalf("unexpected model type after countdown update")
	}
	if len(modelAfterCountdown.steps) != 1 {
		t.Fatalf("expected one step, got %d", len(modelAfterCountdown.steps))
	}
	if modelAfterCountdown.steps[0].label != "collecting metrics (29s remaining)" {
		t.Fatalf("unexpected active label: %s", modelAfterCountdown.steps[0].label)
	}
	if modelAfterCountdown.steps[0].status != statusActive {
		t.Fatalf("expected active step status, got %d", modelAfterCountdown.steps[0].status)
	}
}

func TestStepperModelUpdateActiveAddsStepWhenMissing(t *testing.T) {
	t.Parallel()
	model := newStepperModel("collect", false)
	updated, _ := model.Update(updateActiveMsg{label: "collecting metrics (30s remaining)"})
	modelAfterUpdate, ok := updated.(stepperModel)
	if !ok {
		t.Fatalf("unexpected model type")
	}
	if len(modelAfterUpdate.steps) != 1 {
		t.Fatalf("expected one step, got %d", len(modelAfterUpdate.steps))
	}
	if modelAfterUpdate.steps[0].label != "collecting metrics (30s remaining)" {
		t.Fatalf("unexpected step label: %s", modelAfterUpdate.steps[0].label)
	}
	if modelAfterUpdate.steps[0].status != statusActive {
		t.Fatalf("expected active step status, got %d", modelAfterUpdate.steps[0].status)
	}
}
