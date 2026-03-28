package cli

import (
	"os"
	"testing"

	"github.com/inferLean/inferlean/internal/config"
)

func TestResolveBackendURLFallsBackToDefault(t *testing.T) {
	t.Setenv(backendURLEnv, "")

	got, err := resolveBackendURL("", nil)
	if err != nil {
		t.Fatalf("resolveBackendURL() error = %v", err)
	}
	if got != defaultBackendURL {
		t.Fatalf("resolveBackendURL() = %q, want %q", got, defaultBackendURL)
	}
}

func TestResolveBackendURLPrefersSavedSession(t *testing.T) {
	t.Setenv(backendURLEnv, "")

	got, err := resolveBackendURL("", &config.AuthState{BackendURL: "https://staging.inferlean.com/"})
	if err != nil {
		t.Fatalf("resolveBackendURL() error = %v", err)
	}
	if got != "https://staging.inferlean.com" {
		t.Fatalf("resolveBackendURL() = %q, want %q", got, "https://staging.inferlean.com")
	}
}

func TestResolveBackendURLPrefersEnvVarOverDefault(t *testing.T) {
	t.Setenv(backendURLEnv, "https://env.inferlean.com/")
	defer os.Unsetenv(backendURLEnv)

	got, err := resolveBackendURL("", nil)
	if err != nil {
		t.Fatalf("resolveBackendURL() error = %v", err)
	}
	if got != "https://env.inferlean.com" {
		t.Fatalf("resolveBackendURL() = %q, want %q", got, "https://env.inferlean.com")
	}
}
