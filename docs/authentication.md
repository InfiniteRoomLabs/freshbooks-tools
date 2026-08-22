# Authentication

## OAuth2 flow

The authorization-code flow this library implements, including PKCE, and which of the two documented endpoint sets (`api.freshbooks.com/auth/oauth/*` vs. the RFC 8414 metadata `auth.freshbooks.com/service/auth/oauth/*`) it actually uses -- Phase 1 verifies this live and records the answer here.

## Token lifetimes and rotation

Access tokens live roughly 12 hours; refresh tokens are one-time-use, so every refresh must persist the new pair before returning. Lands in Phase 1 alongside `auth.TokenSource`/`auth.TokenStore`.

## Where each component keeps tokens

The library never owns storage decisions (`TokenStore` is an interface); the CLI owns login and a `credentials.json` file; the MCP server is a token consumer only, reading `FRESHBOOKS_ACCESS_TOKEN` or refresh-token env vars. Lands in Phases 1, 3, and 4 respectively.

## Redirect URIs

Why the loopback listener serves ephemeral self-signed TLS instead of plain HTTP -- the FreshBooks portal rejects `http://localhost` redirect URIs outright. See the `STATE AS OF` callout in the design spec section 3 for the discovery, and `cli/internal/auth/` (Phase 4) for the implementation.

## Further reading

- https://www.freshbooks.com/api/authentication (official docs)
