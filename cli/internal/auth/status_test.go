package auth

import (
	"context"
	"testing"
	"time"

	fbauth "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

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
}
