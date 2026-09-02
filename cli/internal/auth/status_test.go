package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	t.Run("[happy] revokes the refresh token and removes the file", func(t *testing.T) {
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
		got, err := Token(context.Background(), fbauth.Config{}, store, false)
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

		got, err := Token(context.Background(), cfg, store, true)
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

	t.Run("[sad] --refresh with no stored refresh token", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		if err := store.Save(context.Background(), &fbauth.Token{AccessToken: "tok"}); err != nil {
			t.Fatal(err)
		}
		if _, err := Token(context.Background(), fbauth.Config{}, store, true); err == nil {
			t.Fatal("Token() error = nil, want an error for no refresh token")
		}
	})

	t.Run("[sad] no stored token at all", func(t *testing.T) {
		store := fbauth.NewMemoryStore()
		if _, err := Token(context.Background(), fbauth.Config{}, store, false); err == nil {
			t.Fatal("Token() error = nil, want an error")
		}
	})

	t.Run("[sad] a store load error propagates", func(t *testing.T) {
		if _, err := Token(context.Background(), fbauth.Config{}, brokenLoadStore{}, false); err == nil {
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
		if _, err := Token(context.Background(), cfg, store, true); err == nil {
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
		if _, err := Token(context.Background(), cfg, store, true); err == nil {
			t.Fatal("Token() error = nil, want the store's save error")
		}
	})
}
