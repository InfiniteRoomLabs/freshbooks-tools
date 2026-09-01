package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// primaryType returns s's JSON Schema type: Type when set, else the first
// entry of Types that isn't "null" (jsonschema-go emits Types instead of
// Type for a Go slice, which always allows null).
func primaryType(s *jsonschema.Schema) string {
	if s.Type != "" {
		return s.Type
	}
	for _, t := range s.Types {
		if t != "null" {
			return t
		}
	}
	return ""
}

// synth builds the smallest value that satisfies schema's *required*
// properties, recursively -- never an optional (omitempty) one, so every
// scope field is left for Register's defaults to supply. It is deliberately
// generic: it drives every one of the 168 tools' round trips without a
// bespoke payload per tool.
func synth(s *jsonschema.Schema) any {
	if s == nil {
		return map[string]any{}
	}
	switch primaryType(s) {
	case "object":
		m := map[string]any{}
		for _, name := range s.Required {
			m[name] = synthField(name, s.Properties[name])
		}
		return m
	case "string":
		if s.Format == "date" {
			return "2026-01-01"
		}
		// Also a valid RFC 3339 timestamp, so it satisfies DateTime's
		// UnmarshalJSON when a required field happens to be one.
		return "2026-01-01T00:00:00Z"
	case "integer", "number":
		return 1
	case "boolean":
		return false
	case "array":
		// A required array is sometimes validated non-empty by the lib
		// itself (Estimates.Send: "needs at least one recipient"), so this
		// synthesizes one item rather than an empty array.
		if s.Items != nil {
			return []any{synth(s.Items)}
		}
		return []any{}
	default:
		return map[string]any{}
	}
}

// synthField special-cases the required properties whose content must
// actually decode or re-encode, not just satisfy the schema's bare
// "type": every upload tool's content_base64 must be valid base64, and
// retainers_create's fee -- a plain string here that the Call closure
// converts with json.Number(...) (see tools_retainers.go) -- must be a
// syntactically valid JSON number literal, since json.Marshal validates a
// json.Number's content before writing it out unquoted.
func synthField(name string, s *jsonschema.Schema) any {
	switch name {
	case "content_base64":
		return "dGVzdA=="
	case "fee":
		return "100.00"
	default:
		return synth(s)
	}
}

// redirectTransport forces every outgoing request onto addr regardless of
// the scheme or host the lib resolved, so a fixture server on loopback can
// stand in for both api.freshbooks.com (the default base URL) and
// paid.freshbooks.com (payment_options_fb_pay_tokenize and
// payment_options_stripe_tokenize hard-code that host and https,
// bypassing WithBaseURL entirely -- see freshbooks/payment_options.go and
// Client.doOnHost's doc comment). No test in this file ever reaches the
// real network.
type redirectTransport struct {
	addr string
	next http.RoundTripper
}

func (t redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = t.addr
	req.Host = t.addr
	return t.next.RoundTrip(req)
}

// fakeUpstream answers every request with a minimal, valid body for the
// family its path implies, mirroring freshbooks/client.go's
// familyForPath classification closely enough that every lib decode path
// succeeds without error: an accounting response is enveloped
// ({"response":{"result":{}}}), an auth response has no "result" layer
// ({"response":{}}), and everything else (business-family, uploads,
// ledger accounts, tokenization) is flat ({}). Two paths need real bytes
// instead of JSON: invoice PDFs, which the lib rejects unless the body
// starts with the PDF magic bytes, and the invoice-details CSV download.
func fakeUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/pdf"):
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4\nfake\n%%EOF"))
		case strings.HasSuffix(path, ".csv"):
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("a,b\n1,2\n"))
		// A handful of endpoints decode straight into a slice, or require
		// a non-empty id, rather than the generic per-family shape below;
		// see each tool's comment in this switch for the lib decode path
		// that demands it.
		case strings.Contains(path, "partners/applications") && r.Method == http.MethodGet:
			// Identity.Applications decodes {"response": [...]}  directly
			// into []Application (freshbooks/settings.go).
			_, _ = w.Write([]byte(`{"response": []}`))
		case strings.HasSuffix(path, "/team_members") && r.Method == http.MethodGet:
			// TeamMembers.List passes FamilyBusiness on an /auth/-rooted
			// path and decodes into {response: [...], meta: {...}} flat,
			// not the auth family's enveloped shape (freshbooks/team_members.go).
			_, _ = w.Write([]byte(`{"response": [], "meta": {}}`))
		case strings.HasSuffix(path, "/threads") && r.Method == http.MethodGet:
			// Projects.Threads decodes straight into []map[string]any
			// (freshbooks/projects.go). Its sibling create/comment
			// endpoints share the same "/comments/business/..." path
			// root but decode a flat object, so this matches only the
			// GET-list path exactly, not the whole root.
			_, _ = w.Write([]byte(`[]`))
		case strings.Contains(path, "checkout"):
			// Payments.{Create,Update}CheckoutLink require a non-empty id
			// in either the enveloped or flat shape (freshbooks/payments.go);
			// Delete/UpdateGateway ignore the body entirely.
			_, _ = w.Write([]byte(`{"checkout_link": {"id": "cl_test000"}}`))
		case strings.HasPrefix(path, "/auth/"):
			_, _ = w.Write([]byte(`{"response": {}}`))
		case strings.HasPrefix(path, "/accounting/"), strings.HasPrefix(path, "/events/"):
			_, _ = w.Write([]byte(`{"response": {"result": {}}}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
}

// errorContentText renders an IsError result's content for a test failure
// message.
func errorContentText(result *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// testScope is the default scope fixture: valid-shaped, synthetic
// identifiers a real FreshBooks account would never carry.
var testScope = Scope{
	AccountID:    "ACM000TEST",
	BusinessID:   9000001,
	BusinessUUID: "00000000-0000-4000-8000-000000000099",
}

// TestRoundTrip exercises every registered tool through a real MCP client
// session (mcp.NewInMemoryTransports) against a server built by Register,
// backed by a lib client whose transport is redirected to a local fixture
// server. It is the [round trip] contract from docs/phases/3/plan.md:
// one row per registry entry, asserting no IsError and that
// StructuredContent round-trips as valid JSON.
func TestRoundTrip(t *testing.T) {
	upstream := fakeUpstream(t)
	defer upstream.Close()
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}

	client, err := freshbooks.NewClient(
		freshbooks.WithTokenSource(auth.StaticTokenSource("test-token")),
		freshbooks.WithHTTPClient(&http.Client{Transport: redirectTransport{addr: upstreamURL.Host, next: http.DefaultTransport}}),
	)
	if err != nil {
		t.Fatal(err)
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "freshbooks-mcp-test", Version: "test"}, nil)
	Register(mcpServer, client, testScope)

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := mcpServer.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = serverSession.Close() }()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "roundtrip-test-client", Version: "test"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = clientSession.Close() }()

	if len(All) != 168 {
		t.Fatalf("registry has %d tools, want 168", len(All))
	}

	for _, spec := range All {
		t.Run(spec.Name, func(t *testing.T) {
			args := synth(spec.InputSchema)
			result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: spec.Name, Arguments: args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if result.IsError {
				t.Fatalf("tool returned IsError: %s", errorContentText(result))
			}
			if result.StructuredContent != nil {
				if _, err := json.Marshal(result.StructuredContent); err != nil {
					t.Fatalf("StructuredContent did not round-trip as JSON: %v", err)
				}
			}
		})
	}
}
