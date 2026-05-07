package report

import (
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
)

func TestIsIdentityCompleteFalseWhenMissingFields(t *testing.T) {
	t.Parallel()
	if isIdentityComplete(reportIdentity{}) {
		t.Fatal("expected identity to be incomplete")
	}
}

func TestChooseDestinationSkipsSelectorWhenNotTTY(t *testing.T) {
	t.Parallel()
	destination := chooseDestination(false, false)
	if destination != destinationTerminal {
		t.Fatalf("expected terminal fallback when tty is disabled, got %q", destination)
	}
}

func TestChooseDestinationSkipsSelectorWhenNonInteractive(t *testing.T) {
	t.Parallel()
	destination := chooseDestination(true, true)
	if destination != destinationTerminal {
		t.Fatalf("expected terminal fallback when non-interactive is enabled, got %q", destination)
	}
}

func TestReportURL(t *testing.T) {
	t.Parallel()
	url, ok := ReportURL(defaults.AppBaseURL, "inst_456", "run_123")
	if !ok {
		t.Fatal("expected report URL")
	}
	if url != "https://app.inferlean.com/inst_456/run_123" {
		t.Fatalf("unexpected report URL: %s", url)
	}
}
