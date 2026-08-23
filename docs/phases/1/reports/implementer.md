# Phase 1 (lib core) -- implementer report

Branch `phase-1/lib-core`, four commits on top of `a9ba5e8`. `mise run check` is green on a clean tree, `mise run inventory-check` reports `implemented 4 / uncovered 0 / stale 0` for the `Authorization` folder, and `git status --porcelain` is empty.

## Files

### Created -- `freshbooks/`

| File | What it holds |
|---|---|
| `types.go` | `AccountID` / `BusinessID` / `BusinessUUID`, `Family`, `Money` + `Rat()`, `Date`, `DateTime` (all three wire formats, remembers its layout), `VisState`, and the `Include` / `Search` / `Sort` / `PageNumber` / `PerPage` request options with per-family query encoding |
| `errors.go` | `*Error`, the five `errors.Is` sentinels, `RetryAfter()`, `decodeError` for all four observed error shapes, `parseRetryAfter` |
| `client.go` | `Client`, `NewClient`, all 36 service fields wired, `BaseURL()`, the cross-host redirect guard, `familyForPath` |
| `services.go` | The 35 empty service structs Phase 2 fills in (Identity lives in `identity.go`) |
| `options.go` | The seven `With*` options, `RetryPolicy`, `NoRetry`, `DefaultRetryPolicy`, backoff arithmetic |
| `transport.go` | The single `do()` path plus `(*Client).Do`, envelope unwrapping, bounded body reads, retry loop |
| `page.go` | `Page[T]`, `PageMeta`, `All` -> `iter.Seq2[T, error]` |
| `identity.go` | `IdentityService.Me` / `Whoami` / `Register`, `Membership`, `User` |
| `auth/oauth.go` | `Endpoints` (both live-verified sets), `Config`, PKCE `NewVerifier` / `Challenge`, `AuthCodeURL`, `Exchange`, `Refresh`, `Revoke`, `*auth.Error` |
| `auth/token.go` | `Token` (redacting `String`), `Valid`, `Clone`, `TokenSource`, `StaticTokenSource`, `TokenStore`, `NewTokenSource` |
| `auth/store.go` | `MemoryStore`, `FileStore`, `DefaultTokenPath` |

Test files created: `types_test.go`, `errors_test.go`, `client_test.go`, `transport_test.go`, `page_test.go`, `identity_test.go`, `example_test.go`, `integration_test.go` (`//go:build integration`), `live_test.go` (`//go:build live`), `auth/oauth_test.go`, `auth/token_test.go`, `auth/store_test.go`.

Fixtures created under `freshbooks/testdata/`: `accounting/{clients_list,error_404,error_422,error_429}.json`, `projects/{list,error_404}.json`, `auth/{users_me,error_401,token,token_rotated,token_error}.json`. The first six are copies of the sanitized stage 1 captures in `testdata/seed/`; the token fixtures are synthetic and match the live response shape recorded in spec section 3.

### Changed

| File | Change |
|---|---|
| `freshbooks/doc.go` | Added "Getting started" and "Errors and retries" sections and the no-secrets-in-logs guarantee |
| `freshbooks/CHANGELOG.md` | `[Unreleased] / Added` entries for everything above |
| `freshbooks/internal/inventory/testdata/ignore.list` | Removed the four `Authorization/... -- phase-1` todo lines |
| `docs/authentication.md`, `docs/library.md` | Rewritten for real against the 2026-08-23 live findings |
| `docs/superpowers/specs/...-design.md` | One `STATE AS OF 2026-08-23` callout in section 5.1 (see Discrepancies) |
| `.golangci.yml` | Excluded gosec `G101` and `G304` for `_test.go` only |

## Test counts and coverage

| Package | Test/Example funcs | Coverage |
|---|---|---|
| `freshbooks` | 38 (incl. 3 integration-tagged, 1 live-tagged, 2 runnable Examples) | 99.1% |
| `freshbooks/auth` | 17 | 93.7% |
| `freshbooks/internal/inventory` (Phase 0) | 36 | 92.2% |
| **module total** | **91** | **95.3%** (floor 90%) |

121 `t.Run` subtests in the files this phase added, tagged `[happy] [sad] [edge] [corner]`. Every run is `-race`. No `t.Skip` outside `live_test.go`, where the single skip is the documented `FRESHBOOKS_LIVE=1` opt-in gate from spec 8.1 and is annotated as such.

`mcp` and `cli` are untouched and still at 100%.

## `mise run check` tail

```
== inventory-check: freshbooks ==
implemented 4, ignored 0, todo 209, uncovered 0, double-covered 0, stale 0, unknown 0
...
coverage-gate: freshbooks/coverage.out total = 95.3% (floor 90%)
coverage-gate: PASS
...
build: done, artifacts in <repo root>/dist
check.sh: all OK
```

## Inventory coverage

```
implemented 4, ignored 0, todo 209, uncovered 0, double-covered 0, stale 0, unknown 0
```

| Inventory key | Carried by |
|---|---|
| `Authorization/Identity Info Call` | `(*IdentityService).Me` |
| `Authorization/List User` | `(*IdentityService).Me` |
| `Authorization/Register as a new user` | `(*IdentityService).Register` |
| `Authorization/Revoke Refresh Token` | `auth.Config.Revoke` |

`Identity Info Call` and `List User` are the same request (`GET /auth/api/v1/users/me`) listed twice in the collection under two names, so both comments sit on `Me`. The parity checker counts occurrences per key, so this is one comment per key, not a double-cover.

## Design decisions worth a reviewer's attention

- **The access token is resolved once per logical request, not per retry attempt.** Retries happen milliseconds to seconds apart and a 401 is never retried, so re-resolving would only risk provoking a second refresh of a rotating one-time-use token.
- **`Retry-After` is honoured but capped by `RetryPolicy.MaxDelay`** (30s by default). A client should not block for an arbitrary period because a header said so. Documented in `options.go` and `docs/library.md`.
- **Cross-host redirects.** `NewClient` shallow-copies the caller's `*http.Client` and, if it has no `CheckRedirect`, installs one that deletes `Authorization` when the redirect target's `host:port` differs and caps the chain at 10. The standard library only strips the header across *registered domains*, so `a.example.com -> b.example.com` (and two ports on 127.0.0.1) would otherwise keep it. There is an end-to-end test with two `httptest` servers.
- **No secrets in logs or errors.** `auth.Token` has a redacting `String()`. Both packages strip the `*url.Error` wrapper (which repeats the full request URL, query string included) before returning a transport failure. `*Error.Error()` never renders `Raw`. Tests assert the token, the client secret, and a registration password never reach an error string.
- **Bounded reads.** 10MB in the API transport, 1MB in the auth package, both via `io.LimitReader(n+1)` with an explicit over-limit error rather than a silent truncation.
- **`FileStore` durability.** temp file in the same directory -> `Chmod 0600` -> write -> `Sync` -> `Rename`, inside a `MkdirAll(dir, 0700)`. Tests assert both modes and that no temp file survives three consecutive saves.

## Spec discrepancies and how they were resolved

1. **`Page` is named twice in spec 5.1** -- once as the pagination type `Page[T]` and once as a request option `Page(n)`. Go cannot have both in one package. **Resolved:** the type keeps the short name (it appears in every `List` signature); the option ships as `PageNumber(n)`. Recorded as a `STATE AS OF 2026-08-23` callout in spec section 5.1 and in `docs/library.md`.
2. **`Search` is specified two ways** -- `freshbooks.Search{"status": "paid"}` (a composite literal, so a type) in the code sample and `Search(map)` (a call, so a function) in the bullet. **Resolved:** `type Search map[string]string` that also implements `RequestOption`, so both spellings in the spec compile. Same callout.
3. **`Family` has three values, not two.** Spec section 3 describes two families, but its own 2026-08-23 live callout records that the auth family returns `{"response": {...}}` with no `result` layer -- a third envelope. The transport therefore has three cases and `Family` has three constants, matching the inventory tool's existing classifier strings. Same callout.
4. **Business-family query encoding was unspecified.** The spec says only that "the transport knows each family's query encoding". Implemented as: accounting spells filters `search[field]=value`, business-scoped spells them `field=value`; `include[]`, `sort=<field>_<asc|desc>`, `page`, and `per_page` are common. This is INFERRED from the docs, not confirmed live -- Phase 2's first business-scoped list endpoint should verify it and correct the callout if wrong.
5. **testify was not used.** `CLAUDE.md` permits it in tests; nothing here needed it, so the module stays at zero dependencies including test-only ones. Flagging it because the work order named testify explicitly. Say the word and I will convert.
6. **`Authorization/Register as a new user` is a real endpoint but a strange one for a client library** -- it provisions a FreshBooks identity and takes a plaintext password. It is implemented for parity (`IdentityService.Register`), documented as "almost every caller wants the hosted signup flow", and its password field is `json`-tagged but never logged. The alternative was demoting it to `//go:inventory-ignore`, which the work order ruled out.
7. **`.golangci.yml` needed one exclusion.** gosec fires `G101` on the two public endpoint URL blocks and `G117` on `json.Marshal(*Token)` in `FileStore.Save`; those three carry inline `#nosec` with reasons. It also fires `G101`/`G304` on ten test-file sites (synthetic URLs, `testdata` reads). Annotating ten test sites is noise, so `_test.go` is excluded for those two rules only.

## Not done in this phase (by design)

- Resource services have types and fields but no methods -- that is Phase 2, and 209 inventory entries remain `todo`.
- Multipart uploads (`Attachments`, `Images`) have service types only; the transport is JSON-only so far.
- `getting-started.md`, `mcp.md`, and `cli.md` remain stubs, per the roadmap.
