package discovery

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/internal/ui/progress"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

type View struct {
	steps          *progress.Stepper
	nonInteractive bool
}

func NewView() View {
	return View{
		steps: progress.New("discovery", stepperEnabled(false)),
	}
}

func (v *View) SetNonInteractive(nonInteractive bool) {
	if v.nonInteractive == nonInteractive && v.steps != nil {
		return
	}
	v.nonInteractive = nonInteractive
	v.steps = progress.New("discovery", stepperEnabled(nonInteractive))
}

func (v *View) ShowStart() {
	stepper := v.getStepper()
	stepper.Begin(startMessage())
}

func startMessage() string {
	return "scanning for running vLLM targets"
}

func (v *View) ShowSourceStart(source string) {
	label := "checking " + sourceLabel(source)
	if stepperEnabled(v.nonInteractive) {
		label += " (press c to cancel current source)"
	}
	v.getStepper().Step(label)
}

func (v *View) ShowSourceCancelled(source string) {
	v.getStepper().Step("cancelled " + sourceLabel(source) + "; continuing")
}

func (v *View) ShowCandidates(items []vllmdiscovery.Candidate) {
	v.getStepper().Done(fmt.Sprintf("found %d candidate(s)", len(items)))
}

func (v *View) Abort() {
	v.getStepper().Abort()
}

func (v *View) Select(candidates []vllmdiscovery.Candidate, nonInteractive bool) (vllmdiscovery.Candidate, error) {
	if len(candidates) == 0 {
		return vllmdiscovery.Candidate{}, fmt.Errorf("no vLLM targets discovered")
	}
	if !shouldUseTUI(nonInteractive, candidates) {
		return candidates[0], nil
	}
	return selectCandidateTUI(candidates)
}

func (v *View) ShowSelected(item vllmdiscovery.Candidate) {
	fmt.Printf("[discovery] selected source=%s %s\n", item.Source, targetLabel(item))
}

func (v *View) getStepper() *progress.Stepper {
	if v.steps == nil {
		v.steps = progress.New("discovery", stepperEnabled(v.nonInteractive))
	}
	return v.steps
}

func stepperEnabled(nonInteractive bool) bool {
	return progress.InteractiveTTY() && !nonInteractive
}

func sourceLabel(source string) string {
	switch source {
	case "docker":
		return "docker containers"
	default:
		return source
	}
}
