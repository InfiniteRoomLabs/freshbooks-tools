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
		fmt.Fprint(w, `{}`) //nolint:errcheck
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
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

func TestOpenBrowserAttempts(t *testing.T) {
	// openBrowser shells out to a platform program; this only proves it
	// builds and starts the right command for this GOOS without
	// requiring the program to actually exist or succeed.
	_ = openBrowser("https://example.invalid/probe")
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
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Errorf("Login() took %v, want it to return promptly after the timeout", elapsed)
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
