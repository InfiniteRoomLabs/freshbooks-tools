package cmd

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/output"
	"github.com/spf13/cobra"
)

func TestIsTerminalIO(t *testing.T) {
	t.Run("[edge] a bytes.Buffer is never a terminal", func(t *testing.T) {
		if isTerminalIO(&bytes.Buffer{}) {
			t.Error("a bytes.Buffer reported as a terminal")
		}
	})
	t.Run("[edge] a real *os.File that is not a tty (a temp file) is not a terminal", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "notty")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		// Exercises isTerminalFile's real term.IsTerminal call, not just
		// the *os.File type assertion.
		if isTerminalIO(f) {
			t.Error("a plain temp file reported as a terminal")
		}
	})
}

func TestResolveTimeout(t *testing.T) {
	state := &runtimeState{}

	t.Run("[happy] the --timeout flag wins when changed", func(t *testing.T) {
		cc := &cobra.Command{}
		cc.Flags().Duration("timeout", 30*time.Second, "")
		if err := cc.Flags().Set("timeout", "5s"); err != nil {
			t.Fatal(err)
		}
		got, err := state.resolveTimeout(cc)
		if err != nil {
			t.Fatal(err)
		}
		if got != 5*time.Second {
			t.Errorf("got %v, want 5s", got)
		}
	})

	t.Run("[happy] FRESHBOOKS_TIMEOUT is used when the flag was not changed", func(t *testing.T) {
		t.Setenv("FRESHBOOKS_TIMEOUT", "9s")
		cc := &cobra.Command{}
		cc.Flags().Duration("timeout", 30*time.Second, "")
		got, err := state.resolveTimeout(cc)
		if err != nil {
			t.Fatal(err)
		}
		if got != 9*time.Second {
			t.Errorf("got %v, want 9s", got)
		}
	})

	t.Run("[sad] an invalid FRESHBOOKS_TIMEOUT is a usage error", func(t *testing.T) {
		t.Setenv("FRESHBOOKS_TIMEOUT", "not-a-duration")
		cc := &cobra.Command{}
		cc.Flags().Duration("timeout", 30*time.Second, "")
		if _, err := state.resolveTimeout(cc); err == nil {
			t.Fatal("resolveTimeout() error = nil, want an error")
		}
	})

	t.Run("[happy] the 30s default when nothing is set", func(t *testing.T) {
		t.Setenv("FRESHBOOKS_TIMEOUT", "")
		cc := &cobra.Command{}
		cc.Flags().Duration("timeout", 30*time.Second, "")
		got, err := state.resolveTimeout(cc)
		if err != nil {
			t.Fatal(err)
		}
		if got != 30*time.Second {
			t.Errorf("got %v, want 30s", got)
		}
	})
}

func TestBuildLogger(t *testing.T) {
	state := &runtimeState{}
	cases := []struct {
		level string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelWarn}, // default
	}
	for _, tc := range cases {
		cc := &cobra.Command{}
		cc.Flags().String("log-level", "", "")
		if tc.level != "" {
			if err := cc.Flags().Set("log-level", tc.level); err != nil {
				t.Fatal(err)
			}
		}
		var buf bytes.Buffer
		cc.SetErr(&buf)
		logger, err := state.buildLogger(cc)
		if err != nil {
			t.Fatalf("level %q: buildLogger error: %v", tc.level, err)
		}
		if !logger.Enabled(context.Background(), tc.want) {
			t.Errorf("level %q: logger not enabled at its own resolved level %v", tc.level, tc.want)
		}
		if logger.Enabled(context.Background(), tc.want-1) {
			t.Errorf("level %q: logger enabled one level below %v, want it filtered out", tc.level, tc.want)
		}
	}

	t.Run("[sad] an unrecognized --log-level is a usage error", func(t *testing.T) {
		cc := &cobra.Command{}
		cc.Flags().String("log-level", "", "")
		if err := cc.Flags().Set("log-level", "bogus"); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		cc.SetErr(&buf)
		_, err := state.buildLogger(cc)
		var usageErr *usageError
		if !errors.As(err, &usageErr) {
			t.Fatalf("buildLogger error = %v, want a *usageError", err)
		}
	})
}

func TestWriteBinaryResult(t *testing.T) {
	t.Run("[happy] writes to a file path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.pdf")
		cc := &cobra.Command{}
		if err := writeBinaryResult(cc, []byte("%PDF-1.4"), path); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "%PDF-1.4" {
			t.Errorf("got %q", data)
		}
	})

	t.Run("[happy] writes to stdout for \"-\"", func(t *testing.T) {
		cc := &cobra.Command{}
		var buf bytes.Buffer
		cc.SetOut(&buf)
		if err := writeBinaryResult(cc, []byte("hello"), "-"); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "hello" {
			t.Errorf("got %q", buf.String())
		}
	})

	t.Run("[sad] a non-[]byte result is an internal error", func(t *testing.T) {
		cc := &cobra.Command{}
		if err := writeBinaryResult(cc, "not bytes", "-"); err == nil {
			t.Fatal("writeBinaryResult() error = nil, want one for a non-[]byte result")
		}
	})

	t.Run("[sad] an unwritable path is an error", func(t *testing.T) {
		cc := &cobra.Command{}
		if err := writeBinaryResult(cc, []byte("x"), "/nonexistent-dir-xyz/out.bin"); err == nil {
			t.Fatal("writeBinaryResult() error = nil, want a write error")
		}
	})
}

func TestScanStringFlag(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		short, long string
		want        string
	}{
		{"long form space-separated", []string{"--output", "json"}, "-o", "--output", "json"},
		{"short form space-separated", []string{"-o", "yaml"}, "-o", "--output", "yaml"},
		{"long form equals", []string{"--output=table"}, "-o", "--output", "table"},
		{"short form glued", []string{"-oname"}, "-o", "--output", "name"},
		{"absent", []string{"--other", "x"}, "-o", "--output", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanStringFlag(tt.args, tt.short, tt.long); got != tt.want {
				t.Errorf("scanStringFlag(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestVoidErrorPath(t *testing.T) {
	t.Run("[sad] void propagates a non-nil error", func(t *testing.T) {
		if _, err := void(errBoom); err != errBoom {
			t.Errorf("void() error = %v, want errBoom", err)
		}
	})
	t.Run("[happy] void acknowledges success", func(t *testing.T) {
		out, err := void(nil)
		if err != nil {
			t.Fatalf("void() error = %v", err)
		}
		m, ok := out.(map[string]bool)
		if !ok || !m["ok"] {
			t.Errorf("void() = %v", out)
		}
	})
}

func TestSortedKeys(t *testing.T) {
	// output.SortedKeys is exercised indirectly through `config
	// contexts`; this drives it directly for the deterministic-order
	// guarantee itself.
	m := map[string]int{"c": 1, "a": 2, "b": 3}
	got := output.SortedKeys(m)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestOpenUploadMissingFile exercises OpenUpload's error path.
func TestOpenUploadMissingFile(t *testing.T) {
	inv := &Invocation{uploadPath: "/nonexistent-dir-xyz/file.bin"}
	if _, _, err := inv.OpenUpload(); err == nil {
		t.Fatal("OpenUpload() error = nil, want one for a missing file")
	}
}

// TestDecodeBodyOptionalInvalidJSON exercises the error-propagation
// branch of DecodeBodyOptional.
func TestDecodeBodyOptionalInvalidJSON(t *testing.T) {
	inv := &Invocation{body: []byte("{not json"), hasBody: true}
	var v map[string]any
	if _, err := inv.DecodeBodyOptional(&v); err == nil {
		t.Fatal("DecodeBodyOptional() error = nil, want a decode error")
	}
}

// TestExtraOutOfRange exercises Invocation.Extra's bounds checks.
func TestExtraOutOfRange(t *testing.T) {
	inv := &Invocation{extra: []string{"only"}}
	if got := inv.Extra(1); got != "" {
		t.Errorf("Extra(1) = %q, want empty", got)
	}
	if got := inv.Extra(-1); got != "" {
		t.Errorf("Extra(-1) = %q, want empty", got)
	}
	if got := inv.Extra(0); got != "only" {
		t.Errorf("Extra(0) = %q, want %q", got, "only")
	}
}

// errBoom is a fixed sentinel error for TestVoidErrorPath.
var errBoom = &usageError{msg: "boom"}

// TestClassifyRunErrorPassesThroughTypedErrors exercises every branch of
// classifyRunError directly.
func TestClassifyRunErrorPassesThroughTypedErrors(t *testing.T) {
	if classifyRunError(nil) != nil {
		t.Error("classifyRunError(nil) != nil")
	}
	ue := newUsageError("x")
	if classifyRunError(ue) != error(ue) {
		t.Error("a usageError should pass through unchanged")
	}
	plain := &notARegisteredType{}
	got := classifyRunError(plain)
	if _, ok := got.(*runtimeError); !ok {
		t.Errorf("an untyped error should be wrapped as *runtimeError, got %T", got)
	}
}

type notARegisteredType struct{}

func (*notARegisteredType) Error() string { return "plain" }

func TestAppendQueryParseError(t *testing.T) {
	if _, err := appendQuery("http://[::1]:namedport/bad", []string{"a=b"}); err == nil {
		t.Fatal("appendQuery() error = nil, want a URL parse error")
	}
}

// TestDryRunTransportNoBody exercises dryRunTransport.RoundTrip's nil-body
// branch directly (a GET request with no body).
func TestDryRunTransportNoBody(t *testing.T) {
	var buf bytes.Buffer
	rt := dryRunTransport{out: &buf}
	req, err := http.NewRequest(http.MethodGet, "https://example.invalid/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := rt.RoundTrip(req)
	if resp != nil {
		t.Error("dryRunTransport returned a non-nil response")
	}
	if !isDryRun(err) {
		t.Errorf("RoundTrip error = %v, want errDryRun", err)
	}
	if !strings.Contains(buf.String(), "GET") {
		t.Errorf("buf = %q, want GET printed", buf.String())
	}
}
