package cmd

import "github.com/spf13/cobra"

// extraCommands is populated by build-tag-gated files' init() functions
// (docs_cmd.go's hidden `docs` command, currently the only one) so
// NewRootCmd can attach them without the untagged default build ever
// importing what those files import. Kept in its own untagged file so it
// exists regardless of which build tags are set.
var extraCommands []func(root *cobra.Command) *cobra.Command
