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
//
// G5/QA Q10: snake_case json tags, matching the D8 fold's Page[T]/User/
// Membership -- without them `auth status -o json` printed Go-cased field
// names (Context, LoggedIn, ...) instead of the wire-shape convention
// every other command's -o json output follows.
type StatusInfo struct {
	// Context is the context this status was checked for.
	Context string `json:"context"`
	// CredentialsPath is the file the context's credentials live in,
	// whether or not it exists yet.
	CredentialsPath string `json:"credentials_path"`
	// LoggedIn reports whether a token is stored at all.
	LoggedIn bool `json:"logged_in"`
	// Valid reports whether the stored access token is present and not
	// expiring within the lib's DefaultExpirySkew. False when
	// LoggedIn is false.
	Valid bool `json:"valid"`
	// Expiry is the stored access token's expiry, zero if unknown or
	// non-expiring.
	Expiry time.Time `json:"expiry"`
	// Scopes are the scopes the stored token actually carries.
	Scopes []string `json:"scopes"`
}

// clock returns now, or time.Now when the caller passed nil.
func clock(now func() time.Time) func() time.Time {
	if now == nil {
		return time.Now
	}
	return now
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

	info.LoggedIn = true
	info.Valid = tok.Valid(clock(now)(), fbauth.DefaultExpirySkew)
	info.Expiry = tok.Expiry
	info.Scopes = tok.Scopes
	return info, nil
}

// Logout best-effort revokes both the stored refresh token and the
// stored access token (a revocation failure is not fatal -- the
// credentials file is removed regardless) and then removes the
// credentials file. A missing file is not an error.
//
// Both tokens are revoked, not just the refresh token: a copy of the
// access token surviving elsewhere (a shell scrollback from `auth
// token`, a backup, a synced dotfile) would otherwise keep working until
// its natural expiry even after `logout`. Revoke-then-delete is the
// correct order and stays that way even though the revocation result is
// ignored: deleting first would mean a revoke failure leaves a live
// credential with nothing on disk to retry against, and making the
// delete unconditional means a network-down logout still clears local
// state.
func Logout(ctx context.Context, cfg fbauth.Config, credentialsPath string, store fbauth.TokenStore) error {
	tok, err := store.Load(ctx)
	if err == nil {
		if tok.RefreshToken != "" {
			_ = cfg.Revoke(ctx, tok.RefreshToken) //nolint:errcheck // best-effort: the file is removed either way
		}
		if tok.AccessToken != "" {
			_ = cfg.Revoke(ctx, tok.AccessToken) //nolint:errcheck // best-effort, same as above
		}
	}

	if rmErr := os.Remove(credentialsPath); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return fmt.Errorf("auth: removing %s: %w", credentialsPath, rmErr)
	}
	return nil
}

// Token returns the stored access token, refreshing (and persisting the
// rotated pair) first when refresh is true OR when the stored access token
// has already expired.
//
// The expiry check is not a convenience. Without it `auth token` printed a
// dead token and exited 0 (observed live, 2026-09-03: `auth status` reported
// valid: false while `auth token` still printed the stale value), so every
// caller of the documented `TOKEN=$(freshbooks auth token)` idiom got a
// silent 401 on its next call instead of a working credential. A token that
// is about to expire inside the lib's DefaultExpirySkew counts as expired
// here, matching what `auth status` calls valid.
//
// now supplies the clock; nil means time.Now.
func Token(ctx context.Context, cfg fbauth.Config, store fbauth.TokenStore, refresh bool, now func() time.Time) (string, error) {
	tok, err := store.Load(ctx)
	if err != nil {
		return "", fmt.Errorf("auth: loading the stored credentials: %w", err)
	}

	if !refresh && tok.Valid(clock(now)(), fbauth.DefaultExpirySkew) {
		return tok.AccessToken, nil
	}
	if tok.RefreshToken == "" {
		if refresh {
			return "", errors.New("auth: no refresh token is stored; run 'freshbooks auth login' again")
		}
		return "", errors.New("auth: the stored access token has expired and no refresh token is stored; run 'freshbooks auth login' again")
	}
	next, err := cfg.Refresh(ctx, tok.RefreshToken)
	if err != nil {
		return "", err
	}
	if err := store.Save(ctx, next); err != nil {
		// The OAuth server has already consumed the old refresh token
		// (they are one-time use) and the rotated pair never reached
		// disk, so there is nothing left to retry against.
		return "", fmt.Errorf("auth: saving the refreshed token: %w; the rotated token could not be stored, run 'freshbooks auth login' again", err)
	}
	return next.AccessToken, nil
}
