# Phase 7 gate triage (lead, 2026-09-03)

Inputs: `docs/phases/7/reports/{implementer,code-review,simplify,security}.md`. Verdicts: code review **REQUEST CHANGES** (R1 blocking, R2-R7 advisory), security **BLOCK** (A1 blocking, A2-A7 advisory, A8-A9 pass), simplification S1-S3 apply, S4-S6 optional, S7-S10 rejected, plus one out-of-lane behaviour catch. Fourteen items: one opus fix agent with checkpoint commits, `fix(live): apply the review-gate findings (Fa-Fb)`.

## Fix order

| F | Source | Action |
|---|---|---|
| F1 | A1 | `freshbooks/testdata/seed/ledger_accounts/list.json`: rewrite the six `jea_id`/`jesa_id` values into the `700NN` range (keep `null` where null). Add "every integer and uuid in a capture, not only top-level ids" to the capture rule in `docs/phases/7/plan.md`. |
| F2 | R1 | `freshbooks/CHANGELOG.md`: move the ledger taxonomy entry under a `### Changed` heading above `### Fixed`, opening with `**Breaking:**`. |
| F3 | simplify (out of lane) | `freshbooks/expenses.go` `Vendors`: `vendors := []string{}` so an account with no vendors encodes `[]`, not `null`; unit subtest for the empty page asserts a non-nil empty slice. |
| F4 | R2 | `freshbooks/live_conformance_test.go`: the four data-absence `t.Skip`s become `t.Log(...); return`; the `TestLiveDateTimeFormats` skip at the projects branch becomes an assertion on the first project's timestamp layout when `Total != 0`. |
| F5 | R3 | Spec section 5.1 fact-Q bullets: reword the `invoices[].*` and `clients[].signup_date` rows as "observed in an uncaptured CLI probe (2026-09-02)"; no invoices capture (real books). |
| F6 | R4 | Align J1 and E dates: the vendors and gateways probes ran 2026-09-02 (the first run, before the pause). Code comments, `freshbooks/CHANGELOG.md`, and the spec bullets for J1/E all say 2026-09-02. |
| F7 | R5 | Flag help (`auth_cmd.go`), `docsgen.go` header, `paths_test.go` subtest name: "the 43 grantable `user:*` scopes this toolset's endpoints use (each must be enabled on the app)"; `mise run docs`. |
| F8 | R7 | Root `CHANGELOG.md` `[Unreleased]` `### Added`: Phase 7 shipped -- live suite, nine capture families, scopes docs, the two CLI auth fixes, pointer to `docs/phases/7/`. |
| F9 | A2 | Round capture timestamps where no layout test depends on the exact value: gateways dates to midnight (`T00:00:00Z`), ledger `updated_at` fraction zeroed (`.000000Z`, keep the fractional layout), expenses `updated`/`version` fraction zeroed; the spec fact-Q `version` example replaced with a synthetic one. Check `gateways_test.go`/`ledger_accounts_test.go`/`expenses_test.go` do not pin the old values first. |
| F10 | A4 | Both gateway JSON files: `pk_live_example...` -> `pk_test_example...`. |
| F11 | A5 | `cli/internal/auth/status.go`: the save-after-refresh error appends "; the rotated token could not be stored, run 'freshbooks auth login' again"; the `[sad]` save-failure test asserts the new text. |
| F12 | S5 | `cli/internal/auth/status.go`: one `clock(now)` helper replacing the two nil-clock defaults (same commit as F11). |
| F13 | S1 | `live_conformance_test.go`: `liveSetup(t)` helper replaces the four-line preamble in eight tests; `liveScope`/`liveCtx` removed. |
| F14 | S2, S3 | Inline `expenseVendorsEnvelope` (keep the method doc, drop its Phase 2 parenthetical); drop the pass-through closure in `gateways_test.go`. |

## Not applied

- **A3** (perturb the Stripe config booleans): configuration, not PII, and the shape is the evidence; left as captured.
- **S4, S6**: below the threshold where a table or helper helps.
- **A6, A7, A8, A9, S7-S10**: no action; recorded.
- **History scrub for A1**: not needed, the branch is local-only and the values are non-secret row ids; the fix commit is enough.

## Lane-vs-lane

R6 (code review) and A1 (security) are the same finding, found independently. The dates disagreement (R4) is real: the first implementer run captured vendors and gateways on 2026-09-02, the resumed run wrote the callouts on 2026-09-03.
