package interactive

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/inferLean/inferlean-main/cli/internal/types"
)

type optionItem struct {
	option questionOption
}

func (i optionItem) Title() string {
	return i.option.title
}

func (i optionItem) Description() string {
	return i.option.description
}

func (i optionItem) FilterValue() string {
	return strings.Join([]string{i.option.title, i.option.value, i.option.description}, " ")
}

type keyMap struct {
	up     key.Binding
	down   key.Binding
	choose key.Binding
	quit   key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.up, k.down, k.choose, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.up, k.down, k.choose, k.quit}}
}

type questionnaireModel struct {
	questions []question
	index     int
	answers   map[questionKey]string
	help      help.Model
	keys      keyMap
	list      list.Model
	cancelled bool
	width     int
}

func resolveWithTUI(seed types.UserIntent, questions []question) (types.UserIntent, error) {
	model := newQuestionnaireModel(questions)
	output, err := tea.NewProgram(model).Run()
	if err != nil {
		return seed, fmt.Errorf("run intent questionnaire: %w", err)
	}
	final, ok := output.(questionnaireModel)
	if !ok {
		return seed, fmt.Errorf("unexpected questionnaire state")
	}
	if final.cancelled {
		return seed, fmt.Errorf("intent questionnaire cancelled")
	}
	intent := seed
	for _, q := range questions {
		answer := final.answers[q.key]
		if strings.TrimSpace(answer) == "" {
			continue
		}
		applyAnswer(&intent, q.key, answer)
	}
	return intent, nil
}

func newQuestionnaireModel(questions []question) questionnaireModel {
	keys := keyMap{
		up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "move up")),
		down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "move down")),
		choose: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		quit:   key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "cancel")),
	}
	l := buildQuestionList(questions[0], 48, 9)
	return questionnaireModel{
		questions: questions,
		index:     0,
		answers:   make(map[questionKey]string, len(questions)),
		help:      help.New(),
		keys:      keys,
		list:      l,
		width:     48,
	}
}

func buildQuestionList(q question, width, height int) list.Model {
	items := make([]list.Item, 0, len(q.options))
	for _, option := range q.options {
		items = append(items, optionItem{option: option})
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	l := list.New(items, delegate, width, height)
	l.Title = q.prompt
	l.SetShowStatusBar(true)
	l.SetShowPagination(true)
	l.SetShowFilter(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Select(boundedIndex(q.defaultIndex, len(q.options)))
	return l
}

func (m questionnaireModel) Init() tea.Cmd {
	return nil
}

func (m questionnaireModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width - 4
		if m.width < 24 {
			m.width = max(10, typed.Width)
		}
		height := typed.Height - 8
		if height < 9 {
			height = 9
		}
		if height > 16 {
			height = 16
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

func (m questionnaireModel) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.quit) {
		m.cancelled = true
		return m, tea.Quit
	}
	if !key.Matches(msg, m.keys.choose) {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	selected, ok := m.list.SelectedItem().(optionItem)
	if !ok {
		return m, nil
	}
	current := m.questions[m.index]
	m.answers[current.key] = selected.option.value
	if m.index >= len(m.questions)-1 {
		return m, tea.Quit
	}
	m.index++
	m.list = buildQuestionList(m.questions[m.index], m.width, 8)
	return m, nil
}

func (m questionnaireModel) View() string {
	header := fmt.Sprintf("[intent] question %d/%d", m.index+1, len(m.questions))
	description := selectedDescription(m.list.SelectedItem())
	helpView := m.help.View(m.keys)
	lines := []string{
		header,
		"Please answer the following questions:",
	}
	if description != "" {
		lines = append(lines, description)
	}
	lines = append(lines, "", m.list.View(), "", helpView)
	return strings.Join(lines, "\n")
}

func selectedDescription(item list.Item) string {
	option, ok := item.(optionItem)
	if !ok || strings.TrimSpace(option.option.description) == "" {
		return ""
	}
	return "Details: " + option.option.description
}
