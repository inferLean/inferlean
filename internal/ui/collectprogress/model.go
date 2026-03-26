package collectprogress

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/inferLean/inferlean/internal/collector"
)

type resultMsg struct {
	result collector.Result
	err    error
}

type updateMsg struct {
	update collector.StepUpdate
}

type model struct {
	spinner    spinner.Model
	stopwatch  stopwatch.Model
	statuses   map[collector.Step]string
	current    collector.Step
	collectFor time.Duration
	result     collector.Result
	err        error
	messages   <-chan tea.Msg
}

var orderedSteps = []collector.Step{
	collector.StepConfig,
	collector.StepTools,
	collector.StepExporters,
	collector.StepHealthy,
	collector.StepCollect,
	collector.StepFallbacks,
	collector.StepValidate,
	collector.StepPersist,
}

func Run(ctx context.Context, task func(func(collector.StepUpdate)) (collector.Result, error)) (collector.Result, error) {
	msgs := make(chan tea.Msg, 16)
	go func() {
		result, err := task(func(update collector.StepUpdate) {
			msgs <- updateMsg{update: update}
		})
		msgs <- resultMsg{result: result, err: err}
		close(msgs)
	}()

	spin := spinner.New()
	spin.Spinner = spinner.Line
	watch := stopwatch.NewWithInterval(time.Second)

	initial := &model{
		spinner:   spin,
		stopwatch: watch,
		statuses:  map[collector.Step]string{},
		messages:  msgs,
	}

	program := tea.NewProgram(initial)
	finished, err := program.Run()
	if err != nil {
		return collector.Result{}, err
	}

	done := finished.(*model)
	if done.err != nil {
		return done.result, done.err
	}
	return done.result, nil
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForMsg(m.messages))
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case stopwatch.TickMsg:
		var cmd tea.Cmd
		m.stopwatch, cmd = m.stopwatch.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		if typed.String() == "ctrl+c" || typed.String() == "q" {
			m.err = context.Canceled
			return m, tea.Quit
		}
	case updateMsg:
		var cmds []tea.Cmd
		if typed.update.Step == collector.StepCollect {
			m.collectFor = typed.update.CollectFor
			if !m.stopwatch.Running() {
				cmds = append(cmds, m.stopwatch.Start())
			}
		} else if m.stopwatch.Running() {
			cmds = append(cmds, m.stopwatch.Stop())
		}
		m.current = typed.update.Step
		for _, step := range orderedSteps {
			if step == typed.update.Step {
				m.statuses[step] = typed.update.Message
				break
			}
			if _, ok := m.statuses[step]; !ok {
				m.statuses[step] = stepLabel(step)
			}
		}
		cmds = append(cmds, waitForMsg(m.messages))
		return m, tea.Batch(cmds...)
	case resultMsg:
		m.result = typed.result
		m.err = typed.err
		return m, tea.Quit
	}

	return m, nil
}

func (m *model) View() string {
	check := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓")
	pending := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("•")

	out := "InferLean collection\n\n"
	for _, step := range orderedSteps {
		prefix := pending
		switch {
		case m.current == step:
			prefix = m.spinner.View()
		case m.current != "" && stepBefore(step, m.current):
			prefix = check
		}

		label := stepLabel(step)
		if detail, ok := m.statuses[step]; ok && detail != "" {
			label = detail
		}
		if step == collector.StepCollect && m.current == collector.StepCollect {
			label = fmt.Sprintf("%s (%s / %s)", label, truncateSeconds(m.stopwatch.Elapsed()), truncateSeconds(m.collectFor))
		}
		out += fmt.Sprintf("%s %s\n", prefix, label)
	}

	return out
}

func waitForMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return resultMsg{}
		}
		return msg
	}
}

func stepBefore(left, right collector.Step) bool {
	for idx, step := range orderedSteps {
		if step == right {
			for seen := 0; seen < idx; seen++ {
				if orderedSteps[seen] == left {
					return true
				}
			}
		}
	}
	return false
}

func stepLabel(step collector.Step) string {
	switch step {
	case collector.StepConfig:
		return "Loading local installation state"
	case collector.StepTools:
		return "Resolving bundled collection tools"
	case collector.StepExporters:
		return "Starting local exporters"
	case collector.StepHealthy:
		return "Waiting for scrape targets to become healthy"
	case collector.StepCollect:
		return "Collecting local evidence"
	case collector.StepFallbacks:
		return "Capturing fallback and local process evidence"
	case collector.StepValidate:
		return "Validating the run artifact"
	case collector.StepPersist:
		return "Persisting artifact and sidecars"
	default:
		return string(step)
	}
}

func truncateSeconds(value time.Duration) time.Duration {
	if value <= 0 {
		return 0
	}
	return value.Truncate(time.Second)
}
