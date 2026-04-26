package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

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

func TestNoInteractiveFlagEnabled(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "run"}
	cmd.Flags().Bool("non-interactive", false, "")
	if err := cmd.Flags().Set("non-interactive", "true"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	if !noInteractiveFlagEnabled(cmd) {
		t.Fatalf("expected non-interactive flag to be detected")
	}
}
