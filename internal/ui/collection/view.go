package collection

import (
	"fmt"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/ui/progress"
)

type View struct {
	steps         *progress.Stepper
	noInteractive bool
}

const interactiveCollectionHint = " | m:+1m M:-1m s:+15s S:-15s c:stop now | longer collection improves report quality"

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

func (v *View) ShowMetricsCollectionStart(remaining time.Duration) {
	v.getStepper().Step(renderMetricsCollectionCountdown(remaining, v.interactive()))
}

func (v *View) ShowMetricsCollectionCountdown(remaining time.Duration) {
	v.getStepper().UpdateActive(renderMetricsCollectionCountdown(remaining, v.interactive()))
}

func (v *View) ShowMetricsCollectionStopped() {
	if !v.interactive() {
		return
	}
	v.getStepper().UpdateActive("collection stop requested; analyzing based on current data")
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

func (v *View) interactive() bool {
	return stepperEnabled(v.noInteractive)
}

func renderMetricsCollectionCountdown(remaining time.Duration, interactive bool) string {
	seconds := int(remaining.Round(time.Second) / time.Second)
	if remaining > 0 && remaining < time.Second {
		seconds = 1
	}
	if seconds < 0 {
		seconds = 0
	}
	line := fmt.Sprintf("collecting metrics through prometheus scrape manager (%ds remaining)", seconds)
	if !interactive {
		return line
	}
	return line + interactiveCollectionHint
}
