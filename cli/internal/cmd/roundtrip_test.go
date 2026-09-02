package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
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

// recordedRequest is one HTTP request fakeUpstream answered. origHost is
// the request's Host as the lib built it, before useRedirectTransport
// rewrote scheme/host to reach the fixture server -- it is how
// TestRoundTrip's WantHost assertion (F12) can tell a request meant for
// paid.freshbooks.com from one meant for the default API host, even
// though both land on the same fixture server.
type recordedRequest struct {
	method, path, rawQuery, origHost, contentType string
	body                                          []byte
}

// requestRecorder collects every request a fakeUpstream server answers, in
// order, so a test can inspect what the CLI actually sent.
type requestRecorder struct {
	mu   sync.Mutex
	reqs []recordedRequest

	// pendingHost is the Host redirectTransport is about to rewrite away,
	// set just before it forwards the request and consumed by the very
	// next record() call -- the two run strictly sequentially for every
	// command this file drives (one HTTP request in flight at a time),
	// so there is never more than one pending value.
	pendingHost string
}

// setPendingHost is called by redirectTransport immediately before it
// rewrites a request's scheme/host to reach the fixture server, so the
// next record() can still report what the lib actually built (F12 --
// the two tokenization commands hard-code paid.freshbooks.com via
// Client.doOnHost, bypassing --base-url entirely).
func (r *requestRecorder) setPendingHost(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingHost = host
}

func (r *requestRecorder) record(req *http.Request) {
	body, _ := io.ReadAll(req.Body) //nolint:errcheck // best-effort capture for assertions
	req.Body = io.NopCloser(bytes.NewReader(body))
	r.mu.Lock()
	defer r.mu.Unlock()
	origHost := r.pendingHost
	r.pendingHost = ""
	r.reqs = append(r.reqs, recordedRequest{method: req.Method, path: req.URL.Path, rawQuery: req.URL.RawQuery, origHost: origHost, contentType: req.Header.Get("Content-Type"), body: body})
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
	rec  *requestRecorder
	next http.RoundTripper
}

func (t redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.rec != nil {
		t.rec.setPendingHost(req.URL.Host)
	}
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = t.addr
	req.Host = t.addr
	return t.next.RoundTrip(req)
}

// useRedirectTransport points testTransport at upstream's host for the
// duration of the calling test, restoring it to nil on cleanup. rec, when
// non-nil, records the pre-rewrite Host of every request (F12) so
// WantHost assertions can tell a paid.freshbooks.com request from a
// default-host one even though both land on the same fixture server.
func useRedirectTransport(t *testing.T, upstream *httptest.Server, rec *requestRecorder) {
	t.Helper()
	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	testTransport = redirectTransport{addr: u.Host, rec: rec, next: http.DefaultTransport}
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
		if c.HasSort {
			args = append(args, "--sort", "probe_sort_field:desc")
		}
	}
	if c.HasInclude {
		args = append(args, "--include", "probe_include")
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
	case ScopeNone:
		// No scope identifier to require (identity/me, identity/whoami,
		// identity/register, ledger-accounts/sub-types|types, ...).
	default:
		// F12/review B10: an unhandled ScopeFamily value must fail loudly
		// here rather than silently skip the scope check -- adding a new
		// family to registry.go without teaching this switch about it
		// would otherwise pass every command using it unconditionally.
		t.Errorf("%s/%s: assertScopeInPath has no case for scope family %v", c.Group, c.Verb, c.Scope)
	}
}

// wantPath is the expected request path template per "group/verb",
// captured against the real registry Run closures (F12/review B10) so a
// later change that swaps which lib method a command calls -- the same
// risk visStateBodyExpectation guards for bills archive/delete -- fails
// this assertion even when every other one in this file still passes.
// Placeholders: {account} (Scope's AccountID), {business} (BusinessID),
// {uuid} (BusinessUUID), {id} (the positional <id>, int or string).
var wantPath = map[string]string{
	"attachments/upload-expense-receipt":         "/uploads/account/{account}/attachments",
	"bill-payments/create":                       "/accounting/account/{account}/bill_payments/bill_payments",
	"bill-payments/update":                       "/accounting/account/{account}/bill_payments/bill_payments/{id}",
	"bill-vendors/create":                        "/accounting/account/{account}/bill_vendors/bill_vendors",
	"bill-vendors/delete":                        "/accounting/account/{account}/bill_vendors/bill_vendors/{id}",
	"bill-vendors/list":                          "/accounting/account/{account}/bill_vendors/bill_vendors",
	"bill-vendors/update":                        "/accounting/account/{account}/bill_vendors/bill_vendors/{id}",
	"bills/archive":                              "/accounting/account/{account}/bills/bills/{id}",
	"bills/create":                               "/accounting/account/{account}/bills/bills",
	"bills/delete":                               "/accounting/account/{account}/bills/bills/{id}",
	"bills/list":                                 "/accounting/account/{account}/bills/bills",
	"callbacks/delete":                           "/events/account/{account}/events/callbacks/{id}",
	"callbacks/list":                             "/events/account/{account}/events/callbacks",
	"callbacks/register":                         "/events/account/{account}/events/callbacks",
	"callbacks/resend-verification":              "/events/account/{account}/events/callbacks/{id}",
	"callbacks/verify":                           "/events/account/{account}/events/callbacks/{id}",
	"clients/create":                             "/accounting/account/{account}/users/clients",
	"clients/get":                                "/accounting/account/{account}/users/clients/{id}",
	"clients/list":                               "/accounting/account/{account}/users/clients",
	"clients/remove-all-secondary-contacts":      "/accounting/account/{account}/users/clients/{id}",
	"clients/update":                             "/accounting/account/{account}/users/clients/{id}",
	"contacts/delete":                            "/accounting/account/{account}/users/contacts/{id}",
	"contacts/update":                            "/accounting/account/{account}/users/contacts/{id}",
	"credit-notes/create":                        "/accounting/account/{account}/credit_notes/credit_notes",
	"credit-notes/delete":                        "/accounting/account/{account}/credit_notes/credit_notes/{id}",
	"credit-notes/list":                          "/accounting/account/{account}/credit_notes/credit_notes",
	"credit-notes/update":                        "/accounting/account/{account}/credit_notes/credit_notes/{id}",
	"estimates/accept":                           "/accounting/account/{account}/estimates/estimates/{id}",
	"estimates/create":                           "/accounting/account/{account}/estimates/estimates",
	"estimates/delete":                           "/accounting/account/{account}/estimates/estimates/{id}",
	"estimates/get":                              "/accounting/account/{account}/estimates/estimates/{id}",
	"estimates/list":                             "/accounting/account/{account}/estimates/estimates",
	"estimates/send":                             "/accounting/account/{account}/estimates/estimates/{id}",
	"estimates/update":                           "/accounting/account/{account}/estimates/estimates/{id}",
	"expense-categories/create":                  "/accounting/account/{account}/expenses/categories",
	"expense-categories/get":                     "/accounting/account/{account}/expenses/categories/{id}",
	"expense-categories/list":                    "/accounting/account/{account}/expenses/categories",
	"expenses/create":                            "/accounting/account/{account}/expenses/expenses",
	"expenses/create-recurring":                  "/accounting/account/{account}/expense_profiles/expense_profiles",
	"expenses/delete":                            "/accounting/account/{account}/expenses/expenses/{id}",
	"expenses/get":                               "/accounting/account/{account}/expenses/expenses/{id}",
	"expenses/list":                              "/accounting/account/{account}/expenses/expenses",
	"expenses/summaries":                         "/accounting/account/{account}/expenses/summaries",
	"expenses/update":                            "/accounting/account/{account}/expenses/expenses/{id}",
	"expenses/vendors":                           "/accounting/account/{account}/expenses/vendors",
	"gateways/get":                               "/payments/account/{account}/gateway",
	"identity/add-business":                      "/auth/api/v1/users/business",
	"identity/applications":                      "/auth/api/v1/partners/applications",
	"identity/create-application":                "/auth/api/v1/partners/applications",
	"identity/delete-business":                   "/auth/api/v1/users/business/{business}",
	"identity/delete-business-subscription":      "/auth/api/v1/billing/account/{account}/subscription",
	"identity/me":                                "/auth/api/v1/users/me",
	"identity/provision-payments":                "/payments/account/{account}/gateway/fbpay",
	"identity/register":                          "/auth/api/v1/smux/registrations",
	"identity/update-application":                "/auth/api/v1/partners/applications/{id}",
	"identity/whoami":                            "/auth/api/v1/users/me",
	"images/upload":                              "/uploads/account/{account}/images",
	"images/upload-without-account":              "/uploads/images",
	"invoice-profiles/create":                    "/accounting/account/{account}/invoice_profiles/invoice_profiles",
	"invoice-profiles/delete":                    "/accounting/account/{account}/invoice_profiles/invoice_profiles/{id}",
	"invoice-profiles/enable-payment-options":    "/payments/account/{account}/invoice_profile/{id}/payment_options",
	"invoice-profiles/get":                       "/accounting/account/{account}/invoice_profiles/invoice_profiles/{id}",
	"invoice-profiles/list":                      "/accounting/account/{account}/invoice_profiles/invoice_profiles",
	"invoice-profiles/update":                    "/accounting/account/{account}/invoice_profiles/invoice_profiles/{id}",
	"invoices/create":                            "/accounting/account/{account}/invoices/invoices",
	"invoices/delete":                            "/accounting/account/{account}/invoices/invoices/{id}",
	"invoices/enable-payment-options":            "/payments/account/{account}/invoice/{id}/payment_options",
	"invoices/get":                               "/accounting/account/{account}/invoices/invoices/{id}",
	"invoices/invoice-presentation-defaults":     "/accounting/account/{account}/invoices/presentations",
	"invoices/list":                              "/accounting/account/{account}/invoices/invoices",
	"invoices/pdf":                               "/accounting/account/{account}/invoices/invoices/{id}/pdf",
	"invoices/send":                              "/accounting/account/{account}/invoices/invoices/{id}",
	"invoices/share-link":                        "/accounting/account/{account}/invoices/invoices/{id}/share_link",
	"invoices/update":                            "/accounting/account/{account}/invoices/invoices/{id}",
	"items/create":                               "/accounting/account/{account}/items/items",
	"items/delete":                               "/accounting/account/{account}/items/items/{id}",
	"items/get":                                  "/accounting/account/{account}/items/items/{id}",
	"items/list":                                 "/accounting/account/{account}/items/items",
	"items/update":                               "/accounting/account/{account}/items/items/{id}",
	"journal-entries/create":                     "/accounting/account/{account}/journal_entries/journal_entries",
	"journal-entries/details":                    "/accounting/account/{account}/journal_entries/journal_entry_details",
	"journal-entry-accounts/list":                "/accounting/account/{account}/journal_entry_accounts/journal_entry_accounts",
	"ledger-accounts/create":                     "/accounting/businesses/{uuid}/ledger_accounts/accounts",
	"ledger-accounts/get":                        "/accounting/businesses/{uuid}/ledger_accounts/accounts/{id}",
	"ledger-accounts/list":                       "/accounting/businesses/{uuid}/ledger_accounts/accounts",
	"ledger-accounts/sub-type":                   "/accounting/ledger_accounts/sub_types/{id}",
	"ledger-accounts/sub-types":                  "/accounting/ledger_accounts/sub_types",
	"ledger-accounts/types":                      "/accounting/ledger_accounts/types",
	"ledger-accounts/update":                     "/accounting/businesses/{uuid}/ledger_accounts/accounts/{id}",
	"other-income/create":                        "/accounting/account/{account}/other_incomes/other_incomes",
	"other-income/delete":                        "/accounting/account/{account}/other_incomes/other_incomes/{id}",
	"other-income/list":                          "/accounting/account/{account}/other_incomes/other_incomes",
	"other-income/update":                        "/accounting/account/{account}/other_incomes/other_incomes/{id}",
	"payment-options/fb-pay-tokenize":            "/gateway/fbpay/tokenize",
	"payment-options/save-credit-card":           "/payments/account/{account}/credit-card",
	"payment-options/stripe-create-setup-intent": "/payments/account/{account}/gateway/stripe/credit-card/token",
	"payment-options/stripe-tokenize":            "/gateway/stripe/payment-method",
	"payments/create":                            "/accounting/account/{account}/payments/payments",
	"payments/create-checkout-link":              "/payments/account/{account}/checkout-links",
	"payments/delete":                            "/accounting/account/{account}/payments/payments/{id}",
	"payments/delete-checkout-link":              "/payments/account/{account}/checkout-links/{id}",
	"payments/get":                               "/accounting/account/{account}/payments/payments/{id}",
	"payments/list":                              "/accounting/account/{account}/payments/payments",
	"payments/update":                            "/accounting/account/{account}/payments/payments/{id}",
	"payments/update-checkout-link":              "/payments/account/{account}/checkout-links/{id}",
	"payments/update-checkout-link-gateway":      "/payments/account/{account}/checkout_link/{id}/payment_options",
	"projects/abilities":                         "/projects/business/{business}/abilities",
	"projects/add-thread-comment":                "/comments/business/{business}/threads/{id}/comments",
	"projects/create":                            "/projects/business/{business}/project",
	"projects/create-thread":                     "/comments/business/{business}/project/{id}/threads",
	"projects/delete":                            "/projects/business/{business}/project/{id}",
	"projects/get":                               "/projects/business/{business}/projects/{id}",
	"projects/list":                              "/projects/business/{business}/projects",
	"projects/threads":                           "/comments/business/{business}/project/{id}/threads",
	"projects/update":                            "/projects/business/{business}/project/{id}",
	"reports/accounts-aging":                     "/accounting/account/{account}/reports/accounting/accounts_aging",
	"reports/balance-sheet":                      "/accounting/account/{account}/reports/accounting/balance_sheet",
	"reports/bank-reconciliation-summary":        "/accounting/account/{account}/reports/accounting/bank_reconciliation_summary",
	"reports/client-account-statement":           "/accounting/account/{account}/reports/accounting/account_statement",
	"reports/download-invoice-details-csv":       "/accounting/account/{account}/links/reports/probe-download-token/invoice_details.csv",
	"reports/expense-details":                    "/accounting/account/{account}/reports/accounting/expense_details",
	"reports/invoice-details":                    "/accounting/account/{account}/reports/accounting/invoice_details",
	"reports/item-sales":                         "/accounting/account/{account}/reports/accounting/item_sales",
	"reports/payments-collected":                 "/accounting/account/{account}/reports/accounting/payments_collected",
	"reports/profit-loss":                        "/accounting/account/{account}/reports/accounting/profitloss_entity",
	"reports/revenue-by-client":                  "/accounting/account/{account}/reports/accounting/revenue_by_client",
	"reports/sales-tax-summary":                  "/accounting/account/{account}/reports/accounting/taxsummary",
	"reports/time-entry-details":                 "/comments/business/{business}/time_entries/search",
	"reports/trial-balance":                      "/accounting/account/{account}/reports/accounting/trial_balance",
	"retainers/create":                           "/comments/business/{business}/retainers",
	"retainers/delete":                           "/comments/business/{business}/retainer/{id}",
	"retainers/get":                              "/comments/business/{business}/retainer/{id}",
	"retainers/list":                             "/comments/business/{business}/retainers",
	"retainers/undelete":                         "/comments/business/{business}/retainer/{id}",
	"retainers/update":                           "/comments/business/{business}/retainer/{id}",
	"service-rates/get":                          "/comments/business/{business}/service/{id}/rate",
	"service-rates/list":                         "/comments/business/{business}/service_rates",
	"service-rates/update-project-rate":          "/comments/business/{business}/project/42/service/{id}/rate",
	"services/create":                            "/accounting/account/{account}/billable_items/billable_items",
	"services/get":                               "/comments/business/{business}/service/{id}",
	"services/get-billable-item":                 "/accounting/account/{account}/billable_items/{id}",
	"services/list":                              "/comments/business/{business}/services",
	"staff/delete":                               "/accounting/account/{account}/users/staffs/{id}",
	"staff/get":                                  "/accounting/account/{account}/users/staffs/{id}",
	"staff/list":                                 "/auth/api/v1/users/business/{business}",
	"staff/update":                               "/accounting/account/{account}/users/staffs/{id}",
	"systems/get":                                "/accounting/account/{account}/systems/systems/{business}",
	"tasks/create":                               "/accounting/account/{account}/projects/tasks",
	"tasks/delete":                               "/accounting/account/{account}/projects/tasks/{id}",
	"tasks/get":                                  "/accounting/account/{account}/projects/tasks/{id}",
	"tasks/list":                                 "/accounting/account/{account}/projects/tasks",
	"tasks/update":                               "/accounting/account/{account}/projects/tasks/{id}",
	"taxes/create":                               "/accounting/account/{account}/taxes/taxes",
	"taxes/delete":                               "/accounting/account/{account}/taxes/taxes/{id}",
	"taxes/get":                                  "/accounting/account/{account}/taxes/taxes/{id}",
	"taxes/list":                                 "/accounting/account/{account}/taxes/taxes",
	"taxes/update":                               "/accounting/account/{account}/taxes/taxes/{id}",
	"team-members/get":                           "/auth/api/v1/businesses/{business}/team_members/{id}",
	"team-members/invitation-rates":              "/comments/business/{business}/invitation_rates",
	"team-members/invite":                        "/auth/api/v1/users/invitation",
	"team-members/list":                          "/auth/api/v1/businesses/{business}/team_members",
	"team-members/rates":                         "/comments/business/{business}/team_member_rates",
	"team-members/update-rate":                   "/comments/business/{business}/team_member_rate/{id}",
	"time-entries/create":                        "/timetracking/business/{business}/time_entries",
	"time-entries/delete":                        "/timetracking/business/{business}/time_entries/{id}",
	"time-entries/list":                          "/timetracking/business/{business}/time_entries",
	"time-entries/search":                        "/timetracking/business/{business}/time_entries/search",
	"time-entries/update":                        "/timetracking/business/{business}/time_entries/{id}",
}

// wantHost overrides the expected request Host for the two commands whose
// lib method bypasses --base-url entirely via Client.doOnHost (F12/review
// B10): everything else in TestRoundTrip targets --base-url, so its
// origHost equals the fixture server's own host and needs no assertion.
var wantHost = map[string]string{
	"payment-options/fb-pay-tokenize": "paid.freshbooks.com",
	"payment-options/stripe-tokenize": "paid.freshbooks.com",
}

// resolveWantPath substitutes {account}/{business}/{uuid}/{id} in tmpl
// with the values a round-trip invocation of c actually used.
func resolveWantPath(c Command, tmpl string) string {
	out := tmpl
	out = strings.ReplaceAll(out, "{account}", string(testScope.AccountID))
	out = strings.ReplaceAll(out, "{business}", strconv.FormatInt(int64(testScope.BusinessID), 10))
	out = strings.ReplaceAll(out, "{uuid}", string(testScope.BusinessUUID))
	if c.HasID {
		if c.IDKind == "string" {
			out = strings.ReplaceAll(out, "{id}", "probe-str-id")
		} else {
			out = strings.ReplaceAll(out, "{id}", "123")
		}
	}
	return out
}

// assertWantPath checks the recorded path (and, for the two tokenization
// commands, the pre-rewrite Host) against wantPath/wantHost (F12/review
// B10).
func assertWantPath(t *testing.T, c Command, req recordedRequest) {
	t.Helper()
	key := c.Group + "/" + c.Verb
	tmpl, ok := wantPath[key]
	if !ok {
		t.Fatalf("%s: no wantPath entry -- every command in All must have one", key)
	}
	want := resolveWantPath(c, tmpl)
	if req.path != want {
		t.Errorf("%s: path = %q, want %q", key, req.path, want)
	}
	if wantHostVal, ok := wantHost[key]; ok {
		if req.origHost != wantHostVal {
			t.Errorf("%s: origHost = %q, want %q", key, req.origHost, wantHostVal)
		}
	}
}

// probeUploadContent and probeUploadFilename are the local file buildArgs
// points every Upload command's --file at; assertUploadBody checks both
// actually reached the multipart body (F16/review A5).
const (
	probeUploadContent  = "probe upload content"
	probeUploadFilename = "upload.txt"
)

// assertUploadBody parses req's multipart/form-data body and checks the
// one file part carries probeUploadContent under probeUploadFilename --
// not just that some non-empty multipart body was sent.
func assertUploadBody(t *testing.T, c Command, req recordedRequest) {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(req.contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		t.Errorf("%s/%s: Content-Type %q is not multipart/form-data", c.Group, c.Verb, req.contentType)
		return
	}
	mr := multipart.NewReader(bytes.NewReader(req.body), params["boundary"])
	part, err := mr.NextPart()
	if err != nil {
		t.Errorf("%s/%s: reading the multipart file part: %v", c.Group, c.Verb, err)
		return
	}
	if part.FileName() != probeUploadFilename {
		t.Errorf("%s/%s: multipart filename = %q, want %q", c.Group, c.Verb, part.FileName(), probeUploadFilename)
	}
	content, err := io.ReadAll(part)
	if err != nil {
		t.Errorf("%s/%s: reading the multipart file content: %v", c.Group, c.Verb, err)
		return
	}
	if string(content) != probeUploadContent {
		t.Errorf("%s/%s: multipart content = %q, want %q", c.Group, c.Verb, content, probeUploadContent)
	}
}

// binaryFixtureContent is the fixture bytes fakeUpstream answers a Binary
// command's request with, keyed by "group/verb": assertBinaryOutput
// checks these exact bytes reached stdout via -o - (F16/review A5), not
// just that -o - produced non-empty output.
var binaryFixtureContent = map[string][]byte{
	"invoices/pdf":                         []byte("%PDF-1.4\nfake\n%%EOF"),
	"reports/download-invoice-details-csv": []byte("a,b\n1,2\n"),
}

func assertBinaryOutput(t *testing.T, c Command, stdout []byte) {
	t.Helper()
	key := c.Group + "/" + c.Verb
	want, ok := binaryFixtureContent[key]
	if !ok {
		t.Fatalf("%s: no binaryFixtureContent entry -- every Binary command in All must have one", key)
	}
	if !bytes.Equal(stdout, want) {
		t.Errorf("%s: -o - stdout = %q, want the fixture bytes %q", key, stdout, want)
	}
}

// assertMethodMatchesAnnotation checks the recorded HTTP method against
// c.Class: every RO command issues a GET, and every mutating class issues
// something else.
func assertMethodMatchesAnnotation(t *testing.T, c Command, method string) {
	t.Helper()
	if isReadOnly(c) {
		if method != http.MethodGet {
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
	if c.HasInclude && !strings.Contains(rawQuery, "probe_include") {
		t.Errorf("%s/%s: query %q does not carry the probe include value", c.Group, c.Verb, rawQuery)
	}
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
	if c.HasSort && !strings.Contains(rawQuery, "sort=probe_sort_field_desc") {
		t.Errorf("%s/%s: query %q does not carry the probe --sort value", c.Group, c.Verb, rawQuery)
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
	useRedirectTransport(t, upstream, rec)

	tmpDir := t.TempDir()
	bodyFile := filepath.Join(tmpDir, "body.json")
	if err := os.WriteFile(bodyFile, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	uploadFile := filepath.Join(tmpDir, probeUploadFilename)
	if err := os.WriteFile(uploadFile, []byte(probeUploadContent), 0o600); err != nil {
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
			assertWantPath(t, c, req)

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

			if c.Upload {
				// F16/review A5: the multipart body actually carries the
				// local file's content and its base filename, not just a
				// non-empty body.
				assertUploadBody(t, c, req)
			}

			if c.Binary {
				// F16/review A5: the fixture's own bytes made it all the
				// way to stdout via -o -, not just "-o json produced
				// something".
				assertBinaryOutput(t, c, stdout.Bytes())
			} else {
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
