package server

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
	"github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeFreshBooksUpstream answers every request with a minimal accounting-
// or auth-family envelope (mirroring internal/tools' fakeUpstream closely
// enough for identity_whoami, the only tool these tests call), and
// records every Authorization header it sees, in order.
func fakeFreshBooksUpstream(t *testing.T) (*httptest.Server, *bearerLog) {
	t.Helper()
	log := &bearerLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.record(r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"response": {}}`))
	}))
	return srv, log
}

type bearerLog struct {
	mu   sync.Mutex
	seen []string
}

func (l *bearerLog) record(v string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, v)
}

func (l *bearerLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.seen...)
}

// headerTransport injects a bearer Authorization header on every outgoing
// request and records the last response's Mcp-Session-Id header, so tests
// can assert stateless mode never emits one.
type headerTransport struct {
	bearer      string
	next        http.RoundTripper
	lastSession *string
	mu          *sync.Mutex
}

func (t headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.bearer)
	resp, err := t.next.RoundTrip(req)
	if err == nil && t.mu != nil {
		t.mu.Lock()
		*t.lastSession = resp.Header.Get("Mcp-Session-Id")
		t.mu.Unlock()
	}
	return resp, err
}

func testConfig(baseURL string) *config.Config {
	return &config.Config{
		Transport: "http",
		Addr:      "127.0.0.1:0",
		Path:      "/mcp",
		LogLevel:  "error",
		LogFormat: "text",
		BaseURL:   baseURL,
	}
}

func TestHTTPHandlerHealthz(t *testing.T) {
	upstream, _ := fakeFreshBooksUpstream(t)
	defer upstream.Close()

	srv := New(testConfig(upstream.URL), "test")
	ts := httptest.NewServer(srv.HTTPHandler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want 200", resp.StatusCode)
	}
}

func TestHTTPHandlerRequiresBearer(t *testing.T) {
	upstream, _ := fakeFreshBooksUpstream(t)
	defer upstream.Close()

	srv := New(testConfig(upstream.URL), "test")
	ts := httptest.NewServer(srv.HTTPHandler())
	defer ts.Close()

	t.Run("[sad] a missing Authorization header is rejected", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/mcp", "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
		if resp.Header.Get("WWW-Authenticate") == "" {
			t.Error("missing WWW-Authenticate header")
		}
	})

	t.Run("[sad] a malformed Authorization header is rejected", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("[sad] an empty bearer token is rejected", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer ")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("[edge] GET on the MCP path with a valid bearer is 405 in stateless mode", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/mcp", nil)
		req.Header.Set("Authorization", "Bearer tok")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("GET /mcp = %d, want 405", resp.StatusCode)
		}
	})
}

// TestStatelessProperty is the stateless contract from
// docs/phases/3/plan.md: two sequential tools/call requests with
// different bearers each reach FreshBooks with their own bearer, in
// order, and neither response carries a session id.
func TestStatelessProperty(t *testing.T) {
	upstream, bearers := fakeFreshBooksUpstream(t)
	defer upstream.Close()

	srv := New(testConfig(upstream.URL), "test")
	ts := httptest.NewServer(srv.HTTPHandler())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var mu sync.Mutex
	for _, bearer := range []string{"bearer-one", "bearer-two"} {
		var lastSession string
		hc := &http.Client{Transport: headerTransport{bearer: bearer, next: http.DefaultTransport, lastSession: &lastSession, mu: &mu}}
		clientTransport := &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp", HTTPClient: hc, DisableStandaloneSSE: true}
		mcpClient := mcp.NewClient(&mcp.Implementation{Name: "stateless-test-client", Version: "test"}, nil)
		session, err := mcpClient.Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatalf("Connect(%s): %v", bearer, err)
		}
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "identity_whoami", Arguments: map[string]any{}})
		if err != nil {
			t.Fatalf("CallTool(%s): %v", bearer, err)
		}
		if result.IsError {
			t.Fatalf("CallTool(%s) returned IsError", bearer)
		}
		_ = session.Close()
		if lastSession != "" {
			t.Fatalf("response for %s carried Mcp-Session-Id %q, want none (stateless)", bearer, lastSession)
		}
	}

	got := bearers.all()
	if len(got) < 2 {
		t.Fatalf("upstream saw %d requests, want at least 2", len(got))
	}
	if got[0] != "Bearer bearer-one" || got[len(got)-1] != "Bearer bearer-two" {
		t.Fatalf("upstream bearers = %v, want the first to be bearer-one and the last bearer-two", got)
	}
}

// TestGetServerReturnsNilOnClientConstructionFailure drives getServer's
// unreachable-in-production failure path (docs/phases/3/reports/
// security.md finding 7, code-review.md finding 10): an invalid BaseURL
// makes freshbooks.NewClient fail, and the handler must answer a clean
// 400 rather than silently serving a tool-less server.
func TestGetServerReturnsNilOnClientConstructionFailure(t *testing.T) {
	cfg := testConfig("not-an-absolute-url")
	srv := New(cfg, "test")
	ts := httptest.NewServer(srv.HTTPHandler())
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (no server available)", resp.StatusCode)
	}
}

func TestRunStdioFailsWithoutToken(t *testing.T) {
	cfg := &config.Config{Transport: "stdio", LogLevel: "error", LogFormat: "text"}
	srv := New(cfg, "test")
	if err := srv.RunStdio(context.Background()); err == nil {
		t.Fatal("want an error when no token is configured")
	}
}

func TestRunHTTPSurfacesListenError(t *testing.T) {
	upstream, _ := fakeFreshBooksUpstream(t)
	defer upstream.Close()

	// Bind one listener first, then ask RunHTTP to use the same address:
	// ListenAndServe must fail immediately with "address already in use".
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	cfg := testConfig(upstream.URL)
	cfg.Addr = l.Addr().String()
	srv := New(cfg, "test")

	err = srv.RunHTTP(context.Background())
	if err == nil {
		t.Fatal("want a listen error")
	}
}

func TestRunStdioHappyPathReturnsOnCancel(t *testing.T) {
	upstream, _ := fakeFreshBooksUpstream(t)
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	cfg.Transport = "stdio"
	cfg.AccessToken = "test-token"
	srv := New(cfg, "test")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.RunStdio(ctx) }()

	cancel()
	select {
	case <-done:
		// RunStdio returned once ctx was canceled; the specific error
		// (if any) depends on the stdio transport's own shutdown
		// semantics and is not asserted here.
	case <-time.After(5 * time.Second):
		t.Fatal("RunStdio did not return after context cancellation")
	}
}

// TestRunHTTPShutsDownGracefully drives Serve directly with a listener the
// test binds itself, so the bind is synchronous, and polls /healthz until
// the accept loop is actually answering before cancelling -- no sleep, so
// it cannot flake on a loaded runner in either direction (see
// docs/phases/3/reports/code-review.md finding 9).
func TestRunHTTPShutsDownGracefully(t *testing.T) {
	upstream, _ := fakeFreshBooksUpstream(t)
	defer upstream.Close()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	cfg := testConfig(upstream.URL)
	srv := New(cfg, "test")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, l) }()

	waitHealthy(t, "http://"+l.Addr().String()+"/healthz")
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v, want nil on graceful shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after context cancellation")
	}
}

// waitHealthy polls url until it answers 200, or fails the test after 5
// seconds.
func waitHealthy(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec // url is a fixed loopback address this test built itself
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("server at %s never became healthy", url)
}

// TestLoggingNeverLeaksBearer drives one real tools/call through the HTTP
// path (server.HTTPHandler, both the SDK's own StreamableHTTPOptions.Logger
// and the lib's WithLogger) and one through the stdio path's client
// construction (TokenSource + clientOptions, the same building blocks
// RunStdio uses), both at debug level with a known bearer, and asserts the
// bearer never reaches the captured log output in either path
// (docs/phases/3/reports/security.md finding 5).
func TestLoggingNeverLeaksBearer(t *testing.T) {
	const bearer = "super-secret-bearer-token-do-not-leak"

	t.Run("http", func(t *testing.T) {
		var logBuf bytes.Buffer
		upstream, _ := fakeFreshBooksUpstream(t)
		defer upstream.Close()

		cfg := testConfig(upstream.URL)
		cfg.LogLevel = "debug"
		srv := New(cfg, "test")
		srv.logger = slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		ts := httptest.NewServer(srv.HTTPHandler())
		defer ts.Close()

		clientTransport := &mcp.StreamableClientTransport{
			Endpoint:             ts.URL + "/mcp",
			HTTPClient:           &http.Client{Transport: headerTransport{bearer: bearer, next: http.DefaultTransport}},
			DisableStandaloneSSE: true,
		}
		mcpClient := mcp.NewClient(&mcp.Implementation{Name: "log-test-client", Version: "test"}, nil)
		ctx := context.Background()
		session, err := mcpClient.Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = session.Close() }()
		if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "identity_whoami", Arguments: map[string]any{}}); err != nil {
			t.Fatal(err)
		}

		if strings.Contains(logBuf.String(), bearer) {
			t.Fatalf("HTTP path logging leaked the bearer: %s", logBuf.String())
		}
	})

	t.Run("stdio", func(t *testing.T) {
		var logBuf bytes.Buffer
		upstream, _ := fakeFreshBooksUpstream(t)
		defer upstream.Close()

		cfg := testConfig(upstream.URL)
		cfg.LogLevel = "debug"
		srv := New(cfg, "test")
		srv.logger = slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		opts := append([]freshbooks.Option{freshbooks.WithTokenSource(auth.StaticTokenSource(bearer))}, srv.clientOptions()...)
		client, err := freshbooks.NewClient(opts...)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Identity.Whoami(context.Background()); err != nil {
			t.Fatal(err)
		}

		if strings.Contains(logBuf.String(), bearer) {
			t.Fatalf("stdio path logging leaked the bearer: %s", logBuf.String())
		}
	})
}
