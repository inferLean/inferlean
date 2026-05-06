package terminal

import (
	"os"

	"golang.org/x/term"
)

func InteractiveTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}
