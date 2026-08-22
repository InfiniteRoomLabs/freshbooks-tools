package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Run("[happy] prints name and version", func(t *testing.T) {
		var buf bytes.Buffer
		if err := run(&buf, nil, "1.2.3"); err != nil {
			t.Fatalf("run() error = %v", err)
		}
		got := buf.String()
		if !strings.Contains(got, "freshbooks-mcp") || !strings.Contains(got, "1.2.3") {
			t.Fatalf("run() output = %q, want it to contain name and version", got)
		}
	})
}
