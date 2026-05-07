package report

import (
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

	if !model.expanded["verdict"] || !model.expanded["primary-recommendation"] || !model.expanded["frontier"] || !model.expanded["issues"] {
		t.Fatal("expected primary cards to start expanded")
	}
	if model.expanded["quantization"] || model.expanded["secondary-opportunity"] {
		t.Fatal("expected optional opportunity cards to start collapsed")
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
	if updated.expanded["primary-recommendation"] {
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

func TestRenderReportContentHandlesNarrowWidth(t *testing.T) {
	t.Parallel()
	vm := buildReportViewModel(fullReportFixture(), reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	content, _ := renderReportContent(vm, 0, map[string]bool{
		"verdict":                true,
		"primary-recommendation": true,
		"frontier":               true,
		"issues":                 true,
		"evidence":               true,
		"collection-quality":     true,
	}, 0, 48)
	if !strings.Contains(content, "CLI Report Viewer") || !strings.Contains(content, "Verdict") {
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
		"verdict":                true,
		"primary-recommendation": true,
		"frontier":               true,
		"issues":                 true,
		"evidence":               true,
		"collection-quality":     true,
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
			Summary:    "summary row",
			Confidence: "medium",
		})
	}
	vm := buildReportViewModel(report, reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	content, _ := renderReportContent(vm, 5, map[string]bool{
		"verdict":                true,
		"primary-recommendation": true,
		"frontier":               true,
		"quantization":           false,
		"secondary-opportunity":  false,
		"issues":                 true,
		"evidence":               false,
		"collection-quality":     false,
	}, 0, 120)
	if !strings.Contains(content, " 9 ") {
		t.Fatalf("expected later table rows to be present in content: %s", content)
	}
}

func TestRenderReportContentHighlightsActions(t *testing.T) {
	t.Parallel()
	vm := buildReportViewModel(fullReportFixture(), reportIdentity{runID: "run-123", installationID: "inst-123"}, defaults.AppBaseURL, time.Unix(1700000200, 0).UTC(), "")
	content, _ := renderReportContent(vm, 1, map[string]bool{
		"verdict":                true,
		"primary-recommendation": true,
		"frontier":               false,
		"quantization":           false,
		"secondary-opportunity":  false,
		"issues":                 false,
		"evidence":               false,
		"collection-quality":     false,
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
