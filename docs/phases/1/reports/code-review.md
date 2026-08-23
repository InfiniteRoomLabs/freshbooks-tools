# Phase 1 (lib-core) -- code review lane report

Branch `phase-1/lib-core` (f287f86..1616106 on a9ba5e8), reviewed read-only against spec sections 3 / 5.1 / 8.1 (both STATE AS OF callouts), the Postman inventory, the seed captures, and the implementer report. No gate runs performed (QA lane owns them).

## Verdict: REQUEST CHANGES

One blocking finding (a one-line fix), six advisories. Overall the phase is strong: the three-family envelope handling, retry semantics, rotation write-back ordering, query encoding, error decoding, and the test suite all check out against the spec and the live-confirmed callouts. The implementer report's claims were verified and are accurate.

## Findings

### 1. BLOCKING -- `auth.Token.String` is on the pointer receiver, so a dereferenced Token value leaks both secrets

`freshbooks/auth/token.go:38`. `String()` has a pointer receiver, so a `Token` *value* does not satisfy `fmt.Stringer` (Go method sets; fmt does not take the address). `fmt.Sprintf("%v", *tok)` -- or `slog.Info("x", "token", *tok)` -- prints every field, access and refresh token included. This directly contradicts the guarantee stated in the type's own doc comment ("printing one with %v or %s cannot leak a credential", token.go:19-20) and repeated in `docs/authentication.md` ("so %v on a token cannot leak one"). `TestTokenStringRedacts` (token_test.go:40) only exercises the pointer form, so the gap is untested.

**Fix:** change to a value receiver: `func (t Token) String() string`. The pointer form then satisfies Stringer automatically, and fmt renders a nil `*Token` as `<nil>` on its own, so the nil branch can go. Add `fmt.Sprintf("%v", *tok)` to the redaction test.

### 2. ADVISORY -- `familyForPath` classifies `/events/` (webhooks) as FamilyBusiness, but the collection's example response is the accounting envelope

`freshbooks/client.go:198-207`; asserted as intended behavior in `client_test.go:251`. The Postman example for `Webhooks/List Webhook Callbacks` (`GET /events/account/{accountId}/events/callbacks`) responds `{"response": {"result": {"callbacks": [...] ...}}}` -- the accounting envelope, which is consistent with the endpoint being account_id-scoped. As shipped, `(*Client).Do` on a webhooks path hands the caller the un-peeled envelope, and the test bakes the classification in for Phase 2. Evidence is INFERRED (Postman example, not live), so either fix now (`case strings.HasPrefix(path, "/events/"): return FamilyAccounting` plus the test line) or record it as INFERRED with a note so Phase 2's Callbacks batch confirms it live -- but do not leave the current classification looking CONFIRMED. Same caveat applies to `/payments/` and `/uploads/` defaulting to business (the inventory classifier treats events/payments/uploads/ledger as their own families; `types.go:33`'s "constant values match the inventory tool's classifier" overstates the match).

### 3. ADVISORY -- `roundTrip` re-derives the family from `req.URL.Path`, disagreeing with the family `do()` was given when the base URL carries a path prefix

`freshbooks/transport.go:157`. `do()` receives `fam` explicitly, but on a non-2xx `roundTrip` calls `familyForPath(req.URL.Path)`, which includes any `WithBaseURL` path prefix. With `WithBaseURL("https://proxy.example/v1")`, every error is tagged `FamilyBusiness` regardless of the real family (decode itself is shape-agnostic, so only the `Error.Family` field is wrong -- but it is a documented field). **Fix:** thread `fam` from `do()` into `roundTrip` instead of re-deriving it.

### 4. ADVISORY -- a Save failure after a successful refresh discards the rotated pair, guaranteeing the stored refresh token is permanently dead

`freshbooks/auth/token.go:183-185`. When `Refresh` succeeds but `store.Save` fails, the new pair (the only live refresh token in existence -- the old one is already spent server-side) is dropped on the floor: `s.cached` still holds the dead pair, so every subsequent call re-spends the dead token and gets `invalid_grant` until the user re-authenticates. Failing the call is right (the test at token_test.go:160 covers it), but discarding `next` converts a transient store failure (disk momentarily full) into a forced re-auth. **Fix:** stash `next` in a pending field on save failure and retry `Save` (not `Refresh`) on the next `Token` call before returning it. Keeps the persist-before-use invariant while making the failure recoverable.

### 5. ADVISORY -- transport-level errors are retried for non-idempotent methods

`freshbooks/transport.go:75-79` with `isRetryableTransportError` (line 219). The spec mandates retrying 429/502/503/504; the implementation additionally retries any network error that is not a context cancellation. A timeout after the server processed a POST replays the body (`transport_test.go:337` proves the replay) and can double-create an invoice or payment -- a real hazard for an accounting API. 502/504 carry the same theoretical risk but are spec-locked; the network-error extension is not. **Fix (pick one):** gate transport-error retries on idempotent methods (GET/HEAD/PUT/DELETE), or document the at-least-once semantics prominently in `RetryPolicy`/`docs/library.md` and let Phase 2's write-heavy services revisit.

### 6. ADVISORY -- the business-family query encoding is not marked INFERRED in the spec callout

Spec 5.1, `STATE AS OF 2026-08-23` callout: "accounting spells them search[field]=value, business-scoped spells them field=value". The accounting spelling is CONFIRMED (official docs); the business-scoped bare `field=value` spelling is inferred, as the implementer report itself says. Convention 9.6 requires the marker. **Fix:** add "(INFERRED -- Phase 2's first business-scoped list must confirm)" to that sentence in the callout.

### 7. ADVISORY -- two fixtures are copied into `testdata/` but never read by any test

`freshbooks/testdata/accounting/clients_list.json` and `freshbooks/testdata/projects/list.json` duplicate their `testdata/seed/` counterparts and have no consumer this phase (only the error fixtures are exercised). Harmless, but the seed directory is already documented as the fixture source of truth; either leave them in `seed/` only until the Phase 2 batch that uses them, or accept the duplication knowingly. Low priority.

## Judgments on the implementer's seven flagged decisions

1. **Page/PageNumber rename** -- correct and well-recorded (spec callout + doc comment on `PageNumber` + docs/library.md). Accept.
2. **`Search` as a named map type implementing RequestOption** -- clean; both spec spellings compile, merge semantics tested. Accept.
3. **Three-family envelope / three-valued `Family`** -- matches the live CONFIRMED callout exactly; `unwrap`'s three cases plus the empty-envelope and no-result-layer edges are tested. Accept (with the `types.go:33` classifier-match overstatement noted in finding 2).
4. **INFERRED business query encoding** -- implementation is the reasonable reading; the spec callout just needs the INFERRED marker (finding 6). Accept.
5. **No testify** -- the stdlib tests are thorough and idiomatic; zero test dependencies is a better outcome than the work order's letter. Accept; do not convert.
6. **`Identity.Register` for parity** -- endpoint and body match the collection (`POST /auth/api/v1/smux/registrations`, flat body incl. the odd `currencyCode` camelCase); password is json-tagged, never logged, and the doc comment steers callers to hosted signup. Accept.
7. **golangci exclusions** -- `G101`/`G304` scoped to `_test.go` only with a reasoned comment; the three non-test `#nosec` sites each carry a reason. Proportionate. Accept.

## What was checked and found correct (spot-verified, not exhaustive)

- **URL templates and ID families:** `GET /auth/api/v1/users/me` (auth), `smux/registrations` (auth), form-POST revoke (the collection's JSON-body revoke is overridden by the live CONFIRMED form-encoding -- correctly resolved in favor of the live API). `identityResponse` matches the live seed capture (top-level first/last name, `business_memberships[].business.{id,account_id,business_uuid,name}`, `membership.role`).
- **Retry semantics:** 429/502/503/504 only among statuses; 401/400 never retried (tested); `Retry-After` in both RFC 9110 spellings, capped by MaxDelay; jitter injectable; context cancellation before send, during backoff, and mid-flight all tested; token resolved once per logical request (right call -- re-resolving per attempt risks double-spending a rotating refresh token).
- **Rotation write-back:** persist-before-return ordering enforced and tested (memory + file stores, failing-store, no-refresh-token-in-response, 32-goroutine single-flight spending the one-time token exactly once, cross-process durability via FileStore).
- **Expiry arithmetic:** `created_at + expires_in` with `now` fallback, exactly per the live callout; skew boundary tested inclusive.
- **Secret hygiene:** `*url.Error` stripped in both packages, `redactPath`/`endpointName` drop query strings, `Error.Error()` never renders `Raw`, tests assert token/client-secret/absence in error strings. (Finding 1 is the one gap.)
- **Query encoding:** `include[]` repeated via Add, `search[field]` vs bare field per family, `sort=<field>_<asc|desc>`, page/per_page omitted when non-positive -- all tested against `url.Values.Encode()` ground truth, not the implementation.
- **Redirect guard:** shallow copy, caller's CheckRedirect respected, cross-host strip end-to-end tested with two servers. (The 10-hop cap returning `http.ErrUseLastResponse` surfaces a redirect loop as a 3xx `*Error` rather than an error -- unusual but coherent with the transport, and it avoids retrying a redirect loop; no change requested.)
- **Bounded reads:** 10MB / 1MB via `LimitReader(n+1)` with explicit over-limit errors, both tested.
- **Conventions:** doc comments present and accurate on every exported identifier sampled; `// inventory:` comments match the four Authorization keys (the double-listed users/me endpoint correctly carries both keys on `Me`); changelog entry complete; ASCII-only docs; no hard-wrapped prose violations introduced; fixtures use synthetic IDs (ACM123, 8675309); no internal hostnames or real IDs anywhere in the diff.
- **Test hygiene:** no vacuous asserts found; sad/edge/corner paths are systematically present (121 subtests); the single `t.Skip` is the documented spec-8.1 live gate with an in-code justification; determinism via `WithClock`/identity jitter throughout.
