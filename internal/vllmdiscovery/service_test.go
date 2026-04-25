package vllmdiscovery

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/inferLean/inferlean-main/cli/internal/vllmdiscovery/shared"
)

func TestBuildPlan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		opts    DiscoverOptions
		want    []string
		wantErr string
	}{
		{
			name: "default order",
			want: []string{shared.SourceProcesses, shared.SourceDocker, shared.SourceKubernetes},
		},
		{
			name: "exclude docker",
			opts: DiscoverOptions{
				ExcludeDocker: true,
			},
			want: []string{shared.SourceProcesses, shared.SourceKubernetes},
		},
		{
			name: "container only",
			opts: DiscoverOptions{
				ContainerName: "vllm",
			},
			want: []string{shared.SourceDocker},
		},
		{
			name: "pod only",
			opts: DiscoverOptions{
				PodName: "vllm-pod",
			},
			want: []string{shared.SourceKubernetes},
		},
		{
			name: "all excluded",
			opts: DiscoverOptions{
				ExcludeProcesses:  true,
				ExcludeDocker:     true,
				ExcludeKubernetes: true,
			},
			wantErr: "all discovery sources are excluded",
		},
		{
			name: "pid conflicts with excluded processes",
			opts: DiscoverOptions{
				PID:              1234,
				ExcludeProcesses: true,
			},
			wantErr: "--pid conflicts",
		},
		{
			name: "container conflicts with excluded docker",
			opts: DiscoverOptions{
				ContainerName: "vllm",
				ExcludeDocker: true,
			},
			wantErr: "--container conflicts",
		},
		{
			name: "pod conflicts with excluded kubernetes",
			opts: DiscoverOptions{
				PodName:           "vllm-pod",
				ExcludeKubernetes: true,
			},
			wantErr: "--pod conflicts",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildPlan(tc.opts)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("buildPlan() error = nil, want contains %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("buildPlan() error = %q, want contains %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildPlan() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("buildPlan() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDedupeMergesProcessAndDockerWhenPIDMatches(t *testing.T) {
	t.Parallel()
	started := time.Unix(1700000000, 0).UTC()
	candidates := []Candidate{
		{
			Source:         "process",
			PID:            4242,
			Executable:     "/usr/bin/python3",
			RawCommandLine: "python -m vllm.entrypoints.openai.api_server --port 8000",
			StartedAt:      started,
		},
		{
			Source:         "docker",
			PID:            4242,
			ContainerID:    "2f7e03e88af1",
			Executable:     "docker-container:vllm",
			RawCommandLine: "python -m vllm.entrypoints.openai.api_server --port 8000",
		},
	}

	got := dedupe(candidates)
	if len(got) != 1 {
		t.Fatalf("dedupe() length = %d, want 1", len(got))
	}
	merged := got[0]
	if merged.Source != "docker" {
		t.Fatalf("merged source = %q, want docker", merged.Source)
	}
	if merged.ContainerID != "2f7e03e88af1" {
		t.Fatalf("merged container id = %q, want 2f7e03e88af1", merged.ContainerID)
	}
	if merged.StartedAt != started {
		t.Fatalf("merged started_at = %s, want %s", merged.StartedAt, started)
	}
}

func TestDedupeMergesContainerIDsWithSamePrefix(t *testing.T) {
	t.Parallel()
	fullID := "2f7e03e88af1d7a0b0a0658277758a863cdf5964568fbc2f6c4ccf4d294fd40e"
	candidates := []Candidate{
		{
			Source:      "docker",
			ContainerID: "2f7e03e88af1",
			Executable:  "docker-container:vllm",
		},
		{
			Source:      "process",
			ContainerID: fullID,
			Executable:  "/usr/bin/python3",
		},
	}

	got := dedupe(candidates)
	if len(got) != 1 {
		t.Fatalf("dedupe() length = %d, want 1", len(got))
	}
	if got[0].ContainerID != fullID {
		t.Fatalf("merged container id = %q, want %q", got[0].ContainerID, fullID)
	}
}

func TestDedupeKeepsDistinctPIDs(t *testing.T) {
	t.Parallel()
	candidates := []Candidate{
		{Source: "process", PID: 1111, RawCommandLine: "vllm serve --model a"},
		{Source: "docker", PID: 2222, ContainerID: "2f7e03e88af1", RawCommandLine: "vllm serve --model a"},
	}

	got := dedupe(candidates)
	if len(got) != 2 {
		t.Fatalf("dedupe() length = %d, want 2", len(got))
	}
}
