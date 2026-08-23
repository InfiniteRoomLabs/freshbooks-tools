package freshbooks

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// fixedClock is the instant every test's WithClock returns.
var fixedClock = func() time.Time { return time.Unix(1756000000, 0).UTC() }

// testRetry is a deterministic, fast retry policy: no randomness, no real
// waiting.
func testRetry(attempts int) RetryPolicy {
	return RetryPolicy{
		MaxAttempts: attempts,
		BaseDelay:   time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		Jitter:      func(d time.Duration) time.Duration { return d },
	}
}

// newTestClient starts a fixture server running h and returns a client
// pointed at it, plus the server for URL assertions.
func newTestClient(t *testing.T, h http.Handler, opts ...Option) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	base := []Option{
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithTokenSource(auth.StaticTokenSource("test-access-token")),
		WithClock(fixedClock),
		WithRetry(testRetry(1)),
	}
	c, err := NewClient(append(base, opts...)...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

// serveFixture answers every request with testdata/<area>/<name>.json.
func serveFixture(t *testing.T, status int, area, name string) http.HandlerFunc {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", area, name+".json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}
}

func TestNewClientDefaults(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("[happy] base URL", func(t *testing.T) {
		if c.BaseURL() != DefaultBaseURL {
			t.Fatalf("BaseURL = %q", c.BaseURL())
		}
	})
	t.Run("[happy] user agent names the library and version", func(t *testing.T) {
		if !strings.HasPrefix(c.userAgent, "freshbooks-go/") || !strings.HasSuffix(c.userAgent, Version) {
			t.Fatalf("userAgent = %q", c.userAgent)
		}
	})
	t.Run("[happy] retry policy", func(t *testing.T) {
		if c.retry.MaxAttempts != 3 {
			t.Fatalf("MaxAttempts = %d", c.retry.MaxAttempts)
		}
	})
	t.Run("[happy] every service field is wired", func(t *testing.T) {
		services := []any{
			c.Identity, c.Clients, c.Contacts, c.Invoices, c.InvoiceProfiles,
			c.Expenses, c.ExpenseCategories, c.Estimates, c.Payments, c.Items,
			c.Taxes, c.Bills, c.BillPayments, c.BillVendors, c.BillableItems,
			c.CreditNotes, c.OtherIncome, c.JournalEntries, c.JournalEntryAccounts,
			c.LedgerAccounts, c.Reports, c.Systems, c.Staff, c.Tasks, c.Projects,
			c.TimeEntries, c.Services, c.ServiceRates, c.TeamMembers, c.Retainers,
			c.Callbacks, c.Attachments, c.Images, c.Gateways, c.CheckoutLinks,
			c.PaymentOptions,
		}
		if len(services) != 36 {
			t.Fatalf("expected 36 services, listed %d", len(services))
		}
		for i, s := range services {
			if s == nil {
				t.Fatalf("service %d is nil", i)
			}
		}
		if c.Identity.client != c {
			t.Fatal("a service does not point back at its client")
		}
	})
	t.Run("[happy] a redirect guard is installed", func(t *testing.T) {
		if c.httpClient.CheckRedirect == nil {
			t.Fatal("CheckRedirect should be set")
		}
	})
}

func TestOptions(t *testing.T) {
	t.Run("[happy] every option applies", func(t *testing.T) {
		logger := slog.New(slog.DiscardHandler)
		ts := auth.StaticTokenSource("a")
		hc := &http.Client{}
		c, err := NewClient(
			WithTokenSource(ts),
			WithHTTPClient(hc),
			WithBaseURL("https://sandbox.example.test/api/"),
			WithUserAgent("my-app/1.0"),
			WithLogger(logger),
			WithRetry(NoRetry),
			WithClock(fixedClock),
			nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		if c.BaseURL() != "https://sandbox.example.test/api" {
			t.Errorf("BaseURL = %q, want the trailing slash trimmed", c.BaseURL())
		}
		if c.userAgent != "my-app/1.0" || c.logger != logger || c.tokenSource != ts {
			t.Error("an option did not apply")
		}
		if c.retry.MaxAttempts != 1 {
			t.Errorf("MaxAttempts = %d", c.retry.MaxAttempts)
		}
		if !c.now().Equal(fixedClock()) {
			t.Error("WithClock did not apply")
		}
		if c.httpClient == hc {
			t.Error("WithHTTPClient must copy rather than mutate the caller's client")
		}
		if hc.CheckRedirect != nil {
			t.Error("the caller's client was mutated")
		}
	})

	t.Run("[happy] a caller's CheckRedirect is left alone", func(t *testing.T) {
		called := false
		hc := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			called = true
			return nil
		}}
		c, err := NewClient(WithHTTPClient(hc))
		if err != nil {
			t.Fatal(err)
		}
		_ = c.httpClient.CheckRedirect(httptest.NewRequest(http.MethodGet, "/", nil), nil)
		if !called {
			t.Fatal("the caller's CheckRedirect was replaced")
		}
	})

	tests := []struct {
		name string
		opt  Option
	}{
		{"[sad] nil token source", WithTokenSource(nil)},
		{"[sad] nil http client", WithHTTPClient(nil)},
		{"[sad] unparseable base URL", WithBaseURL("://nope")},
		{"[sad] relative base URL", WithBaseURL("/api")},
		{"[sad] scheme-only base URL", WithBaseURL("https://")},
		{"[sad] empty user agent", WithUserAgent("")},
		{"[sad] nil logger", WithLogger(nil)},
		{"[sad] zero attempts", WithRetry(RetryPolicy{})},
		{"[sad] nil clock", WithClock(nil)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(tc.opt); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestRetryPolicyDelay(t *testing.T) {
	identity := func(d time.Duration) time.Duration { return d }
	p := RetryPolicy{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second, Jitter: identity}

	tests := []struct {
		name       string
		attempt    int
		retryAfter time.Duration
		want       time.Duration
	}{
		{"[happy] first backoff is the base delay", 1, 0, 100 * time.Millisecond},
		{"[happy] each attempt doubles", 2, 0, 200 * time.Millisecond},
		{"[happy] and again", 3, 0, 400 * time.Millisecond},
		{"[edge] capped at MaxDelay", 9, 0, time.Second},
		{"[happy] Retry-After wins over the computed backoff", 1, 750 * time.Millisecond, 750 * time.Millisecond},
		{"[edge] Retry-After is capped too", 1, time.Hour, time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := p.delay(tc.attempt, tc.retryAfter); got != tc.want {
				t.Fatalf("delay = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("[edge] a zero base delay means no wait", func(t *testing.T) {
		if got := (RetryPolicy{MaxAttempts: 3}).delay(1, 0); got != 0 {
			t.Fatalf("delay = %v", got)
		}
	})
	t.Run("[edge] a zero MaxDelay falls back to 30s", func(t *testing.T) {
		got := RetryPolicy{MaxAttempts: 3, BaseDelay: time.Minute, Jitter: identity}.delay(1, 0)
		if got != 30*time.Second {
			t.Fatalf("delay = %v", got)
		}
	})
	t.Run("[happy] default jitter stays inside the window", func(t *testing.T) {
		p := RetryPolicy{MaxAttempts: 3, BaseDelay: time.Second, MaxDelay: time.Minute}
		for range 50 {
			d := p.delay(1, 0)
			if d < 0 || d >= time.Second {
				t.Fatalf("jittered delay %v is outside [0, 1s)", d)
			}
		}
		if got := p.jitter(0); got != 0 {
			t.Fatalf("jitter(0) = %v", got)
		}
	})
}

func TestFamilyForPath(t *testing.T) {
	tests := map[string]Family{
		"/accounting/account/ACM123/users/clients":             FamilyAccounting,
		"/accounting/businesses/uuid/ledger_accounts/accounts": FamilyAccounting,
		"/auth/api/v1/users/me":                                FamilyAuth,
		"/projects/business/1/projects":                        FamilyBusiness,
		"/timetracking/business/1/time_entries":                FamilyBusiness,
		"/events/account/ACM123/events/callbacks":              FamilyBusiness,
	}
	for path, want := range tests {
		if got := familyForPath(path); got != want {
			t.Errorf("familyForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestStripAuthOnCrossHostRedirect(t *testing.T) {
	mk := func(host string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "https://"+host+"/x", nil)
		r.Header.Set("Authorization", "Bearer secret")
		return r
	}

	t.Run("[happy] same host keeps the header", func(t *testing.T) {
		req, via := mk("api.example.test"), []*http.Request{mk("api.example.test")}
		if err := stripAuthOnCrossHostRedirect(req, via); err != nil {
			t.Fatal(err)
		}
		if req.Header.Get("Authorization") == "" {
			t.Fatal("header was stripped on a same-host redirect")
		}
	})

	t.Run("[happy] a different host drops the header", func(t *testing.T) {
		req, via := mk("evil.example.test"), []*http.Request{mk("api.example.test")}
		if err := stripAuthOnCrossHostRedirect(req, via); err != nil {
			t.Fatal(err)
		}
		if req.Header.Get("Authorization") != "" {
			t.Fatal("the bearer token survived a cross-host redirect")
		}
	})

	t.Run("[corner] a redirect loop is capped", func(t *testing.T) {
		via := make([]*http.Request, 10)
		for i := range via {
			via[i] = mk("api.example.test")
		}
		if err := stripAuthOnCrossHostRedirect(mk("api.example.test"), via); err != http.ErrUseLastResponse {
			t.Fatalf("err = %v, want http.ErrUseLastResponse", err)
		}
	})

	t.Run("[edge] the first request has no predecessor", func(t *testing.T) {
		req := mk("api.example.test")
		if err := stripAuthOnCrossHostRedirect(req, nil); err != nil {
			t.Fatal(err)
		}
		if req.Header.Get("Authorization") == "" {
			t.Fatal("header was stripped without a redirect")
		}
	})
}

func TestCrossHostRedirectEndToEnd(t *testing.T) {
	var got string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"meta": {}, "projects": []}`))
	}))
	defer target.Close()

	redirector := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/projects/business/1/projects", http.StatusFound)
	})
	c, _ := newTestClient(t, redirector)

	var out map[string]any
	if err := c.Do(context.Background(), http.MethodGet, "/projects/business/1/projects", nil, &out); err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("the bearer token followed the redirect to another host: %q", got)
	}
}
