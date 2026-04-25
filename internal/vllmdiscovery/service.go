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
	indexByKey := map[string]int{}
	out := make([]Candidate, 0, len(items))
	for _, item := range items {
		key := runtimeIdentityKey(item)
		if idx, ok := indexByKey[key]; ok {
			out[idx] = mergeCandidate(out[idx], item)
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, item)
	}
	return out
}

func runtimeIdentityKey(item Candidate) string {
	if item.PID > 0 {
		return fmt.Sprintf("pid:%d", item.PID)
	}
	if containerID := normalizedContainerID(item.ContainerID); containerID != "" {
		return "container:" + containerID
	}
	podName := strings.TrimSpace(item.PodName)
	if podName != "" {
		return "pod:" + strings.TrimSpace(item.Namespace) + "/" + podName
	}
	return fmt.Sprintf(
		"fallback:%s|%s|%s",
		strings.ToLower(strings.TrimSpace(item.Source)),
		strings.TrimSpace(item.Executable),
		strings.TrimSpace(item.RawCommandLine),
	)
}

func mergeCandidate(a, b Candidate) Candidate {
	preferred := a
	secondary := b
	if sourcePriority(b.Source) > sourcePriority(a.Source) {
		preferred = b
		secondary = a
	}

	merged := preferred
	if merged.PID <= 0 {
		merged.PID = secondary.PID
	}
	merged.ContainerID = mergeContainerID(merged.ContainerID, secondary.ContainerID)
	if strings.TrimSpace(merged.PodName) == "" {
		merged.PodName = secondary.PodName
	}
	if strings.TrimSpace(merged.Namespace) == "" {
		merged.Namespace = secondary.Namespace
	}
	if strings.TrimSpace(merged.Executable) == "" {
		merged.Executable = secondary.Executable
	}
	if strings.TrimSpace(merged.RawCommandLine) == "" {
		merged.RawCommandLine = secondary.RawCommandLine
	}
	if strings.TrimSpace(merged.MetricsEndpoint) == "" {
		merged.MetricsEndpoint = secondary.MetricsEndpoint
	}
	if merged.StartedAt.IsZero() {
		merged.StartedAt = secondary.StartedAt
	}
	return merged
}

func sourcePriority(source string) int {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "pod", "kubernetes":
		return 3
	case "docker":
		return 2
	case "process":
		return 1
	default:
		return 0
	}
}

func mergeContainerID(current, incoming string) string {
	current = strings.TrimSpace(current)
	incoming = strings.TrimSpace(incoming)
	if current == "" {
		return incoming
	}
	if incoming == "" {
		return current
	}
	if len(incoming) > len(current) && normalizedContainerID(incoming) == normalizedContainerID(current) {
		return incoming
	}
	return current
}

func normalizedContainerID(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	const prefix = "docker://"
	trimmed = strings.TrimPrefix(trimmed, prefix)
	hexPrefix := make([]byte, 0, len(trimmed))
	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]
		if (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') {
			hexPrefix = append(hexPrefix, ch)
			continue
		}
		break
	}
	if len(hexPrefix) < 12 {
		return ""
	}
	return string(hexPrefix[:12])
}
