package discovery

import (
	"context"
	"errors"
	"testing"

	"github.com/inferLean/inferlean/internal/discovery/process"
)

type stubInspector struct {
	snapshots []process.Snapshot
}

func (s stubInspector) List(context.Context, bool) ([]process.Snapshot, error) {
	return s.snapshots, nil
}

type stubMetadataResolver func(context.Context, []CandidateGroup, Options) ([]CandidateGroup, runtimeInventory, error)

func (s stubMetadataResolver) Enrich(ctx context.Context, groups []CandidateGroup, opts Options) ([]CandidateGroup, runtimeInventory, error) {
	return s(ctx, groups, opts)
}

func TestDiscoverSelectsExplicitPID(t *testing.T) {
	t.Parallel()

	service := NewService(stubInspector{snapshots: []process.Snapshot{
		{PID: 10, Args: []string{"vllm", "serve", "model-a", "--port", "8000"}},
		{PID: 20, Args: []string{"vllm", "serve", "model-b", "--port", "8001"}},
	}})

	result, err := service.Discover(context.Background(), Options{PID: 20})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if result.Selected == nil || result.Selected.PrimaryPID != 20 {
		t.Fatalf("selected pid = %+v, want 20", result.Selected)
	}
}

func TestDiscoverSelectsExplicitContainer(t *testing.T) {
	t.Parallel()

	service := Service{
		inspector: stubInspector{snapshots: []process.Snapshot{
			{PID: 10, Args: []string{"vllm", "serve", "model-a", "--port", "8000"}},
		}},
		metadata: stubMetadataResolver(func(_ context.Context, groups []CandidateGroup, _ Options) ([]CandidateGroup, runtimeInventory, error) {
			groups[0].Target = TargetRef{
				Kind:                TargetKindDocker,
				DockerContainerID:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				DockerContainerName: "vllm-server",
			}
			return groups, runtimeInventory{
				Docker: []dockerContainer{{
					ID:   groups[0].Target.DockerContainerID,
					Name: groups[0].Target.DockerContainerName,
				}},
			}, nil
		}),
	}

	result, err := service.Discover(context.Background(), Options{Container: "vllm-server"})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if result.Selected == nil || result.Selected.Target.Kind != TargetKindDocker {
		t.Fatalf("selected target = %+v, want docker", result.Selected)
	}
}

func TestDiscoverSelectsExplicitPodInDefaultNamespace(t *testing.T) {
	t.Parallel()

	service := Service{
		inspector: stubInspector{snapshots: []process.Snapshot{
			{PID: 10, Args: []string{"vllm", "serve", "model-a", "--port", "8000"}},
		}},
		metadata: stubMetadataResolver(func(_ context.Context, groups []CandidateGroup, _ Options) ([]CandidateGroup, runtimeInventory, error) {
			groups[0].Target = TargetRef{
				Kind:                TargetKindKubernetes,
				KubernetesNamespace: "default",
				KubernetesPodName:   "vllm-0",
			}
			return groups, runtimeInventory{
				Pods: []kubernetesPod{{
					Namespace: "default",
					Name:      "vllm-0",
				}},
			}, nil
		}),
	}

	result, err := service.Discover(context.Background(), Options{Pod: "vllm-0"})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if result.Selected == nil || result.Selected.Target.Kind != TargetKindKubernetes {
		t.Fatalf("selected target = %+v, want kubernetes", result.Selected)
	}
}

func TestDiscoverRejectsMultipleExplicitSelectors(t *testing.T) {
	t.Parallel()

	service := NewService(stubInspector{})
	_, err := service.Discover(context.Background(), Options{
		PID:       10,
		Container: "vllm-server",
	})
	if err == nil || err.Error() != "specify only one of --pid, --container, or --pod" {
		t.Fatalf("err = %v, want selector validation error", err)
	}
}

func TestDiscoverReturnsAmbiguous(t *testing.T) {
	t.Parallel()

	service := NewService(stubInspector{snapshots: []process.Snapshot{
		{PID: 10, Args: []string{"vllm", "serve", "model-a", "--port", "8000"}},
		{PID: 20, Args: []string{"vllm", "serve", "model-b", "--port", "8001"}},
	}})

	result, err := service.Discover(context.Background(), Options{})
	if !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("err = %v, want ambiguous", err)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("candidate count = %d, want 2", len(result.Candidates))
	}
}
