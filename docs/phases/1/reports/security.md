# Phase 1 (lib-core) -- security lane report

Branch `phase-1/lib-core` (5 commits on a9ba5e8), reviewed read-only 2026-08-23. Scope: `git diff main...phase-1/lib-core`, focus on `freshbooks/auth/`, `freshbooks/transport.go`, fixtures, deps, lint config, public-repo hygiene.

## Verdict: BLOCK

One BLOCKING finding (a one-line fix), five advisories. Everything else on the checklist held up under inspection -- evidence below the findings.

## Findings

### 1. BLOCKING -- `Token.String()` redaction does not cover the value form, but the docs promise it does

- `freshbooks/auth/token.go:38` -- `func (t *Token) String() string` is a pointer receiver. The method set of the value type `Token` does not include it, so `fmt.Sprintf("%v", *tok)`, `%+v` on a struct that embeds a `Token` by value, or any interface holding a `Token` value prints the raw struct fields: both `AccessToken` and `RefreshToken` in full.
- The package makes this an explicit guarantee and gets it wrong: `token.go:19-20` ("printing one with %v or %s cannot leak a credential") and `docs/authentication.md` ("implements fmt.Stringer with every secret redacted, so %v on a token cannot leak one").
- The test at `freshbooks/auth/token_test.go:40-46` only exercises the pointer forms (`tok.String()`, `%v`/`%+v` on `*Token`), so it passes while the value form leaks. `log.Printf("%v", *token)` is not an exotic caller mistake for a public auth library.
- **Fix:** change to a value receiver, `func (t Token) String() string`. `*Token` then satisfies `fmt.Stringer` automatically, and `fmt` prints `<nil>` for a nil `*Token` instead of panicking (its `catchPanic` special-cases nil receivers), so the nil case stays safe -- update the `nilTok.String()` assertion accordingly (a direct method call on a nil pointer would now panic; only `fmt`-mediated printing is guarded, which is the leak channel that matters). Add `fmt.Sprintf("%v", *tok)` and a `struct{ T Token }` `%+v` case to `TestTokenStringRedacts`.

### 2. ADVISORY -- a failed `TokenStore.Save` after a successful refresh discards the only live token pair

- `freshbooks/auth/token.go:183-186` -- on `store.Save` failure, `refreshingSource.Token` returns the error without updating `s.cached`. The old refresh token is already spent (one-time-use), and `next` -- the only live pair in existence -- is dropped on the floor. The next `Token()` call refreshes with the dead token, gets `invalid_grant`, and the account is stranded into a full re-authorization. A transient disk-full or permissions blip becomes a lost login.
- Failing the current call is right (spec: persist before returning). Discarding `next` is the avoidable part.
- **Fix:** keep `next` in `s.cached` with a `dirty` flag and retry `Save` at the top of subsequent `Token()` calls before handing the token out; still return the save error to the current caller.

### 3. ADVISORY -- auth package HTTP client: no timeout, no redirect policy, credentials in the request body

- `freshbooks/auth/oauth.go:92-97` -- `Config.httpClient()` defaults to `http.DefaultClient`: no `Timeout` (a hung token endpoint blocks until the caller's ctx fires -- and `Refresh` is called under the `refreshingSource` mutex, so one hang blocks every concurrent API call on that source), and no `CheckRedirect`. Token requests carry `client_secret` and the one-time refresh token in the form body (`oauth.go:155-190`); `strings.NewReader` sets `GetBody`, so a 307/308 from the token endpoint would replay the full credential body to the redirect target. The API transport's cross-host guard (`client.go:186`) does not apply here, and stripping `Authorization` would not help -- the secrets are in the body.
- Exploitation requires a compromised or misbehaving FreshBooks endpoint (both sets are hard-coded HTTPS), so this is defense-in-depth, not an open hole.
- **Fix:** default to `&http.Client{Timeout: 30 * time.Second, CheckRedirect: func(...) error { return http.ErrUseLastResponse }}` -- token endpoints never legitimately redirect, so refuse to follow at all.

### 4. ADVISORY -- two real-looking capture values in the users_me fixture

- `freshbooks/testdata/auth/users_me.json:87` and `:89` (and the copy in `testdata/seed/users_me.json`) -- `"id": 15660096` with `"created_at": "2026-08-22T04:31:09Z"` and the id repeated inside the `links.destroy` path. Amid otherwise obviously synthetic values (`ACM123`, `1111111`, `8675309`, zeroed UUID), this pair reads like the real identity-membership row id and real registration timestamp from the live capture. Linkability is low (a bare internal row id), but the stated convention is fully synthetic fixtures.
- Checked clean otherwise: no `eyJ...` JWTs (the `fasttrack_token` is `"REDACTED.JWT.TOKEN"`), no real emails (all `@example.com`/`@example.test`), no internal hosts, IPs, vault names, or personal correspondents anywhere in the diff.
- **Fix:** replace with a synthetic id (e.g. `70005`, mirrored into the `links.destroy` path) and a round timestamp, in both seed and fixture.

### 5. ADVISORY -- redirect cap returns `http.ErrUseLastResponse` instead of an error

- `freshbooks/client.go:187-189` -- at 10 hops the guard returns `ErrUseLastResponse`, which makes `http.Client` hand back the final 3xx as the response; the transport then surfaces it as a decoded API error with a redirect status, which is misleading. No leak (the header was already stripped on the first cross-host hop, and the transport closes the body), just wrong error shape.
- **Fix:** `return fmt.Errorf("freshbooks: stopped after 10 redirects")` to match stdlib behavior.

### 6. ADVISORY -- govulncheck not run

- `govulncheck` is not installed under mise (`mise exec -- govulncheck` fails); per read-only constraints I did not install it. Exposure is limited to the standard library (see supply chain below). Worth adding to the CI gate as its own step.

## Checklist evidence (items verified as claimed)

1. **Secrets never leak.** The only log lines in the library are the two `DebugContext` calls in `transport.go:132,154` -- method, `redactPath(url)` (query/fragment/userinfo stripped, `transport.go:285-292`), status, attempt; never headers or bodies, and the default logger is `slog.DiscardHandler` (`client.go:114`). The `*url.Error` wrapper (which embeds the full URL incl. query) is stripped in both packages before wrapping: `transport.go:138-141` and `auth/oauth.go:243-249`, with the endpoint rendered query-less via `endpointName` (`oauth.go:253-260`). `freshbooks.Error.Error()` never renders `Raw` (`errors.go:55-82`), `auth.Error` stores no body at all (`oauth.go:265-284`), and `fmt` routes `%v`/`%+v` on error values through `Error()`, so `Raw` cannot leak that way. Tests assert the client secret, tokens, and the registration password never reach an error string. The one gap is finding 1.
2. **Credential storage.** `FileStore.Save` (`auth/store.go:102-152`): `MkdirAll(dir, 0700)`, `os.CreateTemp` in the same directory (0600 by default) + explicit `Chmod(0600)`, write, `Sync`, `Rename` over the target, `defer os.Remove(tmp)` so no temp survives failure. `DefaultTokenPath` honours `XDG_CONFIG_HOME` with a `~/.config` fallback (`store.go:66-75`). Rename replaces a symlink at the path rather than following it; an attacker who can plant one inside the 0700 directory already owns the token. Directory fsync after rename is omitted -- a crash-durability nicety, not a security gap. Note `MkdirAll` will not tighten a pre-existing looser directory (standard Go behavior, acceptable).
3. **OAuth/PKCE/state.** Verifier: 32 bytes of `crypto/rand`, base64url, 43 chars, S256 challenge (`oauth.go:109-121`); never logged, and `AuthCodeURL` documents that the caller must validate `state` before `Exchange`. Refresh rotation persists through `TokenStore` BEFORE returning (`token.go:181-187`), fails loudly if the response omits a new refresh token (`token.go:175-180`), and the mutex held across the whole refresh gives single-flight under concurrency (`token.go:150-152`). Expiry from `created_at + expires_in` (correct for FreshBooks' original-grant `created_at` semantics), 60s skew, skew-boundary tested. `Revoke` sends `client_id`/`client_secret`/`token` matching the live findings. The 401-never-retried + resolve-token-once-per-request design (`transport.go:57-63`) correctly avoids double-spending a rotating refresh token.
4. **Transport/TLS/redirects.** No `tls.Config`, no `InsecureSkipVerify` anywhere in the module; all endpoints HTTPS. API client defaults to a 30s `Timeout` (`client.go:112`); every method takes a ctx. `NewClient` shallow-copies the caller's `*http.Client` (`client.go:131-135`) -- caller's value never mutated, verified by test (`client_test.go:155-168`) -- and installs `stripAuthOnCrossHostRedirect` only when the caller set no policy: deletes `Authorization` when `host:port` changes (stricter than stdlib's registered-domain rule, and correct on the A->B->A hop since the compare is against the previous request), caps the chain at 10. Unit + end-to-end two-`httptest`-server tests exist (`client_test.go:260-330`). Bodies bounded at 10MB API / 1MB auth via `io.LimitReader(n+1)` with explicit over-limit errors (`transport.go:146-152`, `oauth.go:225-231`). `parseRetryAfter` (`errors.go:181-198`): negative/zero -> 0, non-numeric -> HTTP-date or 0; a huge-but-parseable value either overflows to negative (ignored) or is capped by `RetryPolicy.MaxDelay` (30s default) in `delay()` -- bounded in every case. No log line ever contains a header.
5. **Trust boundaries.** All responses decode into typed structs through one envelope-unwrapping path; no `unsafe`, no shell-outs in the diff (the one `os/exec` in the repo is Phase 0 inventory tooling, annotated); `FileStore` takes its path from the calling program, not from request data; `resolve()` rejects absolute request paths (`transport.go:111-113`).
6. **Supply chain.** `freshbooks/go.mod` is two lines (module + `go 1.26`), no `require`, no `go.sum` exists -- zero dependencies including test-only; the implementer's testify-not-used claim is true. `.golangci.yml` change: G101/G304 excluded for `_test.go` only -- justified (fixture reads from `testdata/`, synthetic token-shaped literals). Inline `#nosec`: G101 x2 on public endpoint URL blocks, G117 on `json.Marshal(*Token)` in the store (serializing the token into a 0600 file is the store's purpose), G404 on retry jitter (not a security decision) -- each carries a written reason and each is sound.
7. **Public-repo hygiene.** Diff-scoped grep: no JWTs, no real emails, no `100.x` IPs, no `*.lab.*`/`*.internal.*` hosts, no vault item names, no personal names. Token fixtures are `synthetic-access-token-0001`-style. `live_test.go` logs only membership counts, never identifiers ("Identifiers are account data; log only their presence"). Docs use env-var placeholders for credentials. The one residue is finding 4.

## Bottom line

The auth-critical design is sound and the implementer's security claims verified true with two exceptions: the `String()` redaction guarantee is false for the value form (finding 1, blocking, one-line fix), and the save-failure path in refresh rotation throws away a live token pair (finding 2, advisory). Fix 1 (and ideally 2-4) in the fix commit, re-gate, merge.
