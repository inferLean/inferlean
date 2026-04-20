package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
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

type Stepper struct {
	component string
	enabled   bool
	useColor  bool
	out       io.Writer
	frames    []string
	interval  time.Duration

	mu       sync.Mutex
	title    string
	steps    []stepState
	summary  string
	frame    int
	rendered int
	ticking  bool
	stopTick chan struct{}
	tickDone chan struct{}
	isClosed bool
}

func New(component string, enabled bool) *Stepper {
	return newStepper(component, enabled, os.Stdout)
}

func newStepper(component string, enabled bool, out io.Writer) *Stepper {
	ascii := spinner.Spinner{
		Frames: []string{"-", "\\", "|", "/"},
		FPS:    120 * time.Millisecond,
	}
	return &Stepper{
		component: component,
		enabled:   enabled,
		useColor:  term.IsTerminal(int(os.Stdout.Fd())),
		out:       out,
		frames:    ascii.Frames,
		interval:  ascii.FPS,
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
	defer s.mu.Unlock()
	if s.isClosed {
		return
	}
	s.title = title
	s.summary = ""
	s.steps = nil
	s.frame = 0
	s.renderLocked()
	s.startTickerLocked()
}

func (s *Stepper) Step(label string) {
	if !s.enabled {
		header := colorize(s.useColor, ansiCyan, fmt.Sprintf("[%s]", s.component))
		message := colorize(s.useColor, ansiYellow, label)
		fmt.Fprintf(s.out, "%s %s\n", header, message)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isClosed {
		return
	}
	s.markActiveDoneLocked()
	s.steps = append(s.steps, stepState{label: label, status: statusActive})
	s.renderLocked()
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
	if s.isClosed {
		s.mu.Unlock()
		return
	}
	s.markActiveDoneLocked()
	s.summary = strings.TrimSpace(summary)
	s.renderLocked()
	s.stopTickerLocked()
	s.isClosed = true
	s.mu.Unlock()
}

func (s *Stepper) markActiveDoneLocked() {
	for i := range s.steps {
		if s.steps[i].status == statusActive {
			s.steps[i].status = statusDone
		}
	}
}

func (s *Stepper) startTickerLocked() {
	if s.ticking {
		return
	}
	s.stopTick = make(chan struct{})
	s.tickDone = make(chan struct{})
	s.ticking = true
	interval := s.interval
	if interval <= 0 {
		interval = 120 * time.Millisecond
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(s.tickDone)
		for {
			select {
			case <-ticker.C:
				s.tick()
			case <-s.stopTick:
				return
			}
		}
	}()
}

func (s *Stepper) stopTickerLocked() {
	if !s.ticking {
		return
	}
	close(s.stopTick)
	done := s.tickDone
	s.ticking = false
	s.stopTick = nil
	s.tickDone = nil
	s.mu.Unlock()
	<-done
	s.mu.Lock()
}

func (s *Stepper) tick() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isClosed || !s.ticking {
		return
	}
	if len(s.frames) > 0 {
		s.frame = (s.frame + 1) % len(s.frames)
	}
	s.renderLocked()
}

func (s *Stepper) renderLocked() {
	s.clearLocked()
	lines := s.linesLocked()
	for _, line := range lines {
		fmt.Fprintln(s.out, line)
	}
	s.rendered = len(lines)
}

func (s *Stepper) linesLocked() []string {
	lines := make([]string, 0, len(s.steps)+2)
	if strings.TrimSpace(s.title) != "" {
		lines = append(lines, fmt.Sprintf("[%s] %s", s.component, s.title))
	}
	frame := "-"
	if len(s.frames) > 0 {
		frame = s.frames[s.frame%len(s.frames)]
	}
	for _, step := range s.steps {
		switch step.status {
		case statusActive:
			marker := colorize(s.useColor, ansiCyan, "["+frame+"]")
			label := colorize(s.useColor, ansiYellow, step.label)
			lines = append(lines, fmt.Sprintf("  %s %s", marker, label))
		case statusDone:
			marker := colorize(s.useColor, ansiGreen, "[x]")
			label := colorize(s.useColor, ansiGreen, step.label)
			lines = append(lines, fmt.Sprintf("  %s %s", marker, label))
		default:
			lines = append(lines, fmt.Sprintf("  [ ] %s", step.label))
		}
	}
	if s.summary != "" {
		marker := colorize(s.useColor, ansiGreen, "[x]")
		label := colorize(s.useColor, ansiGreen, s.summary)
		lines = append(lines, fmt.Sprintf("  %s %s", marker, label))
	}
	return lines
}

func colorize(enabled bool, colorCode, text string) string {
	if !enabled || strings.TrimSpace(text) == "" {
		return text
	}
	return colorCode + text + ansiReset
}

func (s *Stepper) clearLocked() {
	if s.rendered == 0 {
		return
	}
	for i := 0; i < s.rendered; i++ {
		fmt.Fprint(s.out, "\x1b[1A\r\x1b[2K")
	}
}
