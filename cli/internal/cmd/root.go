// Package cmd builds the freshbooks CLI's cobra command tree. Phase 4 adds
// the resource commands generated from the inventory parity contract; this
// scaffold wires only the root command plus the version and completion
// subcommands cobra provides.
package cmd

import (
	"fmt"

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
	root.CompletionOptions.DisableDefaultCmd = false

	return root
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
