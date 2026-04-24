package report

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBuildDestinationListDefaultsToBrowser(t *testing.T) {
	t.Parallel()
	model := newDestinationModel()
	selected, ok := model.list.SelectedItem().(destinationItem)
	if !ok {
		t.Fatalf("expected destination item selection")
	}
	if selected.option.value != destinationBrowser {
		t.Fatalf("default selection = %q, want %q", selected.option.value, destinationBrowser)
	}
}

func TestDestinationModelSelectTerminal(t *testing.T) {
	t.Parallel()
	model := newDestinationModel()
	next, _ := model.updateKey(tea.KeyMsg{Type: tea.KeyDown})
	updated, ok := next.(destinationModel)
	if !ok {
		t.Fatalf("updateKey() model type = %T, want destinationModel", next)
	}
	next, _ = updated.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	selectedModel, ok := next.(destinationModel)
	if !ok {
		t.Fatalf("updateKey() model type = %T, want destinationModel", next)
	}
	if selectedModel.selected != destinationTerminal {
		t.Fatalf("selected destination = %q, want %q", selectedModel.selected, destinationTerminal)
	}
}

func TestDestinationModelCancel(t *testing.T) {
	t.Parallel()
	model := newDestinationModel()
	next, _ := model.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	cancelledModel, ok := next.(destinationModel)
	if !ok {
		t.Fatalf("updateKey() model type = %T, want destinationModel", next)
	}
	if !cancelledModel.cancelled {
		t.Fatal("expected selector to mark cancelled state")
	}
}
