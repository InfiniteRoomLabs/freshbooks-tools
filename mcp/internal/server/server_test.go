package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestRunHTTPShutsDownGracefully(t *testing.T) {
	upstream, _ := fakeFreshBooksUpstream(t)
	defer upstream.Close()

	cfg := testConfig(upstream.URL)
	srv := New(cfg, "test")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.RunHTTP(ctx) }()

	// Give ListenAndServe a moment to bind before asking it to stop.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunHTTP returned %v, want nil on graceful shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunHTTP did not return after context cancellation")
	}
}
