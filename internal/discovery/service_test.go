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
