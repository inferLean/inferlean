package report

import (
	"fmt"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/terminal"
	"github.com/inferLean/inferlean-main/cli/internal/ui/report"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

type Presenter struct {
	view report.View
}

type Options struct {
	BackendURL     string
	Payload        map[string]any
	Artifact       *contracts.RunArtifact
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
			Artifact:       opts.Artifact,
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
		if url, ok := report.ReportURL(opts.BackendURL, opts.InstallationID, runID); ok {
			fmt.Printf("browser_url: %s\n", url)
		}
	}
	fmt.Printf("re-upload: inferlean upload --run-id %s\n", runID)
}

func shouldEmitBrowserURL(nonInteractive bool) bool {
	return nonInteractive || !terminal.InteractiveTTY()
}
