package selecttarget

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/inferLean/inferlean/internal/discovery"
)

type errMsg struct {
	err error
}

type keyMap struct {
	confirm key.Binding
	quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.confirm, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.confirm, k.quit}}
}

type item struct {
	group discovery.CandidateGroup
}

func (i item) FilterValue() string {
	return i.group.DisplayModel()
}

func (i item) Title() string {
	label := i.group.DisplayModel()
	if i.group.RuntimeConfig.Port > 0 {
		return fmt.Sprintf("%s • port %d", label, i.group.RuntimeConfig.Port)
	}
	return label
}

func (i item) Description() string {
	description := fmt.Sprintf("%s • %s", i.group.IdentityLabel(), i.group.EntryPoint)
	if !i.group.Target.IsHost() && i.group.PrimaryPID > 0 {
		description += fmt.Sprintf(" • host pid %d", i.group.PrimaryPID)
	}
	if i.group.ProcessCount > 1 {
		description += fmt.Sprintf(" • %d related processes", i.group.ProcessCount)
	}
	if i.group.CommandExcerpt != "" {
		description += " • " + i.group.CommandExcerpt
	}
	return description
}

type model struct {
	list     list.Model
	keys     keyMap
	help     help.Model
	selected *discovery.CandidateGroup
	err      error
}

func Choose(groups []discovery.CandidateGroup) (discovery.CandidateGroup, error) {
	items := make([]list.Item, 0, len(groups))
	for _, group := range groups {
		items = append(items, item{group: group})
	}

	keys := keyMap{
		confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select target")),
		quit:    key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "cancel")),
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Choose the vLLM deployment to inspect"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowPagination(false)
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Padding(0, 0, 1, 0)

	m := &model{
		list: l,
		keys: keys,
		help: help.New(),
	}

	program := tea.NewProgram(m)
	finished, err := program.Run()
	if err != nil {
		return discovery.CandidateGroup{}, err
	}

	done := finished.(*model)
	if done.err != nil {
		return discovery.CandidateGroup{}, done.err
	}
	if done.selected == nil {
		return discovery.CandidateGroup{}, fmt.Errorf("selection cancelled")
	}

	return *done.selected, nil
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(typed.Width, typed.Height-3)
	case tea.KeyMsg:
		switch typed.String() {
		case "enter":
			if selected, ok := m.list.SelectedItem().(item); ok {
				group := selected.group
				m.selected = &group
				return m, tea.Quit
			}
		case "q", "esc", "ctrl+c":
			m.err = fmt.Errorf("selection cancelled")
			return m, tea.Quit
		}
	case errMsg:
		m.err = typed.err
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	return m.list.View() + "\n" + m.help.View(m.keys)
}
