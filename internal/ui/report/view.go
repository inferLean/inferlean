package report

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type View struct{}

func NewView() View {
	return View{}
}

func (View) Render(report map[string]any) {
	tty := interactiveTTY()
	content, summary, err := formatReportForDisplay(report, tty)
	if err != nil {
		fmt.Printf("[report] failed to render report: %v\n", err)
		return
	}
	if !tty {
		printReport(content)
		return
	}

	model := newViewerModel(content, summary)
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Printf("[report] interactive viewer failed, showing plain output: %v\n", err)
		printReport(content)
	}
}

func printReport(content string) {
	fmt.Println("[report] parsed report")
	fmt.Println(content)
}

func interactiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
