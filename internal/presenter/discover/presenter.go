package discover

import (
	"context"
	"errors"
	"fmt"

	"github.com/inferLean/inferlean-main/cli/internal/interrupt"
	"github.com/inferLean/inferlean-main/cli/internal/ui/discovery"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery"
)

type Presenter struct {
	service    vllmdiscovery.Service
	view       discovery.View
	interrupts interrupt.Publisher
}

type Options struct {
	PID               int32
	ContainerName     string
	PodName           string
	Namespace         string
	ExcludeProcesses  bool
	ExcludeDocker     bool
	ExcludeKubernetes bool
	NonInteractive    bool
}

func NewPresenter(service vllmdiscovery.Service, view discovery.View, interrupts interrupt.Publisher) Presenter {
	return Presenter{service: service, view: view, interrupts: interrupts}
}

func (p Presenter) Run(ctx context.Context, opts Options) (vllmdiscovery.Candidate, []vllmdiscovery.Candidate, error) {
	p.view.SetNonInteractive(opts.NonInteractive)
	progressDone := false
	defer func() {
		if !progressDone {
			p.view.Abort()
		}
	}()
	p.view.ShowStart()
	cancelCurrent, stopListening := startCancelCurrentListener(opts.NonInteractive, p.interrupts)
	defer stopListening()
	discoverOpts := vllmdiscovery.DiscoverOptions{
		PID:               opts.PID,
		ContainerName:     opts.ContainerName,
		PodName:           opts.PodName,
		Namespace:         opts.Namespace,
		ExcludeProcesses:  opts.ExcludeProcesses,
		ExcludeDocker:     opts.ExcludeDocker,
		ExcludeKubernetes: opts.ExcludeKubernetes,
		CancelCurrent:     cancelCurrent,
		OnSourceStart:     p.view.ShowSourceStart,
		OnSourceCancelled: p.view.ShowSourceCancelled,
	}
	candidates, err := p.service.Discover(ctx, discoverOpts)
	stopListening()
	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		return vllmdiscovery.Candidate{}, nil, fmt.Errorf("discovery interrupted")
	}
	if err != nil {
		return vllmdiscovery.Candidate{}, nil, err
	}
	p.view.ShowCandidates(candidates)
	progressDone = true
	selected, err := p.view.Select(candidates, opts.NonInteractive)
	if err != nil {
		return vllmdiscovery.Candidate{}, candidates, err
	}
	p.view.ShowSelected(selected)
	return selected, candidates, nil
}
