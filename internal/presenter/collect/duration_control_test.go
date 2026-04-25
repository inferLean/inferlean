package collect

import (
	"testing"
	"time"
)

func TestApplyCollectionDurationActionAdjustsDeadline(t *testing.T) {
	t.Parallel()
	started := time.Unix(1700000000, 0).UTC()
	deadline := started.Add(30 * time.Second)
	now := started.Add(10 * time.Second)

	next, stopNow, changed := applyCollectionDurationAction(
		deadline,
		started,
		now,
		collectionActionIncreaseMinute,
		maxInteractiveCollectDuration,
	)
	if stopNow {
		t.Fatal("expected increase action to avoid stop")
	}
	if !changed {
		t.Fatal("expected increase action to change deadline")
	}
	if got, want := next, deadline.Add(time.Minute); !got.Equal(want) {
		t.Fatalf("deadline after increase = %s, want %s", got, want)
	}
}

func TestApplyCollectionDurationActionClampsToMinimum(t *testing.T) {
	t.Parallel()
	started := time.Unix(1700000000, 0).UTC()
	deadline := started.Add(30 * time.Second)
	now := started.Add(5 * time.Second)

	next, stopNow, changed := applyCollectionDurationAction(
		deadline,
		started,
		now,
		collectionActionDecreaseMinute,
		maxInteractiveCollectDuration,
	)
	if stopNow {
		t.Fatal("expected decrease action to avoid explicit stop")
	}
	if !changed {
		t.Fatal("expected decrease action to change deadline")
	}
	if got, want := next, started.Add(minInteractiveCollectDuration); !got.Equal(want) {
		t.Fatalf("deadline after clamp = %s, want %s", got, want)
	}
}

func TestApplyCollectionDurationActionStop(t *testing.T) {
	t.Parallel()
	started := time.Unix(1700000000, 0).UTC()
	deadline := started.Add(30 * time.Second)
	now := started.Add(5 * time.Second)

	next, stopNow, changed := applyCollectionDurationAction(
		deadline,
		started,
		now,
		collectionActionStopAndAnalyze,
		maxInteractiveCollectDuration,
	)
	if !stopNow {
		t.Fatal("expected stop action to request stop")
	}
	if changed {
		t.Fatal("expected stop action to avoid deadline mutation")
	}
	if !next.Equal(deadline) {
		t.Fatalf("deadline changed to %s, want %s", next, deadline)
	}
}

func TestCollectorDurationWindow(t *testing.T) {
	t.Parallel()
	if got := collectorDurationWindow(30*time.Second, false); got != 30*time.Second {
		t.Fatalf("non-interactive duration = %s, want 30s", got)
	}
	if got := collectorDurationWindow(30*time.Second, true); got != maxInteractiveCollectDuration {
		t.Fatalf("interactive duration = %s, want %s", got, maxInteractiveCollectDuration)
	}
}
