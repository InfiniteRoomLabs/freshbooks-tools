package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("[happy] missing file is an empty config, not an error", func(t *testing.T) {
		f, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if f.CurrentContext != "" || len(f.Contexts) != 0 {
			t.Fatalf("got %+v, want empty", f)
		}
	})

	t.Run("[happy] round trips through Save", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		want := &File{
			CurrentContext: "work",
			Contexts: map[string]Context{
				"work": {Account: "ACM123", Business: "456", BusinessUUID: "uuid-1"},
			},
		}
		if err := Save(path, want); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got.CurrentContext != want.CurrentContext {
			t.Errorf("CurrentContext = %q, want %q", got.CurrentContext, want.CurrentContext)
		}
		if got.Contexts["work"] != want.Contexts["work"] {
			t.Errorf("Contexts[work] = %+v, want %+v", got.Contexts["work"], want.Contexts["work"])
		}
	})

	t.Run("[sad] malformed yaml is an error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte("not: [valid: yaml"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("Load() error = nil, want an error for malformed yaml")
		}
	})

	t.Run("[edge] a file with no contexts key normalizes to an empty map", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte("current-context: solo\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		f, err := Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if f.Contexts == nil {
			t.Fatal("Contexts is nil, want an initialized empty map")
		}
	})

	t.Run("[sad] an unreadable file is an error", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("permission bits behave differently on windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("root bypasses file permissions")
		}
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte("current-context: x\n"), 0o000); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("Load() error = nil, want a permission error")
		}
	})
}

func TestSave(t *testing.T) {
	t.Run("[happy] creates the directory 0700 and the file 0600", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("permission bits behave differently on windows")
		}
		dir := filepath.Join(t.TempDir(), "nested", "freshbooks")
		path := filepath.Join(dir, "config.yaml")
		if err := Save(path, &File{CurrentContext: "x"}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		dirInfo, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if perm := dirInfo.Mode().Perm(); perm != dirMode {
			t.Errorf("dir mode = %o, want %o", perm, dirMode)
		}

		fileInfo, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := fileInfo.Mode().Perm(); perm != fileMode {
			t.Errorf("file mode = %o, want %o", perm, fileMode)
		}
		// F17/review A6: read the file back, not just confirm the mode --
		// the mode assertions above would pass even if Save wrote an
		// empty or truncated file at the right path.
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "current-context: x") {
			t.Errorf("config file = %s, want current-context: x persisted", data)
		}
	})

	t.Run("[sad] renaming over an existing directory of the same name is an error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := Save(path, &File{}); err == nil {
			t.Fatal("Save() error = nil, want a rename error when the target is a directory")
		}
	})

	t.Run("[sad] an unwritable directory is an error", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("permission bits behave differently on windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("root bypasses file permissions")
		}
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o500); err != nil { // #nosec G302 -- deliberately read-only+execute (no write) to force Save's CreateTemp to fail; not a file permission choice
			t.Fatal(err)
		}
		defer func() { _ = os.Chmod(dir, 0o700) }() // #nosec G302 -- best-effort restore so TempDir cleanup can remove it
		path := filepath.Join(dir, "nested", "config.yaml")
		if err := Save(path, &File{}); err == nil {
			t.Fatal("Save() error = nil, want an error for an unwritable directory")
		}
	})
}

func TestFile_Current(t *testing.T) {
	t.Run("[happy] returns the current context", func(t *testing.T) {
		f := &File{CurrentContext: "work", Contexts: map[string]Context{"work": {Account: "ACM1"}}}
		c, ok := f.Current()
		if !ok || c.Account != "ACM1" {
			t.Fatalf("Current() = %+v, %v", c, ok)
		}
	})

	t.Run("[edge] no current-context set", func(t *testing.T) {
		f := &File{}
		_, ok := f.Current()
		if ok {
			t.Fatal("Current() ok = true, want false")
		}
	})

	t.Run("[edge] current-context names a context that does not exist", func(t *testing.T) {
		f := &File{CurrentContext: "ghost"}
		_, ok := f.Current()
		if ok {
			t.Fatal("Current() ok = true, want false")
		}
	})

	t.Run("[corner] nil *File", func(t *testing.T) {
		var f *File
		_, ok := f.Current()
		if ok {
			t.Fatal("Current() ok = true, want false")
		}
	})
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name                       string
		flag, env, file, def, want string
	}{
		{"[happy] flag wins", "f", "e", "c", "d", "f"},
		{"[happy] env wins over file and default", "", "e", "c", "d", "e"},
		{"[happy] file wins over default", "", "", "c", "d", "c"},
		{"[happy] default when nothing else is set", "", "", "", "d", "d"},
		{"[edge] empty env counts as unset", "", "", "c", "d", "c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.flag, tt.env, tt.file, tt.def); got != tt.want {
				t.Errorf("Resolve(%q,%q,%q,%q) = %q, want %q", tt.flag, tt.env, tt.file, tt.def, got, tt.want)
			}
		})
	}
}

func TestDefaultPath(t *testing.T) {
	t.Run("[happy] honors XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
		path, err := DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath() error = %v", err)
		}
		want := filepath.Join("/tmp/xdg-test", "freshbooks", "config.yaml")
		if path != want {
			t.Fatalf("got %q, want %q", path, want)
		}
	})

	t.Run("[happy] falls back to ~/.config", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		path, err := DefaultPath()
		if err != nil {
			t.Fatalf("DefaultPath() error = %v", err)
		}
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".config", "freshbooks", "config.yaml")
		if path != want {
			t.Fatalf("got %q, want %q", path, want)
		}
	})
}
