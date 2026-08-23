//go:build integration

// Package freshbooks_test holds the cross-package seam tests: they exercise
// freshbooks and freshbooks/auth together against a fixture server, which is
// where the token lifecycle and the transport actually meet.
package freshbooks_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

func fixture(t *testing.T, area, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", area, name+".json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return b
}

// TestExpiryRefreshWriteBackRetry is the seam the whole auth design exists
// for: an expired access token triggers a refresh, the rotated pair reaches
// the store before the API request goes out, and the request that provoked
// it completes with the new token.
func TestExpiryRefreshWriteBackRetry(t *testing.T) {
	now := time.Unix(1756000000, 0).UTC()

	var refreshes, apiCalls atomic.Int32
	var sawAuthorization string

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		refreshes.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("parsing the refresh form: %v", err)
		}
		if got := r.Form.Get("refresh_token"); got != "synthetic-refresh-token-0001" {
			t.Errorf("refresh_token = %q, want the stored one", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "auth", "token_rotated"))
	})
	mux.HandleFunc("/auth/api/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		sawAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "auth", "users_me"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store := auth.NewMemoryStore()
	expired := &auth.Token{
		AccessToken:  "synthetic-access-token-0001",
		RefreshToken: "synthetic-refresh-token-0001",
		TokenType:    "Bearer",
		Expiry:       now.Add(-time.Minute),
	}
	if err := store.Save(context.Background(), expired); err != nil {
		t.Fatal(err)
	}

	cfg := auth.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoints:    auth.Endpoints{TokenURL: srv.URL + "/oauth/token"},
		HTTPClient:   srv.Client(),
		Now:          func() time.Time { return now },
	}

	c, err := freshbooks.NewClient(
		freshbooks.WithBaseURL(srv.URL),
		freshbooks.WithHTTPClient(srv.Client()),
		freshbooks.WithTokenSource(auth.NewTokenSource(cfg, store)),
		freshbooks.WithClock(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatal(err)
	}

	memberships, err := c.Identity.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if refreshes.Load() != 1 {
		t.Fatalf("refreshed %d times, want 1", refreshes.Load())
	}
	if apiCalls.Load() != 1 {
		t.Fatalf("called the API %d times, want 1", apiCalls.Load())
	}
	if sawAuthorization != "Bearer synthetic-access-token-0002" {
		t.Fatalf("the API saw %q, want the rotated access token", sawAuthorization)
	}
	stored, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stored.RefreshToken != "synthetic-refresh-token-0002" {
		t.Fatalf("the store holds %q, want the rotated refresh token", stored.RefreshToken)
	}
	if len(memberships) != 1 || memberships[0].AccountID != "ACM123" {
		t.Fatalf("memberships = %+v", memberships)
	}
}

// TestFileStoreSurvivesProcessRestart proves the rotated pair is durable:
// a second TokenSource built over the same file picks up where the first
// left off, and never touches the refresh endpoint again.
func TestFileStoreSurvivesProcessRestart(t *testing.T) {
	now := time.Unix(1756000000, 0).UTC()
	var refreshes atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		refreshes.Add(1)
		_, _ = w.Write(fixture(t, "auth", "token_rotated"))
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "config", "token.json")
	store := auth.NewFileStore(path)
	if err := store.Save(context.Background(), &auth.Token{
		AccessToken:  "synthetic-access-token-0001",
		RefreshToken: "synthetic-refresh-token-0001",
		Expiry:       now.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	cfg := auth.Config{
		Endpoints:  auth.Endpoints{TokenURL: srv.URL},
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return now },
	}

	if _, err := auth.NewTokenSource(cfg, store).Token(context.Background()); err != nil {
		t.Fatal(err)
	}

	// A "restarted process": a brand new store and source over the same file.
	tok, err := auth.NewTokenSource(cfg, auth.NewFileStore(path)).Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "synthetic-access-token-0002" {
		t.Fatalf("access token = %q", tok.AccessToken)
	}
	if refreshes.Load() != 1 {
		t.Fatalf("refreshed %d times, want 1 -- the rotated pair was not durable", refreshes.Load())
	}
}

// TestAllAcrossPagesWithMidStreamRateLimit walks a paginated endpoint whose
// second page rate-limits once, and asserts the iterator neither loses nor
// duplicates an item.
func TestAllAcrossPagesWithMidStreamRateLimit(t *testing.T) {
	const perPage, pages = 2, 3
	var limited atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			_, _ = fmt.Sscanf(p, "%d", &page)
		}
		if page == 2 && limited.CompareAndSwap(false, true) {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": "slow down"}`))
			return
		}

		items := make([]map[string]int, 0, perPage)
		for i := range perPage {
			items = append(items, map[string]int{"id": (page-1)*perPage + i + 1})
		}
		body, err := json.Marshal(map[string]any{
			"meta":     map[string]int{"page": page, "pages": pages, "per_page": perPage, "total": pages * perPage},
			"projects": items,
		})
		if err != nil {
			t.Errorf("encoding the page: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c, err := freshbooks.NewClient(
		freshbooks.WithBaseURL(srv.URL),
		freshbooks.WithHTTPClient(srv.Client()),
		freshbooks.WithRetry(freshbooks.RetryPolicy{
			MaxAttempts: 3,
			BaseDelay:   time.Millisecond,
			MaxDelay:    5 * time.Millisecond,
			Jitter:      func(d time.Duration) time.Duration { return d },
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	fetch := func(ctx context.Context, page int) (*freshbooks.Page[int], error) {
		var body struct {
			Meta     freshbooks.PageMeta `json:"meta"`
			Projects []struct {
				ID int `json:"id"`
			} `json:"projects"`
		}
		err := c.Do(ctx, http.MethodGet, fmt.Sprintf("/projects/business/1/projects?page=%d", page), nil, &body)
		if err != nil {
			return nil, err
		}
		p := &freshbooks.Page[int]{
			Page:    body.Meta.Page,
			Pages:   body.Meta.Pages,
			PerPage: body.Meta.PerPage,
			Total:   body.Meta.Total,
		}
		for _, item := range body.Projects {
			p.Items = append(p.Items, item.ID)
		}
		return p, nil
	}

	var got []int
	for id, err := range freshbooks.All(context.Background(), fetch) {
		if err != nil {
			t.Fatalf("iteration failed: %v", err)
		}
		got = append(got, id)
	}
	if fmt.Sprint(got) != "[1 2 3 4 5 6]" {
		t.Fatalf("got %v, want every item exactly once", got)
	}
	if !limited.Load() {
		t.Fatal("the mid-stream rate limit never fired")
	}
}
