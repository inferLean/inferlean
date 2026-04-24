package collection

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/internal/ui/progress"
)

type View struct {
	steps         *progress.Stepper
	noInteractive bool
}

func NewView() View {
	return View{
		steps: progress.New("collect", stepperEnabled(false)),
	}
}

func (v *View) SetNoInteractive(noInteractive bool) {
	if v.noInteractive == noInteractive && v.steps != nil {
		return
	}
	v.noInteractive = noInteractive
	v.steps = progress.New("collect", stepperEnabled(noInteractive))
}

func (v *View) ShowStart(seconds float64) {
	v.getStepper().Begin(fmt.Sprintf("collecting for %.0fs", seconds))
}

func (v *View) ShowStep(message string) {
	v.getStepper().Step(message)
}

func (v *View) ShowDone(runID string) {
	v.getStepper().Done(fmt.Sprintf("artifact captured (run_id=%s)", runID))
}

func (v *View) getStepper() *progress.Stepper {
	if v.steps == nil {
		v.steps = progress.New("collect", stepperEnabled(v.noInteractive))
	}
	return v.steps
}

func stepperEnabled(noInteractive bool) bool {
	return progress.InteractiveTTY() && !noInteractive
}
