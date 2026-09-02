package cmd

import (
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
func newAuthCmd(state *runtimeState) *cobra.Command {
	root := &cobra.Command{Use: "auth", Short: "Log in, check status, log out, or print the access token"}
	root.AddCommand(newAuthLoginCmd(state))
	root.AddCommand(newAuthStatusCmd(state))
	root.AddCommand(newAuthLogoutCmd(state))
	root.AddCommand(newAuthTokenCmd(state))
	return root
}

// clientCredentialsFlags registers --client-id/--client-secret, the one
// place these flags exist (D5): everywhere else FRESHBOOKS_CLIENT_ID/
// FRESHBOOKS_CLIENT_SECRET env vars are the only source.
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
			ctxName, err := state.contextName(cmd)
			if err != nil {
				return err
			}
			credPath, err := cliauth.CredentialsPath(ctxName)
			if err != nil {
				return &runtimeError{err: err}
			}
			store := libauth.NewFileStore(credPath)

			opts := cliauth.LoginOptions{
				ClientID: id, ClientSecret: secret,
				Port: port, Timeout: timeout, Store: store,
				Stdout: cmd.OutOrStdout(),
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
				return &runtimeError{err: err}
			}
			return nil
		},
	}
	clientID, clientSecret = clientCredentialsFlags(cc)
	cc.Flags().StringArrayVar(&scopes, "scopes", nil, "OAuth scopes to request (default: the full documented user:*:read/write set)")
	cc.Flags().IntVar(&port, "callback-port", cliauth.DefaultPort, "loopback port for the browser callback")
	cc.Flags().BoolVar(&noBrowser, "no-browser", false, "print the URL and read the redirect (or a bare code) from stdin instead of opening a browser")
	cc.Flags().DurationVar(&timeout, "timeout", cliauth.DefaultLoginTimeout, "how long to wait for the browser callback")
	return cc
}

func newAuthStatusCmd(state *runtimeState) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current context's credential state (never the token)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctxName, err := state.contextName(cmd)
			if err != nil {
				return err
			}
			credPath, err := cliauth.CredentialsPath(ctxName)
			if err != nil {
				return &runtimeError{err: err}
			}
			store := libauth.NewFileStore(credPath)
			info, err := cliauth.Status(cmd.Context(), ctxName, credPath, store, nil)
			if err != nil {
				return &runtimeError{err: err}
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
			ctxName, err := state.contextName(cmd)
			if err != nil {
				return err
			}
			credPath, err := cliauth.CredentialsPath(ctxName)
			if err != nil {
				return &runtimeError{err: err}
			}
			store := libauth.NewFileStore(credPath)
			cfg := libauth.Config{ClientID: id, ClientSecret: secret}
			if err := cliauth.Logout(cmd.Context(), cfg, credPath, store); err != nil {
				return &runtimeError{err: err}
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
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
			ctxName, err := state.contextName(cmd)
			if err != nil {
				return err
			}
			credPath, err := cliauth.CredentialsPath(ctxName)
			if err != nil {
				return &runtimeError{err: err}
			}
			store := libauth.NewFileStore(credPath)
			cfg := libauth.Config{ClientID: id, ClientSecret: secret}
			tok, err := cliauth.Token(cmd.Context(), cfg, store, refresh)
			if err != nil {
				return &runtimeError{err: err}
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), tok)
			return err
		},
	}
	clientID, clientSecret = clientCredentialsFlags(cc)
	cc.Flags().BoolVar(&refresh, "refresh", false, "force a refresh, rotating and persisting the token pair before printing")
	return cc
}
