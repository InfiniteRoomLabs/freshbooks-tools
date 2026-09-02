package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnvPrecedence exercises the flag > env > config.yaml file > default
// chain end to end through Run, for the five variables F19/review A9-A10
// named explicitly: FRESHBOOKS_CONTEXT, FRESHBOOKS_ACCOUNT_ID,
// FRESHBOOKS_BUSINESS_ID, FRESHBOOKS_OUTPUT, and FRESHBOOKS_CONFIG. Each
// checks flag beats env, env beats the config.yaml file value, and an
// empty env var counts as unset (falls through to the file/default)
// rather than as an explicit empty value.
func TestEnvPrecedence(t *testing.T) {
	writeConfigFile := func(t *testing.T, dir, content string) {
		t.Helper()
		fbDir := filepath.Join(dir, "freshbooks")
		if err := os.MkdirAll(fbDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fbDir, "config.yaml"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("FRESHBOOKS_CONTEXT", func(t *testing.T) {
		cfgYAML := "current-context: ctx-file\ncontexts:\n  ctx-file:\n    account: ACMFILE\n  ctx-env:\n    account: ACMENV\n  ctx-flag:\n    account: ACMFLAG\n"

		t.Run("[happy] flag wins over env and file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, cfgYAML)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_CONTEXT", "ctx-env")
			var stdout, stderr bytes.Buffer
			code := Run([]string{"auth", "status", "--context", "ctx-flag", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), `"Context": "ctx-flag"`) {
				t.Errorf("stdout = %s, want Context ctx-flag", stdout.String())
			}
		})

		t.Run("[happy] env wins over the file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, cfgYAML)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_CONTEXT", "ctx-env")
			var stdout, stderr bytes.Buffer
			code := Run([]string{"auth", "status", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), `"Context": "ctx-env"`) {
				t.Errorf("stdout = %s, want Context ctx-env", stdout.String())
			}
		})

		t.Run("[edge] an empty env var falls through to the file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, cfgYAML)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_CONTEXT", "")
			var stdout, stderr bytes.Buffer
			code := Run([]string{"auth", "status", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), `"Context": "ctx-file"`) {
				t.Errorf("stdout = %s, want Context ctx-file (the config.yaml current-context)", stdout.String())
			}
		})
	})

	t.Run("FRESHBOOKS_ACCOUNT_ID", func(t *testing.T) {
		cfgYAML := "current-context: default\ncontexts:\n  default:\n    account: ACMFILE\n"

		t.Run("[happy] flag wins over env and file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, cfgYAML)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_ACCOUNT_ID", "ACMENV")
			var stdout, stderr bytes.Buffer
			args := []string{"clients", "get", "123", "--account", "ACMFLAG", "--dry-run"}
			code := Run(args, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "ACMFLAG") {
				t.Errorf("stdout = %q, want ACMFLAG in the dry-run URL", stdout.String())
			}
		})

		t.Run("[happy] env wins over the file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, cfgYAML)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_ACCOUNT_ID", "ACMENV")
			var stdout, stderr bytes.Buffer
			args := []string{"clients", "get", "123", "--dry-run"}
			code := Run(args, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "ACMENV") {
				t.Errorf("stdout = %q, want ACMENV in the dry-run URL", stdout.String())
			}
		})

		t.Run("[edge] an empty env var falls through to the file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, cfgYAML)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_ACCOUNT_ID", "")
			var stdout, stderr bytes.Buffer
			args := []string{"clients", "get", "123", "--dry-run"}
			code := Run(args, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "ACMFILE") {
				t.Errorf("stdout = %q, want ACMFILE (the config.yaml account) in the dry-run URL", stdout.String())
			}
		})
	})

	t.Run("FRESHBOOKS_BUSINESS_ID", func(t *testing.T) {
		cfgYAML := "current-context: default\ncontexts:\n  default:\n    business: \"9000009\"\n"

		t.Run("[happy] flag wins over env and file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, cfgYAML)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_BUSINESS_ID", "9000002")
			var stdout, stderr bytes.Buffer
			args := []string{"time-entries", "list", "--business", "9000001", "--dry-run"}
			code := Run(args, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "9000001") {
				t.Errorf("stdout = %q, want business 9000001 in the dry-run URL", stdout.String())
			}
		})

		t.Run("[happy] env wins over the file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, cfgYAML)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_BUSINESS_ID", "9000002")
			var stdout, stderr bytes.Buffer
			args := []string{"time-entries", "list", "--dry-run"}
			code := Run(args, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "9000002") {
				t.Errorf("stdout = %q, want business 9000002 in the dry-run URL", stdout.String())
			}
		})

		t.Run("[edge] an empty env var falls through to the file", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, cfgYAML)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_BUSINESS_ID", "")
			var stdout, stderr bytes.Buffer
			args := []string{"time-entries", "list", "--dry-run"}
			code := Run(args, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "9000009") {
				t.Errorf("stdout = %q, want business 9000009 (the config.yaml business) in the dry-run URL", stdout.String())
			}
		})
	})

	t.Run("FRESHBOOKS_OUTPUT", func(t *testing.T) {
		// A non-empty config: JSON and YAML render an empty *File
		// identically ("{}\n" either way), so these need real content to
		// tell the two formats apart.
		nonEmptyCfg := "current-context: probe-ctx\n"

		t.Run("[happy] flag wins over env", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, nonEmptyCfg)
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_OUTPUT", "json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{"config", "view", "--output", "yaml"}, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "current-context: probe-ctx") {
				t.Errorf("stdout = %q, want YAML's own key syntax (the flag), not JSON's", stdout.String())
			}
		})

		t.Run("[happy] env overrides the TTY-sensitive default", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, nonEmptyCfg)
			withTerminals(t, nil, boolPtr(true)) // would default to table without the env
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_OUTPUT", "json")
			var stdout, stderr bytes.Buffer
			code := Run([]string{"config", "view"}, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), `"current-context": "probe-ctx"`) {
				t.Errorf("stdout = %q, want JSON's own key syntax", stdout.String())
			}
		})

		t.Run("[edge] an empty env var falls through to the TTY-sensitive default", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, nonEmptyCfg)
			withTerminals(t, nil, boolPtr(false)) // non-TTY default is json
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_OUTPUT", "")
			var stdout, stderr bytes.Buffer
			code := Run([]string{"config", "view"}, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), `"current-context": "probe-ctx"`) {
				t.Errorf("stdout = %q, want the non-TTY default of JSON", stdout.String())
			}
		})
	})

	t.Run("FRESHBOOKS_CONFIG", func(t *testing.T) {
		t.Run("[happy] flag wins over env", func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir) // a default path that must NOT be read
			pathFlag := filepath.Join(t.TempDir(), "flag.yaml")
			pathEnv := filepath.Join(t.TempDir(), "env.yaml")
			if err := os.WriteFile(pathFlag, []byte("current-context: from-flag\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(pathEnv, []byte("current-context: from-env\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("FRESHBOOKS_CONFIG", pathEnv)
			var stdout, stderr bytes.Buffer
			code := Run([]string{"config", "view", "--config", pathFlag, "--output", "json"}, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "from-flag") {
				t.Errorf("stdout = %s, want the --config path's content", stdout.String())
			}
		})

		t.Run("[happy] env wins over the default path", func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir) // a default path that must NOT be read
			pathEnv := filepath.Join(t.TempDir(), "env.yaml")
			if err := os.WriteFile(pathEnv, []byte("current-context: from-env\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("FRESHBOOKS_CONFIG", pathEnv)
			var stdout, stderr bytes.Buffer
			code := Run([]string{"config", "view", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "from-env") {
				t.Errorf("stdout = %s, want FRESHBOOKS_CONFIG's content", stdout.String())
			}
		})

		t.Run("[edge] an empty env var falls through to the default path", func(t *testing.T) {
			dir := t.TempDir()
			writeConfigFile(t, dir, "current-context: from-default-path\n")
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("FRESHBOOKS_CONFIG", "")
			var stdout, stderr bytes.Buffer
			code := Run([]string{"config", "view", "--output", "json"}, discardStdin, &stdout, &stderr, "test")
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "from-default-path") {
				t.Errorf("stdout = %s, want the default XDG path's content", stdout.String())
			}
		})
	})
}
