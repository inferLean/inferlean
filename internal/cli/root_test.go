package cli

import (
	"context"
	"testing"
)

func TestRootCommandSupportsBackendURLFlag(t *testing.T) {
	cmd := newRootCommand(context.Background())

	if err := cmd.ParseFlags([]string{"--backend-url", "http://127.0.0.1:8080"}); err != nil {
		t.Fatalf("parse backend-url: %v", err)
	}
	got, err := cmd.Flags().GetString("backend-url")
	if err != nil {
		t.Fatalf("get backend-url: %v", err)
	}
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("backend-url = %q, want local backend url", got)
	}
}

func TestRootCommandKeepsDeprecatedAppURLAlias(t *testing.T) {
	cmd := newRootCommand(context.Background())

	if err := cmd.ParseFlags([]string{"--app-url", "http://127.0.0.1:8080"}); err != nil {
		t.Fatalf("parse app-url: %v", err)
	}
	got, err := cmd.Flags().GetString("backend-url")
	if err != nil {
		t.Fatalf("get backend-url after app-url parse: %v", err)
	}
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("backend-url via app-url alias = %q, want local backend url", got)
	}

	flag := cmd.Flags().Lookup("app-url")
	if flag == nil {
		t.Fatal("expected deprecated app-url flag to exist")
	}
	if flag.Deprecated == "" {
		t.Fatal("expected app-url flag to be marked deprecated")
	}
}
