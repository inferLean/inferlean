package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreEnsureCreatesAndReusesInstallationID(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreAt(filepath.Join(dir, DefaultDirName, DefaultFileName))

	first, err := store.Ensure()
	if err != nil {
		t.Fatalf("Ensure() first error = %v", err)
	}
	if first.InstallationID == "" {
		t.Fatal("Ensure() first installation_id = empty")
	}

	second, err := store.Ensure()
	if err != nil {
		t.Fatalf("Ensure() second error = %v", err)
	}
	if second.InstallationID != first.InstallationID {
		t.Fatalf("Ensure() installation_id = %q, want %q", second.InstallationID, first.InstallationID)
	}
}

func TestStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreAt(filepath.Join(dir, DefaultDirName, DefaultFileName))

	want := Config{
		InstallationID: "installation-123",
		Auth: &AuthState{
			BackendURL:   "https://api.example.com",
			Issuer:       "https://dex.example.com/dex",
			ClientID:     "inferlean-cli",
			TokenType:    "Bearer",
			AccessToken:  "access-token",
			IDToken:      "id-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    time.Unix(1700000000, 0).UTC(),
			UseIDToken:   true,
		},
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.InstallationID != want.InstallationID {
		t.Fatalf("Load() installation_id = %q, want %q", got.InstallationID, want.InstallationID)
	}
	if got.Auth == nil {
		t.Fatal("Load() auth = nil, want populated auth state")
	}
	if got.Auth.BearerToken() != want.Auth.BearerToken() {
		t.Fatalf("Load() bearer token = %q, want %q", got.Auth.BearerToken(), want.Auth.BearerToken())
	}
	if !got.Auth.HasSession() {
		t.Fatal("Load() auth state should report a reusable session")
	}

	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("config file is empty")
	}
}

func TestAuthStateBearerTokenPrefersIDTokenWhenConfigured(t *testing.T) {
	t.Parallel()

	state := AuthState{
		AccessToken: "access-token",
		IDToken:     "id-token",
		UseIDToken:  true,
	}

	if got := state.BearerToken(); got != "id-token" {
		t.Fatalf("BearerToken() = %q, want %q", got, "id-token")
	}
}
