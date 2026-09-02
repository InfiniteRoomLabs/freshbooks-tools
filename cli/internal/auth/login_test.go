package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	fbauth "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// syncBuffer is an io.Writer safe for one writer goroutine and one reader
// goroutine to share, which strings.Builder and bytes.Buffer are not:
// LoginNoBrowser writes progress text from its own goroutine in these
// tests while waitForState polls the buffer from the test goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// freePort asks the OS for an unused loopback TCP port. There is a small
// race between closing the probe listener and Login binding the same
// port, but it is the standard Go testing pattern for this and is not
// flaky in practice for a single test process.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

// fakeOAuthServer answers a token endpoint, asserting the request carries
// the expected code_verifier, client_id, client_secret, redirect_uri, and
// grant_type, and a revoke endpoint that always succeeds.
type fakeOAuthServer struct {
	srv *httptest.Server

	mu       sync.Mutex
	lastForm url.Values

	// revoked collects the "token" form value of every /revoke request,
	// in order (F15/review A3 -- auth logout revokes both the access and
	// refresh token, and this is how the test proves both were posted).
	revoked []string
}

func newFakeOAuthServer(t *testing.T, wantClientID, wantClientSecret, wantRedirectURI string) *fakeOAuthServer {
	t.Helper()
	f := &fakeOAuthServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.lastForm = r.Form
		f.mu.Unlock()

		switch r.Form.Get("grant_type") {
		case "authorization_code":
			if r.Form.Get("code_verifier") == "" {
				http.Error(w, `{"error":"invalid_request","error_description":"missing code_verifier"}`, http.StatusBadRequest)
				return
			}
			if r.Form.Get("client_id") != wantClientID || r.Form.Get("client_secret") != wantClientSecret {
				http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
				return
			}
			if wantRedirectURI != "" && r.Form.Get("redirect_uri") != wantRedirectURI {
				http.Error(w, `{"error":"invalid_grant","error_description":"redirect_uri mismatch"}`, http.StatusBadRequest)
				return
			}
			if r.Form.Get("code") == "" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			fmt.Fprintf(w, `{"access_token":"fixture-access-token","refresh_token":"fixture-refresh-token","token_type":"Bearer","expires_in":43200,"created_at":%d,"scope":"user:clients:read"}`, time.Now().Unix()) //nolint:errcheck
		case "refresh_token":
			if r.Form.Get("refresh_token") == "" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			fmt.Fprintf(w, `{"access_token":"rotated-access-token","refresh_token":"rotated-refresh-token","token_type":"Bearer","expires_in":43200,"created_at":%d}`, time.Now().Unix()) //nolint:errcheck
		default:
			http.Error(w, `{"error":"unsupported_grant_type"}`, http.StatusBadRequest)
		}
	})
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			f.mu.Lock()
			f.revoked = append(f.revoked, r.Form.Get("token"))
			f.mu.Unlock()
		}
		fmt.Fprint(w, `{}`) //nolint:errcheck
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// revokedTokens returns a snapshot of every "token" value posted to
// /revoke so far, in order.
func (f *fakeOAuthServer) revokedTokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.revoked...)
}

func (f *fakeOAuthServer) endpoints() fbauth.Endpoints {
	return fbauth.Endpoints{
		AuthURL:   f.srv.URL + "/authorize", // never actually fetched by these tests
		TokenURL:  f.srv.URL + "/token",
		RevokeURL: f.srv.URL + "/revoke",
	}
}

// insecureBrowserClient stands in for a real browser hitting the
// loopback HTTPS listener: it must skip certificate verification because
// the listener's certificate is a throwaway generated for this one login,
// exactly as a real browser's "proceed anyway" click does.
var insecureBrowserClient = &http.Client{
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, // #nosec G402 -- deliberately trusting our own ephemeral test cert, mirroring what a real browser's click-through does
	Timeout:   5 * time.Second,
}

func stateFromAuthURL(t *testing.T, authURL string) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parsing authURL: %v", err)
	}
	return u.Query().Get("state")
}

func TestLoginOptionsDefaults(t *testing.T) {
	var o LoginOptions
	if got := o.port(); got != DefaultPort {
		t.Errorf("port() = %d, want %d", got, DefaultPort)
	}
	if got := o.timeout(); got != DefaultLoginTimeout {
		t.Errorf("timeout() = %v, want %v", got, DefaultLoginTimeout)
	}
	if got := o.now(); time.Since(got) > time.Minute {
		t.Errorf("now() = %v, want close to time.Now()", got)
	}
	if o.stdout() == nil {
		t.Error("stdout() = nil, want io.Discard")
	}
	if got := o.scopes(); len(got) != len(DefaultScopes) {
		t.Errorf("scopes() returned %d scopes, want DefaultScopes' %d", len(got), len(DefaultScopes))
	}

	custom := LoginOptions{Port: 9999, Timeout: 42 * time.Second, Scopes: []string{"one"}}
	if got := custom.port(); got != 9999 {
		t.Errorf("port() = %d, want 9999", got)
	}
	if got := custom.timeout(); got != 42*time.Second {
		t.Errorf("timeout() = %v, want 42s", got)
	}
	if got := custom.scopes(); len(got) != 1 || got[0] != "one" {
		t.Errorf("scopes() = %v, want [one]", got)
	}
}

func TestReadLine(t *testing.T) {
	t.Run("[happy] a line with a trailing newline", func(t *testing.T) {
		got, err := readLine(strings.NewReader("hello\n"))
		if err != nil {
			t.Fatalf("readLine() error = %v", err)
		}
		if got != "hello" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("[sad] an empty reader is EOF", func(t *testing.T) {
		if _, err := readLine(strings.NewReader("")); err == nil {
			t.Fatal("readLine() error = nil, want EOF")
		}
	})
}

func TestBrowserCommand(t *testing.T) {
	// browserCommand is pure: no process is ever started here, so this
	// runs on every GOOS regardless of which platform the test binary
	// actually is.
	tests := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{"darwin", "open", []string{"https://example.invalid/probe"}},
		{"windows", "rundll32", []string{"url.dll,FileProtocolHandler", "https://example.invalid/probe"}},
		{"linux", "xdg-open", []string{"https://example.invalid/probe"}},
		{"freebsd", "xdg-open", []string{"https://example.invalid/probe"}}, // the default branch, exercised under an unlisted GOOS
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			name, args := browserCommand(tt.goos, "https://example.invalid/probe")
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("args = %v, want %v", args, tt.wantArgs)
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, args[i], tt.wantArgs[i])
				}
			}
			// The URL is always the last argument, alone.
			if args[len(args)-1] != "https://example.invalid/probe" {
				t.Errorf("last arg = %q, want the raw URL", args[len(args)-1])
			}
		})
	}
}

func TestLogin(t *testing.T) {
	t.Run("[happy] a successful browser round trip exchanges and saves the token", func(t *testing.T) {
		port := freePort(t)
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", fmt.Sprintf("https://localhost:%d/callback", port))
		store := fbauth.NewMemoryStore()

		var authURL string
		opts := LoginOptions{
			ClientID:     "client-1",
			ClientSecret: "secret-1",
			Port:         port,
			Timeout:      3 * time.Second,
			Endpoints:    oauth.endpoints(),
			Store:        store,
			OpenBrowser: func(rawURL string) error {
				authURL = rawURL
				state := stateFromAuthURL(t, rawURL)
				go func() {
					resp, err := insecureBrowserClient.Get(fmt.Sprintf("https://127.0.0.1:%d/callback?code=test-auth-code&state=%s", port, state))
					if err == nil {
						_ = resp.Body.Close()
					}
				}()
				return nil
			},
		}

		tok, err := Login(context.Background(), opts)
		if err != nil {
			t.Fatalf("Login() error = %v", err)
		}
		if tok.AccessToken != "fixture-access-token" {
			t.Errorf("AccessToken = %q", tok.AccessToken)
		}
		if authURL == "" {
			t.Fatal("OpenBrowser was never called")
		}

		saved, err := store.Load(context.Background())
		if err != nil {
			t.Fatalf("store.Load() error = %v", err)
		}
		if saved.AccessToken != tok.AccessToken {
			t.Errorf("stored token does not match the returned token")
		}
	})

	t.Run("[happy] the callback response carries Cache-Control and Referrer-Policy (F26/security A8)", func(t *testing.T) {
		port := freePort(t)
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", fmt.Sprintf("https://localhost:%d/callback", port))
		store := fbauth.NewMemoryStore()

		var respHeader http.Header
		getDone := make(chan struct{})
		opts := LoginOptions{
			ClientID: "client-1", ClientSecret: "secret-1",
			Port: port, Timeout: 3 * time.Second,
			Endpoints: oauth.endpoints(), Store: store,
			OpenBrowser: func(rawURL string) error {
				state := stateFromAuthURL(t, rawURL)
				go func() {
					defer close(getDone)
					resp, err := insecureBrowserClient.Get(fmt.Sprintf("https://127.0.0.1:%d/callback?code=test-auth-code&state=%s", port, state))
					if err == nil {
						respHeader = resp.Header
						_ = resp.Body.Close()
					}
				}()
				return nil
			},
		}

		if _, err := Login(context.Background(), opts); err != nil {
			t.Fatalf("Login() error = %v", err)
		}
		// Login() returning only means the server side (a different
		// goroutine from the client Get() call above) sent to
		// resultCh -- wait for the client goroutine to actually finish
		// populating respHeader before reading it, or the race detector
		// (correctly) flags an unsynchronized access.
		select {
		case <-getDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for the browser client's GET to complete")
		}
		if respHeader == nil {
			t.Fatal("never captured the callback response")
		}
		if got := respHeader.Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want %q", got, "no-store")
		}
		if got := respHeader.Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("Referrer-Policy = %q, want %q", got, "no-referrer")
		}
	})

	t.Run("[sad] a state mismatch is rejected and nothing is saved", func(t *testing.T) {
		port := freePort(t)
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", "")
		store := fbauth.NewMemoryStore()

		opts := LoginOptions{
			ClientID: "client-1", ClientSecret: "secret-1",
			Port: port, Timeout: 3 * time.Second,
			Endpoints: oauth.endpoints(), Store: store,
			OpenBrowser: func(rawURL string) error {
				go func() {
					resp, err := insecureBrowserClient.Get(fmt.Sprintf("https://127.0.0.1:%d/callback?code=test-auth-code&state=wrong-state", port))
					if err == nil {
						_ = resp.Body.Close()
					}
				}()
				return nil
			},
		}

		_, err := Login(context.Background(), opts)
		if err == nil {
			t.Fatal("Login() error = nil, want a state mismatch error")
		}
		if !strings.Contains(err.Error(), "state mismatch") {
			t.Errorf("error = %v, want a state mismatch error", err)
		}
		if _, err := store.Load(context.Background()); err == nil {
			t.Error("a token was saved despite the state mismatch")
		}
	})

	t.Run("[sad] a callback with a code but no state at all is rejected (F1/B1: CSRF)", func(t *testing.T) {
		port := freePort(t)
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", "")
		store := fbauth.NewMemoryStore()

		opts := LoginOptions{
			ClientID: "client-1", ClientSecret: "secret-1",
			Port: port, Timeout: 3 * time.Second,
			Endpoints: oauth.endpoints(), Store: store,
			OpenBrowser: func(rawURL string) error {
				go func() {
					// No &state= at all -- the attack this fix closes:
					// a cross-origin request to the callback URL that
					// never had access to the generated state.
					resp, err := insecureBrowserClient.Get(fmt.Sprintf("https://127.0.0.1:%d/callback?code=attacker-code", port))
					if err == nil {
						_ = resp.Body.Close()
					}
				}()
				return nil
			},
		}

		_, err := Login(context.Background(), opts)
		if err == nil {
			t.Fatal("Login() error = nil, want a missing-state error")
		}
		if !strings.Contains(err.Error(), "no state") {
			t.Errorf("error = %v, want a missing-state error", err)
		}
		if _, err := store.Load(context.Background()); err == nil {
			t.Error("a token was saved despite the missing state")
		}
	})

	t.Run("[sad] a second callback request is rejected with 410 and never delivered again (single-use listener)", func(t *testing.T) {
		// Unit-level, against runCallbackServer directly: routing this
		// through the full Login flow races runCallbackServer's own
		// deferred stop() (which fires the instant Login's select reads
		// the first result off the buffered channel) against the second
		// HTTP request, so the listener can be gone before the second
		// request lands. Testing the handler's single-use property
		// directly avoids that race entirely.
		port := freePort(t)
		resultCh, stop, err := runCallbackServer(port, time.Now())
		if err != nil {
			t.Fatalf("runCallbackServer() error = %v", err)
		}
		defer func() { _ = stop() }()

		url := fmt.Sprintf("https://127.0.0.1:%d/callback?code=test-auth-code&state=probe-state", port)
		resp1, err := insecureBrowserClient.Get(url)
		if err != nil {
			t.Fatalf("first request: %v", err)
		}
		_ = resp1.Body.Close()

		select {
		case res := <-resultCh:
			if res.code != "test-auth-code" {
				t.Errorf("delivered code = %q", res.code)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for the first callback to be delivered")
		}

		resp2, err := insecureBrowserClient.Get(url)
		if err != nil {
			t.Fatalf("second request: %v", err)
		}
		defer func() { _ = resp2.Body.Close() }()
		if resp2.StatusCode != http.StatusGone {
			t.Errorf("second callback status = %d, want %d", resp2.StatusCode, http.StatusGone)
		}

		select {
		case <-resultCh:
			t.Error("a second result was delivered on resultCh; the listener is not single-use")
		default:
		}
	})

	t.Run("[sad] the browser reporting a denial is surfaced as an error", func(t *testing.T) {
		port := freePort(t)
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", "")
		opts := LoginOptions{
			ClientID: "client-1", ClientSecret: "secret-1",
			Port: port, Timeout: 3 * time.Second,
			Endpoints: oauth.endpoints(),
			OpenBrowser: func(rawURL string) error {
				state := stateFromAuthURL(t, rawURL)
				go func() {
					resp, err := insecureBrowserClient.Get(fmt.Sprintf("https://127.0.0.1:%d/callback?error=access_denied&state=%s", port, state))
					if err == nil {
						_ = resp.Body.Close()
					}
				}()
				return nil
			},
		}

		_, err := Login(context.Background(), opts)
		if err == nil {
			t.Fatal("Login() error = nil, want the access_denied error surfaced")
		}
		if !strings.Contains(err.Error(), "access_denied") {
			t.Errorf("error = %v, want it to mention access_denied", err)
		}
	})

	t.Run("[sad] a timeout when no callback ever arrives", func(t *testing.T) {
		port := freePort(t)
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", "")
		opts := LoginOptions{
			ClientID: "client-1", ClientSecret: "secret-1",
			Port: port, Timeout: 50 * time.Millisecond,
			Endpoints:   oauth.endpoints(),
			OpenBrowser: func(string) error { return nil }, // never hits the callback
		}

		start := time.Now()
		_, err := Login(context.Background(), opts)
		if err == nil {
			t.Fatal("Login() error = nil, want a timeout error")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("error = %v, want a timeout error", err)
		}
		// F27/security A7: the message points at the dual-stack loopback
		// gap (a browser resolving "localhost" to ::1 while the listener
		// only bound 127.0.0.1) and the workaround, since that gap is
		// backlog rather than fixed here.
		if !strings.Contains(err.Error(), "::1") {
			t.Errorf("error = %v, want it to mention the ::1/127.0.0.1 mismatch", err)
		}
		if !strings.Contains(err.Error(), "--no-browser") {
			t.Errorf("error = %v, want it to suggest --no-browser", err)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("Login() took %v, want it to return promptly after the timeout", elapsed)
		}
	})

	t.Run("[sad] a Store.Save failure surfaces after a successful exchange", func(t *testing.T) {
		// F20/review A11: the exchange succeeds but persisting it fails
		// (disk full, permissions) -- the caller must see that error, not
		// a token silently returned as if it were saved.
		port := freePort(t)
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", fmt.Sprintf("https://localhost:%d/callback", port))
		store := brokenSaveStore{fbauth.NewMemoryStore()}

		opts := LoginOptions{
			ClientID: "client-1", ClientSecret: "secret-1",
			Port: port, Timeout: 3 * time.Second,
			Endpoints: oauth.endpoints(), Store: store,
			OpenBrowser: func(rawURL string) error {
				state := stateFromAuthURL(t, rawURL)
				go func() {
					resp, err := insecureBrowserClient.Get(fmt.Sprintf("https://127.0.0.1:%d/callback?code=test-auth-code&state=%s", port, state))
					if err == nil {
						_ = resp.Body.Close()
					}
				}()
				return nil
			},
		}

		_, err := Login(context.Background(), opts)
		if err == nil {
			t.Fatal("Login() error = nil, want the Store.Save failure surfaced")
		}
		if !strings.Contains(err.Error(), "disk full") {
			t.Errorf("error = %v, want it to wrap the Store.Save error", err)
		}
	})

	t.Run("[sad] net.Listen failure (port already in use)", func(t *testing.T) {
		// F20/review A11: something else is already bound to the port
		// --callback-port names -- Login must report that, not hang or
		// panic.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()
		port := ln.Addr().(*net.TCPAddr).Port

		opts := LoginOptions{
			ClientID: "client-1", ClientSecret: "secret-1",
			Port: port, Timeout: 3 * time.Second,
			OpenBrowser: func(string) error { return nil },
		}

		_, err = Login(context.Background(), opts)
		if err == nil {
			t.Fatal("Login() error = nil, want the port-in-use error")
		}
		if !strings.Contains(err.Error(), "loopback listener") {
			t.Errorf("error = %v, want it to name the loopback listener", err)
		}
	})
}

func TestLoginNoBrowser(t *testing.T) {
	t.Run("[happy] a pasted full redirect URL validates state and exchanges", func(t *testing.T) {
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", "")
		store := fbauth.NewMemoryStore()
		var stdout syncBuffer
		opts := LoginOptions{
			ClientID: "client-1", ClientSecret: "secret-1",
			Endpoints: oauth.endpoints(), Store: store, Stdout: &stdout,
		}

		// LoginNoBrowser prints the authorization URL (carrying the
		// generated state) to Stdout before reading the pasted line;
		// extract it the same way a human would read it off the
		// terminal.
		done := make(chan struct{})
		var tok *fbauth.Token
		var loginErr error
		pr, pw := writerPipe(t)
		go func() {
			tok, loginErr = LoginNoBrowser(context.Background(), opts, pr)
			close(done)
		}()

		state := waitForState(t, &stdout)
		fmt.Fprintf(pw, "https://localhost:8765/callback?code=pasted-code&state=%s\n", state) //nolint:errcheck
		_ = pw.Close()
		<-done

		if loginErr != nil {
			t.Fatalf("LoginNoBrowser() error = %v", loginErr)
		}
		if tok.AccessToken != "fixture-access-token" {
			t.Errorf("AccessToken = %q", tok.AccessToken)
		}
	})

	t.Run("[happy] a bare pasted code skips state validation", func(t *testing.T) {
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", "")
		var stdout syncBuffer
		opts := LoginOptions{ClientID: "client-1", ClientSecret: "secret-1", Endpoints: oauth.endpoints(), Stdout: &stdout}

		pr, pw := writerPipe(t)
		done := make(chan struct{})
		var tok *fbauth.Token
		var loginErr error
		go func() {
			tok, loginErr = LoginNoBrowser(context.Background(), opts, pr)
			close(done)
		}()
		waitForState(t, &stdout)
		fmt.Fprintln(pw, "bare-code-value") //nolint:errcheck
		_ = pw.Close()
		<-done

		if loginErr != nil {
			t.Fatalf("LoginNoBrowser() error = %v", loginErr)
		}
		if tok.AccessToken != "fixture-access-token" {
			t.Errorf("AccessToken = %q", tok.AccessToken)
		}
	})

	t.Run("[sad] a pasted URL with the wrong state is rejected", func(t *testing.T) {
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", "")
		var stdout syncBuffer
		opts := LoginOptions{ClientID: "client-1", ClientSecret: "secret-1", Endpoints: oauth.endpoints(), Stdout: &stdout}

		pr, pw := writerPipe(t)
		done := make(chan struct{})
		var loginErr error
		go func() {
			_, loginErr = LoginNoBrowser(context.Background(), opts, pr)
			close(done)
		}()
		waitForState(t, &stdout)
		fmt.Fprintln(pw, "https://localhost:8765/callback?code=x&state=totally-wrong") //nolint:errcheck
		_ = pw.Close()
		<-done

		if loginErr == nil {
			t.Fatal("LoginNoBrowser() error = nil, want a state mismatch error")
		}
	})

	t.Run("[sad] an empty pasted line has no code to exchange", func(t *testing.T) {
		// F20/review A11: parsePastedInput's bare-code branch on an empty
		// line yields code == "", which finishExchange must reject
		// rather than exchange an empty code against the token endpoint.
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", "")
		var stdout syncBuffer
		opts := LoginOptions{ClientID: "client-1", ClientSecret: "secret-1", Endpoints: oauth.endpoints(), Stdout: &stdout}

		pr, pw := writerPipe(t)
		done := make(chan struct{})
		var loginErr error
		go func() {
			_, loginErr = LoginNoBrowser(context.Background(), opts, pr)
			close(done)
		}()
		waitForState(t, &stdout)
		fmt.Fprintln(pw) //nolint:errcheck // an empty pasted line
		_ = pw.Close()
		<-done

		if loginErr == nil {
			t.Fatal("LoginNoBrowser() error = nil, want a \"no authorization code\" error")
		}
		if !strings.Contains(loginErr.Error(), "no authorization code") {
			t.Errorf("error = %v, want it to say no authorization code was provided", loginErr)
		}
	})

	t.Run("[sad] a Store.Save failure surfaces after a successful exchange", func(t *testing.T) {
		// F20/review A11: same guarantee as Login's browser path -- a
		// persistence failure must not be silently swallowed.
		oauth := newFakeOAuthServer(t, "client-1", "secret-1", "")
		store := brokenSaveStore{fbauth.NewMemoryStore()}
		var stdout syncBuffer
		opts := LoginOptions{ClientID: "client-1", ClientSecret: "secret-1", Endpoints: oauth.endpoints(), Store: store, Stdout: &stdout}

		pr, pw := writerPipe(t)
		done := make(chan struct{})
		var loginErr error
		go func() {
			_, loginErr = LoginNoBrowser(context.Background(), opts, pr)
			close(done)
		}()
		waitForState(t, &stdout)
		fmt.Fprintln(pw, "bare-code-value") //nolint:errcheck
		_ = pw.Close()
		<-done

		if loginErr == nil {
			t.Fatal("LoginNoBrowser() error = nil, want the Store.Save failure surfaced")
		}
		if !strings.Contains(loginErr.Error(), "disk full") {
			t.Errorf("error = %v, want it to wrap the Store.Save error", loginErr)
		}
	})
}

// writerPipe returns an io.Pipe whose reader is closed on test cleanup.
func writerPipe(t *testing.T) (*io.PipeReader, *io.PipeWriter) {
	t.Helper()
	r, w := io.Pipe()
	t.Cleanup(func() { _ = r.Close() })
	return r, w
}

func waitForState(t *testing.T, buf *syncBuffer) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if idx := strings.Index(buf.String(), "state="); idx >= 0 {
			rest := buf.String()[idx+len("state="):]
			end := strings.IndexAny(rest, "&\n ")
			if end < 0 {
				end = len(rest)
			}
			return rest[:end]
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for the authorization URL to be printed")
	return ""
}
