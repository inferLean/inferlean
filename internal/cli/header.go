package cli

import (
	"fmt"
	"os"

	"github.com/inferLean/inferlean-main/cli/internal/ui/chrome"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	headerTitle = chrome.HeaderTitle
	headerTag   = chrome.HeaderTag
)

func maybePrintHeader(cmd *cobra.Command) {
	if !shouldPrintHeader(cmd) {
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), renderHeader(useColor()))
}

func shouldPrintHeader(cmd *cobra.Command) bool {
	if cmd.Name() == "version" {
		return false
	}
	if noInteractiveFlagEnabled(cmd) {
		return false
	}
	return interactiveTTY()
}

func noInteractiveFlagEnabled(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup("no-interactive")
	return flag != nil && flag.Value.String() == "true"
}

func interactiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func useColor() bool {
	return chrome.UseColor()
}

func renderHeader(color bool) string {
	return chrome.Render(color)
}
