package config

import (
	"os"
	"path/filepath"
	"testing"
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

	want := Config{InstallationID: "installation-123"}
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

	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("config file is empty")
	}
}
