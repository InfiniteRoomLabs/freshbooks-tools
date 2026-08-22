package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	t.Run("[happy] prints the given version", func(t *testing.T) {
		root := NewRootCmd("9.9.9")
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"version"})

		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got := strings.TrimSpace(out.String()); got != "9.9.9" {
			t.Fatalf("Execute() output = %q, want %q", got, "9.9.9")
		}
	})
}

func TestCompletionCommand(t *testing.T) {
	t.Run("[happy] bash completion script generates without error", func(t *testing.T) {
		root := NewRootCmd("0.0.0-dev")
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"completion", "bash"})

		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if out.Len() == 0 {
			t.Fatal("Execute() produced no completion script output")
		}
	})
}

func TestRun(t *testing.T) {
	t.Run("[happy] version command exits 0", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"version"}, &stdout, &stderr, "9.9.9")
		if code != 0 {
			t.Fatalf("Run() exit = %d, stderr = %s", code, stderr.String())
		}
		if got := strings.TrimSpace(stdout.String()); got != "9.9.9" {
			t.Fatalf("Run() stdout = %q, want %q", got, "9.9.9")
		}
	})

	t.Run("[sad] unknown subcommand exits 1 and reports to stderr", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"this-command-does-not-exist"}, &stdout, &stderr, "0.0.0-dev")
		if code != 1 {
			t.Fatalf("Run() exit = %d, want 1", code)
		}
		if stderr.Len() == 0 {
			t.Fatal("Run() wrote nothing to stderr for a failing command")
		}
	})
}
