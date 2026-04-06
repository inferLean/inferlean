package runbrowser

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/inferLean/inferlean/pkg/contracts"
)

type Loader func(string) (contracts.FinalReport, error)

type mode int

const (
	modeList mode = iota
	modeLoading
)

type loadResultMsg struct {
	report contracts.FinalReport
	err    error
}

type runItem struct {
	run contracts.RunSummary
}

func (i runItem) FilterValue() string {
	return strings.Join([]string{i.run.RunID, i.run.InstallationID, i.run.CollectorVersion}, " ")
}

func (i runItem) Title() string {
	return fmt.Sprintf("%s  %s", i.run.RunID, i.run.ReceivedAt.Local().Format("2006-01-02 15:04"))
}

func (i runItem) Description() string {
	return fmt.Sprintf("installation %s  collector %s", i.run.InstallationID, i.run.CollectorVersion)
}

type keyMap struct {
	open key.Binding
	quit key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.open, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.open, k.quit}}
}

type model struct {
	mode        mode
	width       int
	height      int
	list        list.Model
	help        help.Model
	spinner     spinner.Model
	keys        keyMap
	loader      Loader
	selectedRun string
	loadError   string
	report      *contracts.FinalReport
}

func Browse(runs []contracts.RunSummary, loader Loader) (*contracts.FinalReport, error) {
	items := make([]list.Item, 0, len(runs))
	for _, run := range runs {
		items = append(items, runItem{run: run})
	}

	keys := keyMap{
		open: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open report")),
		quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "InferLean runs"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowPagination(true)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Padding(0, 0, 1, 0)

	spin := spinner.New()
	spin.Spinner = spinner.Line

	program := tea.NewProgram(&model{
		mode:    modeList,
		list:    l,
		help:    help.New(),
		spinner: spin,
		keys:    keys,
		loader:  loader,
	})
	finished, err := program.Run()
	if err != nil {
		return nil, err
	}

	done := finished.(*model)
	return done.report, nil
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.list.SetSize(typed.Width, max(1, typed.Height-4))
		return m, nil
	case spinner.TickMsg:
		if m.mode != modeLoading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case loadResultMsg:
		if typed.err != nil {
			m.mode = modeList
			m.loadError = typed.err.Error()
			m.selectedRun = ""
			return m, nil
		}
		m.report = &typed.report
		return m, tea.Quit
	case tea.KeyMsg:
		switch typed.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.mode != modeList {
				return m, nil
			}
			selected, ok := m.list.SelectedItem().(runItem)
			if !ok {
				return m, nil
			}
			m.mode = modeLoading
			m.selectedRun = selected.run.RunID
			m.loadError = ""
			return m, tea.Batch(m.spinner.Tick, loadReportCmd(selected.run.RunID, m.loader))
		}
	}

	if m.mode == modeList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) View() string {
	if m.mode == modeLoading {
		return fmt.Sprintf(
			"InferLean runs\n\n%s Loading canonical report for %s\n\n%s",
			m.spinner.View(),
			m.selectedRun,
			m.help.View(m.keys),
		)
	}

	body := m.list.View()
	if strings.TrimSpace(m.loadError) != "" {
		errorLine := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Load failed: " + m.loadError)
		body += "\n\n" + errorLine
	}
	return body + "\n" + m.help.View(m.keys)
}

func loadReportCmd(runID string, loader Loader) tea.Cmd {
	return func() tea.Msg {
		report, err := loader(runID)
		return loadResultMsg{report: report, err: err}
	}
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}
