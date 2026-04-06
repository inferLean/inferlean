package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/inferLean/inferlean/internal/config"
)

func TestEnsureScanSessionRequiresLoginNonInteractive(t *testing.T) {
	dir := t.TempDir()
	store := config.NewStoreAt(filepath.Join(dir, config.DefaultDirName, config.DefaultFileName))
	cfg := config.Config{InstallationID: "installation-123"}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	_, err := ensureScanSession(cmd, store, cfg, "https://app.inferlean.com", false)
	if err == nil || !strings.Contains(err.Error(), "login required") {
		t.Fatalf("ensureScanSession() error = %v, want login required", err)
	}
}
