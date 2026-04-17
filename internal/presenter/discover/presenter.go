package discover

import (
	"context"

	"github.com/inferLean/inferlean-main/cli/internal/ui/discovery"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

type Presenter struct {
	service vllmdiscovery.Service
	view    discovery.View
}

func NewPresenter(service vllmdiscovery.Service, view discovery.View) Presenter {
	return Presenter{service: service, view: view}
}

func (p Presenter) Run(ctx context.Context, opts vllmdiscovery.DiscoverOptions) (vllmdiscovery.Candidate, []vllmdiscovery.Candidate, error) {
	p.view.ShowStart()
	candidates, err := p.service.Discover(ctx, opts)
	if err != nil {
		return vllmdiscovery.Candidate{}, nil, err
	}
	p.view.ShowCandidates(candidates)
	selected, err := p.service.Select(candidates, opts.NoInteractive)
	if err != nil {
		return vllmdiscovery.Candidate{}, candidates, err
	}
	p.view.ShowSelected(selected)
	return selected, candidates, nil
}
