package auth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	t.Run("[sad] empty store", func(t *testing.T) {
		if _, err := s.Load(ctx); !errors.Is(err, ErrNoToken) {
			t.Fatalf("err = %v, want ErrNoToken", err)
		}
	})

	t.Run("[happy] round-trips and copies", func(t *testing.T) {
		in := &Token{AccessToken: "a", RefreshToken: "r", Scopes: []string{"s"}}
		if err := s.Save(ctx, in); err != nil {
			t.Fatal(err)
		}
		in.AccessToken = "mutated"

		out, err := s.Load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if out.AccessToken != "a" {
			t.Fatalf("store kept a reference to the caller's token: %q", out.AccessToken)
		}
		out.RefreshToken = "mutated"
		again, _ := s.Load(ctx)
		if again.RefreshToken != "r" {
			t.Fatal("Load handed out a mutable reference")
		}
	})

	t.Run("[corner] concurrent use is race-free", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := range 16 {
			wg.Add(2)
			go func() { defer wg.Done(); _ = s.Save(ctx, &Token{AccessToken: "a"}) }()
			go func() { defer wg.Done(); _, _ = s.Load(ctx) }()
			_ = i
		}
		wg.Wait()
	})
}

func TestFileStore(t *testing.T) {
	ctx := context.Background()

	t.Run("[sad] a missing file is ErrNoToken", func(t *testing.T) {
		s := NewFileStore(filepath.Join(t.TempDir(), "nested", "token.json"))
		if _, err := s.Load(ctx); !errors.Is(err, ErrNoToken) {
			t.Fatalf("err = %v, want ErrNoToken", err)
		}
	})

	t.Run("[happy] Save creates a 0700 directory holding a 0600 file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "config", "freshbooks")
		path := filepath.Join(dir, "token.json")
		s := NewFileStore(path)
		if s.Path() != path {
			t.Fatalf("Path() = %q", s.Path())
		}

		want := &Token{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			TokenType:    "Bearer",
			Scopes:       []string{"user:profile:read"},
			Expiry:       time.Unix(1756043200, 0).UTC(),
		}
		if err := s.Save(ctx, want); err != nil {
			t.Fatal(err)
		}

		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Mode().Perm(); got != tokenFileMode {
			t.Errorf("file mode = %04o, want %04o", got, tokenFileMode)
		}
		di, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := di.Mode().Perm(); got != tokenDirMode {
			t.Errorf("dir mode = %04o, want %04o", got, tokenDirMode)
		}

		got, err := s.Load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || !got.Expiry.Equal(want.Expiry) {
			t.Fatalf("loaded %+v", got)
		}
	})

	t.Run("[happy] Save replaces atomically and leaves no temporary files", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token.json")
		s := NewFileStore(path)

		for i := range 3 {
			if err := s.Save(ctx, &Token{AccessToken: "access-" + string(rune('0'+i))}); err != nil {
				t.Fatal(err)
			}
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "token.json" {
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Fatalf("directory holds %v, want just token.json", names)
		}
		got, err := s.Load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got.AccessToken != "access-2" {
			t.Fatalf("last write did not win: %q", got.AccessToken)
		}
	})

	t.Run("[sad] a corrupt file is a parse error, not a silent empty token", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token.json")
		if err := os.WriteFile(path, []byte("{nope"), tokenFileMode); err != nil {
			t.Fatal(err)
		}
		_, err := NewFileStore(path).Load(ctx)
		if err == nil || !strings.Contains(err.Error(), "parsing") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[edge] an empty token object is ErrNoToken", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token.json")
		if err := os.WriteFile(path, []byte("{}"), tokenFileMode); err != nil {
			t.Fatal(err)
		}
		if _, err := NewFileStore(path).Load(ctx); !errors.Is(err, ErrNoToken) {
			t.Fatalf("err = %v, want ErrNoToken", err)
		}
	})

	t.Run("[sad] an unreadable path is surfaced", func(t *testing.T) {
		// A directory is deterministically unreadable as a file on every
		// platform and for every uid, unlike a mode-0000 file (which root
		// can still read).
		if _, err := NewFileStore(t.TempDir()).Load(ctx); err == nil || errors.Is(err, ErrNoToken) {
			t.Fatalf("err = %v, want a read error", err)
		}
	})

	t.Run("[sad] a directory that cannot be created is surfaced", func(t *testing.T) {
		// The parent is a regular file, so MkdirAll fails with ENOTDIR for
		// every uid.
		blocker := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(blocker, []byte("x"), tokenFileMode); err != nil {
			t.Fatal(err)
		}
		err := NewFileStore(filepath.Join(blocker, "freshbooks", "token.json")).Save(ctx, &Token{AccessToken: "a"})
		if err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[edge] the stored JSON is the documented shape", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token.json")
		if err := NewFileStore(path).Save(ctx, &Token{AccessToken: "a", RefreshToken: "r"}); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if m["access_token"] != "a" || m["refresh_token"] != "r" {
			t.Fatalf("stored %s", raw)
		}
		if _, ok := m["expiry"]; ok {
			t.Fatalf("a zero expiry should be omitted: %s", raw)
		}
	})
}

func TestDefaultTokenPath(t *testing.T) {
	t.Run("[happy] honours XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
		got, err := DefaultTokenPath()
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join("/tmp/xdg", "freshbooks", "token.json") {
			t.Fatalf("path = %q", got)
		}
	})

	t.Run("[happy] falls back to the home config directory", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "/tmp/home")
		got, err := DefaultTokenPath()
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join("/tmp/home", ".config", "freshbooks", "token.json") {
			t.Fatalf("path = %q", got)
		}
	})

	t.Run("[sad] no home directory at all", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")
		if _, err := DefaultTokenPath(); err == nil {
			t.Fatal("want an error when neither XDG_CONFIG_HOME nor HOME is set")
		}
	})
}
