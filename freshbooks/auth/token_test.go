package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenValid(t *testing.T) {
	now := time.Unix(1756000000, 0).UTC()
	tests := []struct {
		name string
		tok  *Token
		want bool
	}{
		{"[sad] nil", nil, false},
		{"[sad] no access token", &Token{Expiry: now.Add(time.Hour)}, false},
		{"[happy] no expiry means non-expiring", &Token{AccessToken: "a"}, true},
		{"[happy] well inside its life", &Token{AccessToken: "a", Expiry: now.Add(time.Hour)}, true},
		{"[edge] inside the skew window counts as expired", &Token{AccessToken: "a", Expiry: now.Add(30 * time.Second)}, false},
		{"[edge] exactly at the skew boundary counts as expired", &Token{AccessToken: "a", Expiry: now.Add(DefaultExpirySkew)}, false},
		{"[sad] already expired", &Token{AccessToken: "a", Expiry: now.Add(-time.Second)}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.tok.Valid(now, DefaultExpirySkew); got != tc.want {
				t.Fatalf("Valid = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTokenStringRedacts(t *testing.T) {
	tok := &Token{AccessToken: "super-secret-access", RefreshToken: "super-secret-refresh"}
	for _, s := range []string{tok.String(), fmt.Sprintf("%v", tok), fmt.Sprintf("%+v", tok)} {
		if strings.Contains(s, "super-secret") {
			t.Fatalf("token rendering leaked a credential: %s", s)
		}
	}
	var nilTok *Token
	if nilTok.String() != "auth.Token(nil)" {
		t.Fatalf("nil rendering = %q", nilTok.String())
	}
}

func TestTokenClone(t *testing.T) {
	orig := &Token{AccessToken: "a", Scopes: []string{"user:profile:read"}}
	cp := orig.Clone()
	cp.Scopes[0] = "mutated"
	cp.AccessToken = "b"
	if orig.Scopes[0] != "user:profile:read" || orig.AccessToken != "a" {
		t.Fatal("Clone shares state with the original")
	}
	var nilTok *Token
	if nilTok.Clone() != nil {
		t.Fatal("cloning nil should yield nil")
	}
}

func TestStaticTokenSource(t *testing.T) {
	ts := StaticTokenSource("access-1")
	tok, err := ts.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "access-1" || tok.TokenType != "Bearer" {
		t.Fatalf("token = %+v", tok)
	}
	tok.AccessToken = "mutated"
	again, _ := ts.Token(context.Background())
	if again.AccessToken != "access-1" {
		t.Fatal("StaticTokenSource handed out a mutable reference")
	}
}

// refreshServer serves the rotated-token fixture and counts requests.
func refreshServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "refresh_token" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "auth", "token_rotated"))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// expiredToken is the pre-rotation pair the stores start out holding.
func expiredToken() *Token {
	return &Token{
		AccessToken:  "synthetic-access-token-0001",
		RefreshToken: "synthetic-refresh-token-0001",
		TokenType:    "Bearer",
		Expiry:       time.Unix(1756000000, 0).UTC().Add(-time.Hour),
	}
}

func TestRefreshingTokenSource(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a live token is returned without a refresh", func(t *testing.T) {
		srv, hits := refreshServer(t)
		store := NewMemoryStore()
		live := expiredToken()
		live.Expiry = time.Unix(1756000000, 0).UTC().Add(time.Hour)
		if err := store.Save(ctx, live); err != nil {
			t.Fatal(err)
		}

		tok, err := NewTokenSource(testConfig(srv), store).Token(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if tok.AccessToken != live.AccessToken {
			t.Fatalf("access token = %q", tok.AccessToken)
		}
		if hits.Load() != 0 {
			t.Fatalf("refreshed %d times, want 0", hits.Load())
		}
	})

	t.Run("[happy] the rotated pair is persisted before Token returns", func(t *testing.T) {
		srv, hits := refreshServer(t)
		store := NewMemoryStore()
		if err := store.Save(ctx, expiredToken()); err != nil {
			t.Fatal(err)
		}

		tok, err := NewTokenSource(testConfig(srv), store).Token(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if tok.AccessToken != "synthetic-access-token-0002" {
			t.Fatalf("access token = %q, want the rotated one", tok.AccessToken)
		}
		stored, err := store.Load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if stored.RefreshToken != tok.RefreshToken {
			t.Fatalf("store holds %q but the caller got %q", stored.RefreshToken, tok.RefreshToken)
		}
		if hits.Load() != 1 {
			t.Fatalf("refreshed %d times, want 1", hits.Load())
		}
	})

	t.Run("[sad] a store that cannot save fails the call rather than spending the token silently", func(t *testing.T) {
		srv, _ := refreshServer(t)
		store := &failingStore{inner: NewMemoryStore(), saveErr: errors.New("disk full")}
		if err := store.inner.Save(ctx, expiredToken()); err != nil {
			t.Fatal(err)
		}

		src := NewTokenSource(testConfig(srv), store)
		_, err := src.Token(ctx)
		if err == nil || !strings.Contains(err.Error(), "persisting the rotated token") {
			t.Fatalf("err = %v", err)
		}
		// The failure must not be papered over on a second call.
		if _, err := src.Token(ctx); err == nil {
			t.Fatal("the second call should fail too")
		}
	})

	t.Run("[sad] a refresh response with no refresh_token is rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"access_token": "a", "expires_in": 43200, "created_at": 1756000000}`))
		}))
		defer srv.Close()

		store := NewMemoryStore()
		if err := store.Save(ctx, expiredToken()); err != nil {
			t.Fatal(err)
		}
		_, err := NewTokenSource(testConfig(srv), store).Token(ctx)
		if err == nil || !strings.Contains(err.Error(), "no refresh_token") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] an empty store is ErrNoToken", func(t *testing.T) {
		srv, _ := refreshServer(t)
		_, err := NewTokenSource(testConfig(srv), NewMemoryStore()).Token(ctx)
		if !errors.Is(err, ErrNoToken) {
			t.Fatalf("err = %v, want ErrNoToken", err)
		}
	})

	t.Run("[sad] an expired token with no refresh token is ErrNoToken", func(t *testing.T) {
		srv, _ := refreshServer(t)
		store := NewMemoryStore()
		stale := expiredToken()
		stale.RefreshToken = ""
		if err := store.Save(ctx, stale); err != nil {
			t.Fatal(err)
		}
		_, err := NewTokenSource(testConfig(srv), store).Token(ctx)
		if !errors.Is(err, ErrNoToken) {
			t.Fatalf("err = %v, want ErrNoToken", err)
		}
	})

	t.Run("[sad] a store load failure is surfaced", func(t *testing.T) {
		srv, _ := refreshServer(t)
		store := &failingStore{inner: NewMemoryStore(), loadErr: errors.New("permission denied")}
		_, err := NewTokenSource(testConfig(srv), store).Token(ctx)
		if err == nil || !strings.Contains(err.Error(), "loading the stored token") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] a failing refresh is surfaced unchanged", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write(fixture(t, "auth", "token_error"))
		}))
		defer srv.Close()

		store := NewMemoryStore()
		if err := store.Save(ctx, expiredToken()); err != nil {
			t.Fatal(err)
		}
		_, err := NewTokenSource(testConfig(srv), store).Token(ctx)
		if !errors.Is(err, ErrInvalidGrant) {
			t.Fatalf("err = %v, want ErrInvalidGrant", err)
		}
	})

	t.Run("[corner] concurrent callers share a single refresh", func(t *testing.T) {
		srv, hits := refreshServer(t)
		store := NewMemoryStore()
		if err := store.Save(ctx, expiredToken()); err != nil {
			t.Fatal(err)
		}
		src := NewTokenSource(testConfig(srv), store)

		const callers = 32
		var wg sync.WaitGroup
		errs := make([]error, callers)
		toks := make([]string, callers)
		wg.Add(callers)
		for i := range callers {
			go func() {
				defer wg.Done()
				tok, err := src.Token(ctx)
				errs[i] = err
				if err == nil {
					toks[i] = tok.AccessToken
				}
			}()
		}
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				t.Fatalf("caller %d: %v", i, err)
			}
			if toks[i] != "synthetic-access-token-0002" {
				t.Fatalf("caller %d got %q", i, toks[i])
			}
		}
		if got := hits.Load(); got != 1 {
			t.Fatalf("the one-time-use refresh token was spent %d times, want 1", got)
		}
	})
}

// failingStore wraps a store and injects Load/Save failures.
type failingStore struct {
	inner   TokenStore
	loadErr error
	saveErr error
}

func (f *failingStore) Load(ctx context.Context) (*Token, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return f.inner.Load(ctx)
}

func (f *failingStore) Save(ctx context.Context, t *Token) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	return f.inner.Save(ctx, t)
}
