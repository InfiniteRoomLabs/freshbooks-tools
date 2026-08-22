// Package cmd builds the freshbooks CLI's cobra command tree. Phase 4 adds
// the resource commands generated from the inventory parity contract; this
// scaffold wires only the root command plus the version and completion
// subcommands cobra provides.
package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// NewRootCmd builds the freshbooks root command. version is embedded so
// `freshbooks version` reports the binary that was actually built.
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "freshbooks",
		Short: "A kubectl-style CLI for the FreshBooks REST API",
		Long: "freshbooks is a command-line client for the FreshBooks REST API, " +
			"backed by the github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks library.",
	}

	root.AddCommand(newVersionCmd(version))

	return root
}

// Run builds the root command, executes it against args, and returns the
// process exit code main should pass to os.Exit. It is the seam main.go's
// single os.Exit(cmd.Run(...)) statement calls into, kept separate so it is
// testable without exercising os.Exit itself.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	root := NewRootCmd(version)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, err) //nolint:errcheck // best-effort error report; nothing more we can do if this write also fails
		return 1
	}
	return 0
}

func newVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the freshbooks CLI version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version)
			return err
		},
	}
}
