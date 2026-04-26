package cli

import "testing"

func TestBrowserReportURL(t *testing.T) {
	t.Parallel()
	url, ok := browserReportURL("inst_123", "run_456")
	if !ok {
		t.Fatal("expected browser report URL to be present")
	}
	if url != "https://app.inferlean.com/inst_123/run_456" {
		t.Fatalf("unexpected browser report URL: %s", url)
	}
}

func TestBrowserReportURLMissingIdentity(t *testing.T) {
	t.Parallel()
	if _, ok := browserReportURL("", "run_456"); ok {
		t.Fatal("expected no URL when installation id is missing")
	}
	if _, ok := browserReportURL("inst_123", ""); ok {
		t.Fatal("expected no URL when run id is missing")
	}
}

func TestShouldEmitBrowserURLRespectsNoInteractiveFlag(t *testing.T) {
	t.Parallel()
	if !shouldEmitBrowserURL(true) {
		t.Fatal("expected browser URL emission when non-interactive is enabled")
	}
}
