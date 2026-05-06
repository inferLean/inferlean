package cli

import (
	"fmt"
	"strings"
)

func browserReportURL(backendUrl, installationID, runID string) (string, bool) {
	trimmedInstallationID := strings.TrimSpace(installationID)
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedInstallationID == "" || trimmedRunID == "" {
		return "", false
	}
	return fmt.Sprintf("%s/%s/%s", backendUrl, trimmedInstallationID, trimmedRunID), true
}

func shouldEmitBrowserURL(noInteractive bool) bool {
	return noInteractive || !interactiveTTY()
}
