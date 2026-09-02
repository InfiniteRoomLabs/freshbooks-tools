// Package auth implements the freshbooks CLI's login flow on top of the
// freshbooks library's auth package: a loopback PKCE authorization-code
// exchange over an ephemeral self-signed TLS certificate, a --no-browser
// paste fallback, and the status/logout/token commands' shared plumbing.
// Credentials live in a lib auth.FileStore, one JSON file per context (see
// paths.go); config.yaml never carries a secret.
package auth

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	fbauth "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// DefaultPort is the loopback port `auth login` listens on when --port is
// not given, matching the app's registered redirect URI
// https://localhost:8765/callback.
const DefaultPort = 8765

// DefaultLoginTimeout bounds how long the loopback listener waits for the
// browser to complete the redirect before Login gives up and the listener
// closes.
const DefaultLoginTimeout = 5 * time.Minute

// certValidity is how long the ephemeral loopback TLS certificate is
// valid: long enough to outlast DefaultLoginTimeout and any custom
// --timeout a caller passes, short enough that it is obviously a
// throwaway if anyone ever inspected it (which it cannot be: it is
// generated in-process and never written to disk).
const certValidity = time.Hour

// NewState returns a fresh, unguessable OAuth state value: 32 bytes from
// crypto/rand, base64url-encoded without padding.
func NewState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: generating state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// LoginOptions configures Login and LoginNoBrowser.
type LoginOptions struct {
	// ClientID and ClientSecret are the registered application's OAuth
	// credentials.
	ClientID, ClientSecret string
	// Scopes are the scopes to request. Nil means DefaultScopes.
	Scopes []string
	// Port is the loopback port for the browser flow. Zero means
	// DefaultPort. Unused by LoginNoBrowser.
	Port int
	// Timeout bounds how long the browser flow waits for the callback.
	// Zero means DefaultLoginTimeout. Unused by LoginNoBrowser.
	Timeout time.Duration
	// Endpoints selects the OAuth endpoint set. Zero means the lib
	// default (MetadataEndpoints).
	Endpoints fbauth.Endpoints
	// HTTPClient performs the token exchange. Nil means the lib default.
	HTTPClient *http.Client
	// Store persists the resulting token. Required for the token to be
	// saved; a nil Store means "exchange only, do not persist" (used by
	// tests that want the token without a filesystem side effect).
	Store fbauth.TokenStore
	// Stdout receives the authorization URL and progress messages. Nil
	// means io.Discard.
	Stdout io.Writer
	// OpenBrowser opens url in the user's browser. Nil means the
	// platform-appropriate program (xdg-open/open/rundll32). Tests
	// inject a no-op here so a live browser never launches.
	OpenBrowser func(rawURL string) error
	// Now supplies the current time, for deterministic certificate
	// validity windows in tests. Nil means time.Now.
	Now func() time.Time
}

func (o LoginOptions) port() int {
	if o.Port > 0 {
		return o.Port
	}
	return DefaultPort
}

func (o LoginOptions) timeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return DefaultLoginTimeout
}

func (o LoginOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o LoginOptions) stdout() io.Writer {
	if o.Stdout != nil {
		return o.Stdout
	}
	return io.Discard
}

func (o LoginOptions) scopes() []string {
	if o.Scopes != nil {
		return o.Scopes
	}
	return DefaultScopes
}

func (o LoginOptions) config(redirectURL string) fbauth.Config {
	return fbauth.Config{
		ClientID:     o.ClientID,
		ClientSecret: o.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       o.scopes(),
		Endpoints:    o.Endpoints,
		HTTPClient:   o.HTTPClient,
		Now:          o.Now,
	}
}

// Login runs the full loopback PKCE flow: it starts an HTTPS listener on
// 127.0.0.1:port with an ephemeral self-signed certificate, prints the
// authorization URL, opens the browser, waits for the single callback
// request, validates state, exchanges the code, and (when Store is set)
// persists the token. It returns an error without persisting anything on
// a state mismatch, a browser-reported denial, or a timeout.
func Login(ctx context.Context, o LoginOptions) (*fbauth.Token, error) {
	cfg := o.config(fmt.Sprintf("https://localhost:%d/callback", o.port()))

	state, err := NewState()
	if err != nil {
		return nil, err
	}
	authURL, verifier, err := cfg.AuthCodeURL(state)
	if err != nil {
		return nil, err
	}

	resultCh, stop, err := runCallbackServer(o.port(), o.now())
	if err != nil {
		return nil, err
	}
	defer func() { _ = stop() }() //nolint:errcheck // best-effort shutdown; the process is returning either way

	fmt.Fprintf(o.stdout(), "Open this URL to authorize the CLI:\n\n  %s\n\nWaiting for the browser callback (the browser will warn about a self-signed certificate on localhost -- that is expected; accept it to continue)...\n", authURL) //nolint:errcheck // best-effort progress output

	open := o.OpenBrowser
	if open == nil {
		open = openBrowser
	}
	_ = open(authURL) // best-effort: the printed URL above is the fallback if this fails or there is no display

	timer := time.NewTimer(o.timeout())
	defer timer.Stop()

	var result callbackResult
	select {
	case result = <-resultCh:
	case <-timer.C:
		return nil, errors.New("auth: timed out waiting for the browser callback")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	tok, err := finishExchange(ctx, cfg, verifier, state, result.state, result.code, result.err)
	if err != nil {
		return nil, err
	}
	if o.Store != nil {
		if err := o.Store.Save(ctx, tok); err != nil {
			return nil, fmt.Errorf("auth: saving the token: %w", err)
		}
	}
	fmt.Fprintln(o.stdout(), "Login succeeded.") //nolint:errcheck
	return tok, nil
}

// LoginNoBrowser prints the authorization URL and reads one line from
// stdin: either the full URL the browser was redirected to (state is
// validated against it) or a bare authorization code (no listener runs,
// so there is nothing to validate state against).
func LoginNoBrowser(ctx context.Context, o LoginOptions, stdin io.Reader) (*fbauth.Token, error) {
	cfg := o.config(fmt.Sprintf("https://localhost:%d/callback", o.port()))

	state, err := NewState()
	if err != nil {
		return nil, err
	}
	authURL, verifier, err := cfg.AuthCodeURL(state)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(o.stdout(), "Open this URL in a browser:\n\n  %s\n\nThen paste the full redirected URL (or just the code) here:\n", authURL) //nolint:errcheck

	line, err := readLine(stdin)
	if err != nil {
		return nil, fmt.Errorf("auth: reading the pasted callback: %w", err)
	}
	code, pastedState := parsePastedInput(line)

	tok, err := finishExchange(ctx, cfg, verifier, state, pastedState, code, nil)
	if err != nil {
		return nil, err
	}
	if o.Store != nil {
		if err := o.Store.Save(ctx, tok); err != nil {
			return nil, fmt.Errorf("auth: saving the token: %w", err)
		}
	}
	fmt.Fprintln(o.stdout(), "Login succeeded.") //nolint:errcheck
	return tok, nil
}

// finishExchange validates the callback (or pasted) state and code, then
// exchanges. gotState == "" (the bare-code paste path) skips state
// validation entirely, since there is nothing to compare against.
func finishExchange(ctx context.Context, cfg fbauth.Config, verifier, wantState, gotState, code string, callbackErr error) (*fbauth.Token, error) {
	if callbackErr != nil {
		return nil, callbackErr
	}
	if gotState != "" && gotState != wantState {
		return nil, errors.New("auth: state mismatch on the callback; not proceeding")
	}
	if code == "" {
		return nil, errors.New("auth: no authorization code was provided")
	}
	tok, err := cfg.Exchange(ctx, code, verifier)
	if err != nil {
		return nil, err
	}
	return tok, nil
}

// parsePastedInput extracts an authorization code and state from a pasted
// line: a full URL with a code query parameter yields both; anything else
// is treated as a bare code with no state to validate.
func parsePastedInput(s string) (code, state string) {
	s = strings.TrimSpace(s)
	if u, err := url.Parse(s); err == nil && u.Query().Get("code") != "" {
		return u.Query().Get("code"), u.Query().Get("state")
	}
	return s, ""
}

func readLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return scanner.Text(), nil
}

// callbackResult is what the loopback listener's single /callback request
// carries back to Login.
type callbackResult struct {
	code, state string
	err         error
}

// runCallbackServer starts a one-shot HTTPS server on 127.0.0.1:port,
// generates its own ephemeral self-signed certificate, serves exactly one
// request to /callback, and returns a channel that receives that request's
// outcome plus a stop function the caller must call to shut the listener
// down (idempotent, safe to call even if no request ever arrived).
func runCallbackServer(port int, now time.Time) (<-chan callbackResult, func() error, error) {
	cert, err := selfSignedCert([]string{"localhost", "127.0.0.1"}, now, certValidity)
	if err != nil {
		return nil, nil, err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return nil, nil, fmt.Errorf("auth: starting the loopback listener on port %d: %w", port, err)
	}
	tlsLn := tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})

	resultCh := make(chan callbackResult, 1)
	var once sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		res := callbackResult{code: q.Get("code"), state: q.Get("state")}
		if errStr := q.Get("error"); errStr != "" {
			desc := q.Get("error_description")
			if desc == "" {
				desc = errStr
			}
			res.err = fmt.Errorf("auth: authorization was not granted: %s", desc)
		}
		writeCallbackPage(w, res.err)
		once.Do(func() { resultCh <- res })
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(tlsLn) }()

	stop := func() error {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
	return resultCh, stop, nil
}

// writeCallbackPage renders the page the browser shows after the
// redirect: success tells the user the self-signed certificate warning
// was expected and the tab can be closed; failure names what went wrong.
func writeCallbackPage(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "<!doctype html><title>freshbooks CLI</title><body><h1>Login failed</h1><p>%s</p><p>You can close this tab and return to the terminal.</p></body>", html.EscapeString(err.Error())) //nolint:errcheck
		return
	}
	fmt.Fprint(w, "<!doctype html><title>freshbooks CLI</title><body><h1>Login succeeded</h1><p>The certificate warning your browser showed for this page is expected: the CLI generates a throwaway, self-signed certificate for this one-time localhost callback.</p><p>You can close this tab and return to the terminal.</p></body>") //nolint:errcheck
}

// selfSignedCert builds an ephemeral ECDSA P-256 certificate for hosts,
// valid from just before now through now+validity, generated entirely
// in-process and never written to disk.
func selfSignedCert(hosts []string, now time.Time, validity time.Duration) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("auth: generating the loopback TLS key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("auth: generating a certificate serial: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "freshbooks CLI loopback"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("auth: creating the loopback TLS certificate: %w", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
}

// openBrowser launches url in the platform's default browser via a fixed
// program per OS, passing the URL as an argument -- never through a
// shell, so it cannot be reinterpreted.
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL) // #nosec G204 -- the program is a fixed literal; rawURL is the CLI's own generated authorization URL, passed as one argv element, never through a shell
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL) // #nosec G204 -- same as above
	default:
		cmd = exec.Command("xdg-open", rawURL) // #nosec G204 -- same as above
	}
	return cmd.Start()
}
