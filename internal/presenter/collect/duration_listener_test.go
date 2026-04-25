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
