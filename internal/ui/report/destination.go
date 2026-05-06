package report

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/internal/browser"
)

type reportDestination string

const (
	destinationBrowser  reportDestination = "browser"
	destinationTerminal reportDestination = "terminal"
)

func chooseDestination(identity reportIdentity, noInteractive, tty bool) reportDestination {
	if noInteractive || !tty {
		return destinationTerminal
	}
	destination, err := chooseDestinationWithTUI()
	if err != nil {
		return destinationTerminal
	}
	return destination
}

func inferleanReportURL(backendURL string, identity reportIdentity) string {
	return fmt.Sprintf("%s/%s/%s", backendURL, identity.installationID, identity.runID)
}

func isIdentityComplete(identity reportIdentity) bool {
	return identity.installationID != "" && identity.runID != ""
}

func openBrowser(url string) error {
	return browser.Open(url)
}
