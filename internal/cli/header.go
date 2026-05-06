package cli

import (
	"fmt"

	"github.com/inferLean/inferlean-main/cli/internal/terminal"
	"github.com/inferLean/inferlean-main/cli/internal/ui/chrome"
	"github.com/spf13/cobra"
)

const (
	headerTitle = chrome.HeaderTitle
	headerTag   = chrome.HeaderTag
)

func maybePrintHeader(cmd *cobra.Command, nonInteractive bool) {
	if !shouldPrintHeader(cmd, nonInteractive) {
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), renderHeader(useColor()))
}

func shouldPrintHeader(cmd *cobra.Command, nonInteractive bool) bool {
	if cmd.Name() == "version" {
		return false
	}
	if nonInteractive {
		return false
	}
	return terminal.InteractiveTTY()
}

func useColor() bool {
	return chrome.UseColor()
}

func renderHeader(color bool) string {
	return chrome.Render(color)
}
