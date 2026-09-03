package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Run("[happy] version prints name and version, exits 0", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(&stdout, &stderr, []string{"version"}, "1.2.3")
		if code != 0 {
			t.Fatalf("run() exit = %d, want 0", code)
		}
		got := stdout.String()
		if !strings.Contains(got, "freshbooks-mcp") || !strings.Contains(got, "1.2.3") {
			t.Fatalf("run() stdout = %q, want it to contain name and version", got)
		}
	})

	t.Run("[happy] tools prints valid JSON with every tool", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(&stdout, &stderr, []string{"tools"}, "1.2.3")
		if code != 0 {
			t.Fatalf("run() exit = %d, want 0; stderr = %q", code, stderr.String())
		}
		var manifest []map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
			t.Fatalf("tools output is not valid JSON: %v", err)
		}
		if len(manifest) != 169 {
			t.Fatalf("got %d tools, want 169", len(manifest))
		}
		for _, entry := range manifest {
			if _, ok := entry["name"]; !ok {
				t.Fatalf("entry missing name: %+v", entry)
			}
			if _, ok := entry["inputSchema"]; !ok {
				t.Fatalf("entry missing inputSchema: %+v", entry)
			}
		}
	})

	t.Run("[sad] serve with an invalid transport fails and exits 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(&stdout, &stderr, []string{"serve", "--transport", "bogus"}, "1.2.3")
		if code != 1 {
			t.Fatalf("run() exit = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "bogus") {
			t.Errorf("stderr = %q, want it to mention the bad transport", stderr.String())
		}
	})

	t.Run("[sad] serve stdio with no token configured fails fast and exits 1", func(t *testing.T) {
		for _, e := range []string{"FRESHBOOKS_ACCESS_TOKEN", "FRESHBOOKS_CLIENT_ID", "FRESHBOOKS_CLIENT_SECRET", "FRESHBOOKS_TOKEN_FILE"} {
			t.Setenv(e, "")
		}
		var stdout, stderr bytes.Buffer
		code := run(&stdout, &stderr, []string{"serve", "--transport", "stdio"}, "1.2.3")
		if code != 1 {
			t.Fatalf("run() exit = %d, want 1; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "token") {
			t.Errorf("stderr = %q, want it to mention the missing token", stderr.String())
		}
	})

	t.Run("[sad] an unknown subcommand fails and exits 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(&stdout, &stderr, []string{"nope"}, "1.2.3")
		if code != 1 {
			t.Fatalf("run() exit = %d, want 1", code)
		}
	})
}
