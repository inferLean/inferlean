package interactive

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBuildQuestionList_ShowsAllTargetOptionsOnOnePage(t *testing.T) {
	t.Parallel()

	l := buildQuestionList(targetQuestion(), 48, 9)
	visible := l.VisibleItems()

	if got, want := len(visible), 3; got != want {
		t.Fatalf("len(VisibleItems())=%d, want %d", got, want)
	}
	if !l.ShowPagination() {
		t.Fatal("expected pagination to remain enabled")
	}
}

func TestQuestionnaireTransition_KeepsTargetOptionsOnOnePage(t *testing.T) {
	t.Parallel()

	model := newQuestionnaireModel([]question{
		modeQuestion(),
		targetQuestion(),
	})

	next, _ := model.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := next.(questionnaireModel)
	if !ok {
		t.Fatalf("updateKey() model type = %T, want questionnaireModel", next)
	}
	if updated.index != 1 {
		t.Fatalf("index=%d, want 1", updated.index)
	}
	if updated.list.Title != "Declared optimization target" {
		t.Fatalf("title=%q, want %q", updated.list.Title, "Declared optimization target")
	}
	if got, want := len(updated.list.VisibleItems()), 3; got != want {
		t.Fatalf("len(VisibleItems())=%d, want %d", got, want)
	}
}
