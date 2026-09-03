package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	fbauth "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// brokenLoadStore is a fbauth.TokenStore whose Load always fails with a
// non-ErrNoToken error, for exercising a caller's generic-error branch.
type brokenLoadStore struct{}

func (brokenLoadStore) Load(context.Context) (*fbauth.Token, error) {
	return nil, errors.New("disk on fire")
}
func (brokenLoadStore) Save(context.Context, *fbauth.Token) error { return nil }

// brokenSaveStore wraps a working store but always fails Save, for
// exercising a caller's "persisting the rotated token" error branch.
type brokenSaveStore struct{ *fbauth.MemoryStore }

func (brokenSaveStore) Save(context.Context, *fbauth.Token) error {
	return errors.New("disk full")
}

// TestStatusInfoJSONTags is G5/QA Q10: StatusInfo must marshal to
// snake_case keys, matching the D8 fold's Page[T]/User/Membership
// convention, not the Go-cased field names json.Marshal defaults to
// without explicit tags.
func TestStatusInfoJSONTags(t *testing.T) {
	info := StatusInfo{
		Context: "work", CredentialsPath: "/path/to/creds.json",
		LoggedIn: true, Valid: true,
		Expiry: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		Scopes: []string{"user:clients:read"},
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{`"context"`, `"credentials_path"`, `"logged_in"`, `"valid"`, `"expiry"`, `"scopes"`} {
		if !strings.Contains(got, want) {
			t.Errorf("json = %s, want %s", got, want)
		}
	}
	for _, notWant := range []string{`"Context"`, `"CredentialsPath"`, `"LoggedIn"`, `"Valid"`, `"Expiry"`, `"Scopes"`} {
		if strings.Contains(got, notWant) {
			t.Errorf("json = %s, want no Go-cased field name %s", got, notWant)
		}
	}
}

func TestStatus(t *testing.T) {
	t.Run("[happy] a valid stored token", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		now := time.Now()
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "tok", RefreshToken: "ref", Expiry: now.Add(time.Hour), Scopes: []string{"user:clients:read"}}); err != nil {
			t.Fatal(err)
		}

		info, err := Status(context.Background(), "work", "/path/to/creds.json", store, func() time.Time { return now })
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if !info.LoggedIn || !info.Valid {
			t.Fatalf("got %+v, want LoggedIn=true Valid=true", info)
		}
		if info.Context != "work" || info.CredentialsPath != "/path/to/creds.json" {
			t.Errorf("got %+v", info)
		}
		if len(info.Scopes) != 1 || info.Scopes[0] != "user:clients:read" {
			t.Errorf("Scopes = %v", info.Scopes)
		}
	})

	t.Run("[edge] an expired stored token is LoggedIn but not Valid", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		now := time.Now()
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "tok", Expiry: now.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
		info, err := Status(context.Background(), "work", "path", store, func() time.Time { return now })
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if !info.LoggedIn || info.Valid {
			t.Fatalf("got %+v, want LoggedIn=true Valid=false", info)
		}
	})

	t.Run("[edge] no stored token", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		info, err := Status(context.Background(), "work", "path", store, nil)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if info.LoggedIn || info.Valid {
			t.Fatalf("got %+v, want LoggedIn=false Valid=false", info)
		}
	})

	t.Run("[happy] a nil now func defaults to time.Now for a valid token", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "tok", Expiry: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
		info, err := Status(context.Background(), "work", "path", store, nil)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if !info.LoggedIn || !info.Valid {
			t.Fatalf("got %+v, want LoggedIn=true Valid=true", info)
		}
	})

	t.Run("[sad] a store error other than ErrNoToken propagates", func(t *testing.T) {
		if _, err := Status(context.Background(), "work", "path", brokenLoadStore{}, nil); err == nil {
			t.Fatal("Status() error = nil, want the store's error")
		}
	})
}

func TestLogout(t *testing.T) {
	t.Run("[happy] revokes both the access and refresh token and removes the file", func(t *testing.T) {
		oauth := newFakeOAuthServer(t, "id", "secret", "")
		store := fbauth.NewMemoryStore()
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "tok", RefreshToken: "ref"}); err != nil {
			t.Fatal(err)
		}
		cfg := fbauth.Config{ClientID: "id", ClientSecret: "secret", Endpoints: oauth.endpoints()}

		// Logout removes a file path; MemoryStore has no file, so exercise
		// the file-removal branch against a real temp file that stands in
		// for the credentials path (the store and the path are two
		// separate things Logout coordinates, exactly as the CLI wires a
		// FileStore's own path in as credentialsPath).
		path := t.TempDir() + "/creds.json"
		if err := writeFixtureFile(path); err != nil {
			t.Fatal(err)
		}

		if err := Logout(context.Background(), cfg, path, store); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if fileExists(path) {
			t.Error("credentials file still exists after Logout")
		}
		revoked := oauth.revokedTokens()
		if !slices.Contains(revoked, "ref") {
			t.Errorf("revoked = %v, want the refresh token %q posted", revoked, "ref")
		}
		if !slices.Contains(revoked, "tok") {
			t.Errorf("revoked = %v, want the access token %q posted", revoked, "tok")
		}
	})

	t.Run("[edge] a missing credentials file is not an error", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		cfg := fbauth.Config{}
		if err := Logout(context.Background(), cfg, "/nonexistent/path/creds.json", store); err != nil {
			t.Fatalf("Logout() error = %v, want nil for a missing file", err)
		}
	})

	t.Run("[edge] no refresh token stored still removes the file", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		cfg := fbauth.Config{}
		path := t.TempDir() + "/creds.json"
		if err := writeFixtureFile(path); err != nil {
			t.Fatal(err)
		}
		if err := Logout(context.Background(), cfg, path, store); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
		if fileExists(path) {
			t.Error("credentials file still exists after Logout")
		}
	})

	t.Run("[sad] a removal failure other than \"missing\" is returned", func(t *testing.T) {
		// A non-empty directory at the "credentials file" path: os.Remove
		// fails with "directory not empty", not os.ErrNotExist.
		dir := t.TempDir()
		credPath := filepath.Join(dir, "not-a-file")
		if err := os.Mkdir(credPath, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(credPath, "child"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		store := fbauth.NewMemoryStore()
		if err := Logout(context.Background(), fbauth.Config{}, credPath, store); err == nil {
			t.Fatal("Logout() error = nil, want a removal error for a non-empty directory")
		}
	})
}

func TestToken(t *testing.T) {
	t.Run("[happy] returns the stored access token without refreshing", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "tok", RefreshToken: "ref"}); err != nil {
			t.Fatal(err)
		}
		got, err := Token(context.Background(), fbauth.Config{}, store, false, nil)
		if err != nil {
			t.Fatalf("Token() error = %v", err)
		}
		if got != "tok" {
			t.Errorf("got %q, want tok", got)
		}
	})

	t.Run("[happy] --refresh rotates and persists before returning", func(t *testing.T) {
		oauth := newFakeOAuthServer(t, "id", "secret", "")
		store := fbauth.NewMemoryStore()
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "old", RefreshToken: "ref-1"}); err != nil {
			t.Fatal(err)
		}
		cfg := fbauth.Config{ClientID: "id", ClientSecret: "secret", Endpoints: oauth.endpoints()}

		got, err := Token(context.Background(), cfg, store, true, nil)
		if err != nil {
			t.Fatalf("Token() error = %v", err)
		}
		if got != "rotated-access-token" {
			t.Errorf("got %q, want rotated-access-token", got)
		}
		saved, err := store.Load(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if saved.RefreshToken != "rotated-refresh-token" {
			t.Errorf("stored refresh token = %q, want the rotated one persisted before Token returned", saved.RefreshToken)
		}
	})

	t.Run("[sad] an expired token refreshes instead of printing the dead one", func(t *testing.T) {
		// Phase 7 (live, 2026-09-03): `auth token` without --refresh
		// printed an already-expired token and exited 0, so the
		// documented TOKEN=$(freshbooks auth token) idiom handed every
		// caller a credential that 401s on first use.
		oauth := newFakeOAuthServer(t, "id", "secret", "")
		store := fbauth.NewMemoryStore()
		expired := &fbauth.Token{
			AccessToken:  "stale",
			RefreshToken: "ref-1",
			Expiry:       time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC),
		}
		if err := store.Save(context.Background(), expired); err != nil {
			t.Fatal(err)
		}
		cfg := fbauth.Config{ClientID: "id", ClientSecret: "secret", Endpoints: oauth.endpoints()}
		now := func() time.Time { return time.Date(2026, 9, 3, 13, 0, 0, 0, time.UTC) }

		got, err := Token(context.Background(), cfg, store, false, now)
		if err != nil {
			t.Fatalf("Token() error = %v", err)
		}
		if got != "rotated-access-token" {
			t.Errorf("got %q, want the refreshed token", got)
		}
		saved, err := store.Load(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if saved.RefreshToken != "rotated-refresh-token" {
			t.Errorf("stored refresh token = %q, want the rotation persisted", saved.RefreshToken)
		}
	})

	t.Run("[edge] a token expiring inside the skew is refreshed too", func(t *testing.T) {
		oauth := newFakeOAuthServer(t, "id", "secret", "")
		store := fbauth.NewMemoryStore()
		expiry := time.Date(2026, 9, 3, 13, 0, 0, 0, time.UTC)
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "stale", RefreshToken: "ref-1", Expiry: expiry}); err != nil {
			t.Fatal(err)
		}
		cfg := fbauth.Config{ClientID: "id", ClientSecret: "secret", Endpoints: oauth.endpoints()}
		now := func() time.Time { return expiry.Add(-fbauth.DefaultExpirySkew / 2) }

		got, err := Token(context.Background(), cfg, store, false, now)
		if err != nil {
			t.Fatalf("Token() error = %v", err)
		}
		if got != "rotated-access-token" {
			t.Errorf("got %q, want the refreshed token", got)
		}
	})

	t.Run("[edge] a still-valid token is printed without a network call", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		expiry := time.Date(2026, 9, 3, 13, 0, 0, 0, time.UTC)
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "fresh", RefreshToken: "ref-1", Expiry: expiry}); err != nil {
			t.Fatal(err)
		}
		// An empty Config cannot reach any OAuth endpoint, so this only
		// passes if no refresh was attempted.
		now := func() time.Time { return expiry.Add(-2 * fbauth.DefaultExpirySkew) }
		got, err := Token(context.Background(), fbauth.Config{}, store, false, now)
		if err != nil {
			t.Fatalf("Token() error = %v", err)
		}
		if got != "fresh" {
			t.Errorf("got %q, want fresh", got)
		}
	})

	t.Run("[sad] an expired token with no refresh token says so", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		expiry := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "stale", Expiry: expiry}); err != nil {
			t.Fatal(err)
		}
		now := func() time.Time { return expiry.Add(time.Hour) }
		_, err := Token(context.Background(), fbauth.Config{}, store, false, now)
		if err == nil {
			t.Fatal("Token() error = nil, want an expired-with-no-refresh-token error")
		}
		if !strings.Contains(err.Error(), "expired") {
			t.Errorf("error = %v, want it to name the expiry", err)
		}
	})

	t.Run("[sad] --refresh with no stored refresh token", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "tok"}); err != nil {
			t.Fatal(err)
		}
		if _, err := Token(context.Background(), fbauth.Config{}, store, true, nil); err == nil {
			t.Fatal("Token() error = nil, want an error for no refresh token")
		}
	})

	t.Run("[sad] no stored token at all", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		if _, err := Token(context.Background(), fbauth.Config{}, store, false, nil); err == nil {
			t.Fatal("Token() error = nil, want an error")
		}
	})

	t.Run("[sad] a store load error propagates", func(t *testing.T) {
		if _, err := Token(context.Background(), fbauth.Config{}, brokenLoadStore{}, false, nil); err == nil {
			t.Fatal("Token() error = nil, want the store's error")
		}
	})

	t.Run("[sad] --refresh surfaces the OAuth server's error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"invalid_grant"}`) //nolint:errcheck
		}))
		defer srv.Close()

		store := fbauth.NewMemoryStore()
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "old", RefreshToken: "old-refresh"}); err != nil {
			t.Fatal(err)
		}
		cfg := fbauth.Config{Endpoints: fbauth.Endpoints{TokenURL: srv.URL}}
		if _, err := Token(context.Background(), cfg, store, true, nil); err == nil {
			t.Fatal("Token() error = nil, want the OAuth server's error")
		}
	})

	t.Run("[sad] a save failure after a successful refresh is returned", func(t *testing.T) {
		oauth := newFakeOAuthServer(t, "id", "secret", "")
		inner := fbauth.NewMemoryStore()
		if err := inner.Save(context.Background(), &fbauth.Token{AccessToken: "old", RefreshToken: "old-refresh"}); err != nil {
			t.Fatal(err)
		}
		store := brokenSaveStore{inner}
		cfg := fbauth.Config{ClientID: "id", ClientSecret: "secret", Endpoints: oauth.endpoints()}
		if _, err := Token(context.Background(), cfg, store, true, nil); err == nil {
			t.Fatal("Token() error = nil, want the store's save error")
		}
	})
}
