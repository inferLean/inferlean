package publishprogress

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/inferLean/inferlean/internal/publish"
)

type resultMsg struct {
	result publish.Result
	err    error
}

type updateMsg struct {
	update publish.StepUpdate
}

type model struct {
	spinner  spinner.Model
	statuses map[publish.Step]string
	current  publish.Step
	result   publish.Result
	err      error
	messages <-chan tea.Msg
}

var orderedSteps = []publish.Step{
	publish.StepAuth,
	publish.StepUpload,
	publish.StepWait,
}

func Run(ctx context.Context, task func(func(publish.StepUpdate)) (publish.Result, error)) (publish.Result, error) {
	msgs := make(chan tea.Msg, 16)
	go func() {
		result, err := task(func(update publish.StepUpdate) {
			msgs <- updateMsg{update: update}
		})
		msgs <- resultMsg{result: result, err: err}
		close(msgs)
	}()

	spin := spinner.New()
	spin.Spinner = spinner.Line

	initial := &model{
		spinner:  spin,
		statuses: map[publish.Step]string{},
		messages: msgs,
	}

	program := tea.NewProgram(initial)
	finished, err := program.Run()
	if err != nil {
		return publish.Result{}, err
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

	out := "InferLean publish\n\n"
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

func stepBefore(left, right publish.Step) bool {
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

func stepLabel(step publish.Step) string {
	switch step {
	case publish.StepAuth:
		return "Preparing authenticated backend session"
	case publish.StepUpload:
		return "Uploading the run artifact"
	case publish.StepWait:
		return "Waiting for durable backend acknowledgement"
	default:
		return string(step)
	}
}
