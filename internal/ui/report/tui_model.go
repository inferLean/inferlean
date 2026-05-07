package report

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type reportKeyMap struct {
	nextCard  key.Binding
	prevCard  key.Binding
	toggle    key.Binding
	nextTab   key.Binding
	prevTab   key.Binding
	nextFocus key.Binding
	prevFocus key.Binding
	open      key.Binding
	help      key.Binding
	quit      key.Binding
}

func (k reportKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.nextCard, k.toggle, k.nextTab, k.open, k.help, k.quit}
}

func (k reportKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.nextCard, k.prevCard, k.nextFocus, k.prevFocus},
		{k.toggle, k.nextTab, k.prevTab},
		{k.open, k.help, k.quit},
	}
}

type reportModel struct {
	vm            reportViewModel
	viewport      viewport.Model
	help          help.Model
	keys          reportKeyMap
	focus         int
	width         int
	height        int
	showFullHelp  bool
	statusMessage string
	expanded      map[string]bool
	evidenceTab   int
	cardLines     map[int]int
}

func newReportModel(vm reportViewModel) reportModel {
	keys := reportKeyMap{
		nextCard:  key.NewBinding(key.WithKeys("down", "j", "tab"), key.WithHelp("j/down/tab", "next card")),
		prevCard:  key.NewBinding(key.WithKeys("up", "k", "shift+tab"), key.WithHelp("k/up/shift+tab", "previous card")),
		toggle:    key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter/space", "expand or collapse")),
		nextTab:   key.NewBinding(key.WithKeys("right", "]"), key.WithHelp("]/right", "next evidence tab")),
		prevTab:   key.NewBinding(key.WithKeys("left", "["), key.WithHelp("[/left", "previous evidence tab")),
		nextFocus: key.NewBinding(key.WithKeys("pagedown"), key.WithHelp("pgdn", "scroll down")),
		prevFocus: key.NewBinding(key.WithKeys("pageup"), key.WithHelp("pgup", "scroll up")),
		open:      key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open browser")),
		help:      key.NewBinding(key.WithKeys("h", "?"), key.WithHelp("h/?", "toggle help")),
		quit:      key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
	}
	expanded := make(map[string]bool, len(vm.cards))
	for _, card := range vm.cards {
		expanded[card.id] = card.defaultExpanded
	}
	m := reportModel{
		vm:       vm,
		help:     help.New(),
		keys:     keys,
		expanded: expanded,
		cardLines: map[int]int{
			0: 0,
		},
		statusMessage: "tab through cards, enter toggles, q exits",
	}
	m.help.ShowAll = false
	return m
}

func (m reportModel) Init() tea.Cmd {
	return nil
}

func (m reportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.viewport = viewport.New(max(20, typed.Width), max(8, typed.Height-4))
		m.rebuildViewport()
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(typed)
	}
	return m, nil
}

func (m reportModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.help):
		m.showFullHelp = !m.showFullHelp
		m.help.ShowAll = m.showFullHelp
		return m, nil
	case key.Matches(msg, m.keys.nextCard):
		if m.focus < len(m.vm.cards)-1 {
			m.focus++
			m.rebuildViewport()
		}
		return m, nil
	case key.Matches(msg, m.keys.prevCard):
		if m.focus > 0 {
			m.focus--
			m.rebuildViewport()
		}
		return m, nil
	case key.Matches(msg, m.keys.toggle):
		card := m.vm.cards[m.focus]
		m.expanded[card.id] = !m.expanded[card.id]
		m.rebuildViewport()
		return m, nil
	case key.Matches(msg, m.keys.nextTab):
		if m.focusedCard().id == "evidence" {
			m.evidenceTab = (m.evidenceTab + 1) % len(m.focusedCard().tabs)
			m.rebuildViewport()
		}
		return m, nil
	case key.Matches(msg, m.keys.prevTab):
		if m.focusedCard().id == "evidence" {
			m.evidenceTab--
			if m.evidenceTab < 0 {
				m.evidenceTab = len(m.focusedCard().tabs) - 1
			}
			m.rebuildViewport()
		}
		return m, nil
	case key.Matches(msg, m.keys.open):
		return m.handleOpenBrowser()
	case key.Matches(msg, m.keys.nextFocus):
		m.viewport.ViewDown()
		return m, nil
	case key.Matches(msg, m.keys.prevFocus):
		m.viewport.ViewUp()
		return m, nil
	default:
		return m, nil
	}
}

func (m reportModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading report viewer..."
	}
	return strings.Join([]string{
		m.viewport.View(),
		m.statusLine(),
		m.help.View(m.keys),
	}, "\n")
}

func (m *reportModel) handleOpenBrowser() (tea.Model, tea.Cmd) {
	if !m.canOpenBrowser() {
		m.statusMessage = "browser view unavailable for this report"
		return m, nil
	}
	if err := openBrowser(m.vm.browserURL); err != nil {
		m.statusMessage = "failed to open browser view"
		return m, nil
	}
	m.statusMessage = "opened browser view"
	return m, nil
}

func (m *reportModel) rebuildViewport() {
	content, cardLines := renderReportContent(m.vm, m.focus, m.expanded, m.evidenceTab, max(40, m.width))
	m.cardLines = cardLines
	m.viewport.SetContent(content)
	m.ensureFocusVisible()
}

func (m *reportModel) ensureFocusVisible() {
	line, ok := m.cardLines[m.focus]
	if !ok {
		return
	}
	if line < m.viewport.YOffset {
		m.viewport.SetYOffset(line)
		return
	}
	bottom := m.viewport.YOffset + m.viewport.Height - 3
	if line > bottom {
		m.viewport.SetYOffset(line - max(0, m.viewport.Height/3))
	}
}

func (m reportModel) statusLine() string {
	card := m.focusedCard()
	if card.id == "evidence" && len(card.tabs) > 0 {
		return fmt.Sprintf("[report] %s | evidence tab: %s | %s", card.title, card.tabs[m.evidenceTab].title, m.statusMessage)
	}
	return fmt.Sprintf("[report] %s | %s", card.title, m.statusMessage)
}

func (m reportModel) focusedCard() reportCardViewModel {
	if len(m.vm.cards) == 0 {
		return reportCardViewModel{}
	}
	return m.vm.cards[m.focus]
}

func (m reportModel) canOpenBrowser() bool {
	return strings.TrimSpace(m.vm.browserURL) != ""
}
