package discovery

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/internal/ui/progress"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

type View struct {
	steps *progress.Stepper
}

func NewView() View {
	return View{
		steps: progress.New("discovery", progress.InteractiveTTY()),
	}
}

func (v *View) ShowStart() {
	stepper := v.getStepper()
	stepper.Begin("scanning for running vLLM targets")
	stepper.Step("inspecting processes, containers, and pods")
}

func (v *View) ShowCandidates(items []vllmdiscovery.Candidate) {
	v.getStepper().Done(fmt.Sprintf("found %d candidate(s)", len(items)))
}

func (v *View) Select(candidates []vllmdiscovery.Candidate, noInteractive bool) (vllmdiscovery.Candidate, error) {
	if len(candidates) == 0 {
		return vllmdiscovery.Candidate{}, fmt.Errorf("no vLLM targets discovered")
	}
	if !shouldUseTUI(noInteractive, candidates) {
		return candidates[0], nil
	}
	return selectCandidateTUI(candidates)
}

func (v *View) ShowSelected(item vllmdiscovery.Candidate) {
	fmt.Printf("[discovery] selected source=%s %s\n", item.Source, targetLabel(item))
}

func (v *View) getStepper() *progress.Stepper {
	if v.steps == nil {
		v.steps = progress.New("discovery", progress.InteractiveTTY())
	}
	return v.steps
}
