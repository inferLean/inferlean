package cli

import (
	"os"
	"strings"

	"github.com/inferLean/inferlean/internal/config"
)

const (
	backendURLEnv     = "INFERLEAN_BACKEND_URL"
	defaultBackendURL = "https://app.inferlean.com"
)

func resolveBackendURL(flagValue string, authState *config.AuthState) (string, error) {
	if value := strings.TrimSpace(flagValue); value != "" {
		return strings.TrimRight(value, "/"), nil
	}
	if value := strings.TrimSpace(os.Getenv(backendURLEnv)); value != "" {
		return strings.TrimRight(value, "/"), nil
	}
	if authState != nil && strings.TrimSpace(authState.BackendURL) != "" {
		return strings.TrimRight(authState.BackendURL, "/"), nil
	}
	return defaultBackendURL, nil
}
