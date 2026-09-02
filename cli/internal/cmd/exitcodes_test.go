package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExitCodes drives one command per D6 exit code end to end through
// Run, asserting both the code and (where relevant) the stderr shape.
func TestExitCodes(t *testing.T) {
	t.Run("[happy] 0 on success", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"version"}, discardStdin, &stdout, &stderr, "1.2.3")
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 1 on a 500 API error", func(t *testing.T) {
		setupCredentials(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"response":{"errors":[{"message":"boom"}]}}`))
		}))
		defer srv.Close()

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "get", "--account", "ACM000TEST", "--base-url", srv.URL, "123"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 1 {
			t.Fatalf("exit = %d, want 1; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 2 on an unknown command", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"bogus-command"}, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 2 on malformed JSON in --file", func(t *testing.T) {
		setupCredentials(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"response":{"result":{}}}`))
		}))
		defer srv.Close()

		path := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "create", "--account", "ACM000TEST", "--base-url", srv.URL, "-f", path}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 2 on malformed JSON in --file, even with no stored credentials", func(t *testing.T) {
		// F13/review A1: the body is validated in execute() before
		// buildClient runs, so a bad --file body is a usage error (exit
		// 2) on a machine with no credentials at all, never an auth
		// error (exit 3) discovered only because a client was built for
		// a call that was never going to happen.
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		path := filepath.Join(t.TempDir(), "bad.json")
		if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "create", "--account", "ACM000TEST", "-f", path}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 2 on a missing required extra flag, even with no stored credentials", func(t *testing.T) {
		// Same ordering guarantee for RequiredFlags (callbacks verify's
		// --verifier): a missing required flag is a usage error before
		// buildClient ever runs.
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		var stdout, stderr bytes.Buffer
		args := []string{"callbacks", "verify", "--account", "ACM000TEST", "1"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 2 on a missing scope", func(t *testing.T) {
		// An isolated, empty XDG_CONFIG_HOME: no config.yaml, no
		// credentials, so nothing on this machine's real environment can
		// accidentally supply the account scope this test expects to be
		// missing.
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("FRESHBOOKS_ACCOUNT_ID", "")
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "get", "123", "--base-url", "http://127.0.0.1:1"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "missing required scope") {
			t.Errorf("stderr = %q, want it to name the missing scope", stderr.String())
		}
	})

	t.Run("[sad] 2 on --all combined with --page", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "list", "--account", "ACM000TEST", "--base-url", "http://127.0.0.1:1", "--all", "--page", "2"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 2 when a required --file is missing on a Body command", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "create", "--account", "ACM000TEST"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 2 when a required --file is missing on an Upload command", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		args := []string{"images", "upload", "--account", "ACM000TEST"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 2 on an invalid --output value", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		args := []string{"config", "view", "--output", "bogus"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 3 when no credentials are stored for the context", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // empty: no credentials file exists
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "get", "--account", "ACM000TEST", "--base-url", "http://127.0.0.1:1", "123"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 3 {
			t.Fatalf("exit = %d, want 3; stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "auth login") {
			t.Errorf("stderr = %q, want a hint to run auth login", stderr.String())
		}
	})

	t.Run("[sad] 3 on a 401 API error", func(t *testing.T) {
		setupCredentials(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthenticated","error_description":"bad token"}`))
		}))
		defer srv.Close()

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "get", "--account", "ACM000TEST", "--base-url", srv.URL, "123"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 3 {
			t.Fatalf("exit = %d, want 3; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] 4 on a 404 API error", func(t *testing.T) {
		setupCredentials(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"response":{"errors":[{"message":"not found"}]}}`))
		}))
		defer srv.Close()

		var stdout, stderr bytes.Buffer
		args := []string{"clients", "get", "--account", "ACM000TEST", "--base-url", srv.URL, "123"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 4 {
			t.Fatalf("exit = %d, want 4; stderr = %s", code, stderr.String())
		}
	})
}

// TestLogLevelValidatedOnEveryPath is G1/QA Q1: buildClient's dry-run and
// no-credential branches both returned before buildLogger's own
// validation ran, so --log-level bogus silently succeeded (dry-run) or
// surfaced as an auth error (no credentials) instead of exit 2 on every
// path.
func TestLogLevelValidatedOnEveryPath(t *testing.T) {
	t.Run("[sad] --dry-run", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "list", "--account", "ACM000TEST", "--log-level", "bogus", "--dry-run"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stdout = %s stderr = %s", code, stdout.String(), stderr.String())
		}
	})

	t.Run("[sad] no stored credentials", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "list", "--account", "ACM000TEST", "--log-level", "bogus"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] valid stored credentials", func(t *testing.T) {
		setupCredentials(t)
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "list", "--account", "ACM000TEST", "--log-level", "bogus"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})
}

// TestInvalidContextIsUsageError is G5/QA Q11: an invalid --context used
// to reach buildClient's live path and surface as exit 1 (a runtimeError
// wrapping CredentialsPath's rejection), inconsistent with every other
// bad global flag value (--output, --log-level, --sort, --timeout), all
// of which are exit 2.
func TestInvalidContextIsUsageError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	args := []string{"clients", "get", "123", "--account", "ACM000TEST", "--context", "../evil"}
	code := Run(args, discardStdin, &stdout, &stderr, "test")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
	}
}

// TestJSONErrorObject checks the -o json error shape D6 documents:
// {"error": {"status", "code", "message", "field", "family", "exit"}}.
func TestJSONErrorObject(t *testing.T) {
	setupCredentials(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"response":{"errors":[{"errno":404,"message":"no such client","field":"id"}]}}`))
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	args := []string{"clients", "get", "--account", "ACM000TEST", "--base-url", srv.URL, "--output", "json", "123"}
	code := Run(args, discardStdin, &stdout, &stderr, "test")
	if code != 4 {
		t.Fatalf("exit = %d, want 4; stderr = %s", code, stderr.String())
	}

	var obj struct {
		Error struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
			Field   string `json:"field"`
			Family  string `json:"family"`
			Exit    int    `json:"exit"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &obj); err != nil {
		t.Fatalf("stderr is not a JSON error object: %v (%s)", err, stderr.String())
	}
	if obj.Error.Status != 404 || obj.Error.Exit != 4 || obj.Error.Field != "id" {
		t.Errorf("got %+v", obj.Error)
	}
}

// TestJSONErrorObject_UsageError checks that a usage error (which never
// wraps a *freshbooks.Error) still renders as a valid JSON object with
// just message and exit under -o json.
func TestJSONErrorObject_UsageError(t *testing.T) {
	setupCredentials(t)
	var stdout, stderr bytes.Buffer
	args := []string{"clients", "list", "--account", "ACM000TEST", "--base-url", "http://127.0.0.1:1", "--output", "json", "--all", "--page", "2"}
	code := Run(args, discardStdin, &stdout, &stderr, "test")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
	}
	var obj struct {
		Error struct {
			Message string `json:"message"`
			Exit    int    `json:"exit"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &obj); err != nil {
		t.Fatalf("stderr is not a JSON error object: %v (%s)", err, stderr.String())
	}
	if obj.Error.Exit != 2 || obj.Error.Message == "" {
		t.Errorf("got %+v", obj.Error)
	}
}
