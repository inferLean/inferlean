package vllmdiscovery

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/docker"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/pod"
	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/process"
)

type Service struct{}

func NewService() Service {
	return Service{}
}

func (Service) Discover(ctx context.Context, opts DiscoverOptions) ([]Candidate, error) {
	all := make([]Candidate, 0)
	if strings.TrimSpace(opts.ContainerName) != "" {
		dockerItems, _ := docker.Discover(ctx, opts.ContainerName)
		all = append(all, dockerItems...)
	} else if strings.TrimSpace(opts.PodName) != "" {
		podItems, _ := pod.Discover(ctx, opts.PodName, opts.Namespace)
		all = append(all, podItems...)
	} else {
		procItems, err := process.Discover(ctx, opts.PID)
		if err != nil {
			return nil, err
		}
		all = append(all, procItems...)
		dockerItems, _ := docker.Discover(ctx, "")
		all = append(all, dockerItems...)
		podItems, _ := pod.Discover(ctx, opts.PodName, opts.Namespace)
		all = append(all, podItems...)
	}
	for i := range all {
		if all[i].MetricsEndpoint == "" {
			all[i].MetricsEndpoint = "http://127.0.0.1:8000/metrics"
		}
	}
	return dedupe(all), nil
}

func (Service) Select(candidates []Candidate, noInteractive bool) (Candidate, error) {
	if len(candidates) == 0 {
		return Candidate{}, fmt.Errorf("no vLLM targets discovered")
	}
	if len(candidates) == 1 || noInteractive {
		return candidates[0], nil
	}
	for i, item := range candidates {
		fmt.Printf("[%d] %s pid=%d container=%s pod=%s cmd=%q\n", i+1, item.Source, item.PID, item.ContainerID, item.PodName, short(item.RawCommandLine))
	}
	fmt.Print("Select target [1-", len(candidates), "]: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	idx, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || idx < 1 || idx > len(candidates) {
		return Candidate{}, fmt.Errorf("invalid selection")
	}
	return candidates[idx-1], nil
}

func short(text string) string {
	if len(text) <= 80 {
		return text
	}
	return text[:77] + "..."
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
