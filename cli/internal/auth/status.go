package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	fbauth "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// StatusInfo is what `auth status` reports. It never carries the token
// itself -- see Token for the one command that prints it.
type StatusInfo struct {
	// Context is the context this status was checked for.
	Context string
	// CredentialsPath is the file the context's credentials live in,
	// whether or not it exists yet.
	CredentialsPath string
	// LoggedIn reports whether a token is stored at all.
	LoggedIn bool
	// Valid reports whether the stored access token is present and not
	// expiring within the lib's DefaultExpirySkew. False when
	// LoggedIn is false.
	Valid bool
	// Expiry is the stored access token's expiry, zero if unknown or
	// non-expiring.
	Expiry time.Time
	// Scopes are the scopes the stored token actually carries.
	Scopes []string
}

// Status reports a context's credential state without ever touching the
// network or printing the token.
func Status(ctx context.Context, contextName, credentialsPath string, store fbauth.TokenStore, now func() time.Time) (*StatusInfo, error) {
	info := &StatusInfo{Context: contextName, CredentialsPath: credentialsPath}

	tok, err := store.Load(ctx)
	if errors.Is(err, fbauth.ErrNoToken) {
		return info, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: loading the stored credentials: %w", err)
	}

	nowFn := now
	if nowFn == nil {
		nowFn = time.Now
	}
	info.LoggedIn = true
	info.Valid = tok.Valid(nowFn(), fbauth.DefaultExpirySkew)
	info.Expiry = tok.Expiry
	info.Scopes = tok.Scopes
	return info, nil
}

// Logout best-effort revokes the stored refresh token (a revocation
// failure is not fatal -- the credentials file is removed regardless) and
// then removes the credentials file. A missing file is not an error.
func Logout(ctx context.Context, cfg fbauth.Config, credentialsPath string, store fbauth.TokenStore) error {
	tok, err := store.Load(ctx)
	if err == nil && tok.RefreshToken != "" {
		_ = cfg.Revoke(ctx, tok.RefreshToken) //nolint:errcheck // best-effort: the file is removed either way
	}

	if rmErr := os.Remove(credentialsPath); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return fmt.Errorf("auth: removing %s: %w", credentialsPath, rmErr)
	}
	return nil
}

// Token returns the stored access token, refreshing (and persisting the
// rotated pair) first when refresh is true.
func Token(ctx context.Context, cfg fbauth.Config, store fbauth.TokenStore, refresh bool) (string, error) {
	tok, err := store.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("auth: loading the stored credentials: %w", err)
	}

	if !refresh {
		return tok.AccessToken, nil
	}
	if tok.RefreshToken == "" {
		return "", errors.New("auth: no refresh token is stored; run 'freshbooks auth login' again")
	}
	next, err := cfg.Refresh(ctx, tok.RefreshToken)
	if err != nil {
		return "", err
	}
	if err := store.Save(ctx, next); err != nil {
		return "", fmt.Errorf("auth: saving the refreshed token: %w", err)
	}
	return next.AccessToken, nil
}
