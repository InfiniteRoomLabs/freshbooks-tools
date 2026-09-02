package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMissingScope_BusinessUUID exercises resolveScope's ScopeBusinessUUID
// branch, not otherwise hit by TestExitCodes' ScopeAccount case.
func TestMissingScope_BusinessUUID(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("FRESHBOOKS_BUSINESS_UUID", "")
	var stdout, stderr bytes.Buffer
	args := []string{"ledger-accounts", "list", "--base-url", "http://127.0.0.1:1"}
	code := Run(args, discardStdin, &stdout, &stderr, "test")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
	}
}

// TestInvalidIntID exercises execute()'s int64 id-parse failure branch.
func TestInvalidIntID(t *testing.T) {
	setupCredentials(t)
	var stdout, stderr bytes.Buffer
	args := []string{"clients", "get", "not-a-number", "--account", "ACM000TEST", "--base-url", "http://127.0.0.1:1"}
	code := Run(args, discardStdin, &stdout, &stderr, "test")
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
	}
}

// TestCollectAllStopsOnError proves --all propagates a mid-walk page
// error instead of silently truncating.
func TestCollectAllStopsOnError(t *testing.T) {
	setupCredentials(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "1" || page == "" {
			_, _ = w.Write([]byte(`{"response": {"result": {"clients": [{"id": 1}], "page": 1, "pages": 2, "per_page": 1, "total": 2}}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"response":{"errors":[{"message":"boom"}]}}`))
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	args := []string{"clients", "list", "--account", "ACM000TEST", "--base-url", srv.URL, "--all"}
	code := Run(args, discardStdin, &stdout, &stderr, "test")
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr = %s", code, stderr.String())
	}
	// F17/review A6: --all collects every page before ever formatting
	// output, so a page-2 failure must mean stdout saw nothing at all --
	// not page 1's item silently rendered before the error surfaced.
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty (page 1 must not have been printed)", stdout.String())
	}
}

// TestBuildClient_CorruptCredentials exercises buildClient's non-ErrNoToken
// store.Load error branch.
func TestBuildClient_CorruptCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	writeCredentials(t, dir, "default", "{not valid json")

	var stdout, stderr bytes.Buffer
	args := []string{"clients", "get", "123", "--account", "ACM000TEST", "--base-url", "http://127.0.0.1:1"}
	code := Run(args, discardStdin, &stdout, &stderr, "test")
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (a runtime error, not a clean auth error); stderr = %s", code, stderr.String())
	}
	// G3/QA Q5: without this, deleting the non-ErrNoToken branch entirely
	// and treating a corrupt file as an empty token would still exit 1
	// against this same unroutable --base-url (a real request attempt
	// fails too) -- assert the message actually names the parse failure.
	if !strings.Contains(stderr.String(), "parsing") || !strings.Contains(stderr.String(), "default.json") {
		t.Errorf("stderr = %s, want it to name the credentials parse failure", stderr.String())
	}
}

// TestLoadConfig_UnreadableFile exercises loadConfig's error-wrapping
// branch.
func TestLoadConfig_UnreadableFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgDir := filepath.Join(dir, "freshbooks")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("current-context: x\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(cfgPath, 0o600) //nolint:errcheck // best-effort restore for TempDir cleanup

	// F21/review A12: run `config view` on every platform regardless;
	// only the exit-code assertion is conditional, since root bypasses
	// the permission bit that makes this file unreadable (windows was
	// never guarded here either -- os.Chmod(..., 0o000) is a no-op on
	// windows, which behaves the same way root does on unix).
	var stdout, stderr bytes.Buffer
	code := Run([]string{"config", "view"}, discardStdin, &stdout, &stderr, "test")
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Logf("exit = %d, stderr = %s (windows or root bypasses file permissions, not asserting)", code, stderr.String())
	} else if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr = %s", code, stderr.String())
	}
}

// TestSetContextExplicitConfigFlag exercises the --config flag override
// path in loadConfig, not otherwise reached by tests that rely on
// XDG_CONFIG_HOME.
func TestSetContextExplicitConfigFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "myconfig.yaml")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"config", "set-context", "work", "--account", "ACM1", "--config", path}, discardStdin, &stdout, &stderr, "test")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	// F17/review A6: read the file back, not just confirm it exists --
	// an empty or wrong-content file at the right path would otherwise
	// pass this test.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected the config file at the explicit --config path: %v", err)
	}
	if !strings.Contains(string(data), "work") || !strings.Contains(string(data), "ACM1") {
		t.Errorf("config file = %s, want the \"work\" context and \"ACM1\" account persisted", data)
	}
}
