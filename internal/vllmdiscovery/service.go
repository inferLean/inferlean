package vllmdiscovery

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/docker"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/pod"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/process"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

type Service struct{}

func NewService() Service {
	return Service{}
}

func (Service) Discover(ctx context.Context, opts DiscoverOptions) ([]Candidate, error) {
	plan, err := buildPlan(opts)
	if err != nil {
		return nil, err
	}
	all := make([]Candidate, 0)
	for _, source := range plan {
		if opts.OnSourceStart != nil {
			opts.OnSourceStart(source)
		}
		items, cancelled, err := discoverSource(ctx, opts, source)
		if cancelled {
			if opts.OnSourceCancelled != nil {
				opts.OnSourceCancelled(source)
			}
			continue
		}
		if err != nil {
			if source == shared.SourceProcesses {
				return nil, err
			}
			continue
		}
		all = append(all, items...)
	}
	for i := range all {
		if all[i].MetricsEndpoint == "" {
			all[i].MetricsEndpoint = "http://127.0.0.1:8000/metrics"
		}
	}
	return dedupe(all), nil
}

func discoverSource(ctx context.Context, opts DiscoverOptions, source string) ([]Candidate, bool, error) {
	sourceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cancelByUser := make(chan struct{}, 1)
	doneWatching := make(chan struct{})
	if opts.CancelCurrent != nil {
		go watchCancelCurrent(opts.CancelCurrent, doneWatching, cancel, cancelByUser)
	}
	items, err := runDiscoverySource(sourceCtx, opts, source)
	close(doneWatching)
	select {
	case <-cancelByUser:
		return nil, true, nil
	default:
	}
	if errors.Is(err, context.Canceled) && ctx.Err() == nil {
		return nil, true, nil
	}
	return items, false, err
}

func watchCancelCurrent(cancelCurrent <-chan struct{}, done <-chan struct{}, cancel context.CancelFunc, cancelled chan<- struct{}) {
	select {
	case <-done:
		return
	case <-cancelCurrent:
		select {
		case cancelled <- struct{}{}:
		default:
		}
		cancel()
	}
}

func runDiscoverySource(ctx context.Context, opts DiscoverOptions, source string) ([]Candidate, error) {
	switch source {
	case shared.SourceProcesses:
		return process.Discover(ctx, opts.PID)
	case shared.SourceDocker:
		return docker.Discover(ctx, opts.ContainerName)
	case shared.SourceKubernetes:
		return pod.Discover(ctx, opts.PodName, opts.Namespace)
	default:
		return nil, fmt.Errorf("unknown discovery source %q", source)
	}
}

func buildPlan(opts DiscoverOptions) ([]string, error) {
	if opts.ExcludeProcesses && opts.PID != 0 {
		return nil, fmt.Errorf("--pid conflicts with --exclude-processes")
	}
	if opts.ExcludeDocker && strings.TrimSpace(opts.ContainerName) != "" {
		return nil, fmt.Errorf("--container conflicts with --exclude-docker")
	}
	if opts.ExcludeKubernetes && strings.TrimSpace(opts.PodName) != "" {
		return nil, fmt.Errorf("--pod conflicts with --exclude-kubernetes")
	}

	if strings.TrimSpace(opts.ContainerName) != "" {
		return []string{shared.SourceDocker}, nil
	}
	if strings.TrimSpace(opts.PodName) != "" {
		return []string{shared.SourceKubernetes}, nil
	}

	plan := make([]string, 0, 3)
	if !opts.ExcludeProcesses {
		plan = append(plan, shared.SourceProcesses)
	}
	if !opts.ExcludeDocker {
		plan = append(plan, shared.SourceDocker)
	}
	if !opts.ExcludeKubernetes {
		plan = append(plan, shared.SourceKubernetes)
	}
	if len(plan) == 0 {
		return nil, fmt.Errorf("all discovery sources are excluded")
	}
	return plan, nil
}

func dedupe(items []Candidate) []Candidate {
	seen := map[string]bool{}
	out := make([]Candidate, 0, len(items))
	for _, item := range items {
		key := fmt.Sprintf("%s|%d|%s|%s", item.Source, item.PID, item.ContainerID, item.PodName)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}
