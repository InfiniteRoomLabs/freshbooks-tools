// Package auth implements the FreshBooks OAuth2 authorization-code flow with
// PKCE, plus the token lifecycle the API demands: FreshBooks refresh tokens
// are one-time-use, so every refresh returns a new pair and invalidates the
// old one. TokenSource persists the rotated pair through a TokenStore before
// it hands the token back, so a crash between "refreshed" and "saved" cannot
// strand the caller with a dead refresh token.
//
// Two endpoint sets exist and both accept a registered app. The RFC 8414
// metadata set (MetadataEndpoints) is the default; the set named in the
// prose documentation (DocumentedEndpoints) is the fallback.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Endpoints names the three OAuth2 endpoints this package talks to.
type Endpoints struct {
	// AuthURL is the browser-facing authorization endpoint.
	AuthURL string
	// TokenURL exchanges an authorization code, and refreshes.
	TokenURL string
	// RevokeURL revokes an access or refresh token.
	RevokeURL string
}

// MetadataEndpoints is the endpoint set advertised by the RFC 8414 metadata
// document at https://auth.freshbooks.com/.well-known/oauth-authorization-server.
// It is the default: a live check on 2026-08-23 confirmed it accepts PKCE
// S256 and behaves identically to DocumentedEndpoints.
var MetadataEndpoints = Endpoints{ // #nosec G101 -- public endpoint URLs, not credentials
	AuthURL:   "https://auth.freshbooks.com/service/auth/oauth/authorize",
	TokenURL:  "https://auth.freshbooks.com/service/auth/oauth/token",
	RevokeURL: "https://auth.freshbooks.com/service/auth/oauth/revoke",
}

// DocumentedEndpoints is the endpoint set named in the FreshBooks prose
// documentation. It is the supported fallback; a live check on 2026-08-23
// confirmed it accepts the same requests and returns the same token shape.
var DocumentedEndpoints = Endpoints{ // #nosec G101 -- public endpoint URLs, not credentials
	AuthURL:   "https://auth.freshbooks.com/oauth/authorize/",
	TokenURL:  "https://api.freshbooks.com/auth/oauth/token",
	RevokeURL: "https://api.freshbooks.com/auth/oauth/revoke",
}

// maxTokenResponseBytes bounds what this package will read from a token
// endpoint. Token responses are a few kilobytes; anything larger is a bug or
// a hostile server, not a token.
const maxTokenResponseBytes = 1 << 20

// Config holds the registered application's credentials and the endpoints to
// use. The zero Endpoints value means MetadataEndpoints.
type Config struct {
	// ClientID is the registered application's client ID.
	ClientID string
	// ClientSecret is the registered application's client secret.
	// FreshBooks requires it on every token request; there is no public
	// client mode.
	ClientSecret string
	// RedirectURL must match a redirect URI registered for the app. The
	// FreshBooks portal rejects non-HTTPS URIs, including http://localhost.
	RedirectURL string
	// Scopes are the user:<object>:<read|write> scopes to request.
	Scopes []string
	// Endpoints selects the endpoint set. Zero means MetadataEndpoints.
	Endpoints Endpoints
	// HTTPClient performs the token requests. Nil means
	// http.DefaultClient.
	HTTPClient *http.Client
	// Now supplies the current time, for tests. Nil means time.Now.
	Now func() time.Time
}

func (c Config) endpoints() Endpoints {
	if c.Endpoints == (Endpoints{}) {
		return MetadataEndpoints
	}
	return c.Endpoints
}

func (c Config) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c Config) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// NewVerifier returns a fresh PKCE code verifier: 32 bytes from crypto/rand,
// base64url-encoded without padding, which is 43 characters and inside RFC
// 7636's 43-128 range.
func NewVerifier() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("freshbooks/auth: generating a PKCE verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Challenge returns the S256 PKCE code challenge for verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// AuthCodeURL builds the URL to send the user's browser to, and returns the
// PKCE code verifier alongside it. Keep the verifier: Exchange needs it, and
// it must never leave the process that generated it.
//
// state should be a fresh unguessable value; the caller must compare it
// against the state returned on the redirect before calling Exchange.
func (c Config) AuthCodeURL(state string) (authURL, verifier string, err error) {
	verifier, err = NewVerifier()
	if err != nil {
		return "", "", err
	}

	u, err := url.Parse(c.endpoints().AuthURL)
	if err != nil {
		return "", "", fmt.Errorf("freshbooks/auth: parsing the authorize endpoint: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", c.RedirectURL)
	q.Set("state", state)
	q.Set("code_challenge", Challenge(verifier))
	q.Set("code_challenge_method", "S256")
	if len(c.Scopes) > 0 {
		q.Set("scope", strings.Join(c.Scopes, " "))
	}
	u.RawQuery = q.Encode()
	return u.String(), verifier, nil
}

// Exchange trades an authorization code for a token pair. verifier is the
// value AuthCodeURL returned for this authorization request.
func (c Config) Exchange(ctx context.Context, code, verifier string) (*Token, error) {
	return c.token(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"code":          {code},
		"redirect_uri":  {c.RedirectURL},
		"code_verifier": {verifier},
	})
}

// Refresh exchanges a refresh token for a new pair. FreshBooks refresh
// tokens are one-time-use: on success refreshToken is dead and the returned
// token's RefreshToken must be persisted before anything else can use it.
func (c Config) Refresh(ctx context.Context, refreshToken string) (*Token, error) {
	return c.token(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"refresh_token": {refreshToken},
	})
}

// Revoke invalidates an access or refresh token. It is idempotent from the
// caller's point of view: FreshBooks answers 200 with an empty object.
//
// inventory: Authorization/Revoke Refresh Token
func (c Config) Revoke(ctx context.Context, token string) error {
	form := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"token":         {token},
	}
	_, err := c.postForm(ctx, c.endpoints().RevokeURL, form)
	return err
}

// token posts form to the token endpoint and decodes the token response.
func (c Config) token(ctx context.Context, form url.Values) (*Token, error) {
	body, err := c.postForm(ctx, c.endpoints().TokenURL, form)
	if err != nil {
		return nil, err
	}

	var wire tokenResponse
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("freshbooks/auth: decoding the token response: %w", err)
	}
	if wire.AccessToken == "" {
		return nil, errors.New("freshbooks/auth: the token response carried no access_token")
	}
	return wire.token(c.now()), nil
}

// postForm sends an application/x-www-form-urlencoded POST and returns the
// response body, or an *Error for any non-2xx status.
func (c Config) postForm(ctx context.Context, endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("freshbooks/auth: building the request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("freshbooks/auth: %s: %w", endpointName(endpoint), redactURLError(err))
	}
	defer resp.Body.Close() //nolint:errcheck // response body, close error is not actionable

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("freshbooks/auth: reading the response: %w", err)
	}
	if len(body) > maxTokenResponseBytes {
		return nil, fmt.Errorf("freshbooks/auth: response from %s exceeds %d bytes", endpointName(endpoint), maxTokenResponseBytes)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, newError(resp.StatusCode, body)
	}
	return body, nil
}

// redactURLError strips the *url.Error wrapper the http.Client adds, whose
// message repeats the full request URL including its query string. The
// wrapped cause is preserved, so errors.Is still matches context.Canceled
// and friends.
func redactURLError(err error) error {
	var uerr *url.Error
	if errors.As(err, &uerr) && uerr.Err != nil {
		return uerr.Err
	}
	return err
}

// endpointName renders an endpoint for an error message without its query
// string, which is where a code or token would live.
func endpointName(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "the OAuth endpoint"
	}
	u.RawQuery, u.Fragment, u.User = "", "", nil
	return u.String()
}

// Error is an OAuth2 error response (RFC 6749 section 5.2). Its message
// carries only the server's error code and description, never the request
// form or the response body, so credentials cannot reach a log through it.
type Error struct {
	// StatusCode is the HTTP status of the failing response.
	StatusCode int
	// Code is the OAuth2 error code, e.g. "invalid_grant".
	Code string
	// Description is the server's human-readable explanation.
	Description string
}

// Error implements the error interface.
func (e *Error) Error() string {
	switch {
	case e.Code != "" && e.Description != "":
		return fmt.Sprintf("freshbooks/auth: %d %s: %s", e.StatusCode, e.Code, e.Description)
	case e.Code != "":
		return fmt.Sprintf("freshbooks/auth: %d %s", e.StatusCode, e.Code)
	default:
		return fmt.Sprintf("freshbooks/auth: %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	}
}

// ErrInvalidGrant matches the OAuth2 "invalid_grant" error, which is what a
// reused or revoked refresh token returns.
var ErrInvalidGrant = errors.New("freshbooks/auth: invalid_grant")

// Unwrap maps the OAuth2 error code onto a sentinel where one exists.
func (e *Error) Unwrap() error {
	if e.Code == "invalid_grant" {
		return ErrInvalidGrant
	}
	return nil
}

func newError(status int, body []byte) *Error {
	e := &Error{StatusCode: status}
	var wire struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &wire); err == nil {
		e.Code, e.Description = wire.Error, wire.Description
	}
	return e
}
