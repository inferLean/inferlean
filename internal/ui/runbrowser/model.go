package runbrowser

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/inferLean/inferlean/pkg/contracts"
)

type Loader func(string) (contracts.RunDetailResponse, error)

type mode int

const (
	modeList mode = iota
	modeLoading
	modeDetail
)

type detailMsg struct {
	detail contracts.RunDetailResponse
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
	return fmt.Sprintf("installation %s  schema %s  collector %s", i.run.InstallationID, i.run.SchemaVersion, i.run.CollectorVersion)
}

type keyMap struct {
	open key.Binding
	back key.Binding
	quit key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.open, k.back, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.open, k.back, k.quit}}
}

type model struct {
	mode        mode
	width       int
	height      int
	list        list.Model
	viewport    viewport.Model
	help        help.Model
	spinner     spinner.Model
	keys        keyMap
	loader      Loader
	detailTitle string
	detailBody  string
	err         error
}

func Browse(runs []contracts.RunSummary, loader Loader) error {
	items := make([]list.Item, 0, len(runs))
	for _, run := range runs {
		items = append(items, runItem{run: run})
	}

	keys := keyMap{
		open: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open run")),
		back: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
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
		mode:     modeList,
		list:     l,
		viewport: viewport.New(0, 0),
		help:     help.New(),
		spinner:  spin,
		keys:     keys,
		loader:   loader,
	})
	finished, err := program.Run()
	if err != nil {
		return err
	}

	done := finished.(*model)
	return done.err
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.list.SetSize(typed.Width, max(1, typed.Height-3))
		m.viewport.Width = typed.Width
		m.viewport.Height = max(1, typed.Height-4)
	case spinner.TickMsg:
		if m.mode != modeLoading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case detailMsg:
		if typed.err != nil {
			m.err = typed.err
			return m, tea.Quit
		}

		tree, err := RenderArtifactTree(typed.detail.Artifact)
		if err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.mode = modeDetail
		m.detailTitle = fmt.Sprintf("%s  %s", typed.detail.RunID, typed.detail.ReceivedAt.Local().Format("2006-01-02 15:04:05"))
		m.detailBody = tree
		m.viewport.SetContent(tree)
		return m, nil
	case tea.KeyMsg:
		switch typed.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.mode == modeDetail {
				m.mode = modeList
				return m, nil
			}
		case "enter":
			if m.mode == modeList {
				selected, ok := m.list.SelectedItem().(runItem)
				if !ok {
					return m, nil
				}
				m.mode = modeLoading
				m.detailTitle = selected.run.RunID
				return m, tea.Batch(m.spinner.Tick, loadDetailCmd(selected.run.RunID, m.loader))
			}
		}
	}

	switch m.mode {
	case modeList:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case modeDetail:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m *model) View() string {
	switch m.mode {
	case modeLoading:
		return fmt.Sprintf("InferLean runs\n\n%s Loading artifact for %s\n\n%s", m.spinner.View(), m.detailTitle, m.help.View(m.keys))
	case modeDetail:
		title := lipgloss.NewStyle().Bold(true).Render(m.detailTitle)
		return title + "\n\n" + m.viewport.View() + "\n\n" + m.help.View(m.keys)
	default:
		return m.list.View() + "\n" + m.help.View(m.keys)
	}
}

func loadDetailCmd(runID string, loader Loader) tea.Cmd {
	return func() tea.Msg {
		detail, err := loader(runID)
		return detailMsg{detail: detail, err: err}
	}
}

func RenderArtifactTree(raw json.RawMessage) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("parse run artifact JSON: %w", err)
	}

	var lines []string
	renderNode(&lines, "", true, "artifact", value)
	return strings.Join(lines, "\n"), nil
}

func renderNode(lines *[]string, prefix string, last bool, label string, value any) {
	connector := "|- "
	nextPrefix := prefix + "|  "
	if last {
		connector = "`- "
		nextPrefix = prefix + "   "
	}

	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			*lines = append(*lines, prefix+connector+label+": {}")
			return
		}

		*lines = append(*lines, prefix+connector+label)
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for idx, key := range keys {
			renderNode(lines, nextPrefix, idx == len(keys)-1, key, typed[key])
		}
	case []any:
		if len(typed) == 0 {
			*lines = append(*lines, prefix+connector+label+": []")
			return
		}

		*lines = append(*lines, prefix+connector+fmt.Sprintf("%s [%d]", label, len(typed)))
		for idx, item := range typed {
			renderNode(lines, nextPrefix, idx == len(typed)-1, fmt.Sprintf("[%d]", idx), item)
		}
	default:
		encoded, _ := json.Marshal(typed)
		*lines = append(*lines, prefix+connector+fmt.Sprintf("%s: %s", label, string(encoded)))
	}
}
