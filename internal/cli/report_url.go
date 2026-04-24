package cli

import (
	"fmt"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/defaults"
)

func browserReportURL(installationID, runID string) (string, bool) {
	trimmedInstallationID := strings.TrimSpace(installationID)
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedInstallationID == "" || trimmedRunID == "" {
		return "", false
	}
	return fmt.Sprintf("%s/%s/%s", defaults.AppBaseURL, trimmedInstallationID, trimmedRunID), true
}

func shouldEmitBrowserURL(noInteractive bool) bool {
	return noInteractive || !interactiveTTY()
}
