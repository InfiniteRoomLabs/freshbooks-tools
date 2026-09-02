package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// testScope is the scope every round-trip invocation resolves against:
// valid-shaped, synthetic identifiers a real FreshBooks account would
// never carry.
var testScope = Scope{
	AccountID:    "ACM000TEST",
	BusinessID:   9000001,
	BusinessUUID: "00000000-0000-4000-8000-000000000099",
}

const (
	probePage    = 7
	probePerPage = 13
)

// recordedRequest is one HTTP request fakeUpstream answered.
type recordedRequest struct {
	method, path, rawQuery string
	body                   []byte
}

// requestRecorder collects every request a fakeUpstream server answers, in
// order, so a test can inspect what the CLI actually sent.
type requestRecorder struct {
	mu   sync.Mutex
	reqs []recordedRequest
}

func (r *requestRecorder) record(req *http.Request) {
	body, _ := io.ReadAll(req.Body) //nolint:errcheck // best-effort capture for assertions
	req.Body = io.NopCloser(bytes.NewReader(body))
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, recordedRequest{method: req.Method, path: req.URL.Path, rawQuery: req.URL.RawQuery, body: body})
}

func (r *requestRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = nil
}

func (r *requestRecorder) last() (recordedRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.reqs) == 0 {
		return recordedRequest{}, false
	}
	return r.reqs[len(r.reqs)-1], true
}

// fakeUpstream answers every request with a minimal, valid body for the
// family its path implies, mirroring mcp/internal/tools/roundtrip_test.go's
// fixture -- the same freshbooks library backs both surfaces, so the same
// decode paths need the same shapes. Every request is recorded before it
// is answered.
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
		case strings.Contains(path, "partners/applications") && r.Method == http.MethodGet:
			// Identity.Applications decodes {"response": [...]} directly
			// into []Application.
			_, _ = w.Write([]byte(`{"response": []}`))
		case strings.HasSuffix(path, "/team_members") && r.Method == http.MethodGet:
			// TeamMembers.List passes FamilyBusiness on an /auth/-rooted
			// path and decodes {"response": [...], "meta": {...}} flat.
			_, _ = w.Write([]byte(`{"response": [], "meta": {}}`))
		case strings.HasSuffix(path, "/threads") && r.Method == http.MethodGet:
			// Projects.Threads decodes straight into []map[string]any.
			_, _ = w.Write([]byte(`[]`))
		case strings.Contains(path, "checkout"):
			// Payments.{Create,Update}CheckoutLink require a non-empty id.
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

// visStateBodyExpectation closes the one swap risk no generic assertion
// below catches: bills archive and bills delete take an identical
// command shape (account scope + id, no CLI-level body) and PUT to the
// identical path, differing only in the vis_state literal the lib's own
// Archive/Delete hard-code. Swapping which lib method a Run closure calls
// would pass every other assertion in this file.
var visStateBodyExpectation = map[string]string{
	"bills/archive": `"vis_state":2`,
	"bills/delete":  `"vis_state":1`,
}

// extraFlagArgs supplies the flag/value pairs a command's ExtraFlags
// requires, keyed by "group/verb". Every entry here corresponds to a
// Command whose registry.go declaration sets ExtraFlags.
var extraFlagArgs = map[string][]string{
	"callbacks/verify":                           {"--verifier", "verifier-probe"},
	"payment-options/stripe-create-setup-intent": {"--payment-method", "pm_probe"},
	"service-rates/update-project-rate":          {"--project-id", "42", "--rate", "100.00"},
	"team-members/update-rate":                   {"--rate", "100.00"},
	"projects/create-thread":                     {"--message", "probe message"},
	"projects/add-thread-comment":                {"--message", "probe comment"},
}

// specialBodyContent overrides the generic "{}" body file for the one
// command whose lib method validates its request client-side before ever
// building an HTTP request: EstimatesService.Send rejects a body with no
// recipients (freshbooks/estimates.go), so an empty object never reaches
// the fixture server at all.
var specialBodyContent = map[string]string{
	"estimates/send": `{"email_recipients": ["probe@example.com"]}`,
}

// extraPositionalValue supplies a value for one of a command's
// ExtraPositional arguments.
func extraPositionalValue(name string) string {
	switch name {
	case "download-token":
		return "probe-download-token"
	default:
		return "probe-" + name
	}
}

// redirectTransport forces every outgoing request onto addr regardless of
// the scheme or host the lib resolved, so a fixture server on loopback
// can stand in for both api.freshbooks.com (the default base URL, which
// --base-url already overrides) and paid.freshbooks.com
// (payment_options_fb_pay_tokenize and payment_options_stripe_tokenize
// hard-code that host and https via Client.doOnHost, bypassing
// WithBaseURL entirely). No test in this file ever reaches the real
// network. Set via the package-level testTransport seam (state.go).
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

// useRedirectTransport points testTransport at upstream's host for the
// duration of the calling test, restoring it to nil on cleanup.
func useRedirectTransport(t *testing.T, upstream *httptest.Server) {
	t.Helper()
	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	testTransport = redirectTransport{addr: u.Host, next: http.DefaultTransport}
	t.Cleanup(func() { testTransport = nil })
}

// setupCredentials points $XDG_CONFIG_HOME at a temp dir carrying a
// valid, non-expiring credential for the "default" context, so
// buildClient's non-dry-run path never fails with "no credentials".
func setupCredentials(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	credDir := filepath.Join(dir, "freshbooks", "credentials")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tok := `{"access_token":"test-fixture-token","token_type":"Bearer"}`
	if err := os.WriteFile(filepath.Join(credDir, "default.json"), []byte(tok), 0o600); err != nil {
		t.Fatal(err)
	}
}

// buildArgs renders one command's full argv for the round-trip test:
// scope flags, page/search/include probes, the body/upload flag, the
// positional id and any ExtraPositional, and --yes for a destructive
// command.
func buildArgs(c Command, baseURL, bodyFile, uploadFile string) []string {
	args := []string{c.Group, c.Verb,
		"--account", string(testScope.AccountID),
		"--business", strconv.FormatInt(int64(testScope.BusinessID), 10),
		"--business-uuid", string(testScope.BusinessUUID),
		"--base-url", baseURL,
		"--output", "json",
	}

	if extra, ok := extraFlagArgs[c.Group+"/"+c.Verb]; ok {
		args = append(args, extra...)
	}

	if c.HasID {
		if c.IDKind == "string" {
			args = append(args, "probe-str-id")
		} else {
			args = append(args, "123")
		}
	}
	for _, name := range c.ExtraPositional {
		args = append(args, extraPositionalValue(name))
	}

	if c.List {
		if !c.NoPaging {
			args = append(args, "--page", strconv.Itoa(probePage), "--per-page", strconv.Itoa(probePerPage))
		}
		args = append(args, "--search", "probe_filter=probe_value")
		if c.HasInclude {
			args = append(args, "--include", "probe_include")
		}
	}

	if c.Body && (!c.BodyOptional) {
		args = append(args, "-f", bodyFile)
	}
	if c.Upload {
		args = append(args, "--file", uploadFile)
	}
	if c.Binary {
		args = append(args, "-o", "-")
	}
	if c.Class == ClassD {
		args = append(args, "--yes")
	}
	return args
}

func isReadOnly(c Command) bool { return c.Class == ClassRO }

// assertScopeInPath checks that the recorded request's path contains the
// scope identifier testScope supplied for whichever scope family c
// declares.
func assertScopeInPath(t *testing.T, c Command, path string) {
	t.Helper()
	switch c.Scope {
	case ScopeAccount:
		if !strings.Contains(path, string(testScope.AccountID)) {
			t.Errorf("%s/%s: path %q does not contain the account scope id %q", c.Group, c.Verb, path, testScope.AccountID)
		}
	case ScopeBusiness:
		bid := strconv.FormatInt(int64(testScope.BusinessID), 10)
		if !strings.Contains(path, bid) {
			t.Errorf("%s/%s: path %q does not contain the business scope id %q", c.Group, c.Verb, path, bid)
		}
	case ScopeBusinessUUID:
		if !strings.Contains(path, string(testScope.BusinessUUID)) {
			t.Errorf("%s/%s: path %q does not contain the business_uuid scope id %q", c.Group, c.Verb, path, testScope.BusinessUUID)
		}
	case ScopeAccountAndBusiness:
		bid := strconv.FormatInt(int64(testScope.BusinessID), 10)
		if !strings.Contains(path, string(testScope.AccountID)) || !strings.Contains(path, bid) {
			t.Errorf("%s/%s: path %q does not contain both scope ids", c.Group, c.Verb, path)
		}
	}
}

// assertMethodMatchesAnnotation checks the recorded HTTP method against
// c.Class: every RO command issues a GET, and every mutating class issues
// something else.
func assertMethodMatchesAnnotation(t *testing.T, c Command, method string) {
	t.Helper()
	if isReadOnly(c) {
		if method != http.MethodGet && !c.Binary {
			t.Errorf("%s/%s: method %s, want GET (RO)", c.Group, c.Verb, method)
		}
	} else if method == http.MethodGet {
		t.Errorf("%s/%s: method GET, want a mutating method (class %s)", c.Group, c.Verb, c.Class)
	}
}

// assertProbesInQuery checks that page/per_page/search/include, when the
// command registers them, reached the recorded query string as the exact
// key=value pair.
func assertProbesInQuery(t *testing.T, c Command, rawQuery string) {
	t.Helper()
	if !c.List {
		return
	}
	if !c.NoPaging {
		wantPage := "page=" + strconv.Itoa(probePage)
		if !strings.Contains(rawQuery, wantPage) {
			t.Errorf("%s/%s: query %q does not contain %s", c.Group, c.Verb, rawQuery, wantPage)
		}
		wantPerPage := "per_page=" + strconv.Itoa(probePerPage)
		if !strings.Contains(rawQuery, wantPerPage) {
			t.Errorf("%s/%s: query %q does not contain %s", c.Group, c.Verb, rawQuery, wantPerPage)
		}
	}
	if !strings.Contains(rawQuery, "probe_filter") || !strings.Contains(rawQuery, "probe_value") {
		t.Errorf("%s/%s: query %q does not carry the probe search filter", c.Group, c.Verb, rawQuery)
	}
	if c.HasInclude && !strings.Contains(rawQuery, "probe_include") {
		t.Errorf("%s/%s: query %q does not carry the probe include value", c.Group, c.Verb, rawQuery)
	}
}

// TestRoundTrip drives every registry command through the cobra tree
// in-process (Run), against a local fixture server, and asserts the
// request the CLI actually sent: method matches the annotation class, the
// scope id lands in the path, page/per_page/search/include (when
// registered) land in the query as exact key=value pairs, and exit 0.
func TestRoundTrip(t *testing.T) {
	if len(All) != 168 {
		t.Fatalf("registry has %d commands, want 168", len(All))
	}

	setupCredentials(t)
	upstream, rec := fakeUpstream(t)
	defer upstream.Close()
	useRedirectTransport(t, upstream)

	tmpDir := t.TempDir()
	bodyFile := filepath.Join(tmpDir, "body.json")
	if err := os.WriteFile(bodyFile, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	uploadFile := filepath.Join(tmpDir, "upload.txt")
	if err := os.WriteFile(uploadFile, []byte("probe upload content"), 0o600); err != nil {
		t.Fatal(err)
	}

	specialBodyFiles := make(map[string]string, len(specialBodyContent))
	for key, content := range specialBodyContent {
		path := filepath.Join(tmpDir, strings.ReplaceAll(key, "/", "_")+".json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		specialBodyFiles[key] = path
	}

	for _, c := range All {
		t.Run(c.Group+"_"+c.Verb, func(t *testing.T) {
			rec.reset()
			thisBodyFile := bodyFile
			if special, ok := specialBodyFiles[c.Group+"/"+c.Verb]; ok {
				thisBodyFile = special
			}
			args := buildArgs(c, upstream.URL, thisBodyFile, uploadFile)

			var stdout, stderr bytes.Buffer
			code := Run(args, strings.NewReader(""), &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("Run(%v) exit = %d, stderr = %s", args, code, stderr.String())
			}

			req, ok := rec.last()
			if !ok {
				t.Fatalf("upstream recorded no request for %s/%s", c.Group, c.Verb)
			}
			assertScopeInPath(t, c, req.path)
			assertMethodMatchesAnnotation(t, c, req.method)
			assertProbesInQuery(t, c, req.rawQuery)

			if c.Class != ClassRO && c.Class != ClassD {
				// A non-read-only, non-delete command with a required
				// body sent something.
				if c.Body && !c.BodyOptional && len(req.body) == 0 {
					t.Errorf("%s/%s: a non-read-only command with a required body sent an empty request body", c.Group, c.Verb)
				}
			}

			if want, ok := visStateBodyExpectation[c.Group+"/"+c.Verb]; ok {
				if !strings.Contains(string(req.body), want) {
					t.Errorf("%s/%s: body %s does not contain %s", c.Group, c.Verb, req.body, want)
				}
			}

			if !c.Binary {
				out := stdout.Bytes()
				if len(bytes.TrimSpace(out)) > 0 {
					var v any
					if err := json.Unmarshal(out, &v); err != nil {
						t.Errorf("%s/%s: -o json output did not parse as JSON: %v (%s)", c.Group, c.Verb, err, out)
					}
				}
			}
		})
	}
}

// TestRoundTripAllWalksTwoPages proves --all actually paginates: the
// fixture answers two non-empty pages of clients then an empty one, and
// clients list --all must collect all of the items across both pages
// instead of stopping after the first.
func TestRoundTripAllWalksTwoPages(t *testing.T) {
	setupCredentials(t)

	var calls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		q := r.URL.Query()
		page := q.Get("page")
		switch page {
		case "1", "":
			_, _ = w.Write([]byte(`{"response": {"result": {"clients": [{"id": 1}], "page": 1, "pages": 2, "per_page": 1, "total": 2}}}`))
		case "2":
			_, _ = w.Write([]byte(`{"response": {"result": {"clients": [{"id": 2}], "page": 2, "pages": 2, "per_page": 1, "total": 2}}}`))
		default:
			t.Errorf("unexpected page %q on call %d", page, call)
			_, _ = w.Write([]byte(`{"response": {"result": {"clients": [], "page": 3, "pages": 2, "per_page": 1, "total": 2}}}`))
		}
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	args := []string{"clients", "list",
		"--account", string(testScope.AccountID),
		"--base-url", srv.URL,
		"--output", "json",
		"--all",
	}
	code := Run(args, strings.NewReader(""), &stdout, &stderr, "test")
	if code != 0 {
		t.Fatalf("Run() exit = %d, stderr = %s", code, stderr.String())
	}

	var items []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &items); err != nil {
		t.Fatalf("output did not parse as a JSON array: %v (%s)", err, stdout.String())
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (both pages)", len(items))
	}
}

// TestRoundTripAllRejectsPage asserts --all combined with --page is a
// usage error (exit 2), per registry.go's Command.execute.
func TestRoundTripAllRejectsPage(t *testing.T) {
	setupCredentials(t)
	upstream, _ := fakeUpstream(t)
	defer upstream.Close()

	var stdout, stderr bytes.Buffer
	args := []string{"clients", "list",
		"--account", string(testScope.AccountID),
		"--base-url", upstream.URL,
		"--all", "--page", "2",
	}
	code := Run(args, strings.NewReader(""), &stdout, &stderr, "test")
	if code != 2 {
		t.Fatalf("Run() exit = %d, want 2; stderr = %s", code, stderr.String())
	}
}
