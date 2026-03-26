package progress

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/inferLean/inferlean/internal/discovery"
)

type resultMsg struct {
	result discovery.Result
	err    error
}

type updateMsg struct {
	update discovery.StepUpdate
}

type model struct {
	spinner  spinner.Model
	statuses map[discovery.Step]string
	current  discovery.Step
	result   discovery.Result
	err      error
	messages <-chan tea.Msg
}

var orderedSteps = []discovery.Step{
	discovery.StepEnumerate,
	discovery.StepParse,
	discovery.StepResolve,
}

func Run(ctx context.Context, task func(func(discovery.StepUpdate)) (discovery.Result, error)) (discovery.Result, error) {
	msgs := make(chan tea.Msg, 16)
	go func() {
		result, err := task(func(update discovery.StepUpdate) {
			msgs <- updateMsg{update: update}
		})
		msgs <- resultMsg{result: result, err: err}
		close(msgs)
	}()

	spin := spinner.New()
	spin.Spinner = spinner.Line

	initial := &model{
		spinner:  spin,
		statuses: map[discovery.Step]string{},
		messages: msgs,
	}

	program := tea.NewProgram(initial)
	finished, err := program.Run()
	if err != nil {
		return discovery.Result{}, err
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
	case tea.KeyMsg:
		if typed.String() == "ctrl+c" || typed.String() == "q" {
			m.err = context.Canceled
			return m, tea.Quit
		}
	case updateMsg:
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
		return m, waitForMsg(m.messages)
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

	out := "InferLean discovery\n\n"
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

func stepBefore(left, right discovery.Step) bool {
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

func stepLabel(step discovery.Step) string {
	switch step {
	case discovery.StepEnumerate:
		return "Enumerating local processes"
	case discovery.StepParse:
		return "Parsing vLLM runtime configuration"
	case discovery.StepResolve:
		return "Resolving the target deployment"
	default:
		return string(step)
	}
}
