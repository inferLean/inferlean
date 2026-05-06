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

func NewPresenter(service vllmdiscovery.Service, view discovery.View) Presenter {
	return Presenter{service: service, view: view}
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
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	cancelCurrent, interrupt, stopListening := startCancelCurrentListener(opts.NonInteractive)
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
	candidates, err := p.service.Discover(runCtx, discoverOpts)
	stop()
	if interrupted.Load() {
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
