package discover

import (
	"context"
	"fmt"
	"sync/atomic"

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
	p.view.SetNoInteractive(opts.NoInteractive)
	p.view.ShowStart()
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	cancelCurrent, interrupt, stopListening := startCancelCurrentListener(opts.NoInteractive)
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		stopListening()
	}
	defer stop()
	var interrupted atomic.Bool
	doneInterrupt := make(chan struct{})
	go func() {
		defer close(doneInterrupt)
		select {
		case <-interrupt:
			interrupted.Store(true)
			cancelRun()
		case <-runCtx.Done():
		}
	}()
	defer func() {
		cancelRun()
		<-doneInterrupt
	}()
	opts.CancelCurrent = cancelCurrent
	opts.OnSourceStart = p.view.ShowSourceStart
	opts.OnSourceCancelled = p.view.ShowSourceCancelled
	candidates, err := p.service.Discover(runCtx, opts)
	stop()
	if interrupted.Load() {
		return vllmdiscovery.Candidate{}, nil, fmt.Errorf("discovery interrupted")
	}
	if err != nil {
		return vllmdiscovery.Candidate{}, nil, err
	}
	p.view.ShowCandidates(candidates)
	selected, err := p.view.Select(candidates, opts.NoInteractive)
	if err != nil {
		return vllmdiscovery.Candidate{}, candidates, err
	}
	p.view.ShowSelected(selected)
	return selected, candidates, nil
}
