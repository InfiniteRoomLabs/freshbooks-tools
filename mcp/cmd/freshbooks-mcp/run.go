package main

import (
	"io"
)

// run builds the cobra root command, executes it against args, and returns
// the process exit code main should pass to os.Exit. It is the seam
// main.go's single os.Exit(run(...)) statement calls into, kept separate
// so it is testable without exercising os.Exit or reading real os.Args.
func run(stdout, stderr io.Writer, args []string, version string) int {
	root := newRootCmd(stdout, stderr, resolveVersion(version))
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		return 1
	}
	return 0
}
