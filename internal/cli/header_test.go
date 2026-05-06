package cli

import "testing"

func TestRenderHeaderColor(t *testing.T) {
	t.Parallel()
	header := renderHeader(true)
	if header == headerTitle+headerTag {
		t.Fatalf("expected ANSI styling in colored header")
	}
}

func TestRenderHeaderPlain(t *testing.T) {
	t.Parallel()
	header := renderHeader(false)
	if header != headerTitle+headerTag {
		t.Fatalf("unexpected plain header: %s", header)
	}
}
