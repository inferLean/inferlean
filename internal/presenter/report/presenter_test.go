package report

import (
	"testing"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
	reportui "github.com/inferLean/inferlean-main/cli/internal/ui/report"
)

func TestBrowserReportURL(t *testing.T) {
	t.Parallel()
	url, ok := reportui.ReportURL(defaults.AppBaseURL, "inst_123", "run_456")
	if !ok {
		t.Fatal("expected browser report URL to be present")
	}
	if url != "https://app.inferlean.com/inst_123/run_456" {
		t.Fatalf("unexpected browser report URL: %s", url)
	}
}

func TestBrowserReportURLMissingIdentity(t *testing.T) {
	t.Parallel()
	if _, ok := reportui.ReportURL(defaults.AppBaseURL, "", "run_456"); ok {
		t.Fatal("expected no URL when installation id is missing")
	}
	if _, ok := reportui.ReportURL(defaults.AppBaseURL, "inst_123", ""); ok {
		t.Fatal("expected no URL when run id is missing")
	}
}

func TestShouldEmitBrowserURLRespectsNonInteractiveFlag(t *testing.T) {
	t.Parallel()
	if !shouldEmitBrowserURL(true) {
		t.Fatal("expected browser URL emission when non-interactive is enabled")
	}
}
