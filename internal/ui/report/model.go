package report

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/inferLean/inferlean-main/cli/internal/ui/chrome"
)

const (
	viewerReset = "\x1b[0m"
	viewerBold  = "\x1b[1m"
	viewerCyan  = "\x1b[36m"
	viewerGreen = "\x1b[32m"
	viewerDim   = "\x1b[2m"
)

type keyMap struct {
	up       key.Binding
	down     key.Binding
	pageUp   key.Binding
	pageDown key.Binding
	top      key.Binding
	bottom   key.Binding
	quit     key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.up, k.down, k.pageUp, k.pageDown, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.up, k.down, k.pageUp, k.pageDown},
		{k.top, k.bottom, k.quit},
	}
}

type viewerModel struct {
	content  string
	summary  string
	viewport viewport.Model
	help     help.Model
	keys     keyMap
}

func newViewerModel(content, summary string) viewerModel {
	keys := keyMap{
		up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "line up")),
		down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "line down")),
		pageUp:   key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup/b", "page up")),
		pageDown: key.NewBinding(key.WithKeys("pgdown", "space", "f"), key.WithHelp("pgdown/f", "page down")),
		top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g/home", "top")),
		bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G/end", "bottom")),
		quit:     key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "close")),
	}
	vp := viewport.New(100, 18)
	vp.SetContent(content)
	h := help.New()
	h.ShowAll = false
	return viewerModel{
		content:  content,
		summary:  summary,
		viewport: vp,
		help:     h,
		keys:     keys,
	}
}

func (m viewerModel) Init() tea.Cmd {
	return nil
}

func (m viewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(typed.Width, typed.Height)
		return m, nil
	case tea.KeyMsg:
		if key.Matches(typed, m.keys.quit) {
			return m, tea.Quit
		}
		if m.updateScroll(typed) {
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *viewerModel) resize(width, height int) {
	m.viewport.Width = max(76, width-4)
	m.viewport.Height = max(10, height-8)
	m.viewport.SetContent(m.content)
}

func (m *viewerModel) updateScroll(msg tea.KeyMsg) bool {
	switch {
	case key.Matches(msg, m.keys.up):
		m.viewport.LineUp(1)
	case key.Matches(msg, m.keys.down):
		m.viewport.LineDown(1)
	case key.Matches(msg, m.keys.pageUp):
		m.viewport.ViewUp()
	case key.Matches(msg, m.keys.pageDown):
		m.viewport.ViewDown()
	case key.Matches(msg, m.keys.top):
		m.viewport.GotoTop()
	case key.Matches(msg, m.keys.bottom):
		m.viewport.GotoBottom()
	default:
		return false
	}
	return true
}

func (m viewerModel) View() string {
	header := viewerStyle(viewerBold+viewerGreen, "[report] Full Report")
	status := viewerStyle(viewerDim+viewerCyan, fmt.Sprintf("Scroll %.0f%%", m.viewport.ScrollPercent()*100))
	summary := viewerStyle(viewerBold+viewerCyan, m.summary)
	return strings.Join([]string{
		chrome.Render(chrome.UseColor()),
		"",
		header,
		summary,
		status,
		"",
		m.viewport.View(),
		"",
		m.help.View(m.keys),
	}, "\n")
}

func viewerStyle(code, text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	return code + text + viewerReset
}
