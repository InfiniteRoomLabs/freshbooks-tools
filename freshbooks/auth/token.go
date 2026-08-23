package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// DefaultExpirySkew is how long before an access token's stated expiry a
// TokenSource treats it as already expired, so a token cannot die in flight.
const DefaultExpirySkew = 60 * time.Second

// Token is a FreshBooks OAuth2 token pair.
//
// Token deliberately implements fmt.Stringer with a redacted rendering, so
// printing one with %v or %s cannot leak a credential. Use the fields
// directly, or json.Marshal, when you actually mean to serialize it.
type Token struct {
	// AccessToken is the bearer token sent on API requests.
	AccessToken string `json:"access_token"`
	// RefreshToken is one-time-use: spending it invalidates it.
	RefreshToken string `json:"refresh_token,omitempty"`
	// TokenType is "Bearer".
	TokenType string `json:"token_type,omitempty"`
	// Scopes are the scopes actually granted.
	Scopes []string `json:"scopes,omitempty"`
	// Expiry is when the access token dies, computed as the token
	// response's created_at plus expires_in. FreshBooks reports created_at
	// as the time of the original grant, not of the current response, so
	// computing from "now" would overstate the lifetime after a refresh.
	Expiry time.Time `json:"expiry,omitzero"`
}

// String renders the token with its secrets redacted.
func (t *Token) String() string {
	if t == nil {
		return "auth.Token(nil)"
	}
	return fmt.Sprintf("auth.Token{AccessToken: redacted, RefreshToken: redacted, Expiry: %s}", t.Expiry.Format(time.RFC3339))
}

// Valid reports whether the access token is present and does not expire
// within skew of now. A token with no Expiry is treated as non-expiring.
func (t *Token) Valid(now time.Time, skew time.Duration) bool {
	if t == nil || t.AccessToken == "" {
		return false
	}
	if t.Expiry.IsZero() {
		return true
	}
	return now.Add(skew).Before(t.Expiry)
}

// Clone returns a deep copy, so a caller cannot mutate a TokenSource's cache.
func (t *Token) Clone() *Token {
	if t == nil {
		return nil
	}
	cp := *t
	cp.Scopes = append([]string(nil), t.Scopes...)
	return &cp
}

// tokenResponse is the wire shape of a FreshBooks token endpoint response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	CreatedAt    int64  `json:"created_at"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// token converts the wire response, computing expiry from created_at when
// the server supplies one and falling back to now otherwise.
func (r tokenResponse) token(now time.Time) *Token {
	t := &Token{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		TokenType:    r.TokenType,
	}
	if r.Scope != "" {
		t.Scopes = strings.Fields(r.Scope)
	}
	if r.ExpiresIn > 0 {
		base := now
		if r.CreatedAt > 0 {
			base = time.Unix(r.CreatedAt, 0).UTC()
		}
		t.Expiry = base.Add(time.Duration(r.ExpiresIn) * time.Second)
	}
	return t
}

// A TokenSource supplies an access token for each API request, refreshing it
// when necessary. Implementations must be safe for concurrent use.
type TokenSource interface {
	Token(ctx context.Context) (*Token, error)
}

type staticSource struct{ token *Token }

func (s staticSource) Token(context.Context) (*Token, error) { return s.token.Clone(), nil }

// StaticTokenSource returns a TokenSource that always yields accessToken and
// never refreshes. It is what the MCP server uses when it is handed a token
// out of the environment.
func StaticTokenSource(accessToken string) TokenSource {
	return staticSource{token: &Token{AccessToken: accessToken, TokenType: "Bearer"}}
}

// ErrNoToken is returned by a TokenStore that holds no token yet, and by a
// TokenSource that has nothing to refresh from.
var ErrNoToken = errors.New("freshbooks/auth: no stored token")

// A TokenStore persists a token pair across processes. Implementations must
// be safe for concurrent use. Load returns ErrNoToken when nothing is
// stored.
type TokenStore interface {
	Load(ctx context.Context) (*Token, error)
	Save(ctx context.Context, t *Token) error
}

// refreshingSource refreshes an expiring token and writes the rotated pair
// back through its store before returning it.
type refreshingSource struct {
	cfg   Config
	store TokenStore
	skew  time.Duration

	mu     sync.Mutex
	cached *Token
}

// NewTokenSource returns a TokenSource backed by store. It loads the stored
// token on first use, refreshes it when it expires within DefaultExpirySkew,
// and persists the rotated pair through store before returning it -- so a
// FreshBooks one-time-use refresh token is never spent without being saved.
//
// At most one refresh is in flight per source; concurrent callers wait for
// it and share the result.
func NewTokenSource(cfg Config, store TokenStore) TokenSource {
	return &refreshingSource{cfg: cfg, store: store, skew: DefaultExpirySkew}
}

// Token implements TokenSource.
func (s *refreshingSource) Token(ctx context.Context) (*Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cached == nil {
		stored, err := s.store.Load(ctx)
		if err != nil && !errors.Is(err, ErrNoToken) {
			return nil, fmt.Errorf("freshbooks/auth: loading the stored token: %w", err)
		}
		s.cached = stored
	}
	if s.cached == nil {
		return nil, ErrNoToken
	}
	if s.cached.Valid(s.cfg.now(), s.skew) {
		return s.cached.Clone(), nil
	}
	if s.cached.RefreshToken == "" {
		return nil, fmt.Errorf("freshbooks/auth: the access token expired and no refresh token is stored: %w", ErrNoToken)
	}

	next, err := s.cfg.Refresh(ctx, s.cached.RefreshToken)
	if err != nil {
		return nil, err
	}
	if next.RefreshToken == "" {
		// Defensive: every observed refresh rotates. If a response ever
		// omits it, keeping the old one is wrong (it is spent), so fail
		// loudly rather than cache a pair that cannot refresh again.
		return nil, errors.New("freshbooks/auth: the refresh response carried no refresh_token")
	}
	// The old refresh token is already dead at this point, so the new pair
	// must reach durable storage before any caller gets to use it.
	if err := s.store.Save(ctx, next); err != nil {
		return nil, fmt.Errorf("freshbooks/auth: persisting the rotated token: %w", err)
	}
	s.cached = next
	return next.Clone(), nil
}
