package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFixtureFile(path string) error {
	return os.WriteFile(path, []byte(`{"access_token":"fixture"}`), 0o600)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestCredentialsPath(t *testing.T) {
	t.Run("[happy] honors XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
		path, err := CredentialsPath("work")
		if err != nil {
			t.Fatalf("CredentialsPath() error = %v", err)
		}
		want := filepath.Join("/tmp/xdg-test", "freshbooks", "credentials", "work.json")
		if path != want {
			t.Fatalf("got %q, want %q", path, want)
		}
	})

	t.Run("[happy] falls back to ~/.config", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		path, err := CredentialsPath("default")
		if err != nil {
			t.Fatalf("CredentialsPath() error = %v", err)
		}
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".config", "freshbooks", "credentials", "default.json")
		if path != want {
			t.Fatalf("got %q, want %q", path, want)
		}
	})
}

// TestValidContextName is F2/security B2: a context name reaches
// filepath.Join from --context, FRESHBOOKS_CONTEXT, and config.yaml's
// current-context, so a name like "../../etc/passwd" must never resolve
// outside the credentials directory.
func TestValidContextName(t *testing.T) {
	valid := []string{"default", "work", "client-a", "client_b", "v1.2"}
	for _, name := range valid {
		if !ValidContextName(name) {
			t.Errorf("ValidContextName(%q) = false, want true", name)
		}
	}

	invalid := []string{"../evil", "a/b", ".", "..", "", "../../etc/passwd", "a\\b"}
	for _, name := range invalid {
		if ValidContextName(name) {
			t.Errorf("ValidContextName(%q) = true, want false", name)
		}
	}
}

// TestCredentialsPathRejectsInvalidNames is F2/security B2: CredentialsPath
// itself must reject the same invalid names, not just the standalone
// validator -- a caller that skipped ValidContextName must still be
// stopped before filepath.Join ever sees a traversal segment.
func TestCredentialsPathRejectsInvalidNames(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	for _, name := range []string{"../evil", "a/b", ".", "..", ""} {
		path, err := CredentialsPath(name)
		if err == nil {
			t.Errorf("CredentialsPath(%q) = %q, nil; want an error", name, path)
		}
	}
}

func TestCredentialsDir(t *testing.T) {
	t.Run("[happy] honors XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
		dir, err := CredentialsDir()
		if err != nil {
			t.Fatalf("CredentialsDir() error = %v", err)
		}
		want := filepath.Join("/tmp/xdg-test", "freshbooks", "credentials")
		if dir != want {
			t.Fatalf("got %q, want %q", dir, want)
		}
	})

	t.Run("[happy] falls back to ~/.config", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		dir, err := CredentialsDir()
		if err != nil {
			t.Fatalf("CredentialsDir() error = %v", err)
		}
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".config", "freshbooks", "credentials")
		if dir != want {
			t.Fatalf("got %q, want %q", dir, want)
		}
	})
}

func TestDefaultScopes(t *testing.T) {
	t.Run("[happy] carries both read and write for every object", func(t *testing.T) {
		if len(DefaultScopes) != len(scopeObjects)*2 {
			t.Fatalf("got %d scopes, want %d", len(DefaultScopes), len(scopeObjects)*2)
		}
		want := map[string]bool{}
		for _, obj := range scopeObjects {
			want["user:"+obj+":read"] = true
			want["user:"+obj+":write"] = true
		}
		for _, s := range DefaultScopes {
			if !want[s] {
				t.Errorf("unexpected scope %q", s)
			}
			delete(want, s)
		}
		if len(want) != 0 {
			t.Errorf("missing scopes: %v", want)
		}
	})
}
