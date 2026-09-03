# Phase 7 fix report -- `phase-7/live` (2026-09-03)

Order: `docs/phases/7/triage.md`, F1 through F14 in that order. Three checkpoint commits on `phase-7/live` from `b83d31a`. No live calls were made, no `-tags live` test was run, no token was looked for; every change is text, fixtures, tests, and one deliberate behaviour fix (F3). Nothing pushed, merged, or tagged.

## What was applied

| F | Source | Status | Commit | Note |
|---|---|---|---|---|
| F1 | A1 / R6 | done | `fbbade5` | six `jea_id`/`jesa_id` values -> `70020`..`70025` in file order, `null` kept where null; capture rule added to the plan. **Also redacted the same six literals from the committed `code-review.md`** -- see "Where the triage was wrong". |
| F2 | R1 | done | `fbbade5` | ledger taxonomy entry moved to a new `### Changed` above `### Fixed`, opening `**Breaking:**`. |
| F3 | simplify (out of lane) | done | `fbbade5` | `vendors := []string{}`; the empty-page subtest asserts non-nil **and** that `json.Marshal` yields `[]`. |
| F4 | R2 | done | `fbbade5` | four data-absence `t.Skip` -> `t.Log(...); return`; the projects branch now asserts `created_at`/`updated_at` layouts when `Total != 0`. |
| F5 | R3 | done | `edaaadf` | `invoices[].*` and `clients[].signup_date` reworded as observed in an uncaptured CLI probe (2026-09-02); the captured rows name their capture. |
| F6 | R4 | done | `edaaadf` | J1 and E dated 2026-09-02 in `freshbooks/CHANGELOG.md` and the spec bullets; the code comments already said 2026-09-02, so none changed. |
| F7 | R5 | done | `edaaadf` | flag help, docsgen header, subtest name; `mise run docs` regenerated `docs/cli.md` (both the header paragraph and the flag line). Drift test green. |
| F8 | R7 | done | `edaaadf` | root `CHANGELOG.md` `[Unreleased]` `### Added` Phase 7 line. |
| F9 | A2 | done | this commit | gateway stamps -> `T00:00:00Z` (x4); ledger `updated_at` fractions -> `.000000Z` (x5, layout kept); expenses `version` fraction -> `.000000`; spec fact-Q example replaced with a synthetic one. |
| F10 | A4 | done | this commit | `pk_live_` -> `pk_test_` in both gateway JSON files. |
| F11 | A5 | done | this commit | save-after-refresh error appends the re-login sentence; the `[sad]` subtest asserts both halves of the new text. |
| F12 | S5 | done | this commit | one `clock(now)` helper replaces the two nil-clock defaults. |
| F13 | S1 | done | this commit | `liveSetup(t)` replaces the four-line preamble in all eight tests; `liveScope`/`liveCtx` removed. |
| F14 | S2, S3 | done | this commit | `expenseVendorsEnvelope` inlined, its Phase 2 parenthetical dropped (one CONFIRMED marker kept on the method doc); pass-through closure dropped in `gateways_test.go`. |

Nothing under "Not applied" was touched: A3 (the Stripe config booleans), S4, S6, A6-A9, S7-S10, and the history scrub are all as the triage left them.

## Checks before each F9 fixture edit

Every capture value F9 touches was grepped for a test that pins it, per the order:

- `gateways_test.go` asserts `StripePayoutsScheduleInterval == "daily"`, `StripePayoutsScheduleDelayDays == 7`, and `ChargesFirstEnabledAt.IsZero()`/`StripeAccountUpdatedAt.IsZero()`. No exact timestamp is pinned, so midnight rounding is safe; the two values that *are* pinned belong to A3, which is not applied.
- `ledger_accounts_test.go` pins no timestamp at all. The five seed `updated_at` values kept their fractional layout (`.000000Z`), which is what the live test and the spec callout are about. The `testdata/ledger_accounts/*.json` unit fixtures are Postman-derived 2022 values, not live captures, and were left alone.
- `expenses_test.go` pins only `Money{Amount: "79.73"}`. No timestamp expectation changed anywhere, so no test needed updating for F9.

## Where the triage was wrong

**F1 was incomplete: the six live row ids were in two tracked files, not one.** `docs/phases/7/reports/code-review.md:43` quoted all six literals verbatim while reporting them as a leak. Rewriting only the capture would have left the exact values in a tracked, public-bound markdown file, so F1 as written did not close A1. I redacted them there in the same commit -- the literals become "(two 7-digit values)" / "(four 8-digit values)" and the finding points at the security lane's A1 for the shape evidence, so the report still carries its argument. `grep` over the whole tree now returns zero hits for all six. Flagging rather than assuming: if the lead would rather the report kept its literals, revert that hunk of `fbbade5`.

Three smaller deviations, all inside the letter of the order:

- **F4**: the projects subtest was named "no live producer for the zoneless layout". Since the branch now asserts a producer when one exists, that name would have been actively wrong, so it is "the zoneless layout's only candidate producer is Projects". The zoneless layout constant is duplicated as a documented local const -- `noZoneLayout` is unexported and this is an external test package.
- **F9, expenses**: the order says "expenses `updated`/`version` fraction zeroed". `updated` is `"2026-08-28 18:02:59"` and carries no fraction, so only `version` changed. `updated` keeps its seconds precision, which is also what preserves the fact-Q layout evidence for that row.
- **F10**: `pk_live_` also appears in `freshbooks/internal/inventory/testdata/*` (FreshBooks' own published Postman collection) and in the Phase 0/2 lane reports quoting it. All are pre-existing on `main`, outside this branch's diff, and outside F10's stated scope ("both gateway JSON files"). Left alone; worth a separate decision if the scanner noise is ever a real cost.

## Gate

`mise run check` full-workspace, after the last edit. All three coverage gates pass, no vulnerabilities, inventory parity clean, cross-compile matrix green. The only failing step is the dirty-tree banner, which is expected before the commit this report lands in.

```
coverage-gate: <repo root>/freshbooks/coverage.out total = 91.8% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/cli/coverage.out total = 91.6% (floor 90%)
coverage-gate: PASS
inventory-check: implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0
```

`scripts/redaction-check.sh` was run against the staged index before each of the three commits: clean each time.
