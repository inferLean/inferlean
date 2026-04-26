package collect

import "testing"

func TestMapCollectionDurationKeyAction(t *testing.T) {
	t.Parallel()
	cases := map[byte]collectionDurationAction{
		'm': collectionActionIncreaseMinute,
		'M': collectionActionDecreaseMinute,
		's': collectionActionIncreaseSeconds,
		'S': collectionActionDecreaseSeconds,
		'c': collectionActionStopAndAnalyze,
		'C': collectionActionStopAndAnalyze,
		'x': collectionActionUnknown,
	}
	for input, want := range cases {
		if got := mapCollectionDurationKeyAction(input); got != want {
			t.Fatalf("mapCollectionDurationKeyAction(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestIsCollectionInterruptKey(t *testing.T) {
	t.Parallel()
	if !isCollectionInterruptKey(3) {
		t.Fatal("expected ctrl+c byte to interrupt collection")
	}
	if isCollectionInterruptKey('c') {
		t.Fatal("expected c key to remain a collection action")
	}
}
