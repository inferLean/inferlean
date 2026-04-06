package runbrowser

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/inferLean/inferlean/pkg/contracts"
)

func TestLoadResultErrorReturnsToListAndShowsInlineMessage(t *testing.T) {
	m := &model{mode: modeLoading}

	updated, _ := m.Update(loadResultMsg{err: errors.New("temporary backend error")})
	next := updated.(*model)

	if next.mode != modeList {
		t.Fatalf("mode = %v, want %v", next.mode, modeList)
	}
	if got := next.View(); !strings.Contains(got, "Load failed: temporary backend error") {
		t.Fatalf("View() = %q, want inline error", got)
	}
}

func TestLoadResultSuccessQuitsWithReport(t *testing.T) {
	m := &model{mode: modeLoading}
	report := contracts.FinalReport{Job: contracts.ReportJob{RunID: "run-123"}}

	updated, cmd := m.Update(loadResultMsg{report: report})
	next := updated.(*model)

	if next.report == nil || next.report.Job.RunID != "run-123" {
		t.Fatalf("report = %+v, want run-123", next.report)
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Fatalf("cmd() = %v, want tea.Quit", msg)
	}
}
