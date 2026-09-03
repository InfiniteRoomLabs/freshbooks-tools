# Phase 8 implementer report

Branch `phase-8/converge`, four checkpoint commits (D1, D2, D3, this docs/changelog commit). Unattended; no live calls were made (no token was available, and none was sought).

## D1 -- time-entry totals

`freshbooks/time_entries.go` gains:

- `TimeEntryTotals{TotalLogged int; TotalUnbilled int; PerTeamMember json.RawMessage; PerClient json.RawMessage}`
- `TimeEntriesPage{Page[TimeEntry]; Totals TimeEntryTotals}`
- `TimeEntriesService.ListWithTotals(ctx, businessID BusinessID, opts ...RequestOption) (*TimeEntriesPage, error)` -- one request, decoded into a private `timeEntriesListWithTotalsResponse` (an embedded `PageMeta` + `TimeEntryTotals` under `meta`, plus `time_entries`). `List` is untouched.

**Reality vs. plan:** the two breakdown lists' element shape is empty on the captured account and undocumented on https://www.freshbooks.com/api/time_entries (checked; the docs page shows only `total_logged`, `total_unbilled`, `per_page`, `total`, `page`, `pages` in its meta examples -- no populated `total_logged_per_team_member`/`total_logged_per_client`). Per the plan's fallback, both fields are `json.RawMessage` with a code comment and a spec 5.1 `STATE AS OF 2026-09-03 (Phase 8, convergence)` callout marking them INFERRED.

**Unplanned ripple:** `ListWithTotals` is a new exported method on `*TimeEntriesService`, and both `mcp/internal/tools` and `cli/internal/cmd` carry a `TestParityAgainstClient` that requires every such method to have a registered tool/command. This is not mentioned anywhere in the D1 text. I added `time_entries_list_with_totals` (MCP) and `time-entries list-with-totals` (CLI), both keyless (no inventory key -- same wire endpoint as `time_entries_list`/`time-entries list`, not a new Postman request), which required:
- Bumping `168 -> 169` in both modules' frozen tool/command-surface docs (`docs/phases/3/tools.md`, `docs/phases/4/commands.md`) and every hardcoded `168` count in their parity/roundtrip/unit tests (8 files total).
- Relaxing each module's `TestParityKeyCoverage` from a single hardcoded `"identity_whoami"`/`"identity whoami"` keyless exception to a small allowed-set (`keylessTools`/`keylessCommands`), since a second keyless entry is now legitimate.
- Regenerating `docs/cli.md` (`mise run docs`) so the CLI's docs-drift test stays green.

Both modules' generic, data-driven round-trip tests (`TestRoundTrip`) needed no per-tool/command fixture -- they synthesize input from the JSON schema and hit a generic fixture server, so the new entries were exercised automatically once the count was bumped. Full `mise run check` is green with this ripple included.

Unit tests: `TestTimeEntriesListWithTotals` (4 subtests) + fixture `freshbooks/testdata/time_entries/list_with_totals.json`.

## D2 -- unmodelled keys

`Expense` (`freshbooks/expenses.go`) gains 14 fields, all captured in `freshbooks/testdata/seed/expenses/list.json`:

`AccountingSystemID string`, `AccountName string`, `LegacyAccountID *string` (wire key `accountid`), `BackgroundJobID *int64`, `BankName string`, `BillMatches []json.RawMessage`, `Billable bool`, `ConverseProjectID *int64`, `ModernProjectID *int64`, `ExtAccountID *string`, `ExtInvoiceID int64`, `ExtSystemID int64`, `PotentialBillPayment bool`, `Version string`.

`LedgerAccount` (`freshbooks/ledger_accounts.go`) gains 3, captured in `freshbooks/testdata/seed/ledger_accounts/list.json`: `CategoryID *int64`, `JEAID *int64`, `JESAID *int64`.

Every field carries `omitempty` (no write-body change). Re-seeded `freshbooks/testdata/accounting/expenses_{list,get}.json` and `freshbooks/testdata/ledger_accounts/{list,get,create,update}.json` with the new keys (synthetic values, matching the capture's null/zero pattern, reusing the capture's own `70020`/`70021`/`70023`/`70024` `jea_id`/`jesa_id` values), with new assertions in `expenses_test.go` and `ledger_accounts_test.go`. Extended `TestLiveLedgerAccounts` with a decode-only subtest for the three new fields (no non-null evidence exists to assert a specific value against) and added `TestLiveExpenseFields`.

**Reality vs. plan:** the plan's phrasing for `LedgerAccount`'s three new fields ("`jea_id`/`jesa_id`/`category_id` on `LedgerAccount` as `*int64`/`*string` per the capture") is ambiguous about which type goes with which field, and the capture itself gives zero non-null evidence for `category_id` in any of the 5 rows -- there is no way to derive its type from the data. I typed all three `*int64`, matching this codebase's `*_id` convention elsewhere (`TransactionID`, `ProfileID` on `Expense`) rather than guessing `*string` for `category_id` alone. Flagged in the spec callout and re-cut as backlog item 16 (a coin flip pending real evidence).

No MCP/CLI ripple: `Expense`/`LedgerAccount` are never used as an input type in either module (confirmed by grep), only as return types, and MCP tool schemas are input-only (`Out` is `any`).

## D3 -- redaction script

`scripts/redaction-check.sh` gains `--range <base>..<head>` (scans `git diff <range> -U0`'s added lines instead of the staged index, same term-list logic and exit codes) and, in both modes, a 6+-digit-integer sweep over every changed `freshbooks/testdata/seed/` file, failing on anything not in an allowlist.

**Reality vs. plan, three findings:**

1. The script did **not** already carry the `#!/usr/bin/env -S usage bash` shebang the plan's text assumed -- it was plain `#!/usr/bin/env bash`. Converted it so the new flag is a real `#USAGE flag` (`usage lint` passes); this is the one shebang change in the whole phase, and it's additive (D4's "keep the shebang" decision governs scripts that already have it -- this one gained it because D3 required a `#USAGE` directive to exist at all).
2. The plan's allowlist names `5555550100` as the synthetic-phone-number placeholder. The actual pre-existing value in `freshbooks/testdata/seed/users_me.json:33` (flagged by Phase 7 QA Q4 by name) is `5550100100` -- a different number. Both are now allowlisted: the plan's literal value, and the one actually on disk.
3. The sweep needed a UUID-token exemption I hadn't anticipated: this repo's own `00000000-0000-4000-8000-000000000NNN` synthetic-uuid convention (A9's own description) has a trailing hyphen-segment that is itself an all-decimal 12-digit run (e.g. `000000000001`), which the naive `[0-9]{6,}` sweep flagged as an "unallowlisted number" on real, already-approved seed content. Fixed by stripping UUID-shaped substrings (8-4-4-4-12 hex) before the digit sweep, since a UUID is a hyphenated identifier, not an integer, and UUID redaction is a separate concern (the term-list scan) from this sweep.

**Verification performed** (recorded per the plan's ask for a hand check, since no `_test.sh` exists):

```
$ ./scripts/redaction-check.sh --range main..phase-8/converge
redaction-check: clean
```

Also hand-verified against the **full existing seed corpus**, not just this branch's diff (which touches no `seed/` files): every file under `freshbooks/testdata/seed/` was staged with a throwaway trailing-newline touch (22 files) and swept in staged mode -- clean, then reverted with no trace (`git restore --staged` + a working-tree copy restore, no `git reset --hard` used). Also exercised the detector positively with a scratch probe file containing both allowlisted and non-allowlisted numbers, confirming real leaks (`1825574`, `123456`) are still caught with correct `file:line` in both staged and `--range` modes, and that all-zero UUID segments and the documented placeholders are not false positives.

`scripts/redaction-check.sh` itself is never swept by its own number check (the sweep only scans `freshbooks/testdata/seed/*`), so its allowlist literals are exempt by path, not by number.

## D4 -- no-op, confirmed

The plan's D4 ("`usage` shebang stays") required no code change. Recorded as decided (not just deferred) in `docs/progress.md` backlog item 9, including the note that D3 added a new `usage`-wrapped script.

## D5 -- docs and changelogs

- `freshbooks/CHANGELOG.md`, `mcp/CHANGELOG.md`, `cli/CHANGELOG.md`: `[Unreleased]` `### Added` entries for D1/D2 (lib) and the MCP/CLI ripple.
- Root `CHANGELOG.md`: `### Added` rollup bullet for the phase, `### Changed` bullet for D3.
- `docs/progress.md`: backlog items 9, 12, 14 closed; items 15 (time-entry totals breakdown shape) and 16 (`LedgerAccount.CategoryID`'s unconfirmed type) re-cut from 12 and 14's resolutions; a new "Current state" bullet; the phase ledger table gains a row (numbered 9, since the existing row 8 is a different, already-shipped "v0.2.0 / 0.1.1 releases" step -- GOAL.md's "Phase 8" and the ledger's row count are two different numbering schemes, a pre-existing quirk not introduced here); "Next action" rewritten from "run the goal" (stale) to "the review gate is next."
- `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` section 5.1: two `STATE AS OF 2026-09-03 (Phase 8, convergence)` callouts (D1, D2).
- `docs/library.md`: one sentence on `ListWithTotals`.

## Gate

Coverage-gate lines from the last full `mise run check` (background run, tail below):

```
coverage-gate: freshbooks/coverage.out total = 91.9% (floor 90%)
coverage-gate: PASS
coverage-gate: mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: PASS
coverage-gate: cli/coverage.out total = 91.6% (floor 90%)
coverage-gate: PASS
```

`inventory-check: freshbooks` -- `implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0` (unchanged; D1/D2 added no new inventory-covered methods). `mise run check` otherwise all green (`fmt-check`, `vet`, `lint` 0 issues, `test`, `vuln` no vulnerabilities, `actionlint`, `build` all six platform/module combinations); the only non-zero exit at each intermediate checkpoint was the expected dirty-tree banner while changes were staged but not yet committed. `go vet -tags live ./freshbooks/...` compiles clean (not run -- no token).

## `git log --oneline main..phase-8/converge`

```
d9e9e95 chore(scripts): give redaction-check.sh --range mode and a seed-number sweep
33aae48 feat(freshbooks): model 14 unmodelled Expense keys and 3 on LedgerAccount
3f2b1d3 feat(freshbooks): surface time-entry totals via ListWithTotals
ccccafc docs(phase-8): add the convergence plan and work order
```

(This report's commit lands as a fifth entry after this file is written and committed.)

## `git status --porcelain`

Empty after this report is committed (verify at commit time).

## Summary of everything reality disagreed with the plan on

1. D1's MCP/CLI parity ripple (168 -> 169 tools/commands) -- not in the plan's text at all, required by the existing frozen-surface tests.
2. D2's `LedgerAccount.CategoryID` type is a guess (`*int64`), not derivable from the capture; the plan's phrasing didn't clearly assign it.
3. D3: `scripts/redaction-check.sh` didn't have the `usage` shebang the plan assumed.
4. D3: the plan's `5555550100` placeholder doesn't match the real `5550100100` already on disk.
5. D3: the sweep needed a UUID-token exemption not mentioned in the plan, or it would have failed on the repo's own existing, already-approved synthetic UUIDs.

None of these blocked the work; all five are resolved in the diff and called out above, in the relevant commit messages, and in the spec/progress.md callouts.
