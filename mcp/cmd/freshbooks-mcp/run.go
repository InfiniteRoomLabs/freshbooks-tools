package main

import (
	"fmt"
	"io"
)

// run prints the version string to stdout and returns the process exit
// code main should pass to os.Exit. It is the seam main.go's single
// os.Exit(run(...)) statement calls into, kept separate so it is testable
// without exercising os.Exit itself.
func run(stdout, stderr io.Writer, v string) int {
	if _, err := fmt.Fprintf(stdout, "freshbooks-mcp %s\n", v); err != nil {
		fmt.Fprintln(stderr, err) //nolint:errcheck // best-effort error report; nothing more we can do if this write also fails
		return 1
	}
	return 0
}
