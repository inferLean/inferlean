package collectprogress

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/stopwatch"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateRoutesStopwatchStartAndTickMessages(t *testing.T) {
	t.Parallel()

	interval := 10 * time.Millisecond
	m := &model{
		stopwatch: stopwatch.NewWithInterval(interval),
	}

	start := m.stopwatch.Start()
	msg := start()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("start command message = %T, want tea.BatchMsg", msg)
	}
	if len(batch) == 0 {
		t.Fatal("start command returned no commands")
	}

	updated, _ := m.Update(batch[0]())
	m = updated.(*model)
	if !m.stopwatch.Running() {
		t.Fatal("stopwatch should be running after start message")
	}

	updated, _ = m.Update(stopwatch.TickMsg{ID: m.stopwatch.ID()})
	m = updated.(*model)
	if got := m.stopwatch.Elapsed(); got != interval {
		t.Fatalf("elapsed = %s, want %s", got, interval)
	}
}
