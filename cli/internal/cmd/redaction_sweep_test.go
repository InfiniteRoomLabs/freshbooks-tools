package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRedactionSweep is F29/simplify: the promised redaction test that
// did not exist. Five scenarios -- config view, auth status, a
// --dry-run, an API 500, and a 401 -- each run with --log-level debug so
// the debug log (freshbooks/transport.go's request/response log lines,
// written to the same stderr the CLI's own error output goes to) is part
// of what gets checked, and every one asserts the fixture access token,
// refresh token, and client secret never appear in stdout or stderr.
func TestRedactionSweep(t *testing.T) {
	const (
		fixtureAccess       = "fixture-secret-access-token-abc123"
		fixtureRefresh      = "fixture-secret-refresh-token-def456"
		fixtureClientSecret = "fixture-secret-client-secret-ghi789"
	)

	assertNoLeak := func(t *testing.T, stdout, stderr string) {
		t.Helper()
		combined := stdout + "\n" + stderr
		for _, secret := range []string{fixtureAccess, fixtureRefresh, fixtureClientSecret} {
			if strings.Contains(combined, secret) {
				t.Errorf("leaked %q\nstdout: %s\nstderr: %s", secret, stdout, stderr)
			}
		}
	}

	t.Run("config view", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("FRESHBOOKS_CLIENT_SECRET", fixtureClientSecret)
		writeCredentials(t, dir, "default", `{"access_token":"`+fixtureAccess+`","refresh_token":"`+fixtureRefresh+`"}`)

		var stdout, stderr bytes.Buffer
		code := Run([]string{"config", "view", "--log-level", "debug", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		assertNoLeak(t, stdout.String(), stderr.String())
	})

	t.Run("auth status", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("FRESHBOOKS_CLIENT_SECRET", fixtureClientSecret)
		writeCredentials(t, dir, "default", `{"access_token":"`+fixtureAccess+`","refresh_token":"`+fixtureRefresh+`"}`)

		var stdout, stderr bytes.Buffer
		code := Run([]string{"auth", "status", "--log-level", "debug", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		assertNoLeak(t, stdout.String(), stderr.String())
	})

	t.Run("a --dry-run", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("FRESHBOOKS_CLIENT_SECRET", fixtureClientSecret)
		writeCredentials(t, dir, "default", `{"access_token":"`+fixtureAccess+`","refresh_token":"`+fixtureRefresh+`"}`)
		bodyPath := filepath.Join(t.TempDir(), "body.json")
		if err := os.WriteFile(bodyPath, []byte(`{"fname":"Ada"}`), 0o600); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "create", "--account", "ACM000TEST", "--base-url", "http://127.0.0.1:1", "--dry-run", "-f", bodyPath, "--log-level", "debug"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		assertNoLeak(t, stdout.String(), stderr.String())
	})

	t.Run("an API 500", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("FRESHBOOKS_CLIENT_SECRET", fixtureClientSecret)
		writeCredentials(t, dir, "default", `{"access_token":"`+fixtureAccess+`","refresh_token":"`+fixtureRefresh+`"}`)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"response":{"errors":[{"message":"boom"}]}}`))
		}))
		defer srv.Close()

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "get", "123", "--account", "ACM000TEST", "--base-url", srv.URL, "--log-level", "debug"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 1 {
			t.Fatalf("exit = %d, want 1; stderr = %s", code, stderr.String())
		}
		assertNoLeak(t, stdout.String(), stderr.String())
	})

	t.Run("a 401", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		t.Setenv("FRESHBOOKS_CLIENT_SECRET", fixtureClientSecret)
		writeCredentials(t, dir, "default", `{"access_token":"`+fixtureAccess+`","refresh_token":"`+fixtureRefresh+`"}`)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"response":{"errors":[{"message":"unauthorized"}]}}`))
		}))
		defer srv.Close()

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "get", "123", "--account", "ACM000TEST", "--base-url", srv.URL, "--log-level", "debug"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 3 {
			t.Fatalf("exit = %d, want 3; stderr = %s", code, stderr.String())
		}
		assertNoLeak(t, stdout.String(), stderr.String())
	})
}
