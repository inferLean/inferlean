package report

import "testing"

func TestIsIdentityCompleteFalseWhenMissingFields(t *testing.T) {
	t.Parallel()
	if isIdentityComplete(reportIdentity{}) {
		t.Fatal("expected identity to be incomplete")
	}
}

func TestChooseDestinationSkipsSelectorWhenNotTTY(t *testing.T) {
	t.Parallel()
	destination := chooseDestination(reportIdentity{runID: "run_1", installationID: "inst_1"}, false, false)
	if destination != destinationTerminal {
		t.Fatalf("expected terminal fallback when tty is disabled, got %q", destination)
	}
}

func TestChooseDestinationSkipsSelectorWhenNoInteractive(t *testing.T) {
	t.Parallel()
	destination := chooseDestination(reportIdentity{runID: "run_1", installationID: "inst_1"}, true, true)
	if destination != destinationTerminal {
		t.Fatalf("expected terminal fallback when non-interactive is enabled, got %q", destination)
	}
}

func TestInferleanReportURL(t *testing.T) {
	t.Parallel()
	url := inferleanReportURL(reportIdentity{
		runID:          "run_123",
		installationID: "inst_456",
	})
	if url != "https://app.inferlean.com/inst_456/run_123" {
		t.Fatalf("unexpected report URL: %s", url)
	}
}
