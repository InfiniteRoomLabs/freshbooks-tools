package cmd

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	cliauth "github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/auth"
	"github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/config"
	"github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/output"
	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	libauth "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
	"github.com/spf13/cobra"
)

// runtimeState holds the CLI process's shared, resolved-once-per-run
// state: the loaded config.yaml, and the client/scope/output-formatting
// plumbing every registry command and every non-registry command
// (auth, config, api) shares. One runtimeState is created per NewRootCmd
// call, so tests building a fresh root command never share state with a
// previous test.
type runtimeState struct {
	version string

	cfg     *config.File
	cfgPath string
}

// loadConfig loads config.yaml exactly once per runtimeState, from the
// path --config/FRESHBOOKS_CONFIG/the default resolve to.
func (s *runtimeState) loadConfig(cmd *cobra.Command) (*config.File, string, error) {
	if s.cfg != nil {
		return s.cfg, s.cfgPath, nil
	}
	flagVal, _ := cmd.Flags().GetString("config")
	path := config.Resolve(flagVal, os.Getenv("FRESHBOOKS_CONFIG"), "", "")
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return nil, "", &runtimeError{err: err}
		}
	}
	f, err := config.Load(path)
	if err != nil {
		return nil, "", &runtimeError{err: err}
	}
	s.cfg, s.cfgPath = f, path
	return f, path, nil
}

// contextName resolves --context/FRESHBOOKS_CONTEXT/config.yaml's
// current-context/the "default" fallback.
func (s *runtimeState) contextName(cmd *cobra.Command) (string, error) {
	cfg, _, err := s.loadConfig(cmd)
	if err != nil {
		return "", err
	}
	flagVal, _ := cmd.Flags().GetString("context")
	return config.Resolve(flagVal, os.Getenv("FRESHBOOKS_CONTEXT"), cfg.CurrentContext, "default"), nil
}

// resolveScope resolves sf's required scope field(s) via flag > env >
// current-context > (missing), returning a usageError naming the flag
// when a required field is not resolved anywhere.
func (s *runtimeState) resolveScope(cmd *cobra.Command, sf ScopeFamily) (Scope, error) {
	var scope Scope
	if sf == ScopeNone {
		return scope, nil
	}

	cfg, _, err := s.loadConfig(cmd)
	if err != nil {
		return scope, err
	}
	ctxName, err := s.contextName(cmd)
	if err != nil {
		return scope, err
	}
	ctxData := cfg.Contexts[ctxName]

	if sf == ScopeAccount || sf == ScopeAccountAndBusiness {
		flagVal, _ := cmd.Flags().GetString("account")
		v := config.Resolve(flagVal, os.Getenv("FRESHBOOKS_ACCOUNT_ID"), ctxData.Account, "")
		if v == "" {
			return scope, newUsageErrorf("missing required scope: --account (or FRESHBOOKS_ACCOUNT_ID, or context %q's account)", ctxName)
		}
		scope.AccountID = freshbooks.AccountID(v)
	}
	if sf == ScopeBusiness || sf == ScopeAccountAndBusiness {
		flagVal, _ := cmd.Flags().GetString("business")
		v := config.Resolve(flagVal, os.Getenv("FRESHBOOKS_BUSINESS_ID"), ctxData.Business, "")
		if v == "" {
			return scope, newUsageErrorf("missing required scope: --business (or FRESHBOOKS_BUSINESS_ID, or context %q's business)", ctxName)
		}
		n, perr := strconv.ParseInt(v, 10, 64)
		if perr != nil {
			return scope, newUsageErrorf("--business must be an integer, got %q", v)
		}
		scope.BusinessID = freshbooks.BusinessID(n)
	}
	if sf == ScopeBusinessUUID {
		flagVal, _ := cmd.Flags().GetString("business-uuid")
		v := config.Resolve(flagVal, os.Getenv("FRESHBOOKS_BUSINESS_UUID"), ctxData.BusinessUUID, "")
		if v == "" {
			return scope, newUsageErrorf("missing required scope: --business-uuid (or FRESHBOOKS_BUSINESS_UUID, or context %q's business_uuid)", ctxName)
		}
		scope.BusinessUUID = freshbooks.BusinessUUID(v)
	}
	return scope, nil
}

// credentialStore resolves the current context and opens its credentials
// FileStore: the prologue auth login|status|logout|token and buildClient
// all share (D5 -- one lib FileStore per context). contextName's error is
// already typed; a CredentialsPath failure (an invalid context name --
// F2/G5/QA Q11) is a usage error (exit 2), the same as every other bad
// flag value (--output, --log-level, --sort, --timeout), not a
// runtimeError (exit 1) -- an invalid --context is exactly as much the
// caller's mistake as those.
func (s *runtimeState) credentialStore(cmd *cobra.Command) (ctxName, credPath string, store libauth.TokenStore, err error) {
	ctxName, err = s.contextName(cmd)
	if err != nil {
		return "", "", nil, err
	}
	credPath, err = cliauth.CredentialsPath(ctxName)
	if err != nil {
		return "", "", nil, newUsageErrorf("invalid --context %q: %v", ctxName, err)
	}
	return ctxName, credPath, libauth.NewFileStore(credPath), nil
}

// resolveOutputFormat resolves -o/--output/FRESHBOOKS_OUTPUT/the
// TTY-sensitive default.
func (s *runtimeState) resolveOutputFormat(cmd *cobra.Command) (output.Format, error) {
	flagVal, _ := cmd.Flags().GetString("output")
	v := config.Resolve(flagVal, os.Getenv("FRESHBOOKS_OUTPUT"), "", "")
	if v == "" {
		return output.DefaultFormat(stdoutIsTerminal(cmd.OutOrStdout())), nil
	}
	f := output.Format(v)
	if !f.Valid() {
		return "", newUsageErrorf("invalid --output %q: must be one of json, yaml, table, name", v)
	}
	return f, nil
}

// confirmWriter is where a command's confirmation chatter goes -- lines
// like "Login succeeded.", "Logged out.", or a config `use-context`/
// `set-context` confirmation, which -q/--quiet is documented to suppress
// (D7: "-q suppresses non-result chatter"). It returns io.Discard under
// --quiet and cmd's stdout otherwise. It is deliberately not used for the
// login URL prompt or the --dry-run request dump: both are results a
// caller needs to act on, not chatter.
func (s *runtimeState) confirmWriter(cmd *cobra.Command) io.Writer {
	if quiet, _ := cmd.Flags().GetBool("quiet"); quiet {
		return io.Discard
	}
	return cmd.OutOrStdout()
}

// writeResult formats result per the resolved output format and writes it
// to cmd's stdout.
func (s *runtimeState) writeResult(cmd *cobra.Command, result any) error {
	format, err := s.resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	noHeaders, _ := cmd.Flags().GetBool("no-headers")
	if err := output.Write(cmd.OutOrStdout(), result, output.Options{Format: format, NoHeaders: noHeaders}); err != nil {
		return &runtimeError{err: err}
	}
	return nil
}

// resolveTimeout resolves --timeout/FRESHBOOKS_TIMEOUT/the 30s default.
func (s *runtimeState) resolveTimeout(cmd *cobra.Command) (time.Duration, error) {
	flagVal, _ := cmd.Flags().GetDuration("timeout")
	if cmd.Flags().Changed("timeout") {
		return flagVal, nil
	}
	if raw := os.Getenv("FRESHBOOKS_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return 0, newUsageErrorf("FRESHBOOKS_TIMEOUT=%q is not a valid duration: %v", raw, err)
		}
		return d, nil
	}
	return 30 * time.Second, nil
}

// resolveBaseURL resolves --base-url/FRESHBOOKS_BASE_URL, empty meaning
// "use the lib's default".
func (s *runtimeState) resolveBaseURL(cmd *cobra.Command) string {
	flagVal, _ := cmd.Flags().GetString("base-url")
	return config.Resolve(flagVal, os.Getenv("FRESHBOOKS_BASE_URL"), "", "")
}

// resolveLogLevel resolves --log-level/FRESHBOOKS_LOG_LEVEL, defaulting
// to "warn" (the freshbooks client only ever logs at Debug, so anything
// above that is effectively silent).
func (s *runtimeState) resolveLogLevel(cmd *cobra.Command) string {
	flagVal, _ := cmd.Flags().GetString("log-level")
	return config.Resolve(flagVal, os.Getenv("FRESHBOOKS_LOG_LEVEL"), "", "warn")
}

// buildLogger builds the slog.Logger passed to freshbooks.WithLogger,
// writing to cmd's stderr at the resolved level. An unrecognized
// --log-level/FRESHBOOKS_LOG_LEVEL value is a usage error (exit 2)
// (F14/review A2), not a silent fall-back to warn.
func (s *runtimeState) buildLogger(cmd *cobra.Command) (*slog.Logger, error) {
	raw := s.resolveLogLevel(cmd)
	var level slog.Level
	switch raw {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, newUsageErrorf("invalid --log-level %q: want debug, info, warn, or error", raw)
	}
	return slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: level})), nil
}

// clientIDCredentials resolves FRESHBOOKS_CLIENT_ID/FRESHBOOKS_CLIENT_SECRET
// from the environment for the registry commands' token-refresh path.
// Registry commands never take --client-id/--client-secret flags
// themselves (D5: those exist only on auth login|token|logout); if a
// refresh is needed and these are empty, the token endpoint's own
// invalid_client response surfaces as an ordinary auth-family API error.
func clientIDCredentials() (string, string) {
	return os.Getenv("FRESHBOOKS_CLIENT_ID"), os.Getenv("FRESHBOOKS_CLIENT_SECRET")
}

// buildClient builds the *freshbooks.Client a registry command or the api
// command calls into: the real thing backed by the context's stored
// credentials, or (under --dry-run) a client whose transport prints the
// request and sends nothing (D4).
func (s *runtimeState) buildClient(cmd *cobra.Command) (*freshbooks.Client, error) {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	timeout, err := s.resolveTimeout(cmd)
	if err != nil {
		return nil, err
	}
	baseURL := s.resolveBaseURL(cmd)

	// G1/QA Q1: validate --log-level before the dry-run and credential
	// branches, both of which return before buildLogger's own validation
	// ever ran -- an unrecognized level must be exit 2 on every path
	// (dry-run, no credentials, credentials alike), not 0 or 3.
	logger, err := s.buildLogger(cmd)
	if err != nil {
		return nil, err
	}

	if dryRun {
		return s.buildDryRunClient(cmd, timeout, baseURL)
	}

	ctxName, _, store, err := s.credentialStore(cmd)
	if err != nil {
		return nil, err
	}

	// Fail fast with an auth error (exit 3) if nothing is stored at all,
	// rather than letting the first API call surface an opaque error deep
	// inside the lib.
	if _, err := store.Load(cmd.Context()); err != nil {
		if errors.Is(err, libauth.ErrNoToken) {
			return nil, newAuthErrorf("no credentials for context %q; run 'freshbooks auth login'", ctxName)
		}
		return nil, &runtimeError{err: err}
	}

	clientID, clientSecret := clientIDCredentials()
	tokenSource := libauth.NewTokenSource(libauth.Config{ClientID: clientID, ClientSecret: clientSecret}, store)

	opts := []freshbooks.Option{
		freshbooks.WithTokenSource(tokenSource),
		freshbooks.WithHTTPClient(&http.Client{Transport: httpTransport(), Timeout: timeout}),
		freshbooks.WithLogger(logger),
	}
	if baseURL != "" {
		opts = append(opts, freshbooks.WithBaseURL(baseURL))
	}
	client, err := freshbooks.NewClient(opts...)
	if err != nil {
		return nil, &runtimeError{err: err}
	}
	return client, nil
}

// testTransport, when non-nil, overrides the HTTP transport buildClient's
// non-dry-run path uses. It is only ever set by this package's own
// round-trip test (roundtrip_test.go) to redirect every request --
// including the two payment_options tokenization calls that bypass
// WithBaseURL entirely via the lib's doOnHost -- onto a local fixture
// server; no flag or environment variable can reach it, so production
// behavior is unaffected.
var testTransport http.RoundTripper

func httpTransport() http.RoundTripper {
	if testTransport != nil {
		return testTransport
	}
	return http.DefaultTransport
}

// errDryRun is the sentinel dryRunTransport.RoundTrip returns, so no
// request ever actually reaches the network. Command.execute maps any
// error wrapping it to exit 0 with nothing further printed.
var errDryRun = errors.New("cli: dry run -- no request sent")

// isDryRun reports whether err is (or wraps) errDryRun. The lib's
// transport wraps a RoundTrip failure in *url.Error and then unwraps and
// re-wraps that with %w (freshbooks/transport.go's roundTrip), so a plain
// errors.Is finds it however many layers deep it ends up.
func isDryRun(err error) bool {
	return errors.Is(err, errDryRun)
}

// dryRunTransport implements http.RoundTripper: for every request it
// prints "METHOD URL" and the request body (never a header, so the
// Authorization bearer token can never reach this path) to out, then
// fails the request with errDryRun before anything reaches the network.
type dryRunTransport struct{ out io.Writer }

func (t dryRunTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body) //nolint:errcheck // best-effort: printed for the operator, not decoded
	}
	fmt.Fprintf(t.out, "%s %s\n", req.Method, req.URL.String()) //nolint:errcheck // best-effort progress output; a write failure here has nothing more useful to do
	if len(body) > 0 {
		_, _ = t.out.Write(body) // #nosec G104 -- best-effort progress output, same as above
		fmt.Fprintln(t.out)      //nolint:errcheck // best-effort progress output
	}
	return nil, errDryRun
}

// buildDryRunClient builds a client whose token source is a static
// placeholder that never refreshes (D4: "a dry run needs a token source
// but must never refresh") and whose transport is dryRunTransport with
// retries disabled -- without NoRetry, the lib's default retry policy
// would treat dryRunTransport's error as a transient transport failure
// and print the same request up to three times with real backoff delays
// between them.
func (s *runtimeState) buildDryRunClient(cmd *cobra.Command, timeout time.Duration, baseURL string) (*freshbooks.Client, error) {
	opts := []freshbooks.Option{
		freshbooks.WithTokenSource(libauth.StaticTokenSource("dry-run-placeholder")),
		freshbooks.WithHTTPClient(&http.Client{Transport: dryRunTransport{out: cmd.OutOrStdout()}, Timeout: timeout}),
		freshbooks.WithRetry(freshbooks.NoRetry),
	}
	if baseURL != "" {
		opts = append(opts, freshbooks.WithBaseURL(baseURL))
	}
	client, err := freshbooks.NewClient(opts...)
	if err != nil {
		return nil, &runtimeError{err: err}
	}
	return client, nil
}

// writeBinaryResult writes a Binary command's []byte result to the local
// -o/--output path (or stdout for "-"/unset). F18/review A8: "-" on a TTY
// stdout is refused (binary bytes would corrupt the terminal), and an
// existing file at path is never silently overwritten -- --force is
// required.
func writeBinaryResult(cmd *cobra.Command, result any, path string, force bool) error {
	b, ok := result.([]byte)
	if !ok {
		return &runtimeError{err: fmt.Errorf("internal error: binary command returned %T, want []byte", result)}
	}
	if path == "" || path == "-" {
		if stdoutIsTerminal(cmd.OutOrStdout()) {
			return newUsageError("binary output would corrupt your terminal; redirect it or pass -o <file>")
		}
		if _, err := cmd.OutOrStdout().Write(b); err != nil {
			return &runtimeError{err: err}
		}
		return nil
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return newUsageErrorf("%s already exists; pass --force to overwrite it", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return &runtimeError{err: fmt.Errorf("checking %s: %w", path, err)}
		}
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return &runtimeError{err: fmt.Errorf("writing %s: %w", path, err)}
	}
	return nil
}

// stdoutIsTerminal and stdinIsTerminal are package vars, not plain
// functions, so tests can inject a fixed answer without needing a real
// pty: TestMain-less unit tests exercise the --yes gate and the output
// format's TTY-sensitive default by swapping these out for the duration
// of one test.
var (
	stdoutIsTerminal = func(w io.Writer) bool { return isTerminalIO(w) }
	stdinIsTerminal  = func(r io.Reader) bool { return isTerminalIO(r) }
)

func isTerminalIO(v any) bool {
	f, ok := v.(*os.File)
	if !ok {
		return false
	}
	return isTerminalFile(f)
}
