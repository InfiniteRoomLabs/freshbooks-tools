package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	libauth "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// fakeOAuthServer answers a minimal token + revoke endpoint pair for the
// auth subcommand tests: any authorization_code exchange succeeds, and
// any refresh_token exchange rotates to a new pair.
func fakeOAuthServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			fmt.Fprintf(w, `{"access_token":"fixture-access-token","refresh_token":"fixture-refresh-token","token_type":"Bearer","expires_in":43200,"created_at":%d}`, time.Now().Unix()) //nolint:errcheck
		case "refresh_token":
			fmt.Fprintf(w, `{"access_token":"rotated-access-token","refresh_token":"rotated-refresh-token","token_type":"Bearer","expires_in":43200,"created_at":%d}`, time.Now().Unix()) //nolint:errcheck
		default:
			http.Error(w, `{"error":"unsupported_grant_type"}`, http.StatusBadRequest)
		}
	})
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{}`) //nolint:errcheck
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// useFakeOAuthEndpoints points every auth subcommand's libauth.Config at
// srv for the duration of the calling test, restoring the zero value on
// cleanup, so no auth subcommand test can reach the real internet.
func useFakeOAuthEndpoints(t *testing.T, srv *httptest.Server) {
	t.Helper()
	testAuthEndpoints = libauth.Endpoints{
		AuthURL:   srv.URL + "/authorize",
		TokenURL:  srv.URL + "/token",
		RevokeURL: srv.URL + "/revoke",
	}
	t.Cleanup(func() { testAuthEndpoints = libauth.Endpoints{} })
}

// writeCredentials writes a raw credentials JSON body for context under
// dir (an XDG_CONFIG_HOME root), returning the file path.
func writeCredentials(t *testing.T, xdgConfigHome, context, json string) string {
	t.Helper()
	credDir := filepath.Join(xdgConfigHome, "freshbooks", "credentials")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(credDir, context+".json")
	if err := os.WriteFile(path, []byte(json), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fileExistsCLI(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestAuthLoginNoBrowser(t *testing.T) {
	t.Run("[happy] a bare pasted code exchanges and persists credentials", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		oauth := fakeOAuthServer(t)
		useFakeOAuthEndpoints(t, oauth)

		var stdout, stderr bytes.Buffer
		args := []string{"auth", "login", "--no-browser", "--client-id", "id", "--client-secret", "secret"}
		code := Run(args, strings.NewReader("bare-probe-code\n"), &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Open this URL") {
			t.Errorf("stdout = %q, want the authorization URL printed", stdout.String())
		}

		credPath := filepath.Join(dir, "freshbooks", "credentials", "default.json")
		if !fileExistsCLI(credPath) {
			t.Fatal("credentials file was not created")
		}
		if data, err := os.ReadFile(credPath); err != nil || !strings.Contains(string(data), "fixture-access-token") {
			t.Errorf("credentials file = %s, err = %v", data, err)
		}
	})

	t.Run("[sad] --client-id/--client-secret are required", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("FRESHBOOKS_CLIENT_ID", "")
		t.Setenv("FRESHBOOKS_CLIENT_SECRET", "")
		var stdout, stderr bytes.Buffer
		code := Run([]string{"auth", "login", "--no-browser"}, strings.NewReader("some-code\n"), &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})
}

func TestAuthStatusLogoutToken(t *testing.T) {
	t.Run("[edge] status on a context with no stored credentials", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		code := Run([]string{"auth", "status", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(strings.ToLower(stdout.String()), "false") {
			t.Errorf("stdout = %s, want LoggedIn false somewhere", stdout.String())
		}
	})

	t.Run("[happy] token prints exactly the stored access token", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		writeCredentials(t, dir, "default", `{"access_token":"probe-access-token","refresh_token":"probe-refresh-token"}`)

		var stdout, stderr bytes.Buffer
		code := Run([]string{"auth", "token"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if strings.TrimSpace(stdout.String()) != "probe-access-token" {
			t.Errorf("stdout = %q, want exactly the access token", stdout.String())
		}
	})

	t.Run("[happy] token --refresh rotates and persists before printing", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		writeCredentials(t, dir, "default", `{"access_token":"old","refresh_token":"old-refresh"}`)
		oauth := fakeOAuthServer(t)
		useFakeOAuthEndpoints(t, oauth)

		var stdout, stderr bytes.Buffer
		args := []string{"auth", "token", "--refresh", "--client-id", "id", "--client-secret", "secret"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if strings.TrimSpace(stdout.String()) != "rotated-access-token" {
			t.Errorf("stdout = %q, want the rotated access token", stdout.String())
		}

		credPath := filepath.Join(dir, "freshbooks", "credentials", "default.json")
		data, err := os.ReadFile(credPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "rotated-refresh-token") {
			t.Errorf("credentials file = %s, want the rotated refresh token persisted", data)
		}
	})

	t.Run("[happy] status reports a valid, non-expired token without printing it", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
		writeCredentials(t, dir, "default", `{"access_token":"secret-tok","refresh_token":"ref","expiry":"`+future+`"}`)

		var stdout, stderr bytes.Buffer
		code := Run([]string{"auth", "status", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if strings.Contains(stdout.String(), "secret-tok") {
			t.Errorf("auth status leaked the token: %s", stdout.String())
		}
	})

	t.Run("[happy] logout removes the credentials file", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		path := writeCredentials(t, dir, "default", `{"access_token":"tok"}`)
		oauth := fakeOAuthServer(t)
		useFakeOAuthEndpoints(t, oauth)

		var stdout, stderr bytes.Buffer
		code := Run([]string{"auth", "logout"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if fileExistsCLI(path) {
			t.Error("credentials file still exists after logout")
		}
	})

	t.Run("[edge] logout on a missing credentials file is not an error", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		code := Run([]string{"auth", "logout"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] token with no stored credentials is a runtime error", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		code := Run([]string{"auth", "token"}, discardStdin, &stdout, &stderr, "test")
		if code != 1 {
			t.Fatalf("exit = %d, want 1; stderr = %s", code, stderr.String())
		}
	})
}
