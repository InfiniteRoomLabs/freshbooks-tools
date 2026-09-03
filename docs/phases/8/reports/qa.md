# Phase 8 QA / reality check -- `phase-8/converge`

Subject: branch `phase-8/converge` at `33ea8e1`, 11 commits ahead of `main` (`fbc62c4`). Read-only against the source; the only file this lane writes is this report. All live calls were read-only (`GET`), against the operator's real production books; no token value was printed, logged, or stored, and no create/update/delete was issued.

## Verdict: **NEEDS WORK**

One blocking finding (Q1). Everything else on the mandate passed, most of it with independent evidence rather than by re-reading the fix report: the gate is green, the live suite is 857/0/0 with the new coverage present, the captured-response decode probe is 29/29 including a non-vacuity re-run, the redaction control fires for real in both modes against a planted term drawn from the live private resolver, and the wire output carries `totals` and the new expense keys.

Q1 is not a code defect. It is a triage item (F11) that the fix report records as landed and verified, which demonstrably did not land: six documentation line references that were **correct on `main`** are wrong on this branch, all by exactly +4. The branch left `docs/phases/3/tools.md` less accurate than it found it, in the one section F11 existed to refresh, and nothing in the gate can see it.

---

## Findings

### Q1 -- BLOCKING -- F11 did not land: `docs/phases/3/tools.md` rows 164-169 point at the wrong lines

`docs/phases/3/tools.md:172-177`

The file's convention -- verified against 143 other rows that are exactly right -- is that `(file.go:N)` names the line of the `func` declaration. Example, untouched by this branch:

```
| 153 | `taxes_list` | `Taxes.List` (taxes.go:101) | ... |
freshbooks/taxes.go:101:func (s *TaxesService) List(...)      # exact
```

**Expected** (row -> the `func` line at HEAD) vs **observed** (what the row says):

| Row | Method | Doc says | `func` is at | Line the doc points at |
|---|---|---|---|---|
| 164 | `TimeEntries.List` | `time_entries.go:161` | `:157` | blank line |
| 165 | `TimeEntries.Search` | `:183` | `:179` | `}` |
| 166 | `TimeEntries.Create` | `:208` | `:204` | `var resp timeEntryResponse` |
| 167 | `TimeEntries.Update` | `:235` | `:231` | `var resp timeEntryResponse` |
| 168 | `TimeEntries.Delete` | `:251` | `:247` | blank line |
| 169 | `TimeEntries.ListWithTotals` | `:173` | `:169` | the doc comment of `Search` |

Row 169 is the worst of the six: it points a reader at a different method entirely.

**This is a regression, not inherited debt.** On `main` all five pre-existing rows were exact:

```
main: | 164 | `TimeEntries.List` (time_entries.go:108) |
main: freshbooks/time_entries.go:108:func (s *TimeEntriesService) List(...)   # exact
main: 119 Search / 144 Create / 171 Update / 187 Delete -- all exact
```

`docs/phases/8/reports/fix.md` states these were "re-checked after F13 and corrected in the same commit -- final values `time_entries.go:161/183/208/235/251` for rows 164-168 and `:173` for row 169". Those are the values in the file, and all six are wrong. The +4 offset in every row is consistent with a re-derivation taken against the checkpoint-2 tree, before F13 removed four lines above these functions.

**Correct values at HEAD**: 164 -> `:157`, 165 -> `:179`, 166 -> `:204`, 167 -> `:231`, 168 -> `:247`, 169 -> `:169`.

**Why the gate is green anyway.** The parity tests parse these rows (`mcp/internal/tools/parity_test.go:35` documents the format and the row regex captures it) but assert only the tool name, method name and inventory keys -- never the line number. The drift is entirely ungated.

Verification command (reproducible; it re-derives every row's `func` line and diffs):

```
grep -oE '`[A-Za-z]+\.[A-Za-z]+` \([a-z_]+\.go:[0-9]+\)' docs/phases/3/tools.md
# for each: sed -n "${line}p" freshbooks/${file} must contain ") ${Method}("
# result at 33ea8e1: 169 refs checked, 26 drifted
# result at main:    168 refs checked, 14 drifted
```

### Q2 -- ADVISORY -- the same branch newly drifted six `Expenses.*` rows it did not claim

`docs/phases/3/tools.md` rows for `Expenses.List/Get/Create/Update/Delete/Summaries`

D2 added 14 fields to the `Expense` struct, pushing every `ExpensesService` method down ~56 lines. Six rows that were exact on `main` are now wrong (`Expenses.List` says `expenses.go:171`, the `func` is at `:227`). Unlike Q1 this was never a triage item, so it is collateral rather than an unmet commitment -- but it is the same defect class, introduced by the same branch, and it should be fixed in the same pass. Correct values: List `:227`, Get `:255`, Create `:274`, Update `:297`, Delete `:326`, Summaries `:364`, and while in there the two pre-existing ones, Vendors `:394`, CreateRecurring `:478`.

Fourteen further refs (all `LedgerAccounts.*`, all `Staff.*`, `Gateways.Get`, `Expenses.Vendors`, `Expenses.CreateRecurring`) were already drifted on `main` and are out of this phase's scope. Net: `main` 14 drifted, branch 26.

### Q3 -- ADVISORY -- consider a gate check for the line references

`mcp/internal/tools/parity_test.go:35`

Q1 and Q2 both exist because nothing validates the `(file.go:N)` half of a row that two tests already parse. The check is ~10 lines inside the existing parser (`sed -n Np` must contain `) Method(`), and it would have caught both. Without it the next struct edit silently re-breaks the table. Recommend as a Phase 9 backlog item rather than a Phase 8 blocker.

### Q4 -- ADVISORY -- F21's baseline skip makes a committed numeric leak permanently invisible to staged mode

`scripts/redaction-check.sh` (the F21 skip)

Verified working as designed, in my own scratch repo (below): a 6+-digit number already present in `git show HEAD:$file` is skipped on a later staged edit. The consequence, which the fix report notes as a rule but does not spell out as a residual: **once an unallowlisted number reaches HEAD, no mode ever reports it again** -- staged mode skips it by baseline, and range mode only ever reads added lines. A leak that slips into one commit is invisible to every later run.

Two things keep this from being blocking, both of which I confirmed by test rather than by reading: the **term scan is not baselined** (probe c3 below -- the committed term still fires on the next staged edit, so the high-value control is intact), and the digit sweep's job is new content by design. Worth one sentence in `docs/progress.md` so the next phase does not assume the sweep is a corpus audit. If a corpus audit is wanted, it is a separate full-tree mode, not a change to this one.

### Q5 -- ADVISORY -- `Expense.AccountName` and `.BankName` have no fixture that would catch a wrong tag

`freshbooks/expenses.go` (the `account_name` / `bank_name` fields)

Both are `""` in the seed capture and in both re-seeded unit fixtures, and `freshbooks/expenses_test.go` asserts neither. A typo in either json tag would pass the gate, the live suite and my probe. The residual risk is small -- I verified mechanically that **every** key in the capture carries a matching tag, so the tags are correct today (see Probe 3) -- but nothing defends them going forward. Same shape as, and cheaper to fix than, the seed capture's all-zero time-entry totals, which the unit fixture `time_entries/list_with_totals.json` does defend properly with distinct non-zero values (54000 / 12600). One fixture value would close it.

### Q6 -- ADVISORY -- term findings name a file but no line

`scripts/redaction-check.sh`

`redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #0)` -- the digit sweep reports `file:line`, the term scan reports only `file`. On a large fixture that leaves the operator grepping for a term whose value the script deliberately never prints. The extractor already carries the line number (`numbered_lines` emits `lineno<TAB>content`), so this is a formatting change, not a redesign. Pre-existing behaviour, not introduced here.

### Q7 -- ADVISORY (observation, not a defect) -- table mode for `list-with-totals` is unproven against real rows

The authorized account has **zero** time entries (`items: 0`, `total: 0`, all totals 0). `-o table` therefore emits 0 bytes -- byte-for-byte identical to `time-entries list -o table` on the same empty resource, so it is the pre-existing empty-list rendering, not a Phase 8 regression. I could not exercise the populated table path: doing so would require creating a time entry on the operator's real books, which this lane is forbidden to do. F10's claim ("table mode shows the entries") is verified only in unit tests and by the shared output layer, not live.

---

## Probes

### Probe 1 -- gate, current tree: **PASS**

`mise run check > /tmp/qa-gate.log 2>&1` -> exit **0**, no dirty-tree banner.

```
coverage-gate: <repo root>/freshbooks/coverage.out total = 91.9% (floor 90%)   PASS
coverage-gate: <repo root>/mcp/coverage.out        total = 92.1% (floor 90%)   PASS
coverage-gate: <repo root>/cli/coverage.out        total = 91.6% (floor 90%)   PASS
```

```
== redaction-selftest ==
... 19 probes, all PASS (incl. the two private-resolver probes -- the real resolver is present on this machine, so they were not skipped)
redaction-selftest: OK
```

- `git status --porcelain` -> empty before the run, and empty after (this report is the only file this lane adds; `dist/` from the build step is gitignored).
- `mise run inventory-check` -> `implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0`.
- `mise run vuln` -> `No vulnerabilities found.` in all three modules.
- `mise run docs` twice -> `docs: regenerated docs/cli.md` both times, `git status --porcelain` and `git diff --stat` empty after each. Idempotent.
- The gate's test step reported `(cached)`; I re-ran `mise exec -- go test -race -count=1 ./...` in `freshbooks/` uncached -- all `ok`.

### Probe 2 -- live suite against the real account: **PASS**

```
FRESHBOOKS_LIVE=1 FRESHBOOKS_ACCESS_TOKEN="$(fnox exec -- /tmp/relbin/freshbooks auth token)" \
  fnox exec -- mise exec -- go test -tags live -count=1 -v ./freshbooks/ > /tmp/qa-live.log 2>&1
LIVE_EXIT=0
```

**857 PASS, 0 SKIP, 0 FAIL.** Auth was already valid (`auth status -o json` -> `"valid": true`), so no refresh was triggered.

The ten live conformance tests, all PASS: `TestLiveExpenseVendors`, **`TestLiveExpenseFields`**, `TestLiveGateways`, `TestLiveLedgerAccounts`, `TestLiveStaffFields`, `TestLiveBusinessFilterEncoding`, `TestLiveBusinessSortEcho`, `TestLiveCallbacksEnvelope`, `TestLiveDateTimeFormats`, `TestLiveIdentity`.

Both new-surface requirements are present:

```
--- PASS: TestLiveExpenseFields (0.39s)
--- PASS: TestLiveLedgerAccounts/category_id/jea_id/jesa_id_decode_without_dropping_a_populated_value (0.75s)
```

**Leak scan of `/tmp/qa-live.log`** -- every mandated pattern, plus the account's own ids:

| Pattern | Occurrences |
|---|---|
| `Version=` (raw value) | 0 |
| `Version set=` (the F16 form) | 1 -- `expense 0: Billable=false ExtInvoiceID=0 ExtSystemID=0 Version set=true` |
| `eyJ` (JWT) | 0 |
| `Bearer` (any case) | 0 |
| any 6+-digit number | **0 in the entire log** |
| the account id from `identity me` | 0 |
| the business id from `identity me` | 0 |

F16 is confirmed on real data: `Version set=true` proves the field decoded from the live account while the value never reached the log.

### Probe 3 -- captured-response decode, independent module: **PASS (29/29)**

A throwaway module under `/tmp/qa-probe` with `replace ... => <repo root>/freshbooks`, serving each capture verbatim over `httptest` through the real `Client`. Nothing in the repo's own test suite was reused.

`freshbooks/testdata/seed/time_entries/list.json` via `TimeEntries.ListWithTotals`:

```
PROBE PASS  TE totals.TotalLogged == capture total_logged
PROBE PASS  TE totals.TotalUnbilled == capture total_unbilled
PROBE PASS  TE totals.PerTeamMember raw list non-nil
PROBE PASS  TE totals.PerClient raw list non-nil
PROBE PASS  TE PerTeamMember bytes == capture bytes
PROBE PASS  TE PerClient bytes == capture bytes
PROBE PASS  TE List().Page deep-equals ListWithTotals().Page      # F13's projection is byte-identical
```

**F9 on the marshalled type:**

```
PROBE PASS  F9 marshalled TimeEntriesPage has key "totals"
PROBE PASS  F9 marshalled TimeEntriesPage has no key "Totals"
PROBE PASS  F9 embedded Page promotes "items" flat
```

`seed/expenses/list.json` via `Expenses.List` -- all 14 new fields against the capture's own values:

```
PROBE PASS  EX AccountingSystemID / AccountName / BankName / Billable
PROBE PASS  EX Version non-empty and == capture version
PROBE PASS  EX PotentialBillPayment / ExtInvoiceID / ExtSystemID
PROBE PASS  EX BillMatches non-nil (capture has [])
PROBE PASS  EX LegacyAccountID   nil iff capture accountid null
PROBE PASS  EX BackgroundJobID   nil iff capture background_jobid null
PROBE PASS  EX ConverseProjectID nil iff capture converse_projectid null
PROBE PASS  EX ModernProjectID   nil iff capture modern_projectid null
PROBE PASS  EX ExtAccountID      nil iff capture ext_accountid null
```

`seed/ledger_accounts/list.json` via `LedgerAccounts.List`:

```
PROBE PASS  LA decoded same count as capture
PROBE PASS  LA JEAID non-nil exactly where the capture has a number
PROBE INFO  ledger: jea non-nil 2/2, jesa 4/4, category 0/0 (of 5 accounts)
PROBE PASS  LA JEAID discriminates (capture has both null and non-null)
```

The `JEAID` assertion is genuinely discriminating -- the capture has 2 populated of 5, so a swapped or dropped tag fails it.

**Non-vacuity re-run.** The seed capture's totals are all zero and its `per_*` lists empty, so probe 1 could not distinguish "decoded" from "zero value". I re-ran the identical code path against the unit fixture `freshbooks/testdata/time_entries/list_with_totals.json`, which carries distinct non-zero values:

```
NONVAC PASS  TotalLogged == 54000 (not the zero value)
NONVAC PASS  TotalUnbilled == 12600 (distinct from TotalLogged)
NONVAC PASS  PerTeamMember carries the member row
NONVAC PASS  PerClient carries the client row
NONVAC PASS  PerTeamMember != PerClient (tags not swapped)
NONVAC PASS  PageMeta.Total survived alongside the totals    # F13's PageMeta is intact
NONVAC PASS  PageMeta.PerPage survived
NONVAC PASS  items decoded
```

**D2 completeness, checked mechanically.** Every key in `seed/expenses/list.json` `expenses[0]` (41 keys) and `seed/ledger_accounts/list.json` `data[0]` carries a matching `json:` tag on `Expense` / `LedgerAccount`. Zero unmodelled keys remain in either.

### Probe 4 -- redaction script, exercised for real: **PASS**

**(a)** `scripts/redaction-selftest.sh` standalone -> exit **0**, 19 probes, all `PASS`, `redaction-selftest: OK`. The two private-resolver probes ran (not skipped).

**(b)** `scripts/redaction-check.sh --range main..phase-8/converge` -> `redaction-check: clean`, exit **0**.

**(c) My own scratch repo, independent of the self-test.** `mktemp -d`, fresh `git init`, a copy of the script, one real term drawn live from `~/projects/infinite-room-labs/agent-ops/scripts/resolve-redaction-terms.py` (length 34; **value withheld and never printed**), planted mid-identifier as `prefix-<TERM>-suffix` in `freshbooks/testdata/seed/x.json` alongside a 7-digit `9182736`:

```
=== (c1) STAGED mode ===
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #0)
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #1)
redaction-check: unallowlisted 6+-digit number 9182736 in freshbooks/testdata/seed/x.json:3
exit=1

=== (c2) --range mode, identical content ===
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #0)
redaction-check: possible leak in freshbooks/testdata/seed/x.json (term #1)
redaction-check: unallowlisted 6+-digit number 9182736 in freshbooks/testdata/seed/x.json:3
exit=1
```

Byte-identical findings, same line. **A1 is genuinely fixed** -- range mode's term scan fires, which was the whole point of the blocking security finding.

**F21, tested against a real HEAD baseline:**

```
=== (c3) unrelated edit; 9182736 already at HEAD ===
redaction-check: possible leak ... (term #0)
redaction-check: possible leak ... (term #1)
exit=1            # the number is correctly skipped; the TERM scan is NOT baselined and still fires

=== (c4) a NEW 7654321 in the same baselined file ===
redaction-check: unallowlisted 6+-digit number 7654321 in .../x.json:4
exit=1            # new number caught; 9182736 correctly absent from the output
```

F21 behaves exactly as `fix.md` documents, and -- the part worth recording -- the baseline skip applies **only** to the digit sweep, never to the term scan. That is what keeps Q4 advisory rather than blocking.

**(d)** `mise exec -- usage lint` -> `No issues found.` for `scripts/redaction-check.sh`, `scripts/redaction-selftest.sh` (and `scripts/check.sh`).

**(e)** `./scripts/redaction-check.sh --range no-such-ref..HEAD` -> `redaction-check: unusable range: no-such-ref..HEAD`, exit **2**. F3 confirmed.

Both scratch repos were deleted. The working tree was never involved.

### Probe 5 -- wire output, binaries built from the branch: **PASS**

Built to `/tmp/qa-bin/` from `33ea8e1`. All calls read-only, real token, real account.

`freshbooks time-entries list-with-totals --business <id> -o json | jq 'keys'`:

```
[ "items", "page", "pages", "per_page", "total", "totals" ]
```

`totals` is on the wire (F9), and the embedded page keys are still promoted flat. `.totals | keys` -> `["total_logged", "total_logged_per_client", "total_logged_per_team_member", "total_unbilled"]`. `-o yaml` shows the same six top-level keys.

`-o table`: renders, exit 0, **0 bytes** -- the account has no time entries, and plain `time-entries list -o table` emits 0 bytes on the same resource. See Q7.

`freshbooks expenses list --account <id> --per-page 1 -o json | jq '.items[0] | keys'`:

```
accounting_systemid amount categoryid date expenseid from_bulk_import id
isduplicate markup_percent notes staffid updated vendor version vis_state
```

Two of the 14 new keys reach live output -- `accounting_systemid` and `version` -- and `git show main:freshbooks/expenses.go` confirms **neither tag existed on `main`**, so this is the branch's new surface arriving on the wire. The other twelve are absent because the live expense has zero/null values for them and every new field carries `omitzero`/`omitempty` -- exactly D5's additive contract (no wire-shape change for existing callers).

`freshbooks-mcp tools | jq length` -> **169**.

```json
{ "name": "time_entries_list_with_totals", "readOnlyHint": true }
```

`docs/mcp.md:3` -> "169 tools ... One of the 169, `time_entries_list_with_totals`, carries no inventory key of its own" (F11's count half, with the keyless caveat).

### Probe 6 -- fix verification, F1-F21 against `git diff 2bc0ff7..33ea8e1`: **20 of 21 landed**

| F | Verified at | Result |
|---|---|---|
| F1 | `redaction-check.sh:234` `content=$(numbered_lines "$file" \| cut -f2-)`; BRE pipeline gone (only a comment about it remains at `:232`) | landed |
| F2 | `scripts/redaction-selftest.sh`; wired at `scripts/check.sh:81` and `mise.toml:37` | landed |
| F3 | `redaction-check.sh:28` `git rev-list --count`; probe (e) exits 2 | landed |
| F4 | synthetic-UUID exemption; both A3 rows in the self-test, both PASS | landed |
| F5 | sweep path `freshbooks/testdata/` | landed |
| F6 | `:83` one anchored alternation; no `^700[0-9]{2}$` branch; `4242424` comment says "the repo's synthetic identity_id" | landed |
| F7 | `README.md:65` names `mise install` and the self-test | landed |
| F8 | spec `:357` "its self-test runs in the gate, and the check itself is run by the lead ..." | landed |
| F9 | `time_entries.go:81` `json:"totals"`; proven on the wire and in Probe 3 | landed |
| F10 | `docs/cli.md:7928,8044` and the branch binary's `--help` both carry the caveat | landed |
| F11 | count -> 169 in `docs/mcp.md:3` **landed**; line references **NOT landed** | **FAILED -- Q1** |
| F12 | `const wantRegistrySize = 169` in `mcp/internal/tools/parity_test.go:24` and `cli/internal/cmd/parity_test.go:26`, consumed by both roundtrip guards (`:548`, `:658`) | landed |
| F12b | 212-key totals still asserted: `mcp/.../parity_test.go:244`, `cli/.../parity_test.go:244` | landed |
| F13 | exactly one list-response struct -- `time_entries.go:90 timeEntriesListResponse`; no `...ListWithTotalsResponse` anywhere; `List` is a projection, proven byte-identical in Probe 3 | landed |
| F14 | `time_tracking` in all three citations: `time_entries.go:62`, `docs/progress.md:63`, spec `:216` | landed |
| F15 | `live_conformance_test.go` -- raw `[]map[string]json.RawMessage` read beside the typed call, count precondition, then `wireCount(key) == decodedCount(pick)` per key. Falsifiable; passed live | landed |
| F16 | `live_conformance_test.go:94` logs `Version set=%v`, `e.Version != ""`. Confirmed in the live log | landed |
| F17 | `expenses_get.json:52` and `expenses_list.json:29` both `2026-08-22 00:00:00.000000`; doc example `expenses.go:141` `2026-08-28 00:00:00.000000` | landed |
| F18 | `docs/progress.md:7` "both are allowlisted", naming where each lives | landed |
| F19 | `mcp/internal/tools/shapes.go:41-43` and `cli/internal/cmd/invocation.go:146-149` both name `TimeEntries.ListWithTotals` | landed |
| F20 | `TimeEntryTotals` has four tagged fields and no restating comments | landed |
| F21 | HEAD-baseline skip; independently reproduced in probe (c3)/(c4) | landed |

Nothing under the triage's "Not applied" list was touched.

---

## Commands run

```
mise run check > /tmp/qa-gate.log 2>&1                                  # exit 0
mise run inventory-check ; mise run vuln ; mise run docs (x2)
mise exec -- go test -race -count=1 ./...            # in freshbooks/, uncached
fnox exec -- /tmp/relbin/freshbooks auth status -o json                 # valid: true
FRESHBOOKS_LIVE=1 FRESHBOOKS_ACCESS_TOKEN="$(fnox exec -- /tmp/relbin/freshbooks auth token)" \
  fnox exec -- mise exec -- go test -tags live -count=1 -v ./freshbooks/ > /tmp/qa-live.log 2>&1   # exit 0
mise exec -- go -C /tmp/qa-probe  run .        # 29/29 decode assertions
mise exec -- go -C /tmp/qa-probe2 run .        # 8/8 non-vacuity assertions
./scripts/redaction-selftest.sh                                         # exit 0, 19 PASS
./scripts/redaction-check.sh --range main..phase-8/converge             # clean, exit 0
./scripts/redaction-check.sh --range no-such-ref..HEAD                  # exit 2
mise exec -- usage lint scripts/redaction-{check,selftest}.sh           # No issues found
mise exec -- go -C cli build -o /tmp/qa-bin/freshbooks ./cmd/freshbooks
mise exec -- go -C mcp build -o /tmp/qa-bin/freshbooks-mcp ./cmd/freshbooks-mcp
/tmp/qa-bin/freshbooks-mcp tools | jq length                            # 169
git worktree add --detach <tmp> main    # for the main-vs-branch line-ref comparison; removed
```

## Cleanup

`/tmp/qa-probe`, `/tmp/qa-probe2`, `/tmp/qa-bin`, `/tmp/qa-gate.log`, `/tmp/qa-live.log`, `/tmp/qa-ident.json`, `/tmp/qa-exp.json`, `/tmp/qa-refs*.txt`, `/tmp/qa-lineref*.sh`, the redaction scratch repo and the `main` worktree are all removed. `git worktree list` shows only the primary. `git status --porcelain` shows only this report.

## What PASS needs

Fix Q1: six line references in `docs/phases/3/tools.md` rows 164-169 -> `:157`, `:179`, `:204`, `:231`, `:247`, `:169`. Q2 (the six `Expenses.*` rows) belongs in the same commit. Neither touches code, so a re-gate plus a re-read of the table is sufficient to clear this lane; no probe above needs re-running.
