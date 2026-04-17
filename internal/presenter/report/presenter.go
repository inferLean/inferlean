package report

import "github.com/inferLean/inferlean-main/new-cli/internal/ui/report"

type Presenter struct {
	view report.View
}

func NewPresenter(view report.View) Presenter {
	return Presenter{view: view}
}

func (p Presenter) Run(payload map[string]any) {
	p.view.Render(payload)
}
