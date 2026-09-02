package auth

import (
	"fmt"
	"os"
	"path/filepath"
)

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

// CredentialsPath is the credentials file for one named context.
func CredentialsPath(context string) (string, error) {
	dir, err := CredentialsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, context+".json"), nil
}
