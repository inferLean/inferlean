package report

import (
	"fmt"
	"os"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/ui/report"
	"golang.org/x/term"
)

type Presenter struct {
	view report.View
}

type Options struct {
	BackendURL     string
	Payload        map[string]any
	RunID          string
	InstallationID string
	NonInteractive bool
}

func NewPresenter(view report.View) Presenter {
	return Presenter{view: view}
}

func (p Presenter) Run(opts Options) {
	if len(opts.Payload) > 0 {
		p.view.Render(opts.Payload, report.RenderOptions{
			BackendURL:     opts.BackendURL,
			RunID:          opts.RunID,
			InstallationID: opts.InstallationID,
			NonInteractive: opts.NonInteractive,
		})
	}
	p.showRunAccess(opts)
}

func (p Presenter) showRunAccess(opts Options) {
	runID := strings.TrimSpace(opts.RunID)
	if runID == "" {
		return
	}
	fmt.Printf("run_id: %s\n", runID)
	if shouldEmitBrowserURL(opts.NonInteractive) {
		if url, ok := browserReportURL(opts.BackendURL, opts.InstallationID, runID); ok {
			fmt.Printf("browser_url: %s\n", url)
		}
	}
	fmt.Printf("view again: inferlean upload --run-id %s\n", runID)
}

func browserReportURL(backendURL, installationID, runID string) (string, bool) {
	trimmedInstallationID := strings.TrimSpace(installationID)
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedInstallationID == "" || trimmedRunID == "" {
		return "", false
	}
	return fmt.Sprintf("%s/%s/%s", backendURL, trimmedInstallationID, trimmedRunID), true
}

func shouldEmitBrowserURL(nonInteractive bool) bool {
	return nonInteractive || !interactiveTTY()
}

func interactiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
