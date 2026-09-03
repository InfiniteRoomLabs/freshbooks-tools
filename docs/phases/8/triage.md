# Phase 8 gate triage (lead, 2026-09-03)

Inputs: `docs/phases/8/reports/{implementer,code-review,simplify,security}.md`. Verdicts: code review **REQUEST CHANGES** (R1, R2 blocking; R3-R12 advisory), security **BLOCK** (A1 blocking; A2-A9 advisory), simplification S1, S3-S6, S8 apply, S7/S9 optional, S2/S10/S11 rejected, S12 escalated (= R2 + R9). One opus fix agent, four checkpoint commits `fix(converge): apply the review-gate findings (Fa-Fb)`.

## Fix order

| F | Source | Action |
|---|---|---|
| F1 | A1, S6 | `scripts/redaction-check.sh` range mode: build the term-scan `content` from `added_lines_with_numbers ... | cut -f2-` (one extractor for both scans, no grep-dialect dependency); delete the BRE pipeline. |
| F2 | A1 | New `scripts/redaction-selftest.sh` (`usage` shebang, no args): creates a scratch git repo under `mktemp -d`, copies `redaction-check.sh`, plants a 7-digit integer in `freshbooks/testdata/seed/x.json` and, when the private resolver is present, one short and one long term from it; asserts BOTH modes exit 1 naming the file and line, and that a `700NN`, `8675309`, all-zero run, and `00000000-0000-4000-8000-000000000123` pass; cleans up. Wire it into `scripts/check.sh` once per gate (beside actionlint) and `mise.toml` as `redaction-selftest`. |
| F3 | A2 | Range mode validates the range first (`git rev-list --count "$range" >/dev/null` or exit 2 with `redaction-check: unusable range`). |
| F4 | A3, R4 | UUID handling: instead of stripping every 8-4-4-4-12 token, exempt only tokens whose segments are all zeros except the version/variant nibbles (this repo's synthetic convention) or that contain at least one non-decimal hex character; anything else stays subject to the digit sweep. Add both A3 probe rows to the selftest (`12345678-0000-4000-8000-000000000001` must fail, the synthetic one must pass). |
| F5 | A5, R5 | Sweep path widened to `freshbooks/testdata/*`. |
| F6 | S4, S5, S8, R3, A4 | Allowlist as one anchored alternation (`^(8675309|4242424|5555550100|5550100100|999999999|1111111|0+)$`; the dead `^700[0-9]{2}$` branch goes, with a comment that 5-digit ids are below the sweep threshold); digit sweep as one `grep -oE '[0-9]{6,}'` pass over the pre-processed content; `local` loop vars; the `4242424` comment says "the repo's synthetic identity_id". |
| F7 | A8 | `README.md` contributor line: the script needs `usage` on PATH (`mise install`), then no-ops without the private term list. |
| F8 | A9 | Spec section 10: "also run in CI" becomes "its self-test runs in the gate; the check itself is run by the lead before each commit and by the gate lanes with `--range`". |
| F9 | R2, S12 | `TimeEntriesPage.Totals` gets `json:"totals"`; unit test asserts the marshalled key. |
| F10 | R9, S12 | `time-entries list-with-totals`: `Short` says the totals are only in `-o json`/`-o yaml` (table mode shows the entries); regenerate `docs/cli.md`. No output-layer change. |
| F11 | R1, R7 | `docs/mcp.md` tool count 169 with the keyless caveat; `docs/phases/3/tools.md` rows 164-168 line references refreshed. |
| F12 | S1 | One `const wantRegistrySize = 169` per module used by the parity and roundtrip tests; `unit_test` asserts `len(Manifest()) == len(All)`; `run_test.go` asserts against `len(tools.All)`. |
| F13 | S3 | One private list-response struct in `time_entries.go`; `list` becomes a projection over the totals decoder (byte-identical `PageMeta`). |
| F14 | R8 | Every citation for the totals evidence uses `https://www.freshbooks.com/api/time_tracking` (code comment, spec bullet, progress.md). |
| F15 | R10 | Live ledger subtest: decode the raw body alongside and assert the count of non-null `jea_id`/`jesa_id`/`category_id` keys equals the count of non-nil decoded pointers. |
| F16 | A7 | `TestLiveExpenseFields` logs `Version != ""`, never the value. |
| F17 | A6 | Round the copied real instants: `Expense.Version` doc example and the two `testdata/accounting/expenses_*.json` `version` values to `...00:00:00.000000`. |
| F18 | R6 | `docs/progress.md` discovery: "the corpus carries both phone placeholders; both are allowlisted". |
| F19 | R12 | `reqOpts()`/`ReqOpts()` doc comments name `TimeEntries.ListWithTotals` too. |
| F20 | S9 | Drop the two `TimeEntryTotals` field comments that restate the type doc. |

Checkpoints: F1-F8 (script + docs), F9-F11 (wire tag + user-facing docs), F12-F14 (tests + decoder), F15-F20 (live tests, fixtures, prose). Full gate after each.

## Not applied

- **R11** (commit scope `feat(freshbooks)` on a cross-module commit): history only.
- **S2, S2b, S7, S10, S11**: rejected by the lane or below the threshold.
- **A6 in the seed capture and the Phase 1/7 precedent**: only the three copies this branch added are rounded; the earlier accepted values stay (Phase 7 A2 decision).
- **A9 as a CI step**: the self-test runs in the gate (F2); the check itself stays a lead/lane tool (F8 corrects the spec claim). Wiring `--range origin/main..HEAD` into CI needs a non-shallow checkout and adds nothing on `main` pushes.
- **`/tmp/probe/` housekeeping** (security, not a finding): the lead deletes it at ship.

## Lane-vs-lane

R2 and S12 (untagged `Totals`) and R9 and S12 (table mode drops totals) converged independently. A1 and S6 point at the same line from opposite directions: the simplification (one extractor) is the security fix.

## QA round (2026-09-03): NEEDS WORK -> fixed by the lead

- **Q1** (BLOCKING, docs only): the six time-entry rows in `docs/phases/3/tools.md` were off by four after F13 moved the functions again; **Q2** the six expense rows drifted when D2 grew `Expense`. All twelve recomputed from the `func` declaration lines in the QA-report commit; verified programmatically that every corrected row points at its `func` line.
- **Q3** (add a line-reference assertion to the parity parser): `docs/progress.md` backlog item 17; would have caught Q1/Q2 and the pre-existing drift on other rows.
- **Q4** (F21's baseline skip hides a numeric leak once it reaches HEAD): accepted; the term scan is not baselined, the range mode is not baselined, and the pre-commit control's job is new content. Recorded in the script header.
- **Q5** (`AccountName`/`BankName` asserted nowhere): backlog item 17 companion; the capture has them empty.
- **Q6** (term findings carry no line number): backlog item 17.
- **Q7** (`-o table` for list-with-totals unproven with real rows): the account has no time entries; F10 documented the limitation.
