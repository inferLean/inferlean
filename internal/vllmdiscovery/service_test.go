package vllmdiscovery

import (
	"reflect"
	"strings"
	"testing"

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
