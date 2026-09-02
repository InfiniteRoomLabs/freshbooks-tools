package auth

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	t.Run("[happy] is pinned at 43 -- every scope the developer portal offers", func(t *testing.T) {
		// Q4 (Phase 4 QA): len(DefaultScopes) != len(objects)*2+... is
		// mutation-blind -- emptying the object lists leaves both sides
		// at 0. A literal 43 (and the hard-coded scope strings below)
		// can only pass against the real, portal-observed scope set.
		if len(DefaultScopes) != 43 {
			t.Fatalf("got %d scopes, want 43", len(DefaultScopes))
		}
	})

	t.Run("[happy] contains a known read scope and a known write scope", func(t *testing.T) {
		for _, w := range []string{"user:clients:read", "user:invoices:write"} {
			if !slices.Contains(DefaultScopes, w) {
				t.Errorf("DefaultScopes missing %q", w)
			}
		}
	})

	t.Run("[sad] omits the three write scopes FreshBooks does not define", func(t *testing.T) {
		// Phase 7 (live, 2026-09-02): requesting any of these rejects the
		// whole consent with "The requested scope is invalid, unknown, or
		// malformed" -- the portal has profile, notifications, and reports
		// as read-only objects.
		for _, w := range []string{"user:profile:write", "user:notifications:write", "user:reports:write"} {
			if slices.Contains(DefaultScopes, w) {
				t.Errorf("DefaultScopes must not request the nonexistent scope %q", w)
			}
		}
	})

	t.Run("[edge] carries the undocumented uploads scopes the upload endpoints need", func(t *testing.T) {
		for _, w := range []string{"user:uploads:read", "user:uploads:write"} {
			if !slices.Contains(DefaultScopes, w) {
				t.Errorf("DefaultScopes missing %q", w)
			}
		}
	})

	t.Run("[edge] leaves the undocumented account and riskhub objects out", func(t *testing.T) {
		for _, s := range DefaultScopes {
			if strings.HasPrefix(s, "user:account:") || strings.HasPrefix(s, "user:riskhub:") {
				t.Errorf("DefaultScopes should not request %q by default", s)
			}
		}
	})

	t.Run("[happy] is exactly read+write per read-write object plus read per read-only object", func(t *testing.T) {
		want := map[string]bool{}
		for _, obj := range readWriteScopeObjects {
			want["user:"+obj+":read"] = true
			want["user:"+obj+":write"] = true
		}
		for _, obj := range readOnlyScopeObjects {
			want["user:"+obj+":read"] = true
		}
		if len(DefaultScopes) != len(want) {
			t.Fatalf("got %d scopes, want %d", len(DefaultScopes), len(want))
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
