package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
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
// generic: it drives every one of the 169 tools' round trips without a
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

// Probe values injected into the optional fields synth would otherwise
// omit (page, per_page, search, include), distinct from anything else in
// a synthesized payload, so TestRoundTrip can assert they reach the
// recorded request's query string instead of being silently dropped by a
// Call closure that forgot to forward them (docs/phases/3/reports/
// code-review.md finding 1's "options silently dropped" suggestion).
const (
	probePage    = 7
	probePerPage = 13
)

var (
	probeSearch  = map[string]string{"probe_filter": "probe_value"}
	probeInclude = []string{"probe_include"}
)

// synthWithProbes builds the same payload synth does, then additionally
// fills page/per_page/search/include wherever the top-level schema
// declares them.
func synthWithProbes(s *jsonschema.Schema) map[string]any {
	v, _ := synth(s).(map[string]any)
	if v == nil {
		v = map[string]any{}
	}
	if s == nil {
		return v
	}
	if _, ok := s.Properties["page"]; ok {
		v["page"] = probePage
	}
	if _, ok := s.Properties["per_page"]; ok {
		v["per_page"] = probePerPage
	}
	if _, ok := s.Properties["search"]; ok {
		v["search"] = probeSearch
	}
	if _, ok := s.Properties["include"]; ok {
		v["include"] = probeInclude
	}
	return v
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

// recordedRequest is one HTTP request fakeUpstream answered, captured for
// TestRoundTrip's assertions.
type recordedRequest struct {
	method, path, rawQuery string
	body                   []byte
}

// requestRecorder collects every request a fakeUpstream server answers,
// in order, so a test can inspect what the lib actually sent -- the
// request-recording gap docs/phases/3/reports/code-review.md finding 1
// named as the round-trip test's central defect.
type requestRecorder struct {
	mu   sync.Mutex
	reqs []recordedRequest
}

func (r *requestRecorder) record(req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewReader(body))
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, recordedRequest{method: req.Method, path: req.URL.Path, rawQuery: req.URL.RawQuery, body: body})
}

// reset clears every request recorded so far, so each subtest starts from
// a clean slate.
func (r *requestRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = nil
}

// last returns the most recently recorded request, or false if none was
// recorded.
func (r *requestRecorder) last() (recordedRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.reqs) == 0 {
		return recordedRequest{}, false
	}
	return r.reqs[len(r.reqs)-1], true
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
// Every request is recorded before it is answered.
func fakeUpstream(t *testing.T) (*httptest.Server, *requestRecorder) {
	t.Helper()
	rec := &requestRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
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
	return srv, rec
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

// newTestSession wires a lib client pointed at upstream (whose transport
// is redirected to it, and which logs through logger when non-nil), one
// *mcp.Server with every tool registered against defaults, and one
// connected client session. Both the client and server sessions are
// closed automatically via t.Cleanup. Shared by every test in this
// package that needs a live MCP round trip (roundtrip_test.go,
// redaction_test.go, unit_test.go).
func newTestSession(t *testing.T, upstream *httptest.Server, defaults Scope, logger *slog.Logger) *mcp.ClientSession {
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

	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "test"}, nil)
	Register(mcpServer, client, defaults)

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := mcpServer.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

// isReadOnly reports whether spec's annotations mark it read-only.
func isReadOnly(spec Spec) bool {
	return spec.Annotations != nil && spec.Annotations.ReadOnlyHint
}

// assertScopeInPath checks that the recorded request's path contains the
// scope identifier testScope supplied for whichever scope field(s) the
// tool's schema declares (account_id, business_id, business_uuid) -- proof
// that the right identifier, not a different scope family, reached the
// URL (the "staff_get's scope.AccountID crossed to scope.BusinessID"
// class of defect code-review.md finding 1 named).
func assertScopeInPath(t *testing.T, spec Spec, path string) {
	t.Helper()
	if spec.InputSchema == nil {
		return
	}
	props := spec.InputSchema.Properties
	if _, ok := props["account_id"]; ok {
		if !strings.Contains(path, string(testScope.AccountID)) {
			t.Errorf("%s: path %q does not contain the account scope id %q", spec.Name, path, testScope.AccountID)
		}
	}
	if _, ok := props["business_id"]; ok {
		bid := strconv.FormatInt(int64(testScope.BusinessID), 10)
		if !strings.Contains(path, bid) {
			t.Errorf("%s: path %q does not contain the business scope id %q", spec.Name, path, bid)
		}
	}
	if _, ok := props["business_uuid"]; ok {
		if !strings.Contains(path, string(testScope.BusinessUUID)) {
			t.Errorf("%s: path %q does not contain the business_uuid scope id %q", spec.Name, path, testScope.BusinessUUID)
		}
	}
}

// assertMethodMatchesAnnotation checks the recorded HTTP method against
// the tool's ReadOnlyHint: every List/Get/Search/report method in the lib
// issues a GET, and every mutating method issues something else -- a
// cheap, name-independent sanity check that a Call closure did not
// silently swap a read for a write or vice versa.
func assertMethodMatchesAnnotation(t *testing.T, spec Spec, method string) {
	t.Helper()
	if isReadOnly(spec) {
		if method != http.MethodGet {
			t.Errorf("%s: method %s, want GET (ReadOnlyHint)", spec.Name, method)
		}
	} else if method == http.MethodGet {
		t.Errorf("%s: method GET, want a mutating method (not ReadOnlyHint)", spec.Name)
	}
}

// leafStrings collects every string value nested anywhere inside v.
func leafStrings(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case map[string]any:
		var out []string
		for _, e := range x {
			out = append(out, leafStrings(e)...)
		}
		return out
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, leafStrings(e)...)
		}
		return out
	default:
		return nil
	}
}

// bodyLeafStrings is leafStrings with the top-level "id" field excluded:
// by consistent convention across every tool in this registry, a
// string- or int-valued "id" addresses the resource in the URL path,
// never the request body (e.g. payments_delete_checkout_link's
// string-valued checkout-link id).
func bodyLeafStrings(args map[string]any) []string {
	filtered := make(map[string]any, len(args))
	for k, v := range args {
		if k != "id" {
			filtered[k] = v
		}
	}
	return leafStrings(filtered)
}

// assertBodyCarriesInput checks, for a non-read-only tool, that the
// recorded request body is non-empty and contains at least one string
// value from the synthesized input -- proof a Call closure actually
// forwarded the input rather than sending an empty or input-independent
// body (code-review.md finding 1's "drop &in.Body" scenario). Tools whose
// synthesized input carries no string leaves at all (an all-scope,
// all-numeric, or empty input) have nothing to check here and are
// skipped; they are still covered by assertMethodMatchesAnnotation.
func assertBodyCarriesInput(t *testing.T, spec Spec, args map[string]any, body []byte) {
	t.Helper()
	// Scope fields (account_id, business_id, business_uuid) are all
	// omitempty, so synth never includes them; a tool synthesized with no
	// string leaves at all is one whose only fields are scope and an id
	// (every callbacks_delete/contacts_delete/projects_delete/
	// taxes_delete/time_entries_delete-shaped tool) -- a legitimate empty
	// body for a plain DELETE with nothing else to send. Nothing to check
	// here; assertMethodMatchesAnnotation already covers these.
	leaves := bodyLeafStrings(args)
	if len(leaves) == 0 {
		return
	}
	if len(body) == 0 {
		t.Errorf("%s: a non-read-only tool with string-valued input sent an empty request body", spec.Name)
		return
	}
	for _, leaf := range leaves {
		if strings.Contains(string(body), leaf) {
			return
		}
	}
	t.Errorf("%s: request body %s contains none of the synthesized input's string values %v", spec.Name, body, leaves)
}

// assertProbesInQuery checks that page/per_page/search/include, when
// present in the tool's schema, reached the recorded query string --
// proof a Call closure forwards pagination and filter options instead of
// silently dropping them.
func assertProbesInQuery(t *testing.T, spec Spec, rawQuery string) {
	t.Helper()
	if spec.InputSchema == nil {
		return
	}
	props := spec.InputSchema.Properties
	if _, ok := props["page"]; ok {
		// The exact key=value pair, not just the digit anywhere in the
		// query string -- "page=7" could otherwise false-match on an
		// unrelated "per_page=17" or similar.
		want := "page=" + strconv.Itoa(probePage)
		if !strings.Contains(rawQuery, want) {
			t.Errorf("%s: query %q does not contain %s", spec.Name, rawQuery, want)
		}
	}
	if _, ok := props["per_page"]; ok {
		want := "per_page=" + strconv.Itoa(probePerPage)
		if !strings.Contains(rawQuery, want) {
			t.Errorf("%s: query %q does not contain %s", spec.Name, rawQuery, want)
		}
	}
	if _, ok := props["search"]; ok {
		for k, v := range probeSearch {
			if !strings.Contains(rawQuery, k) || !strings.Contains(rawQuery, v) {
				t.Errorf("%s: query %q does not carry the probe search filter %s=%s", spec.Name, rawQuery, k, v)
			}
		}
	}
	if _, ok := props["include"]; ok {
		for _, v := range probeInclude {
			if !strings.Contains(rawQuery, v) {
				t.Errorf("%s: query %q does not carry the probe include value %s", spec.Name, rawQuery, v)
			}
		}
	}
}

// visStateBodyExpectation closes the one swap risk this module has that
// no generic check above catches: bills_archive and bills_delete take the
// identical input shape (AcctScope + id, no body field the model
// controls) and PUT to the identical path, differing only in the
// literal vis_state their closures hard-code (freshbooks/bills.go's
// visStatePut: VisStateArchived = 2, VisStateDeleted = 1). Swapping their
// two Call closures would pass every other assertion in this file.
var visStateBodyExpectation = map[string]string{
	"bills_archive": `"vis_state":2`,
	"bills_delete":  `"vis_state":1`,
}

// representativeDecode unmarshals a successful result's StructuredContent
// back into the exact lib type the tool returns, for one tool per API
// family, proving the type -- including its Date/DateTime custom
// marshal/unmarshal pair -- really does round-trip end to end rather than
// merely being valid JSON (the marshal-only check this replaced could not
// tell a Date apart from any other string).
var representativeDecode = map[string]func(t *testing.T, raw []byte){
	"identity_whoami": func(t *testing.T, raw []byte) {
		var v freshbooks.User
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Errorf("identity_whoami: decoding into freshbooks.User: %v", err)
		}
	},
	"invoices_get": func(t *testing.T, raw []byte) {
		var v freshbooks.Invoice
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Errorf("invoices_get: decoding into freshbooks.Invoice: %v", err)
		}
	},
	"projects_get": func(t *testing.T, raw []byte) {
		var v freshbooks.Project
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Errorf("projects_get: decoding into freshbooks.Project: %v", err)
		}
	},
	"time_entries_create": func(t *testing.T, raw []byte) {
		var v freshbooks.TimeEntry
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Errorf("time_entries_create: decoding into freshbooks.TimeEntry: %v", err)
		}
	},
}

// TestRoundTrip exercises every registered tool through a real MCP client
// session (mcp.NewInMemoryTransports) against a server built by Register,
// backed by a lib client whose transport is redirected to a local fixture
// server. It is the [round trip] contract from docs/phases/3/plan.md: one
// row per registry entry, asserting the request the lib sent (method,
// path/scope id, body for writes, forwarded pagination/filter options),
// no IsError, and that a representative sample of StructuredContent
// values decode back into their real lib type.
func TestRoundTrip(t *testing.T) {
	upstream, rec := fakeUpstream(t)
	defer upstream.Close()
	clientSession := newTestSession(t, upstream, testScope, nil)
	ctx := context.Background()

	if len(All) != 169 {
		t.Fatalf("registry has %d tools, want 169", len(All))
	}

	for _, spec := range All {
		t.Run(spec.Name, func(t *testing.T) {
			rec.reset()
			args := synthWithProbes(spec.InputSchema)
			result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: spec.Name, Arguments: args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if result.IsError {
				t.Fatalf("tool returned IsError: %s", errorContentText(result))
			}

			req, ok := rec.last()
			if !ok {
				t.Fatalf("upstream recorded no request for %s", spec.Name)
			}
			assertScopeInPath(t, spec, req.path)
			assertMethodMatchesAnnotation(t, spec, req.method)
			assertProbesInQuery(t, spec, req.rawQuery)
			if !isReadOnly(spec) {
				assertBodyCarriesInput(t, spec, args, req.body)
			}
			if want, ok := visStateBodyExpectation[spec.Name]; ok {
				if !strings.Contains(string(req.body), want) {
					t.Errorf("%s: body %s does not contain %s", spec.Name, req.body, want)
				}
			}

			if result.StructuredContent != nil {
				raw, err := json.Marshal(result.StructuredContent)
				if err != nil {
					t.Fatalf("StructuredContent did not round-trip as JSON: %v", err)
				}
				if decode, ok := representativeDecode[spec.Name]; ok {
					decode(t, raw)
				}
			}
		})
	}
}
