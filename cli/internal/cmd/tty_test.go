package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// withTerminals swaps stdinIsTerminal and/or stdoutIsTerminal for the
// duration of the calling test (F11/review B9), restoring the previous
// values on cleanup. Passing nil for either leaves it untouched.
func withTerminals(t *testing.T, stdinTTY, stdoutTTY *bool) {
	t.Helper()
	prevIn, prevOut := stdinIsTerminal, stdoutIsTerminal
	if stdinTTY != nil {
		v := *stdinTTY
		stdinIsTerminal = func(io.Reader) bool { return v }
	}
	if stdoutTTY != nil {
		v := *stdoutTTY
		stdoutIsTerminal = func(io.Writer) bool { return v }
	}
	t.Cleanup(func() {
		stdinIsTerminal, stdoutIsTerminal = prevIn, prevOut
	})
}

func boolPtr(b bool) *bool { return &b }

// TestYesGateTTY exercises registry.go's ClassD --yes gate (Command.execute)
// against every combination of a TTY-attached stdin and --yes, by swapping
// stdinIsTerminal rather than needing a real pty (F11/review B9).
func TestYesGateTTY(t *testing.T) {
	t.Run("[sad] D-class + TTY + no --yes is a usage error", func(t *testing.T) {
		withTerminals(t, boolPtr(true), nil)
		setupCredentials(t)
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "remove-all-secondary-contacts", "--account", "ACM000TEST", "123"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[happy] D-class + TTY + --yes succeeds", func(t *testing.T) {
		withTerminals(t, boolPtr(true), nil)
		setupCredentials(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"response":{"result":{}}}`))
		}))
		defer srv.Close()
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "remove-all-secondary-contacts", "--account", "ACM000TEST", "--base-url", srv.URL, "--yes", "123"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[happy] D-class + non-TTY + no --yes succeeds (nothing to confirm)", func(t *testing.T) {
		withTerminals(t, boolPtr(false), nil)
		setupCredentials(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"response":{"result":{}}}`))
		}))
		defer srv.Close()
		var stdout, stderr bytes.Buffer
		args := []string{"clients", "remove-all-secondary-contacts", "--account", "ACM000TEST", "--base-url", srv.URL, "123"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr = %s", code, stderr.String())
		}
	})
}

// TestOutputDefaultTTY covers state.go's DefaultFormat(stdoutIsTerminal(...))
// seam: -o defaults to table on a TTY stdout and json otherwise, for a
// command that never reaches the network or reads any flag but --output
// (F11/review B9).
func TestOutputDefaultTTY(t *testing.T) {
	t.Run("[happy] table on a TTY stdout", func(t *testing.T) {
		withTerminals(t, nil, boolPtr(true))
		setupCredentials(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"response":{"result":{"id":1,"name":"probe"}}}`))
		}))
		defer srv.Close()
		var stdout, stderr bytes.Buffer
		args := []string{"taxes", "get", "--account", "ACM000TEST", "--base-url", srv.URL, "1"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr = %s", code, stderr.String())
		}
		if bytes.HasPrefix(bytes.TrimSpace(stdout.Bytes()), []byte("{")) {
			t.Errorf("stdout = %q, want table output (not JSON) on a TTY", stdout.String())
		}
	})

	t.Run("[happy] json on a non-TTY stdout", func(t *testing.T) {
		withTerminals(t, nil, boolPtr(false))
		setupCredentials(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"response":{"result":{"id":1,"name":"probe"}}}`))
		}))
		defer srv.Close()
		var stdout, stderr bytes.Buffer
		args := []string{"taxes", "get", "--account", "ACM000TEST", "--base-url", srv.URL, "1"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, want 0; stderr = %s", code, stderr.String())
		}
		if !bytes.HasPrefix(bytes.TrimSpace(stdout.Bytes()), []byte("{")) {
			t.Errorf("stdout = %q, want JSON output on a non-TTY", stdout.String())
		}
	})
}

// TestErrorIsJSONTTY covers root.go's errorIsJSON, the top-level error
// path's own -o resolution (F11/review B9): it must agree with the
// TTY-sensitive default when no -o/--output flag or FRESHBOOKS_OUTPUT env
// var is present.
func TestErrorIsJSONTTY(t *testing.T) {
	t.Run("[happy] JSON error object on a non-TTY stdout", func(t *testing.T) {
		withTerminals(t, nil, boolPtr(false))
		var stdout, stderr bytes.Buffer
		code := Run([]string{"bogus-command"}, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
		if !bytes.HasPrefix(bytes.TrimSpace(stderr.Bytes()), []byte("{")) {
			t.Errorf("stderr = %q, want a JSON error object on a non-TTY", stderr.String())
		}
	})

	t.Run("[happy] plain error line on a TTY stdout", func(t *testing.T) {
		withTerminals(t, nil, boolPtr(true))
		var stdout, stderr bytes.Buffer
		code := Run([]string{"bogus-command"}, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
		if bytes.HasPrefix(bytes.TrimSpace(stderr.Bytes()), []byte("{")) {
			t.Errorf("stderr = %q, want a plain error line on a TTY, not JSON", stderr.String())
		}
	})
}
