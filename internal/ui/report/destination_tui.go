package report

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/inferLean/inferlean-main/cli/internal/ui/chrome"
)

const (
	defaultDestinationListWidth  = 58
	defaultDestinationListHeight = 9
)

type destinationOption struct {
	title       string
	description string
	value       reportDestination
}

type destinationItem struct {
	option destinationOption
}

func (i destinationItem) Title() string {
	return i.option.title
}

func (i destinationItem) Description() string {
	return i.option.description
}

func (i destinationItem) FilterValue() string {
	return strings.Join([]string{i.option.title, i.option.description}, " ")
}

type destinationKeyMap struct {
	up     key.Binding
	down   key.Binding
	choose key.Binding
	quit   key.Binding
}

func (k destinationKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.up, k.down, k.choose, k.quit}
}

func (k destinationKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.up, k.down, k.choose, k.quit}}
}

type destinationModel struct {
	list      list.Model
	help      help.Model
	keys      destinationKeyMap
	width     int
	selected  reportDestination
	cancelled bool
}

func chooseDestinationWithTUI() (reportDestination, error) {
	output, err := tea.NewProgram(newDestinationModel(), tea.WithAltScreen()).Run()
	if err != nil {
		return destinationTerminal, fmt.Errorf("run report destination selector: %w", err)
	}
	final, ok := output.(destinationModel)
	if !ok {
		return destinationTerminal, fmt.Errorf("unexpected report destination selector state")
	}
	if final.cancelled {
		return destinationTerminal, nil
	}
	if final.selected == "" {
		return destinationBrowser, nil
	}
	return final.selected, nil
}

func newDestinationModel() destinationModel {
	keys := destinationKeyMap{
		up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "move up")),
		down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "move down")),
		choose: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		quit:   key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "cancel")),
	}
	l := buildDestinationList(defaultDestinationListWidth, defaultDestinationListHeight)
	return destinationModel{
		list:     l,
		help:     help.New(),
		keys:     keys,
		width:    defaultDestinationListWidth,
		selected: destinationBrowser,
	}
}

func buildDestinationList(width, height int) list.Model {
	items := []list.Item{
		destinationItem{
			option: destinationOption{
				title:       "browser",
				description: "Open app.inferlean.com for this run (default).",
				value:       destinationBrowser,
			},
		},
		destinationItem{
			option: destinationOption{
				title:       "terminal",
				description: "Show the current report in the CLI viewer.",
				value:       destinationTerminal,
			},
		},
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	l := list.New(items, delegate, width, height)
	l.Title = "Show report in browser or terminal?"
	l.SetShowStatusBar(true)
	l.SetShowPagination(true)
	l.SetShowFilter(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Select(0)
	return l
}

func (m destinationModel) Init() tea.Cmd {
	return nil
}

func (m destinationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width - 4
		if m.width < 24 {
			m.width = typed.Width
		}
		if m.width < 10 {
			m.width = 10
		}
		height := typed.Height - 8
		if height < defaultDestinationListHeight {
			height = defaultDestinationListHeight
		}
		if height > 14 {
			height = 14
		}
		m.list.SetSize(m.width, height)
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(typed)
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m destinationModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.quit) {
		m.cancelled = true
		return m, tea.Quit
	}
	if !key.Matches(msg, m.keys.choose) {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	selected, ok := m.list.SelectedItem().(destinationItem)
	if ok {
		m.selected = selected.option.value
	}
	return m, tea.Quit
}

func (m destinationModel) View() string {
	selectedDescription := destinationSelectedDescription(m.list.SelectedItem())
	lines := []string{
		chrome.Render(chrome.UseColor()),
		"",
		"[report] destination",
	}
	if selectedDescription != "" {
		lines = append(lines, selectedDescription)
	}
	lines = append(lines, "", m.list.View(), "", m.help.View(m.keys))
	return strings.Join(lines, "\n")
}

func destinationSelectedDescription(item list.Item) string {
	selected, ok := item.(destinationItem)
	if !ok || strings.TrimSpace(selected.option.description) == "" {
		return ""
	}
	return "Details: " + selected.option.description
}
