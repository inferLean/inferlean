package report

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/inferLean/inferlean-main/cli/internal/ui/chrome"
)

var (
	reportHeaderStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005F87", Dark: "#5FD7FF"}).Bold(true)
	reportChipStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005F87", Dark: "#5FD7FF"}).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "#BFD7EA", Dark: "#3A6478"}).Padding(0, 1)
	reportMutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#8A8A8A"})
	reportWarningStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8A5200", Dark: "#FFD75F"}).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "#D9B36C", Dark: "#8A6A1F"}).Padding(0, 1).Bold(true)
	reportCardStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "#A8C5D6", Dark: "#2B5B6E"}).Padding(0, 1)
	reportFocusedStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "#005F87", Dark: "#5FD7FF"}).Padding(0, 1)
	reportCardTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005F87", Dark: "#5FD7FF"}).Bold(true)
	reportActionBoxStyle = lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "#875F00", Dark: "#FFD75F"}).Padding(0, 1)
	reportActionHead     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#875F00", Dark: "#FFD75F"}).Bold(true)
	reportActionKey      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#005F87", Dark: "#87D7FF"}).Bold(true)
)

func renderReportContent(vm reportViewModel, focus int, expanded map[string]bool, evidenceTab, width int) (string, map[int]int) {
	contentWidth := max(40, width-2)
	cardWidth := max(38, contentWidth-2)
	lines := []string{
		chrome.Render(chrome.UseColor()),
		"",
		reportHeaderStyle.Render("CLI Report Viewer"),
		renderChipRow(vm.headerChips, contentWidth),
		"",
	}
	if vm.validationWarning != "" {
		warning := reportWarningStyle.Width(cardWidth).Render(vm.validationWarning)
		lines = append(lines, warning, "")
	}
	cardLines := map[int]int{}
	currentLine := len(lines)
	for idx, card := range vm.cards {
		cardLines[idx] = currentLine
		box := renderCard(card, idx == focus, expanded[card.id], evidenceTab, cardWidth)
		lines = append(lines, box, "")
		currentLine += strings.Count(box, "\n") + 2
	}
	lines = append(lines, reportMutedStyle.Render("Metadata"), renderChipRow(vm.footerChips, contentWidth))
	return strings.Join(lines, "\n"), cardLines
}

func renderChipRow(chips []string, width int) string {
	rendered := make([]string, 0, len(chips))
	for _, chip := range chips {
		rendered = append(rendered, reportChipStyle.Render(chip))
	}
	return lipgloss.NewStyle().Width(width).Render(lipgloss.JoinHorizontal(lipgloss.Top, rendered...))
}

func renderCard(card reportCardViewModel, focused, expanded bool, evidenceTab, width int) string {
	state := "collapsed"
	if expanded {
		state = "expanded"
	}
	body := []string{
		reportCardTitleStyle.Render(card.title + " [" + state + "]"),
	}
	if strings.TrimSpace(card.summary) != "" {
		body = append(body, reportMutedStyle.Render(card.summary))
	}
	if expanded {
		body = append(body, renderCardSections(card, evidenceTab, width-4))
	}
	style := reportCardStyle.Width(width)
	if focused {
		style = reportFocusedStyle.Width(width)
	}
	return style.Render(strings.Join(body, "\n"))
}

func renderCardSections(card reportCardViewModel, evidenceTab, width int) string {
	if card.id == "evidence" && len(card.tabs) > 0 {
		return renderEvidenceCard(card.tabs, evidenceTab, width)
	}
	parts := make([]string, 0, len(card.sections))
	for _, section := range card.sections {
		parts = append(parts, renderSection(section, width))
	}
	return strings.Join(parts, "\n")
}

func renderEvidenceCard(tabs []reportTabViewModel, evidenceTab, width int) string {
	active := tabs[evidenceTab%len(tabs)]
	tabNames := make([]string, 0, len(tabs))
	for idx, tab := range tabs {
		label := tab.title
		if idx == evidenceTab%len(tabs) {
			label = "[" + label + "]"
		}
		tabNames = append(tabNames, label)
	}
	parts := []string{
		"Tabs: " + strings.Join(tabNames, " "),
		reportMutedStyle.Render(active.summary),
	}
	for _, section := range active.sections {
		parts = append(parts, renderSection(section, width))
	}
	return strings.Join(parts, "\n")
}

func renderSection(section reportSectionViewModel, width int) string {
	if section.title == "Actions" {
		return renderActionSection(section, width)
	}
	parts := make([]string, 0, 3)
	if title := strings.TrimSpace(section.title); title != "" {
		parts = append(parts, reportCardTitleStyle.Render(title))
	}
	if len(section.lines) > 0 {
		parts = append(parts, lipgloss.NewStyle().Width(width).Render(strings.Join(section.lines, "\n")))
	}
	if section.table != nil {
		parts = append(parts, renderTable(*section.table, width))
	}
	return strings.Join(parts, "\n")
}

func renderTable(vm reportTableViewModel, width int) string {
	columns := make([]table.Column, 0, len(vm.headers))
	columnWidth := max(10, width/len(vm.headers)-2)
	for _, header := range vm.headers {
		columns = append(columns, table.Column{Title: header, Width: columnWidth})
	}
	rows := make([]table.Row, 0, len(vm.rows))
	for _, row := range vm.rows {
		rows = append(rows, table.Row(row))
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(max(1, len(vm.rows)+1)),
	)
	styles := table.DefaultStyles()
	styles.Header = styles.Header.Foreground(lipgloss.AdaptiveColor{Light: "#005F87", Dark: "#5FD7FF"}).Bold(true)
	styles.Selected = lipgloss.NewStyle()
	t.SetStyles(styles)
	return t.View()
}

func renderActionSection(section reportSectionViewModel, width int) string {
	parts := []string{reportCardTitleStyle.Render(section.title)}
	blocks := splitActionBlocks(section.lines)
	for _, block := range blocks {
		lines := make([]string, 0, len(block))
		for idx, line := range block {
			trimmed := strings.TrimSpace(line)
			switch {
			case idx == 0:
				lines = append(lines, reportActionHead.Render(trimmed))
			case strings.HasPrefix(trimmed, "Current:"):
				lines = append(lines, reportActionKey.Render("Change: ")+strings.TrimSpace(strings.TrimPrefix(trimmed, "Current:")))
			case strings.HasPrefix(trimmed, "Why:"):
				lines = append(lines, reportActionKey.Render("Why: ")+strings.TrimSpace(strings.TrimPrefix(trimmed, "Why:")))
			case strings.HasPrefix(trimmed, "Risk:"):
				lines = append(lines, reportActionKey.Render("Risk: ")+strings.TrimSpace(strings.TrimPrefix(trimmed, "Risk:")))
			default:
				lines = append(lines, trimmed)
			}
		}
		parts = append(parts, reportActionBoxStyle.Width(width).Render(strings.Join(lines, "\n")))
	}
	return strings.Join(parts, "\n")
}

func splitActionBlocks(lines []string) [][]string {
	blocks := make([][]string, 0)
	var current []string
	for _, line := range lines {
		if isActionHeadline(line) {
			if len(current) > 0 {
				blocks = append(blocks, current)
			}
			current = []string{line}
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		blocks = append(blocks, current)
	}
	return blocks
}

func isActionHeadline(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	dot := strings.Index(trimmed, ". ")
	if dot <= 0 {
		return false
	}
	for _, ch := range trimmed[:dot] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
