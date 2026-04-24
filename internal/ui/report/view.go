package report

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type View struct{}

type RenderOptions struct {
	RunID          string
	InstallationID string
	NoInteractive  bool
}

func NewView() View {
	return View{}
}

func (View) Render(report map[string]any, opts RenderOptions) {
	tty := interactiveTTY()
	identity := resolveIdentity(report, opts)
	content, summary, err := formatReportForDisplay(report, tty)
	if err != nil {
		fmt.Printf("[report] failed to render report: %v\n", err)
		return
	}

	if opts.NoInteractive || !tty {
		printReport(content)
		return
	}

	if destination := chooseDestination(identity, opts.NoInteractive, tty); destination == destinationBrowser {
		if !isIdentityComplete(identity) {
			fmt.Println("[report] browser view unavailable (missing run_id or installation_id), showing terminal report")
		} else {
			reportURL := inferleanReportURL(identity)
			if err := openBrowser(reportURL); err == nil {
				fmt.Printf("[report] opened in browser: %s\n", reportURL)
				return
			}
			fmt.Println("[report] failed to open browser, showing terminal report")
		}
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

type reportIdentity struct {
	runID          string
	installationID string
}

func resolveIdentity(report map[string]any, opts RenderOptions) reportIdentity {
	identity := reportIdentity{
		runID:          strings.TrimSpace(opts.RunID),
		installationID: strings.TrimSpace(opts.InstallationID),
	}
	job, ok := report["job"].(map[string]any)
	if !ok {
		return identity
	}
	if identity.runID == "" {
		identity.runID = strings.TrimSpace(stringField(job, "run_id"))
	}
	if identity.installationID == "" {
		identity.installationID = strings.TrimSpace(stringField(job, "installation_id"))
	}
	return identity
}

func stringField(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}
