package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIdentityApplicationsRedaction is F25/F30 (security A3-A4): `identity
// applications` must never print a fixture client_secret in its default
// output, in either json or table format, and must print it when the
// caller explicitly opts in with --show-secrets.
func TestIdentityApplicationsRedaction(t *testing.T) {
	const fixtureSecret = "probe-fixture-client-secret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":[{"client_id":"probe-client-id","client_secret":"` + fixtureSecret + `","name":"Probe App","redirect_uri":"https://example.invalid/callback"}]}`))
	}))
	defer srv.Close()

	t.Run("[happy] json output redacts client_secret by default", func(t *testing.T) {
		setupCredentials(t)
		var stdout, stderr bytes.Buffer
		args := []string{"identity", "applications", "--base-url", srv.URL, "--output", "json"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if strings.Contains(stdout.String(), fixtureSecret) {
			t.Errorf("stdout leaked the client_secret: %s", stdout.String())
		}
		if !strings.Contains(stdout.String(), "probe-client-id") {
			t.Errorf("stdout = %s, want the rest of the application still printed", stdout.String())
		}
	})

	t.Run("[happy] table output redacts client_secret by default", func(t *testing.T) {
		setupCredentials(t)
		var stdout, stderr bytes.Buffer
		args := []string{"identity", "applications", "--base-url", srv.URL, "--output", "table"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if strings.Contains(stdout.String(), fixtureSecret) {
			t.Errorf("stdout leaked the client_secret: %s", stdout.String())
		}
	})

	t.Run("[happy] --show-secrets includes the client_secret", func(t *testing.T) {
		setupCredentials(t)
		var stdout, stderr bytes.Buffer
		args := []string{"identity", "applications", "--base-url", srv.URL, "--output", "json", "--show-secrets"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), fixtureSecret) {
			t.Errorf("stdout = %s, want the client_secret with --show-secrets", stdout.String())
		}
	})
}
