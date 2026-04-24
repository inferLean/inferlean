package discovery

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/inferLean/inferlean-main/cli/internal/ui/chrome"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
	"golang.org/x/term"
)

type candidateItem struct {
	candidate vllmdiscovery.Candidate
}

func (i candidateItem) Title() string {
	return fmt.Sprintf("%s  %s", strings.ToUpper(strings.TrimSpace(i.candidate.Source)), targetLabel(i.candidate))
}

func (i candidateItem) Description() string {
	meta := []string{
		"metrics=" + strings.TrimSpace(i.candidate.MetricsEndpoint),
		"cmd=" + shorten(i.candidate.RawCommandLine),
	}
	return strings.Join(meta, "  ")
}

func (i candidateItem) FilterValue() string {
	return strings.Join([]string{
		i.candidate.Source,
		i.candidate.Executable,
		i.candidate.RawCommandLine,
		i.candidate.ContainerID,
		i.candidate.PodName,
		i.candidate.Namespace,
	}, " ")
}

type selectorModel struct {
	list      list.Model
	selected  *vllmdiscovery.Candidate
	cancelled bool
}

func newSelectorModel(candidates []vllmdiscovery.Candidate) selectorModel {
	items := make([]list.Item, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, candidateItem{candidate: candidate})
	}
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	l := list.New(items, delegate, 48, 9)
	l.Title = "Select vLLM Target"
	l.SetShowStatusBar(true)
	l.SetShowPagination(true)
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	return selectorModel{list: l}
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if window, ok := msg.(tea.WindowSizeMsg); ok {
		m.resize(window)
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			item, ok := m.list.SelectedItem().(candidateItem)
			if ok {
				selected := item.candidate
				m.selected = &selected
			}
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectorModel) View() string {
	return strings.Join([]string{
		chrome.Render(chrome.UseColor()),
		"",
		m.list.View(),
	}, "\n")
}

func (m *selectorModel) resize(window tea.WindowSizeMsg) {
	width := window.Width - 4
	if width < 24 {
		width = max(10, window.Width)
	}
	height := window.Height - 6
	if height < 9 {
		height = 9
	}
	if height > 18 {
		height = 18
	}
	m.list.SetSize(width, height)
}

func selectCandidateTUI(candidates []vllmdiscovery.Candidate) (vllmdiscovery.Candidate, error) {
	model := newSelectorModel(candidates)
	output, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	if err != nil {
		return vllmdiscovery.Candidate{}, fmt.Errorf("run discovery selector: %w", err)
	}
	finalModel, ok := output.(selectorModel)
	if !ok {
		return vllmdiscovery.Candidate{}, fmt.Errorf("unexpected selector state")
	}
	if finalModel.cancelled {
		return vllmdiscovery.Candidate{}, fmt.Errorf("discovery selection cancelled")
	}
	if finalModel.selected == nil {
		return vllmdiscovery.Candidate{}, fmt.Errorf("invalid selection")
	}
	return *finalModel.selected, nil
}

func shouldUseTUI(noInteractive bool, candidates []vllmdiscovery.Candidate) bool {
	if noInteractive || len(candidates) <= 1 {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func targetLabel(item vllmdiscovery.Candidate) string {
	parts := make([]string, 0, 4)
	containerName := ""
	if isDockerSource(item) {
		containerName = dockerContainerName(item)
	}
	if item.PID > 0 && !isDockerSource(item) {
		parts = append(parts, fmt.Sprintf("pid=%d", item.PID))
	}
	if containerName != "" {
		parts = append(parts, "container="+containerName)
	}
	if containerID := strings.TrimSpace(item.ContainerID); containerID != "" && containerName == "" {
		parts = append(parts, "container="+containerID)
	}
	if podName := strings.TrimSpace(item.PodName); podName != "" {
		parts = append(parts, "pod="+podName)
	}
	if namespace := strings.TrimSpace(item.Namespace); namespace != "" {
		parts = append(parts, "ns="+namespace)
	}
	if len(parts) == 0 {
		parts = append(parts, "unknown")
	}
	return strings.Join(parts, " ")
}

func isDockerSource(item vllmdiscovery.Candidate) bool {
	return strings.EqualFold(strings.TrimSpace(item.Source), "docker")
}

func dockerContainerName(item vllmdiscovery.Candidate) string {
	const prefix = "docker-container:"
	executable := strings.TrimSpace(item.Executable)
	if !strings.HasPrefix(executable, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(executable, prefix))
}

func shorten(text string) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= 48 {
		return trimmed
	}
	return trimmed[:45] + "..."
}
