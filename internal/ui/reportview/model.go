package reportview

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/inferLean/inferlean/pkg/contracts"
)

type viewMode string

const (
	modeBrief viewMode = "brief"
	modeFull  viewMode = "full"
)

type keyMap struct {
	prevTarget key.Binding
	nextTarget key.Binding
	toggleMode key.Binding
	help       key.Binding
	quit       key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.prevTarget, k.nextTarget, k.toggleMode, k.help, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.prevTarget, k.nextTarget, k.toggleMode}, {k.help, k.quit}}
}

type model struct {
	report     contracts.FinalReport
	mode       viewMode
	target     string
	targets    []string
	viewport   viewport.Model
	help       help.Model
	keys       keyMap
	showHelp   bool
	width      int
	height     int
	titleStyle lipgloss.Style
	metaStyle  lipgloss.Style
}

func Run(report contracts.FinalReport) error {
	program := tea.NewProgram(newModel(report))
	_, err := program.Run()
	if err != nil {
		return err
	}
	return nil
}

func newModel(report contracts.FinalReport) *model {
	target := defaultTarget(report)
	m := &model{
		report:   report,
		mode:     defaultMode(report),
		target:   target,
		targets:  []string{"latency", "balanced", "throughput"},
		viewport: viewport.New(0, 0),
		help:     help.New(),
		keys: keyMap{
			prevTarget: key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("left/h", "prev target")),
			nextTarget: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("right/l", "next target")),
			toggleMode: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "brief/full")),
			help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		},
		titleStyle: lipgloss.NewStyle().Bold(true),
		metaStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
	m.viewport.SetContent(Render(report, string(m.mode), m.target))
	return m
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.viewport.Width = typed.Width
		m.viewport.Height = max(1, typed.Height-4)
		return m, nil
	case tea.KeyMsg:
		switch typed.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "m":
			if m.mode == modeBrief {
				m.mode = modeFull
			} else {
				m.mode = modeBrief
			}
			m.refresh()
			return m, nil
		case "left", "h":
			m.shiftTarget(-1)
			return m, nil
		case "right", "l":
			m.shiftTarget(1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	title := m.titleStyle.Render("InferLean report")
	meta := m.metaStyle.Render(strings.Join([]string{
		"run " + firstNonEmpty(m.report.Job.RunID, "unknown"),
		"tier " + firstNonEmpty(m.report.Entitlement.Tier, "unknown"),
		"mode " + string(m.mode),
		"target " + m.target,
	}, "  •  "))

	body := title + "\n" + meta + "\n\n" + m.viewport.View()
	if m.showHelp {
		return body + "\n\n" + m.help.View(m.keys)
	}
	return body + "\n\n" + m.help.ShortHelpView(m.keys.ShortHelp())
}

func (m *model) refresh() {
	m.viewport.SetContent(Render(m.report, string(m.mode), m.target))
	m.viewport.GotoTop()
}

func (m *model) shiftTarget(delta int) {
	index := 0
	for i, target := range m.targets {
		if target == m.target {
			index = i
			break
		}
	}
	index = (index + delta + len(m.targets)) % len(m.targets)
	m.target = m.targets[index]
	m.refresh()
}

func defaultMode(report contracts.FinalReport) viewMode {
	switch strings.TrimSpace(report.UIHints.DefaultMode) {
	case string(modeFull):
		return modeFull
	default:
		return modeBrief
	}
}

func defaultTarget(report contracts.FinalReport) string {
	switch strings.TrimSpace(report.UIHints.DefaultTarget) {
	case "latency", "throughput", "balanced":
		return report.UIHints.DefaultTarget
	default:
		return "balanced"
	}
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}

var _ tea.Model = (*model)(nil)
