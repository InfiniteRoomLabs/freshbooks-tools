package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// contextNameRE is the character set a context name may use once "", ".",
// and ".." are rejected separately below: letters, digits, '.', '_', '-'
// -- nothing that means anything to a filesystem path (no '/', no
// backslash, no leading/trailing whitespace).
var contextNameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidContextName reports whether name is safe to use as the filesystem
// component CredentialsPath builds a path from. A context name reaches
// filepath.Join from several places -- a --context flag,
// $FRESHBOOKS_CONTEXT, and config.yaml's current-context (which a
// config.yaml the operator did not author, e.g. one checked into a repo
// or shipped in a container image, can also set) -- so this is validated
// centrally here rather than trusted at each call site.
func ValidContextName(name string) bool {
	return name != "" && name != "." && name != ".." && contextNameRE.MatchString(name)
}

// CredentialsDir is $XDG_CONFIG_HOME/freshbooks/credentials (falling back
// to ~/.config), where one FileStore-backed JSON file lives per context
// (D5: "one lib FileStore per context").
func CredentialsDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "freshbooks", "credentials"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("auth: locating the config directory: %w", err)
	}
	return filepath.Join(home, ".config", "freshbooks", "credentials"), nil
}

// CredentialsPath is the credentials file for one named context. It
// rejects an invalid context name (see ValidContextName) rather than
// letting it reach filepath.Join, where a name like "../../etc/passwd"
// would resolve outside the credentials directory entirely.
func CredentialsPath(context string) (string, error) {
	if !ValidContextName(context) {
		return "", fmt.Errorf("auth: invalid context name %q: use only letters, digits, '.', '_', '-'", context)
	}
	dir, err := CredentialsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, context+".json"), nil
}
