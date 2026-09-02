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

// TestDryRun is the D4 contract: --dry-run prints METHOD/URL/body, sends
// nothing, exits 0, and never prints an Authorization header.
func TestDryRun(t *testing.T) {
	t.Run("[happy] a GET command prints method and URL, sends nothing", func(t *testing.T) {
		setupCredentials(t)
		var upstreamHit bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upstreamHit = true
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "get", "123", "--account", "ACM000TEST", "--base-url", srv.URL, "--dry-run"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if upstreamHit {
			t.Error("--dry-run sent a real request to the upstream server")
		}
		out := stdout.String()
		if !strings.Contains(out, "GET") || !strings.Contains(out, "/accounting/account/ACM000TEST/users/clients/123") {
			t.Errorf("stdout = %q, want the METHOD and URL printed", out)
		}
		if strings.Contains(strings.ToLower(out), "authorization") || strings.Contains(strings.ToLower(out), "bearer") {
			t.Errorf("stdout leaked something Authorization-shaped: %q", out)
		}
	})

	t.Run("[happy] a write command prints the request body too", func(t *testing.T) {
		setupCredentials(t)
		path := writeTempJSON(t, `{"fname":"Ada"}`)

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "create", "--account", "ACM000TEST", "--base-url", "http://127.0.0.1:1", "--dry-run", "-f", path}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "POST") {
			t.Errorf("stdout = %q, want POST", stdout.String())
		}
		if !strings.Contains(stdout.String(), "Ada") {
			t.Errorf("stdout = %q, want the request body printed", stdout.String())
		}
	})

	t.Run("[happy] never retries and never actually connects even with an unroutable base-url", func(t *testing.T) {
		setupCredentials(t)
		var stdout, stderr bytes.Buffer
		// A deliberately unroutable address: if dry-run's NoRetry policy
		// were not wired up, the default 3-attempt policy with backoff
		// would make this test slow; instead it must return quickly.
		args := []string{"clients", "list", "--account", "ACM000TEST", "--base-url", "http://127.0.0.1:1", "--dry-run"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
	})
}

// writeTempJSON writes content to a temp file and returns its path.
func writeTempJSON(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
