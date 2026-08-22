package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRun(t *testing.T) {
	t.Run("[happy] prints name and version, exits 0", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(&stdout, &stderr, "1.2.3")
		if code != 0 {
			t.Fatalf("run() exit = %d, want 0", code)
		}
		got := stdout.String()
		if !strings.Contains(got, "freshbooks-mcp") || !strings.Contains(got, "1.2.3") {
			t.Fatalf("run() stdout = %q, want it to contain name and version", got)
		}
	})

	t.Run("[sad] stdout write failure exits 1 and reports to stderr", func(t *testing.T) {
		var stderr bytes.Buffer
		writeErr := errors.New("broken pipe")
		code := run(errWriter{err: writeErr}, &stderr, "1.2.3")
		if code != 1 {
			t.Fatalf("run() exit = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "broken pipe") {
			t.Errorf("stderr = %q, want it to mention the write error", stderr.String())
		}
	})
}
