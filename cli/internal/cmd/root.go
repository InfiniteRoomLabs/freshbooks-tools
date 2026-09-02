// Package cmd builds the freshbooks CLI's cobra command tree: the
// data-driven registry of 168 resource commands (registry.go,
// commands_*.go), the auth/config/api non-registry commands, and the
// global flag plumbing (scope resolution, output formatting, dry-run,
// exit codes) every command shares.
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the freshbooks root command: every global flag, the
// non-registry commands (auth, config, api, version, docs), and the full
// 168-command registry tree. version is embedded so `freshbooks version`
// reports the binary that was actually built.
func NewRootCmd(version string) *cobra.Command {
	state := &runtimeState{version: version}

	root := &cobra.Command{
		Use:   "freshbooks",
		Short: "A kubectl-style CLI for the FreshBooks REST API",
		Long: "freshbooks is a command-line client for the FreshBooks REST API, " +
			"backed by the github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks library. " +
			"See docs/cli.md in the repository for the full command reference.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	registerGlobalFlags(root)

	root.AddCommand(newVersionCmd(version))
	root.AddCommand(newAuthCmd(state))
	root.AddCommand(newConfigCmd(state))
	root.AddCommand(newAPICmd(state))
	for _, extra := range extraCommands {
		root.AddCommand(extra(root))
	}
	BuildTree(root, state)

	return root
}

// registerGlobalFlags adds every persistent flag documented in the work
// order: output format, scope, context/config file, display toggles,
// dry-run/confirm, timeout, log level, and the hidden --base-url test
// escape hatch. Each has the documented FRESHBOOKS_* env twin, resolved
// lazily by runtimeState (flag > env > config file > default) rather than
// here, so a flag left unset is distinguishable from one set to its zero
// value.
func registerGlobalFlags(root *cobra.Command) {
	flags := root.PersistentFlags()
	flags.StringP("output", "o", "", "output format: json, yaml, table, or name (default: table on a terminal, json otherwise)")
	flags.String("account", "", "FreshBooks account id (accounting-family scope)")
	flags.String("business", "", "FreshBooks business id (business-family scope)")
	flags.String("business-uuid", "", "FreshBooks business UUID (ledger-accounts scope)")
	flags.String("context", "", "config context to use (default: config.yaml's current-context, else \"default\")")
	flags.String("config", "", "path to config.yaml (default: $XDG_CONFIG_HOME/freshbooks/config.yaml, or ~/.config/freshbooks/config.yaml if $XDG_CONFIG_HOME is unset)")
	flags.Bool("no-headers", false, "suppress the header row in table output")
	flags.BoolP("quiet", "q", false, "suppress non-result output (errors still print)")
	flags.Bool("dry-run", false, "print the request that would be sent and send nothing")
	flags.Bool("yes", false, "confirm a destructive command")
	flags.Duration("timeout", 30*time.Second, "per-request timeout")
	flags.String("log-level", "", "log level: debug, info, warn, or error (default warn; env twin FRESHBOOKS_LOG_LEVEL)")
	flags.String("base-url", "", "override the FreshBooks API base URL (testing and sandboxes)")
	if err := flags.MarkHidden("base-url"); err != nil {
		panic(err) // a fixed, always-present flag name; only a programming error can make this fail
	}
}

// Run builds the root command, executes it against args, and returns the
// process exit code main should pass to os.Exit. It is the seam main.go's
// single os.Exit(cmd.Run(...)) statement calls into, kept separate so it
// is testable without exercising os.Exit itself. stdin feeds --file -
// bodies and the `auth login --no-browser` paste prompt.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, version string) int {
	root := NewRootCmd(resolveVersion(version))
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.Execute()
	if err == nil {
		return 0
	}

	// -q/--quiet silences non-error chatter, never errors (D6), so it has
	// no effect here.
	printError(stderr, err, errorIsJSON(args, stdout))
	return exitCodeFor(err)
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

// apiErrorJSON is the stderr error object D6 documents for -o json:
// {"error": {"status", "code", "message", "field", "family", "exit"}}.
type apiErrorJSON struct {
	Status  int    `json:"status,omitempty"`
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Family  string `json:"family,omitempty"`
	Exit    int    `json:"exit"`
}

// printError writes err to stderr: one JSON object under -o json,
// otherwise one human-readable line.
func printError(stderr io.Writer, err error, jsonFormat bool) {
	if !jsonFormat {
		fmt.Fprintln(stderr, err) //nolint:errcheck // best-effort; nothing more we can do if this write also fails
		return
	}

	obj := apiErrorJSON{Message: err.Error(), Exit: exitCodeFor(err)}
	var fbErr *freshbooks.Error
	if errors.As(err, &fbErr) {
		obj.Status, obj.Code, obj.Field, obj.Family = fbErr.StatusCode, fbErr.Code, fbErr.Field, string(fbErr.Family)
		if fbErr.Message != "" {
			obj.Message = fbErr.Message
		}
	}
	b, marshalErr := json.Marshal(map[string]apiErrorJSON{"error": obj})
	if marshalErr != nil {
		fmt.Fprintln(stderr, err) //nolint:errcheck // fall back to the plain line if the object itself cannot marshal
		return
	}
	fmt.Fprintln(stderr, string(b)) //nolint:errcheck
}

// errorIsJSON reproduces the -o/--output resolution (flag > env >
// TTY-sensitive default) using only args and stdout, for the top-level
// error path in Run -- which runs after root.Execute() has already
// failed, possibly before any command-specific flag parsing completed, so
// it cannot rely on runtimeState or cmd.Flags().
func errorIsJSON(args []string, stdout io.Writer) bool {
	v := scanStringFlag(args, "-o", "--output")
	if v == "" {
		v = envOutput()
	}
	if v == "" {
		return !stdoutIsTerminal(stdout)
	}
	return strings.EqualFold(v, "json")
}

// scanStringFlag looks for -short/--long value, -short=value/--long=value,
// or the pflag shorthand-glued form -svalue, returning "" if not present.
// It is a best-effort, dependency-free re-scan of raw args for the one
// case (the top-level error path) that runs before cobra's own flag
// parsing state can be trusted.
func scanStringFlag(args []string, short, long string) string {
	for i, a := range args {
		switch {
		case a == short || a == long:
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, long+"="):
			return strings.TrimPrefix(a, long+"=")
		case strings.HasPrefix(a, short) && a != short && len(short) == 2:
			return strings.TrimPrefix(a, short)
		}
	}
	return ""
}

func envOutput() string { return os.Getenv("FRESHBOOKS_OUTPUT") }
