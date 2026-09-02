// Package config handles the freshbooks CLI's configuration file:
// $XDG_CONFIG_HOME/freshbooks/config.yaml (falling back to ~/.config), the
// context set it stores, and the flag > env > file > default precedence
// every global flag and scope field follows. Never a secret: credentials
// live in the freshbooks library's auth.FileStore instead (see
// cli/internal/auth).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// File permissions: config.yaml is not a secret, but it does carry account
// and business identifiers, so it stays owner-only like the token file
// beside it.
const (
	fileMode = 0o600
	dirMode  = 0o700
)

// Context is one named scope: the account, business, and business-UUID
// identifiers a command resolves against when no flag or env var
// overrides them. Never a client secret or a token -- see the package
// doc comment.
type Context struct {
	Account      string `yaml:"account,omitempty"`
	Business     string `yaml:"business,omitempty"`
	BusinessUUID string `yaml:"business_uuid,omitempty"`
}

// File is the decoded shape of config.yaml.
type File struct {
	CurrentContext string             `yaml:"current-context,omitempty"`
	Contexts       map[string]Context `yaml:"contexts,omitempty"`
}

// Current returns the current context, and whether one is configured.
func (f *File) Current() (Context, bool) {
	if f == nil || f.CurrentContext == "" {
		return Context{}, false
	}
	c, ok := f.Contexts[f.CurrentContext]
	return c, ok
}

// DefaultPath is where the CLI keeps config.yaml: $XDG_CONFIG_HOME (or
// ~/.config when unset) plus freshbooks/config.yaml. Mirrors
// freshbooks/auth.DefaultTokenPath's resolution exactly, so the config
// file and the per-context credential files live side by side.
func DefaultPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "freshbooks", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: locating the config directory: %w", err)
	}
	return filepath.Join(home, ".config", "freshbooks", "config.yaml"), nil
}

// Load reads path. A missing file is not an error: it returns an empty
// *File, exactly as if config.yaml existed but declared nothing.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from the CLI's own flag/env/default resolution, not a request
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("config: reading %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("config: parsing %s: %w", path, err)
	}
	if f.Contexts == nil {
		f.Contexts = map[string]Context{}
	}
	return &f, nil
}

// Save writes f to path atomically: a temp file in the same directory,
// chmod 0600, fsync, then rename over the target. The directory is
// created 0700 if it does not exist.
func Save(path string, f *File) error {
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("config: encoding %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("config: creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("config: creating a temporary file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup; a successful rename makes it a no-op

	if err := writeAtomic(tmp, data); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("config: replacing %s: %w", path, err)
	}
	return nil
}

func writeAtomic(tmp *os.File, data []byte) (err error) {
	defer func() {
		if cerr := tmp.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("config: closing the temporary file: %w", cerr)
		}
	}()
	if err := tmp.Chmod(fileMode); err != nil {
		return fmt.Errorf("config: setting permissions on the temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("config: writing the temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("config: syncing the temporary file: %w", err)
	}
	return nil
}

// Resolve applies the flag > env > file > default precedence chain to one
// setting: flag wins if non-empty, then env (an empty env var counts as
// unset, per the CLI's documented rule), then file, then def.
func Resolve(flag, env, file, def string) string {
	if flag != "" {
		return flag
	}
	if env != "" {
		return env
	}
	if file != "" {
		return file
	}
	return def
}
