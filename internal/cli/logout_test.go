package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inferLean/inferlean/internal/config"
)

func TestLogoutClearsSavedAuthState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	store := config.NewStoreAt(filepath.Join(dir, config.DefaultDirName, config.DefaultFileName))
	cfg := config.Config{
		InstallationID: "installation-123",
		Auth: &config.AuthState{
			BackendURL:   "https://app.inferlean.com",
			Issuer:       "https://app.inferlean.com/dex",
			ClientID:     "inferlean-cli",
			TokenType:    "Bearer",
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Unix(1700000000, 0).UTC(),
		},
	}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var stdout bytes.Buffer
	cmd := newLogoutCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetContext(context.Background())

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	saved, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if saved.InstallationID != cfg.InstallationID {
		t.Fatalf("installation_id = %q, want %q", saved.InstallationID, cfg.InstallationID)
	}
	if saved.Auth != nil {
		t.Fatal("auth = non-nil, want cleared session")
	}

	output := stdout.String()
	if !strings.Contains(output, "InferLean login session cleared.") {
		t.Fatalf("stdout = %q, want logout confirmation", output)
	}
	if !strings.Contains(output, cfg.InstallationID) {
		t.Fatalf("stdout = %q, want installation id", output)
	}
}

func TestLogoutReportsAlreadyLoggedOut(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	store := config.NewStoreAt(filepath.Join(dir, config.DefaultDirName, config.DefaultFileName))
	if err := store.Save(config.Config{InstallationID: "installation-123"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var stdout bytes.Buffer
	cmd := newLogoutCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetContext(context.Background())

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "InferLean is already logged out." {
		t.Fatalf("stdout = %q, want %q", got, "InferLean is already logged out.")
	}
}
