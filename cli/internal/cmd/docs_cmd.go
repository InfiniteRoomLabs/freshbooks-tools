package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// newDocsCmd builds the hidden `docs <file>` command scripts/docs.sh
// (mise run docs) uses to regenerate docs/cli.md. root is the same
// command tree this command is itself attached to, so the generated
// reference documents every command including this one's siblings; docs
// itself is hidden (IsAvailableCommand() is false for a Hidden command),
// so it does not document itself.
func newDocsCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "docs <file>",
		Short:  "Regenerate the full command reference into a single Markdown file",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := GenerateDocs(root)
			if err != nil {
				return &runtimeError{err: err}
			}
			if err := os.WriteFile(args[0], content, 0o644); err != nil { //nolint:gosec // docs.md is not sensitive; permissive so any reader/editor can open it
				return &runtimeError{err: err}
			}
			return nil
		},
	}
}
