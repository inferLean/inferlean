package report

import (
	"fmt"
	"strings"

	"github.com/inferLean/inferlean-main/cli/internal/browser"
)

type reportDestination string

const (
	destinationBrowser  reportDestination = "browser"
	destinationTerminal reportDestination = "terminal"
)

func chooseDestination(nonInteractive, tty bool) reportDestination {
	if nonInteractive || !tty {
		return destinationTerminal
	}
	destination, err := chooseDestinationWithTUI()
	if err != nil {
		return destinationTerminal
	}
	return destination
}

func ReportURL(backendURL, installationID, runID string) (string, bool) {
	trimmedInstallationID := strings.TrimSpace(installationID)
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedInstallationID == "" || trimmedRunID == "" {
		return "", false
	}
	return fmt.Sprintf("%s/%s/%s", backendURL, trimmedInstallationID, trimmedRunID), true
}

func isIdentityComplete(identity reportIdentity) bool {
	return identity.installationID != "" && identity.runID != ""
}

var openBrowser = browser.Open
