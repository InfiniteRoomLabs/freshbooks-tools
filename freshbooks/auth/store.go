package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// MemoryStore is a TokenStore that keeps the token in memory only. It is the
// right store for tests and for short-lived processes handed a token out of
// the environment.
type MemoryStore struct {
	mu    sync.Mutex
	token *Token
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

// Load implements TokenStore.
func (m *MemoryStore) Load(context.Context) (*Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token == nil {
		return nil, ErrNoToken
	}
	return m.token.Clone(), nil
}

// Save implements TokenStore.
func (m *MemoryStore) Save(_ context.Context, t *Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = t.Clone()
	return nil
}

// File permissions for the token file and its directory. A token is a
// credential: it is owner-only, in an owner-only directory.
const (
	tokenFileMode = 0o600
	tokenDirMode  = 0o700
)

// FileStore is a TokenStore backed by a JSON file, written 0600 inside a
// 0700 directory and replaced atomically so a crash mid-write cannot leave a
// truncated token behind.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore returns a FileStore backed by the file at path. The file and
// its directory are created on the first Save.
func NewFileStore(path string) *FileStore { return &FileStore{path: path} }

// Path reports the file this store reads and writes.
func (f *FileStore) Path() string { return f.path }

// DefaultTokenPath is where the CLI keeps its token: $XDG_CONFIG_HOME (or
// ~/.config when unset) plus freshbooks/token.json.
func DefaultTokenPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "freshbooks", "token.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("freshbooks/auth: locating the config directory: %w", err)
	}
	return filepath.Join(home, ".config", "freshbooks", "token.json"), nil
}

// Load implements TokenStore. A missing file is ErrNoToken, not an error.
func (f *FileStore) Load(context.Context) (*Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path) // #nosec G304 -- the path is supplied by the calling program, not by a request
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoToken
		}
		return nil, fmt.Errorf("freshbooks/auth: reading %s: %w", f.path, err)
	}

	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("freshbooks/auth: parsing %s: %w", f.path, err)
	}
	if t.AccessToken == "" && t.RefreshToken == "" {
		return nil, ErrNoToken
	}
	return &t, nil
}

// Save implements TokenStore, writing atomically: a temporary file in the
// same directory, chmod 0600, fsync, then rename over the target.
func (f *FileStore) Save(_ context.Context, t *Token) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("freshbooks/auth: encoding the token: %w", err)
	}

	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, tokenDirMode); err != nil {
		return fmt.Errorf("freshbooks/auth: creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".token-*.tmp")
	if err != nil {
		return fmt.Errorf("freshbooks/auth: creating a temporary file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup; a successful rename makes it a no-op

	if err := writeTokenFile(tmp, data); err != nil {
		return err
	}
	if err := os.Rename(tmpName, f.path); err != nil {
		return fmt.Errorf("freshbooks/auth: replacing %s: %w", f.path, err)
	}
	return nil
}

// writeTokenFile chmods, writes, syncs, and closes tmp.
func writeTokenFile(tmp *os.File, data []byte) (err error) {
	defer func() {
		if cerr := tmp.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("freshbooks/auth: closing the temporary token file: %w", cerr)
		}
	}()

	if err := tmp.Chmod(tokenFileMode); err != nil {
		return fmt.Errorf("freshbooks/auth: setting permissions on the temporary token file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("freshbooks/auth: writing the temporary token file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("freshbooks/auth: syncing the temporary token file: %w", err)
	}
	return nil
}
