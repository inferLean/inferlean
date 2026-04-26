package collection

import (
	"testing"
	"time"
)

func TestRenderMetricsCollectionCountdown(t *testing.T) {
	t.Parallel()
	rendered := renderMetricsCollectionCountdown(30*time.Second, false)
	if rendered != "collecting metrics through prometheus scrape manager (30s remaining)" {
		t.Fatalf("unexpected countdown: %s", rendered)
	}
}

func TestRenderMetricsCollectionCountdownRoundsSmallPositiveToOne(t *testing.T) {
	t.Parallel()
	rendered := renderMetricsCollectionCountdown(500*time.Millisecond, false)
	if rendered != "collecting metrics through prometheus scrape manager (1s remaining)" {
		t.Fatalf("unexpected countdown for sub-second duration: %s", rendered)
	}
}

func TestRenderMetricsCollectionCountdownInteractiveHint(t *testing.T) {
	t.Parallel()
	rendered := renderMetricsCollectionCountdown(45*time.Second, true)
	if rendered != "collecting metrics through prometheus scrape manager (45s remaining)"+interactiveCollectionHint() {
		t.Fatalf("unexpected interactive countdown: %s", rendered)
	}
}
