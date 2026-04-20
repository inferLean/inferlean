package collection

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/internal/ui/progress"
)

type View struct {
	steps *progress.Stepper
}

func NewView() View {
	return View{
		steps: progress.New("collect", progress.InteractiveTTY()),
	}
}

func (v *View) ShowStart(seconds float64) {
	v.getStepper().Begin(fmt.Sprintf("collecting for %.0fs", seconds))
}

func (v *View) ShowStep(message string) {
	v.getStepper().Step(message)
}

func (v *View) ShowDone(path string) {
	v.getStepper().Done(fmt.Sprintf("artifact written: %s", path))
}

func (v *View) getStepper() *progress.Stepper {
	if v.steps == nil {
		v.steps = progress.New("collect", progress.InteractiveTTY())
	}
	return v.steps
}
