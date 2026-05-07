package report

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/inferLean/inferlean-main/cli/internal/terminal"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type View struct{}

type RenderOptions struct {
	BackendURL     string
	RunID          string
	InstallationID string
	NonInteractive bool
}

func NewView() View {
	return View{}
}

func (View) Render(report map[string]any, opts RenderOptions) {
	tty := terminal.InteractiveTTY()
	identity := resolveIdentity(report, opts)
	content, _, err := formatReportForDisplay(report, tty && !opts.NonInteractive)
	if err != nil {
		fmt.Printf("[report] failed to render report: %v\n", err)
		return
	}

	if opts.NonInteractive || !tty {
		printReport(content)
		return
	}

	if destination := chooseDestination(opts.NonInteractive, tty); destination == destinationBrowser {
		if !isIdentityComplete(identity) {
			fmt.Println("[report] browser view unavailable (missing run_id or installation_id), showing terminal report")
		} else {
			reportURL, _ := ReportURL(opts.BackendURL, identity.installationID, identity.runID)
			if err := openBrowser(reportURL); err == nil {
				fmt.Printf("[report] opened in browser: %s\n", reportURL)
				return
			}
			fmt.Println("[report] failed to open browser, showing terminal report")
		}
	}

	decoded, err := decodeFinalReport(report)
	if err != nil {
		printReport(content)
		return
	}
	if err := runReportTUI(decoded, identity, opts); err != nil {
		fmt.Printf("[report] interactive viewer unavailable: %v\n", err)
		printReport(content)
	}
}

func printReport(content string) {
	fmt.Println("[report] parsed report")
	fmt.Println(content)
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

func runReportTUI(report contracts.FinalReport, identity reportIdentity, opts RenderOptions) error {
	validationWarning := ""
	if err := report.Validate(); err != nil {
		validationWarning = "Schema validation warning: " + err.Error()
	}
	vm := buildReportViewModel(report, identity, opts.BackendURL, time.Now().UTC(), validationWarning)
	program := tea.NewProgram(newReportModel(vm), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func stringField(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}
