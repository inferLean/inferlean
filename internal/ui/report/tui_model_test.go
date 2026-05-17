package report

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/inferLean/inferlean-main/cli/internal/defaults"
	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func TestReportModelDefaultExpandedState(t *testing.T) {
	t.Parallel()
	vm := buildReportViewModel(fullReportFixture(), reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	model := newReportModel(vm)

	if !model.expanded["top-issue-1"] || !model.expanded["saturation"] || !model.expanded["ranked-issues"] {
		t.Fatal("expected primary report cards to start expanded")
	}
	if model.expanded["top-opportunity-1"] {
		t.Fatal("expected top opportunity cards to start collapsed")
	}
}

func TestReportModelNavigationAndCollapse(t *testing.T) {
	t.Parallel()
	model := newSizedReportModel()
	updated := updateReportModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if updated.focus != 1 {
		t.Fatalf("focus = %d, want 1", updated.focus)
	}
	updated = updateReportModel(t, updated, tea.KeyMsg{Type: tea.KeyEnter})
	if updated.expanded["top-issue-2"] {
		t.Fatal("expected enter to collapse the focused card")
	}
	updated = updateReportModel(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if updated.focus != 0 {
		t.Fatalf("focus = %d, want 0", updated.focus)
	}
}

func TestReportModelEvidenceTabSwitching(t *testing.T) {
	t.Parallel()
	model := newSizedReportModel()
	for model.vm.cards[model.focus].id != "evidence" {
		model = updateReportModel(t, model, tea.KeyMsg{Type: tea.KeyTab})
	}
	updated := updateReportModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if updated.evidenceTab != 1 {
		t.Fatalf("evidenceTab = %d, want 1", updated.evidenceTab)
	}
	updated = updateReportModel(t, updated, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if updated.evidenceTab != 0 {
		t.Fatalf("evidenceTab = %d, want 0", updated.evidenceTab)
	}
}

func TestReportModelBrowserOpenAvailabilityDependsOnIdentity(t *testing.T) {
	t.Parallel()
	withBrowser := newReportModel(buildReportViewModel(fullReportFixture(), reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), ""))
	if !withBrowser.canOpenBrowser() {
		t.Fatal("expected browser action to be available")
	}

	report := fullReportFixture()
	report.Job.InstallationID = ""
	withoutBrowser := newReportModel(buildReportViewModel(report, reportIdentity{runID: "run-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), ""))
	if withoutBrowser.canOpenBrowser() {
		t.Fatal("expected browser action to be unavailable")
	}
}

func TestReportModelBrowserOpenFailureShowsManualURL(t *testing.T) {
	original := openBrowser
	openBrowser = func(string) error { return errors.New("no browser") }
	t.Cleanup(func() { openBrowser = original })

	model := newReportModel(buildReportViewModel(fullReportFixture(), reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), ""))
	updated, _ := model.handleOpenBrowser()
	reportModel, ok := updated.(*reportModel)
	if !ok {
		t.Fatalf("updated model = %T, want *reportModel", updated)
	}
	if !strings.Contains(reportModel.statusMessage, "failed to open browser; open https://app.inferlean.com/inst-123/run-123") {
		t.Fatalf("status message = %q", reportModel.statusMessage)
	}
}

func TestRenderReportContentHandlesNarrowWidth(t *testing.T) {
	t.Parallel()
	vm := buildReportViewModel(fullReportFixture(), reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	content, _ := renderReportContent(vm, 0, map[string]bool{
		"top-issue-1":        true,
		"saturation":         true,
		"ranked-issues":      true,
		"evidence":           true,
		"collection-quality": true,
	}, 0, 48)
	if !strings.Contains(content, "CLI Report Viewer") || !strings.Contains(content, "Top Issue Recommendation") {
		t.Fatalf("narrow render missing expected content: %s", content)
	}
}

func TestRenderReportContentShowsValidationWarning(t *testing.T) {
	t.Parallel()
	vm := buildReportViewModel(
		fullReportFixture(),
		reportIdentity{runID: "run-123", installationID: "inst-123"},
		defaults.AppBaseURL,
		time.Unix(1700000200, 0).UTC(),
		"Schema validation warning: detector output is malformed",
	)
	content, _ := renderReportContent(vm, 0, map[string]bool{
		"top-issue-1":        true,
		"saturation":         true,
		"ranked-issues":      true,
		"evidence":           true,
		"collection-quality": true,
	}, 0, 80)
	if !strings.Contains(content, "Schema validation warning: detector output is malformed") {
		t.Fatalf("expected validation warning in TUI content: %s", content)
	}
}

func TestRenderReportContentIncludesAllIssueRows(t *testing.T) {
	t.Parallel()
	report := fullReportFixture()
	report.Issues = make([]contracts.Issue, 0, 9)
	for i := 1; i <= 9; i++ {
		report.Issues = append(report.Issues, contracts.Issue{
			ID:         "issue:test",
			Rank:       i,
			Label:      "Issue",
			Confidence: "medium",
		})
	}
	vm := buildReportViewModel(report, reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	content, _ := renderReportContent(vm, 2, map[string]bool{
		"ranked-issues":      true,
		"evidence":           false,
		"collection-quality": false,
	}, 0, 120)
	if !strings.Contains(content, " 9 ") {
		t.Fatalf("expected later table rows to be present in content: %s", content)
	}
}

func TestRenderReportContentHighlightsActions(t *testing.T) {
	t.Parallel()
	vm := buildReportViewModel(fullReportFixture(), reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	content, _ := renderReportContent(vm, 0, map[string]bool{
		"top-issue-1":        true,
		"top-issue-2":        false,
		"saturation":         false,
		"evidence":           false,
		"collection-quality": false,
	}, 0, 120)
	if !strings.Contains(content, "Actions") || !strings.Contains(content, "Risk: Shorter maximum context for some requests.") {
		t.Fatalf("expected highlighted action content to remain visible: %s", content)
	}
}

func newSizedReportModel() reportModel {
	vm := buildReportViewModel(fullReportFixture(), reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	model := newReportModel(vm)
	next, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	return next.(reportModel)
}

func updateReportModel(t *testing.T, model reportModel, msg tea.Msg) reportModel {
	t.Helper()
	next, _ := model.Update(msg)
	updated, ok := next.(reportModel)
	if !ok {
		t.Fatal("unexpected model type")
	}
	return updated
}
