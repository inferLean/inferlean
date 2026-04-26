package collection

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/inferLean/inferlean-main/cli/internal/ui/progress"
)

type View struct {
	steps         *progress.Stepper
	noInteractive bool
}

type Hint struct {
	Key   string
	Value string
}

var hintStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#777777", Dark: "#777777"})
var hintButtonStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#eeeeee", Dark: "#eeeeee"})
var keyDescription = []Hint{
	{"M:        ", "increase collection time by 1 minute"},
	{"SHIFT + M:", "decrease collection time by 1 minute"},
	{"S:        ", "increase collection time by 15 seconds"},
	{"SHIFT + S:", "decrease collection time by 15 seconds"},
	{"C:        ", "stop now and analyze"},
}

func interactiveCollectionHint() string {
	var hint = hintStyle.Render("\n\n\t\tlonger collection improves report quality\n")
	for _, item := range keyDescription {
		hint = hint + "\n\t\t" + hintButtonStyle.Render(item.Key) + " " + hintStyle.Render(item.Value)
	}
	return hint
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

func (v *View) Abort() {
	v.getStepper().Abort()
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
	hint := interactiveCollectionHint()
	if seconds < 2 {
		hint = ""
	}
	return line + hint
}
