# Changelog

All notable changes to the `freshbooks` module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Core client: `Client` with all 36 resource services declared as fields,
  `NewClient` and the `With*` options (`WithTokenSource`, `WithHTTPClient`,
  `WithBaseURL`, `WithUserAgent`, `WithLogger`, `WithRetry`, `WithClock`),
  and `(*Client).Do` as the escape hatch for unmodelled endpoints.
- Transport: a single `do()` path with family-aware query encoding, accounting
  and auth envelope unwrapping, family-specific error decoding, jittered
  exponential backoff on 429/502/503/504 honouring `Retry-After`, context
  cancellation during backoff, a 10MB response cap, and `Authorization`
  stripped on cross-host redirects.
- Types: `AccountID`, `BusinessID`, `BusinessUUID`, `Money` with `Rat()`,
  `Date` and `DateTime` covering all three FreshBooks wire formats,
  `VisState`, and the `Include` / `Search` / `Sort` / `PageNumber` / `PerPage`
  request options.
- Errors: `*Error` with `errors.Is` sentinels (`ErrUnauthorized`,
  `ErrForbidden`, `ErrNotFound`, `ErrValidation`, `ErrRateLimited`) and
  `RetryAfter()`.
- Pagination: `Page[T]`, `PageMeta`, and the `All` iterator (`iter.Seq2`).
- `IdentityService` with `Me`, `Whoami`, and `Register`, covering the four
  `Authorization` inventory entries together with `auth.Config.Revoke`.
- `freshbooks/auth`: the PKCE authorization-code flow (`AuthCodeURL`,
  `Exchange`, `Refresh`, `Revoke`), both live-verified endpoint sets,
  `Token` / `TokenSource` / `StaticTokenSource`, `NewTokenSource` with expiry
  skew, single-flight refresh, and rotation write-back before return, plus
  `FileStore` (0600 in a 0700 directory, temp + rename) and `MemoryStore`.
- Repository scaffold: module skeleton, doc.go package overview, and the
  `freshbooks/internal/inventory` tool that normalizes the FreshBooks
  Postman collection into a parity contract for future phases.

### Fixed

- `auth.Token.String` now takes a value receiver, so `%v` on a `Token` value
  (or on a struct that embeds one) redacts the credentials instead of
  printing them.
- A `TokenStore.Save` failure after a successful refresh no longer discards
  the rotated pair. The source keeps it and retries the save on the next
  `Token` call, so a transient store failure is recoverable rather than a
  forced re-authentication.
- The `auth` package no longer falls back to `http.DefaultClient`: its
  default has a 30s timeout and refuses to follow redirects, which would
  otherwise replay the client secret and refresh token to the redirect
  target.
- `*Error.Family` is now the family the request was built for rather than one
  re-derived from the request path, which disagreed under a `WithBaseURL`
  path prefix.
- Webhook callback paths (`/events/`) are classified as the accounting family
  so their envelope is unwrapped.
- The client's redirect cap returns a real error instead of handing back the
  final 3xx as a response.
