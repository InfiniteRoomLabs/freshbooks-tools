# Authentication

FreshBooks uses OAuth2 authorization-code. This page describes what the `freshbooks/auth` package implements and why, and every claim here was verified against the live API on 2026-08-23 unless it says otherwise.

## Endpoints

Two endpoint sets exist, and both accept a registered application end-to-end. They behave identically: same token response, same rotation, same revoke semantics.

| Set | Authorize | Token | Revoke |
|---|---|---|---|
| `auth.MetadataEndpoints` (default) | `https://auth.freshbooks.com/service/auth/oauth/authorize` | `.../service/auth/oauth/token` | `.../service/auth/oauth/revoke` |
| `auth.DocumentedEndpoints` (fallback) | `https://auth.freshbooks.com/oauth/authorize/` | `https://api.freshbooks.com/auth/oauth/token` | `https://api.freshbooks.com/auth/oauth/revoke` |

The default is the RFC 8414 metadata set, advertised at `https://auth.freshbooks.com/.well-known/oauth-authorization-server`. Leave `Config.Endpoints` zero to get it; set it to `auth.DocumentedEndpoints` to use the other one.

## The flow

```go
cfg := auth.Config{
    ClientID:     os.Getenv("FRESHBOOKS_CLIENT_ID"),
    ClientSecret: os.Getenv("FRESHBOOKS_CLIENT_SECRET"),
    RedirectURL:  "https://localhost:8765/callback",
    Scopes:       []string{"user:profile:read", "user:clients:read"},
}

authURL, verifier, err := cfg.AuthCodeURL(state)   // send the browser here, keep the verifier
token, err := cfg.Exchange(ctx, code, verifier)    // after the redirect comes back
```

`AuthCodeURL` always generates a PKCE code verifier -- 32 bytes from `crypto/rand`, base64url, 43 characters -- and returns it to you alongside the URL. The verifier never appears in the URL; only its S256 challenge does. Keep it in memory for the length of the flow and pass it to `Exchange`.

`state` is yours to generate and yours to check: compare the `state` on the redirect against the one you sent before calling `Exchange`. The library cannot do that for you because it never sees the redirect.

PKCE S256 is accepted by both authorize endpoints. Whether FreshBooks *rejects* a missing or wrong verifier was not tested (each test costs a browser consent), so treat PKCE as accepted-but-possibly-unenforced. `client_secret` is required either way; there is no public-client mode.

## Redirect URIs must be HTTPS

The developer portal rejects `http://localhost:...` outright, contradicting several third-party guides. The registered development redirect is `https://localhost:8765/callback`. The CLI's loopback listener therefore serves an ephemeral self-signed certificate on `127.0.0.1` (the browser shows a one-time warning) and always offers a paste-the-redirected-URL fallback that needs no listener at all. See `cli/internal/auth/` once Phase 4 lands.

## Token lifetimes and rotation

The token response looks like this:

```json
{
  "access_token": "<jwt>",
  "token_type": "Bearer",
  "expires_in": 43200,
  "created_at": 1756000000,
  "refresh_token": "<64 hex characters>",
  "scope": "user:profile:read user:clients:read"
}
```

Two facts drive the whole design:

1. **Expiry is `created_at + expires_in`, not `now + expires_in`.** A refresh can return the *original* grant's `created_at` with a decremented `expires_in`; computing from the wall clock would overstate the remaining life by however long the first token had already been alive. `auth.Token.Expiry` is computed from `created_at`.
2. **Refresh tokens are one-time-use.** A refresh returns a new pair and kills the old refresh token immediately; spending it twice returns `400 invalid_grant`. Whoever refreshes *must* persist the new pair, and must do it before anything else gets to use the token.

`auth.NewTokenSource(cfg, store)` implements both. It refreshes when the access token expires within `auth.DefaultExpirySkew` (60 seconds), writes the rotated pair through the store, and only then returns it. If the store cannot save, the call fails -- returning a token whose refresh half is already dead and unrecorded would strand the next process.

At most one refresh is in flight per source, so a burst of concurrent API calls spends the one-time-use token exactly once.

## Revocation

```go
err := cfg.Revoke(ctx, token)   // access or refresh token
```

A successful revoke answers `200 {}`, and the access token is dead immediately afterwards.

## Where each component keeps tokens

The library never owns a storage decision. `auth.TokenStore` is an interface with two implementations:

- `auth.NewFileStore(path)` -- JSON at `path`, written 0600 inside a 0700 directory via a temporary file plus `rename`, so a crash mid-write cannot leave a truncated token behind. `auth.DefaultTokenPath()` resolves `$XDG_CONFIG_HOME/freshbooks/token.json`, falling back to `~/.config/freshbooks/token.json`.
- `auth.NewMemoryStore()` -- in-process only, for tests and short-lived programs.

`auth.StaticTokenSource(accessToken)` is the third shape: no refresh, no store, for a process that was handed a token and is not responsible for its lifecycle. That is what the MCP server uses.

The CLI owns login and the token file. The MCP server is a token consumer only. Neither ships in Phase 1.

## Keeping tokens out of logs

- `auth.Token` implements `fmt.Stringer` with every secret redacted, so `%v` on a token cannot leak one.
- OAuth error messages carry the server's error code and description only, never the request form or the response body.
- The `*url.Error` wrapper the standard library adds -- which repeats the full request URL, query string included -- is stripped before an error is returned.
- The API client logs method, URL, status, and attempt number at debug level and nothing else. It never logs headers or bodies.

## Further reading

- https://www.freshbooks.com/api/authentication (official docs)
- https://www.freshbooks.com/api/scopes (scope list)
- Design spec section 3, `STATE AS OF 2026-08-23` callout (the live verification these facts come from)
