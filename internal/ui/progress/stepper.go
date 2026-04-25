package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/inferLean/inferlean-main/cli/internal/ui/chrome"
	"golang.org/x/term"
)

const (
	statusPending = iota
	statusActive
	statusDone
)

const (
	ansiReset  = "\x1b[0m"
	ansiCyan   = "\x1b[36m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

type stepState struct {
	label  string
	status int
}

type beginMsg struct {
	title string
}

type stepMsg struct {
	label string
}

type updateActiveMsg struct {
	label string
}

type doneMsg struct {
	summary string
}

type stepperModel struct {
	component string
	useColor  bool
	spin      spinner.Model
	title     string
	steps     []stepState
	summary   string
}

var (
	stepperHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005F87", Dark: "#5FD7FF"})
	stepperActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#875F00", Dark: "#FFD75F"})
	stepperDoneStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005F00", Dark: "#87FF87"})
)

func newStepperModel(component string, useColor bool) stepperModel {
	spin := spinner.New(spinner.WithSpinner(spinner.Line))
	return stepperModel{
		component: component,
		useColor:  useColor,
		spin:      spin,
	}
}

func (m stepperModel) Init() tea.Cmd {
	return m.spin.Tick
}

func (m stepperModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case beginMsg:
		m.title = typed.title
		m.steps = nil
		m.summary = ""
		return m, nil
	case stepMsg:
		m.markActiveDone()
		m.steps = append(m.steps, stepState{label: typed.label, status: statusActive})
		return m, nil
	case updateActiveMsg:
		if idx := m.activeStepIndex(); idx >= 0 {
			m.steps[idx].label = typed.label
			return m, nil
		}
		m.steps = append(m.steps, stepState{label: typed.label, status: statusActive})
		return m, nil
	case doneMsg:
		m.markActiveDone()
		m.summary = strings.TrimSpace(typed.summary)
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m stepperModel) View() string {
	lines := make([]string, 0, len(m.steps)+4)
	lines = append(lines, chrome.Render(chrome.UseColor()), "")
	if strings.TrimSpace(m.title) != "" {
		header := "[" + m.component + "]"
		if m.useColor {
			header = stepperHeaderStyle.Render(header)
		}
		lines = append(lines, fmt.Sprintf("%s %s", header, m.title))
	}
	spinFrame := strings.TrimSpace(m.spin.View())
	if spinFrame == "" {
		spinFrame = "-"
	}
	for _, step := range m.steps {
		switch step.status {
		case statusActive:
			marker := "[" + spinFrame + "]"
			label := step.label
			if m.useColor {
				marker = stepperHeaderStyle.Render(marker)
				label = stepperActiveStyle.Render(label)
			}
			lines = append(lines, fmt.Sprintf("  %s %s", marker, label))
		case statusDone:
			marker := "[x]"
			label := step.label
			if m.useColor {
				marker = stepperDoneStyle.Render(marker)
				label = stepperDoneStyle.Render(label)
			}
			lines = append(lines, fmt.Sprintf("  %s %s", marker, label))
		default:
			lines = append(lines, fmt.Sprintf("  [ ] %s", step.label))
		}
	}
	if m.summary != "" {
		marker := "[x]"
		label := m.summary
		if m.useColor {
			marker = stepperDoneStyle.Render(marker)
			label = stepperDoneStyle.Render(label)
		}
		lines = append(lines, fmt.Sprintf("  %s %s", marker, label))
	}
	return strings.Join(lines, "\n")
}

func (m *stepperModel) markActiveDone() {
	for i := range m.steps {
		if m.steps[i].status == statusActive {
			m.steps[i].label = stripTransientHint(m.steps[i].label)
			m.steps[i].status = statusDone
		}
	}
}

func (m *stepperModel) activeStepIndex() int {
	for i := len(m.steps) - 1; i >= 0; i-- {
		if m.steps[i].status == statusActive {
			return i
		}
	}
	return -1
}

func stripTransientHint(label string) string {
	trimmed := strings.TrimSpace(label)
	for _, suffix := range []string{
		" (press c to cancel current source)",
		" (press c to cancel)",
		" | m:+1m M:-1m s:+15s S:-15s c:stop now | longer collection improves report quality",
	} {
		if strings.HasSuffix(trimmed, suffix) {
			return strings.TrimSpace(strings.TrimSuffix(trimmed, suffix))
		}
	}
	return label
}

type Stepper struct {
	component string
	enabled   bool
	useColor  bool
	out       io.Writer

	mu      sync.Mutex
	program *tea.Program
	doneCh  chan struct{}
	closed  bool
}

func New(component string, enabled bool) *Stepper {
	return newStepper(component, enabled, os.Stdout)
}

func newStepper(component string, enabled bool, out io.Writer) *Stepper {
	return &Stepper{
		component: component,
		enabled:   enabled,
		useColor:  term.IsTerminal(int(os.Stdout.Fd())),
		out:       out,
	}
}

func InteractiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func (s *Stepper) Begin(title string) {
	if !s.enabled {
		header := colorize(s.useColor, ansiCyan, fmt.Sprintf("[%s]", s.component))
		message := colorize(s.useColor, ansiCyan, title)
		fmt.Fprintf(s.out, "%s %s\n", header, message)
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if s.program == nil {
		model := newStepperModel(s.component, s.useColor)
		s.program = tea.NewProgram(model, tea.WithOutput(s.out), tea.WithInput(nil), tea.WithAltScreen())
		s.doneCh = make(chan struct{})
		go func(program *tea.Program, doneCh chan struct{}) {
			_, _ = program.Run()
			close(doneCh)
		}(s.program, s.doneCh)
	}
	program := s.program
	s.mu.Unlock()
	program.Send(beginMsg{title: title})
}

func (s *Stepper) Step(label string) {
	if !s.enabled {
		header := colorize(s.useColor, ansiCyan, fmt.Sprintf("[%s]", s.component))
		message := colorize(s.useColor, ansiYellow, label)
		fmt.Fprintf(s.out, "%s %s\n", header, message)
		return
	}
	s.mu.Lock()
	program := s.program
	closed := s.closed
	s.mu.Unlock()
	if closed || program == nil {
		return
	}
	program.Send(stepMsg{label: label})
}

func (s *Stepper) UpdateActive(label string) {
	if !s.enabled {
		return
	}
	s.mu.Lock()
	program := s.program
	closed := s.closed
	s.mu.Unlock()
	if closed || program == nil {
		return
	}
	program.Send(updateActiveMsg{label: label})
}

func (s *Stepper) Done(summary string) {
	if !s.enabled {
		if strings.TrimSpace(summary) != "" {
			header := colorize(s.useColor, ansiGreen, fmt.Sprintf("[%s]", s.component))
			message := colorize(s.useColor, ansiGreen, summary)
			fmt.Fprintf(s.out, "%s %s\n", header, message)
		}
		return
	}
	s.mu.Lock()
	program := s.program
	doneCh := s.doneCh
	if s.closed || program == nil {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	program.Send(doneMsg{summary: summary})
	<-doneCh
	if strings.TrimSpace(summary) != "" {
		header := colorize(s.useColor, ansiGreen, fmt.Sprintf("[%s]", s.component))
		message := colorize(s.useColor, ansiGreen, summary)
		fmt.Fprintf(s.out, "%s %s\n", header, message)
	}
}

func colorize(enabled bool, colorCode, text string) string {
	if !enabled || strings.TrimSpace(text) == "" {
		return text
	}
	return colorCode + text + ansiReset
}
