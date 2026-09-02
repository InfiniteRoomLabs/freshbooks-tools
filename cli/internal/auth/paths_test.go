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
