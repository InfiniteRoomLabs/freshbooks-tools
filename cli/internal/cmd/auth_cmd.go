package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	cliauth "github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/auth"
	libauth "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
	"github.com/spf13/cobra"
)

// newAuthCmd builds `auth login|status|logout|token`. Every subcommand
// resolves its own context/credentials path from state, independent of
// the registry commands' client-building path, since these commands
// manage the credentials the other path reads.
//
// --dry-run is rejected here (F3/security B3): the flag's contract is
// "send nothing", and every one of these subcommands either sends a real
// request (login exchanges a code; token --refresh rotates a one-time-use
// refresh token; logout revokes) or reports/mutates local state in a way
// a dry run cannot meaningfully preview. Silently ignoring --dry-run on
// `auth logout` in particular would mean the one flag a cautious operator
// reaches for before a destructive command does nothing to stop it.
func newAuthCmd(state *runtimeState) *cobra.Command {
	root := &cobra.Command{
		Use:   "auth",
		Short: "Log in, check status, log out, or print the access token",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return rejectDryRun(cmd)
		},
	}
	root.AddCommand(newAuthLoginCmd(state))
	root.AddCommand(newAuthStatusCmd(state))
	root.AddCommand(newAuthLogoutCmd(state))
	root.AddCommand(newAuthTokenCmd(state))
	return root
}

// rejectDryRun returns a usage error (exit 2) if --dry-run was passed,
// for command families that cannot honour it.
func rejectDryRun(cmd *cobra.Command) error {
	if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
		return newUsageErrorf("%s does not support --dry-run", cmd.CommandPath())
	}
	return nil
}

// clientCredentialsFlags registers --client-id/--client-secret, the one
// place these flags exist (D5): everywhere else FRESHBOOKS_CLIENT_ID/
// FRESHBOOKS_CLIENT_SECRET env vars are the only source. Prefer the env
// vars over these flags where possible: a value passed on the command
// line is visible to any other process of the same user via
// /proc/<pid>/cmdline while this command runs, and lands in shell history
// afterwards (docs/cli.md's Security notes say the same).
func clientCredentialsFlags(cc *cobra.Command) (clientID, clientSecret *string) {
	clientID = cc.Flags().String("client-id", "", "the registered application's client id (default: FRESHBOOKS_CLIENT_ID)")
	clientSecret = cc.Flags().String("client-secret", "", "the registered application's client secret (default: FRESHBOOKS_CLIENT_SECRET)")
	return
}

func resolveClientCredentials(clientID, clientSecret string) (string, string) {
	if clientID == "" {
		clientID = os.Getenv("FRESHBOOKS_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("FRESHBOOKS_CLIENT_SECRET")
	}
	return clientID, clientSecret
}

// testAuthEndpoints, when non-zero, overrides the OAuth endpoint set
// every auth subcommand's libauth.Config uses. It is only ever set by
// this package's own tests (auth_cmd_test.go), redirecting the exchange/
// refresh/revoke calls onto a local fixture server so no test can reach
// the real internet; no flag or environment variable can reach it. Q22
// (Phase 4 QA): safe as a shared mutable package var only because no
// test in this package calls t.Parallel().
var testAuthEndpoints libauth.Endpoints

func authEndpoints() libauth.Endpoints { return testAuthEndpoints }

// classifyAuthError maps an error from cliauth.Token/Status/Logout the
// same way state.go's buildClient maps a missing credentials file: a
// store.Load that failed because nothing is stored at all is an auth
// error (exit 3, D6), not a generic runtime error (exit 1) -- the
// primary automation idiom is `TOKEN=$(freshbooks auth token) ||
// handle $?`, and it needs to be able to tell "not logged in" apart from
// "something broke." Everything else goes through classifyRunError,
// which already leaves a *freshbooks/auth.Error (an OAuth token endpoint
// failure, e.g. a revoked refresh token) alone for exitCodeFor to map by
// status code.
func classifyAuthError(err error, ctxName string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, libauth.ErrNoToken) {
		return newAuthErrorf("no credentials for context %q; run 'freshbooks auth login'", ctxName)
	}
	return classifyRunError(err)
}

func newAuthLoginCmd(state *runtimeState) *cobra.Command {
	var scopes []string
	var port int
	var noBrowser bool
	var timeout time.Duration
	var clientID, clientSecret *string

	cc := &cobra.Command{
		Use:   "login",
		Short: "Authorize the CLI against FreshBooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, secret := resolveClientCredentials(*clientID, *clientSecret)
			if id == "" || secret == "" {
				return newUsageError("--client-id/--client-secret (or FRESHBOOKS_CLIENT_ID/FRESHBOOKS_CLIENT_SECRET) are required")
			}
			ctxName, _, store, err := state.credentialStore(cmd)
			if err != nil {
				return err
			}

			opts := cliauth.LoginOptions{
				ClientID: id, ClientSecret: secret,
				Port: port, Timeout: timeout, Store: store,
				Stdout: cmd.OutOrStdout(), Endpoints: authEndpoints(),
			}
			if len(scopes) > 0 {
				opts.Scopes = scopes
			}

			if noBrowser {
				_, err = cliauth.LoginNoBrowser(cmd.Context(), opts, cmd.InOrStdin())
			} else {
				_, err = cliauth.Login(cmd.Context(), opts)
			}
			if err != nil {
				return classifyAuthError(err, ctxName)
			}
			_, err = fmt.Fprintln(state.confirmWriter(cmd), "Login succeeded.")
			return err
		},
	}
	clientID, clientSecret = clientCredentialsFlags(cc)
	cc.Flags().StringArrayVar(&scopes, "scopes", nil, "OAuth scopes to request (default: the full documented user:*:read/write set)")
	cc.Flags().IntVar(&port, "callback-port", cliauth.DefaultPort, "loopback port for the browser callback")
	cc.Flags().BoolVar(&noBrowser, "no-browser", false, "print the URL and read the redirect (or a bare code) from stdin instead of opening a browser")
	// Q12 (Phase 4 QA): named "login-timeout", not "timeout", so it cannot
	// be confused with (and does not shadow) the global --timeout, which
	// is the per-request timeout every other command shares.
	cc.Flags().DurationVar(&timeout, "login-timeout", cliauth.DefaultLoginTimeout, "how long to wait for the browser callback")
	return cc
}

func newAuthStatusCmd(state *runtimeState) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current context's credential state (never the token)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctxName, credPath, store, err := state.credentialStore(cmd)
			if err != nil {
				return err
			}
			info, err := cliauth.Status(cmd.Context(), ctxName, credPath, store, nil)
			if err != nil {
				return classifyAuthError(err, ctxName)
			}
			return state.writeResult(cmd, info)
		},
	}
}

func newAuthLogoutCmd(state *runtimeState) *cobra.Command {
	var clientID, clientSecret *string
	cc := &cobra.Command{
		Use:   "logout",
		Short: "Revoke and remove the current context's credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, secret := resolveClientCredentials(*clientID, *clientSecret)
			ctxName, credPath, store, err := state.credentialStore(cmd)
			if err != nil {
				return err
			}
			cfg := libauth.Config{ClientID: id, ClientSecret: secret, Endpoints: authEndpoints()}
			if err := cliauth.Logout(cmd.Context(), cfg, credPath, store); err != nil {
				return classifyAuthError(err, ctxName)
			}
			_, err = fmt.Fprintln(state.confirmWriter(cmd), "Logged out.")
			return err
		},
	}
	clientID, clientSecret = clientCredentialsFlags(cc)
	return cc
}

func newAuthTokenCmd(state *runtimeState) *cobra.Command {
	var refresh bool
	var clientID, clientSecret *string
	cc := &cobra.Command{
		Use:   "token",
		Short: "Print the current context's access token (the one place this CLI ever prints one)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, secret := resolveClientCredentials(*clientID, *clientSecret)
			ctxName, _, store, err := state.credentialStore(cmd)
			if err != nil {
				return err
			}
			cfg := libauth.Config{ClientID: id, ClientSecret: secret, Endpoints: authEndpoints()}
			tok, err := cliauth.Token(cmd.Context(), cfg, store, refresh)
			if err != nil {
				return classifyAuthError(err, ctxName)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), tok)
			return err
		},
	}
	clientID, clientSecret = clientCredentialsFlags(cc)
	cc.Flags().BoolVar(&refresh, "refresh", false, "force a refresh, rotating and persisting the token pair before printing")
	return cc
}
