package cmd

import (
	"os"

	"golang.org/x/term"
)

// isTerminalFile reports whether f is connected to a terminal.
func isTerminalFile(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
