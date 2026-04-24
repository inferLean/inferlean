package collection

import (
	"testing"
	"time"
)

func TestRenderMetricsCollectionCountdown(t *testing.T) {
	t.Parallel()
	rendered := renderMetricsCollectionCountdown(30 * time.Second)
	if rendered != "collecting metrics through prometheus scrape manager (30s remaining)" {
		t.Fatalf("unexpected countdown: %s", rendered)
	}
}

func TestRenderMetricsCollectionCountdownRoundsSmallPositiveToOne(t *testing.T) {
	t.Parallel()
	rendered := renderMetricsCollectionCountdown(500 * time.Millisecond)
	if rendered != "collecting metrics through prometheus scrape manager (1s remaining)" {
		t.Fatalf("unexpected countdown for sub-second duration: %s", rendered)
	}
}
