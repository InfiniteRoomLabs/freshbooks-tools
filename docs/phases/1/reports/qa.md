# Phase 1 (lib-core) -- QA / reality-check lane report

Branch `phase-1/lib-core` at `09d7fc8` (6 commits on `a9ba5e8`), checked 2026-08-23. QA is the only lane that ran the gate. All commands ran from the repo root; the one throwaway fidelity test file (`freshbooks/qa_scratch_test.go`) was deleted before this report was written and the tree re-verified clean.

## Verdict: NEEDS WORK

One BLOCKING finding, empirically confirmed -- the same `auth.Token.String` pointer-receiver leak the code-review and security lanes flagged. Everything else on the stage 2 checklist is met with evidence. The fix is one line plus a test assertion; after the fix commit and a re-gate this phase is merge-ready.

## 1. Gate and tree state

Commands: `mise run check` (full), then `go -C freshbooks clean -testcache && mise run test` (fresh, uncached, includes `-race -tags integration`), then `git status --porcelain`.

`mise run check` tail (exit 0):

```
coverage-gate: .../freshbooks/coverage.out total = 95.3% (floor 90%)
coverage-gate: PASS
== vuln: freshbooks ==
No vulnerabilities found.
== inventory-check: freshbooks ==
implemented 4, ignored 0, todo 209, uncovered 0, double-covered 0, stale 0, unknown 0
...
coverage-gate: .../mcp/coverage.out total = 100.0% (floor 90%)   [vuln: clean]
coverage-gate: .../cli/coverage.out total = 100.0% (floor 90%)   [vuln: clean]
== actionlint ==
== build ==  (12 artifacts, linux/darwin/windows x amd64/arm64)
check.sh: all OK
```

Because the first run showed `(cached)` test lines, I cleared the test cache and re-ran `mise run test`: all packages pass fresh (`freshbooks` 1.554s + integration 1.863s, `auth` 1.158s/1.167s, inventory 2.546s/3.353s), coverage identical (99.1% / 93.7% / 92.2%, module 95.3%).

`git status --porcelain` is empty apart from this file (`freshbooks/coverage.out` and `dist/` are gitignored -- verified with `git check-ignore`). No green-HEAD-dirty-tree condition.

`scripts/redaction-check.sh`: `clean`.

## 2. GOAL.md stage 2 deliverables, enumerated

| Deliverable | Status | Evidence |
|---|---|---|
| `client.go`: `Client`, `NewClient(opts...)`, every spec 5.1 service pre-declared | MET | All 36 service fields declared (`client.go:29-99`) and wired in `registerServices` (`client.go:142-179`); the 35 empty Phase-2 structs live in `services.go`, `Identity` in `identity.go` |
| `options.go`: the 7 options | MET | `WithTokenSource`, `WithHTTPClient`, `WithBaseURL`, `WithUserAgent`, `WithLogger`, `WithRetry`, `WithClock` all present with validation (`options.go:21-107`) |
| `transport.go`: single `do()` path, headers, JSON body, envelope unwrap, family error decoding, retry 429/502/503/504 w/ jittered backoff + `Retry-After`, ctx cancellation, bounded bodies, no cross-host `Authorization` | MET | `do()` (`transport.go:43`); `retryableStatuses` exactly {429,502,503,504} (`transport.go:23-28`); `Retry-After` wins but capped by `MaxDelay` (`options.go:145-166`); 10MB bound via `LimitReader(n+1)` (`transport.go:146-152`); redirect guard (`client.go:186-194`); all tested |
| `errors.go`: `*Error`, sentinels, `RetryAfter()` | MET | 5 sentinels via `Unwrap` (`errors.go:14-29`), `Error()` never renders `Raw`, `parseRetryAfter` handles both RFC 9110 spellings |
| `page.go`: `Page[T]`, `All` -> `iter.Seq2[T, error]` | MET | `page.go:12-64`; stops on first error and `ctx.Done()` |
| `types.go` incl. all three date wire formats | MET | `AccountID`/`BusinessID`/`BusinessUUID`, `Money`+`Rat()`, `Date`, `DateTime` (RFC 3339, `YYYY-MM-DD HH:MM:SS`, `YYYY-MM-DD`; remembers its layout), `VisState`; `Include`/`Search`/`Sort`/`PageNumber`/`PerPage` with per-family encoding (`types.go:303-325`) |
| `identity.go` + 4 inventory comments, flipped from `ignore.list` | MET | `Me` carries `Authorization/Identity Info Call` + `Authorization/List User` (`identity.go:86-87`), `Register` carries `Register as a new user` (`identity.go:134`), `auth.Config.Revoke` carries `Revoke Refresh Token` (`oauth.go:181`); `ignore.list` holds exactly 209 `//go:inventory-todo` lines, zero Authorization entries |
| `(*Client).Do` escape hatch | MET | `transport.go:38-40`, same transport, exercised by the integration seam test |
| `auth/` package complete incl. FileStore semantics | MET | Both endpoint sets as named vars, metadata set default (`oauth.go:42-55, 85-90`); PKCE `NewVerifier` (32 bytes crypto/rand, 43 chars) + S256 `Challenge`; `Exchange`/`Refresh`/`Revoke`; `Token`, `TokenSource`, `StaticTokenSource`; `NewTokenSource` with 60s skew, mutex single-flight, Save-before-return (`token.go:181-187`); `FileStore` MkdirAll 0700 + CreateTemp + Chmod 0600 + write + Sync + Close + Rename (`store.go:102-152`); `MemoryStore`; `DefaultTokenPath` honours `XDG_CONFIG_HOME` |
| Test list from the work order | MET | Envelope unwrap (3 families + edges), both error shapes, 401/404(x2 families)/422/429+Retry-After/malformed/cancelled-context/retry-exhaustion tests all present by name (`transport_test.go:206-272, 273+`, `errors_test.go`); every Date/DateTime format (`types_test.go:48,111`); PKCE URL, exchange, refresh-rotation-persisted-before-return, concurrent single-flight under `-race`, FileStore permissions + atomicity (`auth/*_test.go`); integration + live tags below |
| `doc.go` overview + runnable `Example` | MET | 2 runnable Examples with `// Output:` blocks in `example_test.go` (executed in the fresh test run) |
| `freshbooks/CHANGELOG.md` `[Unreleased]` | MET | Complete Added section matching the shipped surface |
| `docs/authentication.md` + `docs/library.md` written for real | MET | 94 + 116 lines, contents match the live 2026-08-23 findings (endpoint table, created_at expiry math, rotation, HTTPS-redirect story); ASCII, no hard wraps |
| Acceptance: gate green on clean tree, `implemented 4` / no uncovered / no stale, govulncheck clean, no secret in logs/errors/fixtures/docs | PARTIAL | All met except the last clause: the `Token` value-form `%v` leak (finding 1) is a documented-guarantee violation |

## 3. Request-by-request fidelity (throwaway httptest run, then deleted)

I wrote `freshbooks/qa_scratch_test.go` (4 tests), ran `go -C freshbooks test -run QAScratch -count=1 -v .`, and deleted it. Results:

| Inventory entry | Expected (docs/inventory/live callout) | Observed on the wire | Verdict |
|---|---|---|---|
| `Authorization/Identity Info Call` + `Authorization/List User` | `GET /auth/api/v1/users/me`, `Authorization: Bearer <tok>` | `GET /auth/api/v1/users/me`, `Bearer qa-token` | PASS |
| `Authorization/Register as a new user` | `POST /auth/api/v1/smux/registrations`, JSON body with `id`, `email`, `password`, `company_name`, `country`, `currencyCode` (camelCase, per the Postman body) | exactly that; Content-Type `application/json`; the Postman body's analytics-only fields (`optimizely_*`, `provisioner`, ...) are deliberately unmodelled | PASS |
| `Authorization/Revoke Refresh Token` | Postman shows a JSON body, but the live-CONFIRMED spec callout (ground truth) says form POST `client_id`+`client_secret`+`token` | form-encoded POST with exactly those three fields, Content-Type `application/x-www-form-urlencoded` | PASS (correctly resolved in the live API's favour) |
| (supporting) `AuthCodeURL`/`Exchange` | `response_type=code`, `code_challenge_method=S256`, `state`, 43-char verifier; exchange form carries `grant_type=authorization_code` + `code_verifier` | all present | PASS |

Seed decode: serving `testdata/seed/users_me.json` verbatim through the real client, `Identity.Whoami` yields `User{ID:4242424, Email:"owner@example.com", FirstName:"Sam", LastName:"Owner"}` and one `Membership{AccountID:"ACM123", BusinessID:8675309, BusinessUUID:"00000000-0000-4000-8000-000000000001", Name:"Example Business LLC", Role:"owner"}` -- every field the spec callout names survives. `Me` returns the identical membership.

## 4. Seams

`mise run test` runs `go test -race -tags integration ./...` per module (`scripts/check.sh:42`). Executed fresh and by name (`go -C freshbooks test -race -tags integration -count=1 -v -run '...'`):

- `TestExpiryRefreshWriteBackRetry` -- PASS. Genuinely exercises expired token -> exactly 1 refresh (asserting the stored refresh token was the one spent) -> store holds the rotated pair -> the API call went out once with `Bearer synthetic-access-token-0002`. This is the promised expiry -> refresh -> write-back -> request seam.
- `TestFileStoreSurvivesProcessRestart` -- PASS. A second source over the same file uses the rotated pair without a second refresh (durability of the write-back).
- `TestAllAcrossPagesWithMidStreamRateLimit` -- PASS. 3 pages x 2 items through `All()`, page 2 429s once with `Retry-After: 0`, iterator yields `[1 2 3 4 5 6]` exactly once each and the limiter provably fired.

## 5. Test quality

- **Fixtures vs the live callout:** `testdata/auth/token.json` / `token_rotated.json` carry exactly the live-confirmed field set -- `access_token`, `token_type: "Bearer"`, `expires_in: 43200`, `created_at` (unix seconds), `refresh_token`, space-separated `scope`, and the undocumented `direct_buy_tokens: {}`. Expiry math from `created_at + expires_in` (not now-based) is explicitly asserted (`auth/oauth_test.go:195-196`) -- the exact trap the live refresh observation flagged.
- **Sad/edge coverage:** 401 (auth shape), 404 (both family shapes), 422 field errors, 429 + `Retry-After` (numeric and HTTP-date), malformed JSON (success and error paths), cancelled context (before send, during backoff, mid-flight), retry exhaustion -- all present and asserting sentinel + message + no-token-in-error-string.
- **`t.Skip` audit:** exactly one, in `live_test.go:26` -- the documented `FRESHBOOKS_LIVE=1` opt-in gate from spec 8.1. Compliant.
- **No committed `-run` filters**; `-race` on everything via the gate.
- **Vacuous tests (confirming the simplification lane):** `version_test.go` asserts a compile-time constant is non-empty, and `TestPageMeta` (`page_test.go:135-142`) asserts struct field assignment without ever exercising the json tags it exists for. Neither pads coverage (no counted statements involved), so this is hygiene, not dishonesty. ADVISORY only.

## 6. Parity

`mise run check`'s inventory step: `implemented 4, ignored 0, todo 209, uncovered 0, double-covered 0, stale 0, unknown 0`. `ignore.list` contains 209 `//go:inventory-todo` directives and zero `Authorization/` lines (the 4 were removed, not demoted to ignore). 213 - 4 = 209 reconciles. The double-listed `users/me` endpoint carries both keys on `Me`, one comment per key -- not a double-cover, per the checker's per-key counting.

## Findings

### 1. BLOCKING -- `auth.Token` leaks both secrets through the value form of `%v`, contradicting its own documented guarantee

- `freshbooks/auth/token.go:38` -- `func (t *Token) String() string` (pointer receiver).
- Computed expectation: per the doc comment (`token.go:18-20`, "printing one with %v or %s cannot leak a credential") and `docs/authentication.md`, `fmt.Sprintf("%v", anyForm)` must redact.
- Observed (scratch test, run and then deleted): pointer form prints `auth.Token{AccessToken: redacted, ...}` but **`fmt.Sprintf("%v", *tok)` prints `{SECRET-ACCESS SECRET-REFRESH  [] 0001-01-01 ...}`** -- both credentials in full. `TestTokenStringRedacts` only exercises the pointer form, so the suite is green while the guarantee is false. This also fails the stage 2 acceptance clause "no token or secret in any log, error string, fixture, or doc" as a documented behaviour.
- This independently confirms the code-review and security lanes' blocking finding. Their proposed fix (value receiver + a `%v`-on-value assertion in the test) is correct and sufficient.

### 2. ADVISORY -- confirmed lane findings (verified cheaply, no fix attempted)

- **`testdata/{seed,auth}/users_me.json` residue** (security #4): `roles[0].id: 15660096` + `created_at: "2026-08-22T04:31:09Z"` do read as live-capture values amid otherwise synthetic IDs (70001/70004/1111111/8675309/4242424). Confirmed by inspection of both copies.
- **`roundTrip` re-derives the family from `req.URL.Path`** (code-review #3 / simplify #1): confirmed at `transport.go:157`. Extra teeth: `docs/library.md` explicitly documents "A path prefix is preserved" for `WithBaseURL`, so the misclassifying configuration is a documented, supported one, not a hypothetical.
- **Transport-error retries replay non-idempotent bodies** (code-review #5): confirmed by inspection -- `isRetryableTransportError` (`transport.go:219-221`) retries every non-context network error and `newRequest` rebuilds the body per attempt. Real at-least-once hazard for Phase 2 writes; fix or document.
- **`/events/` classified FamilyBusiness while the Postman example shows the accounting envelope** (code-review #2): confirmed at `client.go:198-207` (only `/auth/` and `/accounting/` prefixes special-cased). Needs an INFERRED marker or the fix before Phase 2's Callbacks batch.
- **Save-failure discards the only live token pair** (code-review #4 / security #2): confirmed by inspection of `token.go:183-187`. The persist-before-return invariant is right; the unrecoverability is the avoidable part.

### 3. ADVISORY -- security lane finding 6 (govulncheck not run) is now REFUTED

The gate runs `go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...` per module (`scripts/check.sh:55-59`) and all three printed `No vulnerabilities found` in this QA run. The security lane could not run it under read-only constraints; no CI gap exists. No action needed beyond noting it in triage.

### 4. ADVISORY -- vacuous tests

`version_test.go` and `TestPageMeta` as described in section 5. The simplification lane's proposed fixes (delete / convert to a real unmarshal assertion) are correct.

## Beyond-spec surface (checked, accepted)

`Identity.Whoami` and `Identity.Register` exceed spec 5.1's `Identity.Me`, justified by the method-vocabulary clause and inventory parity; `Sort` treats any `d`-prefixed direction as descending (documented on the function); `PageNumber`/`Search`-as-type are covered by the spec's own 2026-08-23 callout. Nothing undocumented or unjustified found.

## Commands run

```
mise run check                                   # full gate, exit 0
go -C freshbooks clean -testcache && mise run test    # fresh, incl. -race -tags integration
go -C freshbooks test -race -tags integration -count=1 -v -run 'TestExpiry...|TestFileStore...|TestAllAcross...' .
go -C freshbooks test -run QAScratch -count=1 -v .    # throwaway fidelity tests, file deleted after
scripts/redaction-check.sh                       # clean
git status --porcelain                           # empty apart from this report
```

## Bottom line

Fix the `Token.String` receiver (finding 1) in the fix commit -- ideally alongside the fixture residue and the save-failure/`roundTrip` advisories the other lanes proposed -- re-run `mise run check`, and this phase meets every stage 2 acceptance criterion.
