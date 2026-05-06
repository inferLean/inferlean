package report

import "github.com/inferLean/inferlean-main/cli/internal/ui/report"

type Presenter struct {
	view report.View
}

type Options struct {
	BackendURL     string
	Payload        map[string]any
	RunID          string
	InstallationID string
	NoInteractive  bool
}

func NewPresenter(view report.View) Presenter {
	return Presenter{view: view}
}

func (p Presenter) Run(opts Options) {
	p.view.Render(opts.Payload, report.RenderOptions{
		BackendURL:     opts.BackendURL,
		RunID:          opts.RunID,
		InstallationID: opts.InstallationID,
		NoInteractive:  opts.NoInteractive,
	})
}
