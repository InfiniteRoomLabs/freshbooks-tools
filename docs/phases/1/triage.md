# Phase 1 (lib-core) -- gate triage

Lead triage of the four lane reports (`docs/phases/1/reports/{code-review,simplify,security,qa}.md`), 2026-08-23. Verdicts as received: code-review REQUEST CHANGES (1 blocking / 6 advisory), simplification 3 APPLY / 4 OPTIONAL / 7 DO-NOT-APPLY, security BLOCK (1 blocking / 5 advisory), QA NEEDS WORK (1 blocking, empirically confirmed). All three judging lanes converged on the same blocker.

## Fix in the fix commit (one commit: `fix(lib-core): apply the review-gate findings`)

| # | Finding | Source | Action |
|---|---|---|---|
| F1 | `auth.Token.String` pointer receiver leaks both secrets via `%v` on a Token value | review 1 + security 1 + QA 1 (confirmed) | Value receiver; drop the manual nil branch (fmt renders nil `*Token` as `<nil>`); extend `TestTokenStringRedacts` with `%v` on the value and `%+v` on a struct embedding `Token` by value |
| F2 | Save failure after a successful refresh discards the only live token pair | security 2 + review 4 | Keep the rotated pair in the source with a pending-save flag; retry `Save` (not `Refresh`) at the top of later `Token` calls; still return the save error to the current caller. Test: failing-then-healed store recovers without a second refresh |
| F3 | Auth package HTTP client defaults: no timeout, redirects would replay the credential body | security 3 | Default `&http.Client{Timeout: 30s, CheckRedirect: ErrUseLastResponse}` when the caller supplies none; token endpoints never legitimately redirect |
| F4 | Residual real capture values in `users_me` fixtures (`roles[0].id` 15660096 + registration timestamp, `links.destroy` path) | security 4 + QA 2a | Replace with synthetic id (70005) and round timestamps in BOTH `testdata/seed/users_me.json` and `testdata/auth/users_me.json` |
| F5 | Redirect cap hands back the final 3xx via `ErrUseLastResponse` instead of erroring | security 5 | Return a real "stopped after 10 redirects" error |
| F6 | `roundTrip` re-derives the family from `req.URL.Path`, disagreeing with `do()` under a `WithBaseURL` path prefix | simplify 1 + review 3 + QA 2b | Thread `fam` from `do()` into `roundTrip`; delete the recompute |
| F7 | `/events/` classified FamilyBusiness but the Postman example response is the accounting envelope | review 2 + QA 2d | Reclassify `/events/` as FamilyAccounting; fix the test line; soften the `types.go` "matches the inventory classifier" comment; add an INFERRED spec callout (Postman evidence only) for Phase 2's Callbacks batch to confirm live |
| F8 | Business-family bare `field=value` query encoding not marked INFERRED in spec 5.1 callout | review 6 | Add the INFERRED marker + "Phase 2's first business-scoped list must confirm" |
| F9 | Transport-error retries replay non-idempotent bodies (at-least-once hazard) | review 5 + QA 2c | Document the at-least-once semantics on `RetryPolicy` and in `docs/library.md` now; revisit idempotency gating in Phase 2 when write-heavy services land. Deliberate deferral, recorded here |
| F10 | Vacuous tests: `version_test.go`, `TestPageMeta` | simplify 2, 7 + QA 4 | Delete `version_test.go`; rewrite `TestPageMeta` to unmarshal a JSON object so the json tags are actually exercised |
| F11 | `serveFixture` duplicates `readFixture` | simplify 3 | Call `readFixture` from `serveFixture` |

## Considered, not applied (recorded so nobody re-derives them)

- **simplify 4 (delete `testdata/seed/`):** kept. The spec designates `seed/` as the Phase 1 fixture source of truth and Phase 2 batches seed from it; ~210 duplicated lines is acceptable rent. Review 7 (two copied fixtures unread this phase) falls with this.
- **simplify 5 (unused envelope fields `Object`/`Value`):** kept -- they document the observed wire shape per the 2026-08-23 callout.
- **simplify 6 (`max(attempts,1)` armor):** kept -- one cheap guard on a retry loop, per the lane's own hedge.
- **simplify 8-14 (DO-NOT-APPLY list):** agreed, not applied.
- **security 6 (govulncheck not run):** REFUTED by QA -- the gate runs govulncheck per module (`scripts/check.sh:55-59`); all three modules clean this run. No action.
- **review 5 alternative (gate transport-error retries on idempotent methods):** deferred to Phase 2 by F9, not silently dropped.
- **testify conversion (implementer report 5):** rejected per code-review -- zero test dependencies beats the work order's letter.

## Overrides

None. Every blocking finding is being fixed; every deferral is recorded above.
