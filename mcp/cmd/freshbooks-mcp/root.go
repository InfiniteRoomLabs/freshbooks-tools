package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/config"
	"github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/server"
	"github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/tools"
	"github.com/spf13/cobra"
)

// newRootCmd builds the cobra command tree: serve, version, tools. stdout
// and stderr are wired explicitly into every subcommand's output rather
// than read from cmd.OutOrStdout(), so run's tests can assert on them
// without touching the real os.Stdout/os.Stderr.
func newRootCmd(stdout, stderr io.Writer, version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "freshbooks-mcp",
		Short:         "A stateless Model Context Protocol server for the FreshBooks API",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newVersionCmd(stdout, version))
	root.AddCommand(newToolsCmd(stdout))
	root.AddCommand(newServeCmd(version))
	return root
}

// newVersionCmd prints the process's version to stdout.
func newVersionCmd(stdout io.Writer, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the freshbooks-mcp version",
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintf(stdout, "freshbooks-mcp %s\n", version)
			return err
		},
	}
}

// newToolsCmd prints the tool manifest as JSON: name, description,
// annotations, and inputSchema for every registered tool, sorted by name.
// Docs generation and the mcp-side parity test both read this shape.
func newToolsCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "tools",
		Short: "Print the MCP tool manifest as JSON",
		RunE: func(*cobra.Command, []string) error {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(tools.Manifest())
		},
	}
}

// newServeCmd serves the MCP endpoint over stdio or HTTP, per
// --transport. It shuts down gracefully on SIGINT/SIGTERM.
func newServeCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the MCP endpoint over stdio or HTTP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.Load(cmd)
			if err := cfg.Validate(); err != nil {
				return err
			}

			srv := server.New(cfg, version)
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if cfg.Transport == "http" {
				return srv.RunHTTP(ctx)
			}
			return srv.RunStdio(ctx)
		},
	}
	config.AddFlags(cmd)
	return cmd
}
