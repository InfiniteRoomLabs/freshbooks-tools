package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// newTestSession wires a lib client pointed at upstream (whose transport
// is redirected to it, and which logs through logger when non-nil), one
// *mcp.Server with every tool registered, and one connected client
// session. The session is closed automatically via t.Cleanup.
func newTestSession(t *testing.T, upstream *httptest.Server, logger *slog.Logger) *mcp.ClientSession {
	t.Helper()
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}

	opts := []freshbooks.Option{
		freshbooks.WithTokenSource(auth.StaticTokenSource("test-token")),
		freshbooks.WithHTTPClient(&http.Client{Transport: redirectTransport{addr: upstreamURL.Host, next: http.DefaultTransport}}),
	}
	if logger != nil {
		opts = append(opts, freshbooks.WithLogger(logger))
	}
	client, err := freshbooks.NewClient(opts...)
	if err != nil {
		t.Fatal(err)
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "redaction-test", Version: "test"}, nil)
	Register(mcpServer, client, testScope)

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := mcpServer.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "redaction-test-client", Version: "test"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

// applicationFixture is a fixture server that answers every request with
// an Application carrying a live-looking client_secret, in whichever
// envelope the request's family and shape need.
func applicationFixture(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const secret = `super-secret-oauth-client-secret-do-not-leak`
		app := `{"client_id": "cid_123", "client_secret": "` + secret + `", "name": "Test App", "redirect_uri": "https://example.com/cb"}`
		switch {
		case strings.Contains(r.URL.Path, "partners/applications") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"response": [` + app + `]}`))
		default:
			_, _ = w.Write([]byte(`{"response": ` + app + `}`))
		}
	}))
}

// TestApplicationSecretRedacted is the written security constraint from
// docs/phases/3/plan.md: every tool returning freshbooks.Application
// zeroes ClientSecret before it reaches the result.
func TestApplicationSecretRedacted(t *testing.T) {
	upstream := applicationFixture(t)
	defer upstream.Close()
	ctx := context.Background()
	clientSession := newTestSession(t, upstream, nil)

	for _, name := range []string{"identity_create_application", "identity_applications", "identity_update_application"} {
		t.Run(name, func(t *testing.T) {
			var spec *Spec
			for i := range All {
				if All[i].Name == name {
					spec = &All[i]
				}
			}
			if spec == nil {
				t.Fatalf("no such tool %q", name)
			}
			args := synth(spec.InputSchema)
			result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if result.IsError {
				t.Fatalf("tool returned IsError: %s", errorContentText(result))
			}
			raw, err := json.Marshal(result.StructuredContent)
			if err != nil {
				t.Fatalf("marshaling StructuredContent: %v", err)
			}
			if strings.Contains(string(raw), "super-secret-oauth-client-secret") {
				t.Fatalf("%s leaked client_secret: %s", name, raw)
			}
			if strings.Contains(string(raw), "client_secret") {
				t.Fatalf("%s: client_secret key survived (should be omitted, not just emptied): %s", name, raw)
			}
		})
	}
}

// TestTokenizationNeverEchoesCardData is the written security constraint
// from docs/phases/3/plan.md: a card number given as input to a
// tokenization tool never reaches a result, an error, or a log.
func TestTokenizationNeverEchoesCardData(t *testing.T) {
	const cardNumber = "4242424242424242"

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	upstream := fakeUpstream(t)
	defer upstream.Close()
	clientSession := newTestSession(t, upstream, logger)

	ctx := context.Background()
	args := map[string]any{
		"body": map[string]any{
			"name":         "Test Cardholder",
			"card_number":  cardNumber,
			"expiry_month": "12",
			"expiry_year":  "2030",
			"email":        "test@example.com",
			"cvv":          "123",
			"postal_code":  "12345",
			"country":      "US",
		},
	}
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "payment_options_fb_pay_tokenize", Arguments: args})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	if !result.IsError {
		raw, mErr := json.Marshal(result.StructuredContent)
		if mErr != nil {
			t.Fatalf("marshaling StructuredContent: %v", mErr)
		}
		if strings.Contains(string(raw), cardNumber) {
			t.Fatalf("result leaked the card number: %s", raw)
		}
	} else if strings.Contains(errorContentText(result), cardNumber) {
		t.Fatalf("error result leaked the card number: %s", errorContentText(result))
	}

	if strings.Contains(logBuf.String(), cardNumber) {
		t.Fatalf("log output leaked the card number: %s", logBuf.String())
	}
}
