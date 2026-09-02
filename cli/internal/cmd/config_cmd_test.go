package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfigCmd(t *testing.T) {
	t.Run("[happy] set-context creates a context, use-context switches, view and contexts read it back", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configDir)

		var stdout, stderr bytes.Buffer
		code := Run([]string{"config", "set-context", "work", "--account", "ACM1", "--business", "42", "--business-uuid", "uuid-1"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("set-context exit = %d, stderr = %s", code, stderr.String())
		}

		stdout.Reset()
		code = Run([]string{"config", "use-context", "work"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("use-context exit = %d, stderr = %s", code, stderr.String())
		}

		stdout.Reset()
		code = Run([]string{"config", "view", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("view exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "ACM1") || !strings.Contains(stdout.String(), "\"current-context\": \"work\"") {
			t.Errorf("view output = %s", stdout.String())
		}

		stdout.Reset()
		code = Run([]string{"config", "contexts", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("contexts exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "\"current\": true") {
			t.Errorf("contexts output = %s, want the work context marked current", stdout.String())
		}
	})

	t.Run("[happy] set-context updates only the fields passed", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configDir)

		var stdout, stderr bytes.Buffer
		Run([]string{"config", "set-context", "work", "--account", "ACM1"}, discardStdin, &stdout, &stderr, "test") //nolint:errcheck
		stdout.Reset()
		Run([]string{"config", "set-context", "work", "--business", "99"}, discardStdin, &stdout, &stderr, "test") //nolint:errcheck

		stdout.Reset()
		code := Run([]string{"config", "view", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("view exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "ACM1") || !strings.Contains(stdout.String(), "\"business\": \"99\"") {
			t.Errorf("expected both the earlier account and the new business to survive: %s", stdout.String())
		}
	})

	t.Run("[sad] use-context on an unknown name is a usage error", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		code := Run([]string{"config", "use-context", "ghost"}, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[edge] view on a missing config file prints an empty config", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		code := Run([]string{"config", "view", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
	})

	t.Run("[edge] contexts on an empty config prints nothing", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		var stdout, stderr bytes.Buffer
		code := Run([]string{"config", "contexts", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
	})
}
