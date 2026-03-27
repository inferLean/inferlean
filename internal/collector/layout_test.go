package collector

import (
	"strings"
	"testing"
	"time"
)

func TestNewRunIDUsesSortableTimestampPrefix(t *testing.T) {
	first, err := newRunID()
	if err != nil {
		t.Fatalf("newRunID() error = %v", err)
	}

	time.Sleep(2 * time.Millisecond)

	second, err := newRunID()
	if err != nil {
		t.Fatalf("newRunID() error = %v", err)
	}

	firstPrefix, firstSuffix := splitRunID(t, first)
	secondPrefix, secondSuffix := splitRunID(t, second)

	if len(firstSuffix) != runIDSuffixSize*2 {
		t.Fatalf("first suffix length = %d, want %d", len(firstSuffix), runIDSuffixSize*2)
	}
	if len(secondSuffix) != runIDSuffixSize*2 {
		t.Fatalf("second suffix length = %d, want %d", len(secondSuffix), runIDSuffixSize*2)
	}

	if _, err := time.Parse(runIDTimeLayout, firstPrefix); err != nil {
		t.Fatalf("first prefix %q did not match layout: %v", firstPrefix, err)
	}
	if _, err := time.Parse(runIDTimeLayout, secondPrefix); err != nil {
		t.Fatalf("second prefix %q did not match layout: %v", secondPrefix, err)
	}

	if first >= second {
		t.Fatalf("run IDs should sort by creation time: first=%q second=%q", first, second)
	}
}

func splitRunID(t *testing.T, runID string) (string, string) {
	t.Helper()

	index := strings.LastIndex(runID, "-")
	if index == -1 {
		t.Fatalf("run ID %q missing suffix separator", runID)
	}

	return runID[:index], runID[index+1:]
}
