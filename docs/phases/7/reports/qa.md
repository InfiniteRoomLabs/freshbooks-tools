# Phase 7 QA report -- `phase-7/live` at `f89a5c9` (2026-09-03)

Verdict: **PASS** (four advisories, no blockers). Read-only throughout: the live suite and the CLI probes made GET calls only; nothing was created, updated, or deleted; no token value was printed, logged, or stored. Every number below is a count or a shape; real ids, names, emails, and amounts are not reproduced.

## 1. Gate

`mise run check > /tmp/qa-gate.log 2>&1; echo $?` -> **0** on the committed tree (`git status --porcelain` empty before this report existed; after, it lists only `docs/phases/7/reports/qa.md`).

```
coverage-gate: <repo root>/freshbooks/coverage.out total = 91.8% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo root>/cli/coverage.out total = 91.6% (floor 90%)
coverage-gate: PASS
== inventory-check: freshbooks ==
implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0
== actionlint ==
== build ==            12 artifacts (mcp + cli x linux/darwin/windows x amd64/arm64)
check.sh: all OK
```

- `mise run inventory-check` -> `implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0`, `check.sh: inventory-check OK`.
- `mise run vuln` -> `No vulnerabilities found.` for all three modules.
- `mise run docs` twice -> exit 0 both times, `git status --porcelain` empty after each, `git diff --stat` empty: `docs/cli.md` is byte-identical to the generator's output (covers the F7 drift check).

## 2. Live suite (read-only, real books)

Credentials: `fnox exec -- /tmp/relbin/freshbooks auth status -o json` -> `logged_in: true, valid: true`, expiry ~12h out, 45 scopes; no refresh was needed. Bridge: `FRESHBOOKS_LIVE=1 FRESHBOOKS_ACCESS_TOKEN="$(fnox exec -- /tmp/relbin/freshbooks auth token)" fnox exec -- mise exec -- go test -tags live -count=1 -v ./freshbooks/ > /tmp/qa-live.log 2>&1` -> **exit 0**, `ok github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks 6.662s`.

| Test | Result | Log line (shape only) |
|---|---|---|
| `TestLiveExpenseVendors` | PASS | decoded 1 vendor name from the paginated envelope |
| `TestLiveGateways` | PASS | -- |
| `TestLiveLedgerAccounts` (3 subtests) | PASS | decoded 95 ledger accounts; types/sub-types asserted |
| `TestLiveStaffFields` | PASS | decoded 1 business-group member |
| `TestLiveBusinessFilterEncoding` (3 subtests) | PASS | -- |
| `TestLiveBusinessSortEcho` | PASS | server echoed `sort=[updated_at_desc]` for the library's `Sort()` |
| `TestLiveCallbacksEnvelope` | PASS | -- |
| `TestLiveDateTimeFormats` (3 subtests) | PASS | "the account holds no projects, so the zoneless layout stays INFERRED" (a `t.Log`, not a skip) |
| `TestLiveIdentity` (Phase 1) | PASS | 1 membership |

**Zero `SKIP` lines in the whole log** (F4 verified live). Leak grep on `/tmp/qa-live.log`: `eyJ` 0, 64-hex 0, `Bearer` 0, and every value from `identity me -o json` (the three id-shaped fields: 6-char account id, 8-digit business id, 36-char business uuid) 0 hits; no 6+-digit number appears in the log at all. The log was deleted afterwards.

## 3. Captured-response decode probe

Throwaway module at `/tmp/qa-probe` (`replace` -> `<repo root>/freshbooks`, run through the repo-pinned toolchain with `mise exec -- go test -C /tmp/qa-probe -count=1 -v ./...`; deleted afterwards). Each capture is served verbatim from an `httptest` server and decoded through the same lib method the matching `TestLive*` calls. **8/8 PASS, zero decode errors across all 15 branch captures**, and the specific corrected fields are populated from the capture, not zero:

| Capture(s) | Method | Corrected-field assertions that held |
|---|---|---|
| `seed/expenses/vendors.json` | `Expenses.Vendors` | 1 non-empty name; `json.Marshal` yields a JSON array |
| `seed/expenses/list.json` | `Expenses.List` | 1 item, `total=1 per_page=3`; `date` and `updated` parse; `updated` layout == `DateTimeLayout`; amount string decoded |
| `seed/gateways/get.json` (== `testdata/gateways/get_stripe_unified.json`, byte-identical) | `Gateways.Get` | `StripeUnified != nil`, `Stripe == nil`, `FBPay == nil`; account id, publishable key, `account_status`, 8 capabilities, payout schedule (`daily`/7) populated; `stripe_account_updated_at` parses with `RFC3339Layout` |
| `seed/ledger_accounts/list.json` | `LedgerAccounts.List` | 5 accounts; `uuid`/`type`/`sub_type` set; fractional `updated_at` parses |
| `seed/ledger_accounts/types.json` | `LedgerAccounts.Types` | 5 `{name}` objects, every `Name` non-empty, includes `income` |
| `seed/ledger_accounts/sub_types.json` | `LedgerAccounts.SubTypes` | 10 entries, numeric `ID != 0`, `BaseNumber` non-empty on all |
| `seed/ledger_accounts/sub_type.json` | `LedgerAccounts.SubType("1")` | `{ID:1 Type:asset Name:"Cash & Bank" BaseNumber:"1000"}` |
| `seed/staff/list.json` | `Staff.List` | 1 member; `IdentityUUID` 36 chars, `Language == "en"`; ids and role set; **no wire key dropped** |
| `seed/callbacks/list.json` | `Callbacks.List` | envelope peeled: `page=1 pages=0 per_page=2 total=0`, 0 items |
| `seed/projects/{list,list_sort_minus,list_sort_suffix}.json` | `Projects.List` | `Page.Sort` = `[]`, `["-updated_at"]`, `["updated_at_desc"]` respectively |
| `seed/time_entries/{list,list_bracket_filter_ignored}.json` | `TimeEntries.List` | `page=1 total=0`, 0 items |
| `seed/time_entries/error_422_bare_filter.json` (served as 422) | `TimeEntries.List` | surfaces as `freshbooks: 422 business: {"updated_since": ...} (errno 2001)` -- names the field |

Silent-drop sweep (wire keys vs struct tags; see Q3): `StripeUnifiedConnection` and `BusinessGroupMember` model every key on the wire; `LedgerAccount` drops 3, `Expense` drops 14. Both are Phase 2 models and not claims of this phase.

## 4. Callout backing

Every `STATE AS OF 2026-09-0x (Phase 7, live)` line and every `CONFIRMED live` marker is backed:

| Where | Fact | Backing |
|---|---|---|
| spec:61 (2026-09-02, scopes) | portal vs docs scope lists | `reports/lead-stage1.md` (findings 1 + addendum), `reports/lead-sandbox.md`; no capture possible (consent page). Consistent with the live token: 45 scopes on the token = the code's 43 default + `account:read/write`, exactly as the callout says |
| spec:99 (2026-09-03, J1) | vendors CORRECTED; delete/custom-category/secondary-contact unconfirmed | `seed/expenses/vendors.json` + `TestLiveExpenseVendors`; the deferrals point at `lead-sandbox.md` |
| spec:174 (2026-09-03, B + C) | bare filter CONFIRMED; sort direction **unresolved** | `seed/time_entries/{error_422_bare_filter,list_bracket_filter_ignored,list}.json` + `TestLiveBusinessFilterEncoding`; `seed/projects/list_sort_{minus,suffix}.json` + `TestLiveBusinessSortEcho`. Reads as NOT resolved ("the negative result is itself the finding") -- correct |
| spec 5.1:178-204 (O, time-entry meta, Q) | `meta.sort` CORRECTED; meta totals recorded; timestamp producers | O: `seed/projects/*` (`Page.Sort` probed above). Meta totals: `seed/time_entries/list.json` carries all four extra keys. Q: `seed/expenses/list.json` (date-only + space layouts), `seed/accounting_clients_list.json` (`updated`), `seed/users_me.json` + `seed/gateways/get.json` (RFC 3339), `seed/ledger_accounts/list.json` (fractional). The invoices rows are labelled "uncaptured CLI probe" honestly. Zoneless layout reads as **INFERRED, no live producer** -- correct (the live run confirmed 0 projects). See Q2 for one over-cautious line |
| spec:189 (2026-09-03, P) | staff `identity_uuid`/`language` | `seed/staff/list.json` + `TestLiveStaffFields` |
| spec:191 (2026-09-03, F) | ledger flat + taxonomy shapes | `seed/ledger_accounts/{list,types,sub_types,sub_type}.json` + `TestLiveLedgerAccounts` |
| spec:197 (2026-09-03, E + D) | payments flat + `stripe_unified`; events enveloped; uploads unconfirmed | `seed/gateways/get.json` + `TestLiveGateways`; `seed/callbacks/list.json` + `TestLiveCallbacksEnvelope` |
| `freshbooks/CHANGELOG.md:15-18` | sort echo, staff (09-03); gateways, vendors (09-02) | same captures as above; dates match the implementer's account of the two runs (F6) |
| `page.go:29`, `staff.go:19,25`, `ledger_accounts.go:188,197,250`, `gateways.go:88`, `expenses.go:332` | code-comment markers | `seed/projects/list_sort_*`, `seed/staff/list.json`, `seed/ledger_accounts/*`, `seed/gateways/get.json`, `seed/expenses/vendors.json` |

Nothing is backed by neither a capture nor a lead report.

## 5. Wire output, branch binaries

Built with `mise exec -- go -C mcp build -o /tmp/qa-bin/freshbooks-mcp ./cmd/freshbooks-mcp` and `mise exec -- go -C cli build -o /tmp/qa-bin/freshbooks ./cmd/freshbooks` (both deleted afterwards). Scope flags were filled from `identity me` inside the shell, never printed. Output reduced to shapes with `jq`:

| Command | Observed |
|---|---|
| `ledger-accounts types --business-uuid ... -o json` | array, len 5, each `{"name": string}`: `asset equity expense income liability` |
| `ledger-accounts sub-types --business-uuid ... -o json` | array, len 10, keys `base_number id name type`, `id` a JSON number, `base_number` a string |
| `ledger-accounts sub-type 1 --business-uuid ... -o json` | object, keys `base_number id name type` |
| `expenses vendors --account ... -o json` | **JSON array**, len 1, element type string |
| `gateways get --account ... -o json` | array, len 1; connection keys `fbpay paypal stripe stripe_unified`; `stripe_unified` **non-null** with 21 keys, `account_status` string, 8 capabilities, `stripe_account_updated_at` string; `stripe` null |
| `auth status -o json` | `logged_in: true, valid: true`, 45 scopes, keys `context credentials_path expiry logged_in scopes valid` |
| `auth token > /dev/null; echo $?` | `0`, empty stderr |

## 6. Redaction re-sweep

- `grep -rnoE '[0-9]{6,}' freshbooks/testdata/seed docs/phases/7 | grep -vE '700[0-9]{2}|8675309|4242424|555555|0000'` -> three hits, all Phase 1 seeds already on `main`, all obvious placeholders: `seed/accounting_error_404.json` `999999999`, `seed/users_me.json` `5550100100` and `1111111`. Nothing on this branch's captures (see Q4).
- `git grep -nE '4550415|4550417|12108003|12108005|12108007|12108015' HEAD` -> **empty**.
- `scripts/redaction-check.sh` run for real: scratch worktree at `main` (`git worktree add --detach /tmp/qa-wt main`), `git read-tree phase-7/live` there so the 56 branch files were the staged content, script -> `redaction-check: clean`, exit 0; worktree removed and pruned, the real index untouched (`git status --porcelain` unchanged).

## 7. Fix verification (`git diff b83d31a..f89a5c9`)

| F | Landed | Evidence |
|---|---|---|
| F1 | yes | `seed/ledger_accounts/list.json` six ids -> `70020..70025`, nulls kept; `docs/phases/7/plan.md:48` capture rule; `reports/code-review.md:43` literals redacted |
| F2 | yes | `freshbooks/CHANGELOG.md:9-11` `### Changed` above `### Fixed`, entry opens `**Breaking:**` |
| F3 | yes | `expenses.go:343` `vendors := []string{}`; `expenses_test.go:318-327` asserts non-nil and `json.Marshal == "[]"`; green in the gate |
| F4 | yes | four `t.Skip` -> `t.Log(...); return` (vendors, gateways, expenses, clients); projects branch asserts `created_at`/`updated_at` layouts when `Total != 0`; live run shows zero SKIPs |
| F5 | yes | spec 5.1 fact-Q rows reworded "uncaptured CLI probe (2026-09-02)" |
| F6 | yes | J1/E dated 2026-09-02 in `freshbooks/CHANGELOG.md:17-18` and spec:99/:198-199 |
| F7 | yes | `auth_cmd.go:147`, `docsgen.go:80`, `paths_test.go:109`, `docs/cli.md:61,305`; `mise run docs` idempotent |
| F8 | yes | root `CHANGELOG.md:11` Phase 7 line under `### Added` |
| F9 | yes | four gateway stamps -> `T00:00:00Z` (both JSON files), five ledger `updated_at` -> `.000000Z`, expenses `version` -> `.000000`, spec example synthetic |
| F10 | yes | `pk_test_` in both gateway JSON files |
| F11 | yes | `status.go:134` error text; `status_test.go:397-400` asserts both halves |
| F12 | yes | `status.go:39-45` `clock(now)`; both call sites use it |
| F13 | yes | `liveSetup(t)` in all eight tests; `liveScope`/`liveCtx` gone |
| F14 | yes | `expenseVendorsEnvelope` inlined at `expenses.go:346-351`; `gateways_test.go:50` passes `serveFixture` directly |

## Findings

**Q1 -- ADVISORY -- the six live row ids are still in three branch commits, and the `--no-ff` merge will push them to the public `origin`.** `git log -p main..HEAD` carries `4550415|4550417|12108003|...` in `a96e9f7` (introduced), `b83d31a` (quoted by `reports/code-review.md`), and `fbbade5` (the removal diff): 14 lines total. The triage's "not needed, the branch is local-only" holds only until the push. `git remote -v`: `origin` is the public GitHub remote; the `gitea` push URL is disabled. Expected: the lead re-affirms the no-scrub decision knowingly (they are non-secret ledger row ids), or squashes/filter-repos before push. Observed: no decision recorded against the push step.

**Q2 -- ADVISORY -- spec 5.1 fact-Q understates the evidence for `clients[].signup_date`.** `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md:200` says `clients[].signup_date` was "observed in an uncaptured CLI probe (2026-09-02) and carr[ies] no capture or assertion of their own". `freshbooks/testdata/seed/accounting_clients_list.json` (Phase 1) carries `"signup_date": "2026-08-22 04:32:55"` in the space-separated layout. Expected: name the capture; observed: the row claims none exists. One clause, docs only.

**Q3 -- ADVISORY (backlog, not this gate) -- live captures show unmodelled keys on two Phase 2 read models.** Wire keys present in the capture and absent from the struct tags: `Expense` (`freshbooks/expenses.go`) drops 14 -- `accountid`, `accounting_systemid`, `account_name`, `background_jobid`, `bank_name`, `billable`, `bill_matches`, `converse_projectid`, `ext_accountid`, `ext_invoiceid`, `ext_systemid`, `modern_projectid`, `potential_bill_payment`, `version`; `LedgerAccount` (`freshbooks/ledger_accounts.go`) drops `category_id`, `jea_id`, `jesa_id`. `billable` is the one a caller would miss. `StripeUnifiedConnection` and `BusinessGroupMember` drop nothing. Expected: a `docs/progress.md` backlog line (the implementer recorded the time-entry `meta` totals drop as item 12 but not these); observed: not recorded.

**Q4 -- ADVISORY -- the 6+-digit sweep is not yet silent on `main`'s own seeds.** `seed/accounting_error_404.json:9` `999999999`, `seed/users_me.json:33` `5550100100`, `:39` `1111111` are Phase 1 placeholders, plainly synthetic, pre-existing. Expected: the sweep's exclusion pattern (in the gate prompt) grows to cover them so the next phase's sweep returns nothing without a human filter; observed: three hits to eyeball every time.

No blocking finding. The gate is green, the live suite is green with no skips, every branch capture decodes with its corrected fields populated, every callout is backed, the six ids are gone from HEAD, and F1-F14 all landed as ordered.

## Commands run (in order)

```
mise run check > /tmp/qa-gate.log 2>&1; echo $?                       # 0
git status --porcelain                                                 # empty
mise run inventory-check; mise run vuln; mise run docs (x2) + git diff  # 213/0; clean; no diff
fnox exec -- /tmp/relbin/freshbooks auth status -o json                # valid: true
FRESHBOOKS_LIVE=1 FRESHBOOKS_ACCESS_TOKEN="$(fnox exec -- /tmp/relbin/freshbooks auth token)" fnox exec -- mise exec -- go test -tags live -count=1 -v ./freshbooks/ > /tmp/qa-live.log 2>&1; echo $?   # 0
grep -c 'eyJ' / -cE '[0-9a-f]{64}' / -ci bearer / identity values      # 0 0 0 0
mise exec -- go -C mcp build -o /tmp/qa-bin/freshbooks-mcp ./cmd/freshbooks-mcp; same for cli
/tmp/qa-bin/freshbooks {ledger-accounts types|sub-types|sub-type 1, expenses vendors, gateways get, auth status, auth token} ... -o json | jq (shapes)
mise exec -- go test -C /tmp/qa-probe -count=1 -v ./...               # 8/8 PASS
grep -rnoE '[0-9]{6,}' ...; git grep -nE '<six ids>' HEAD              # 3 pre-existing placeholders; empty
git worktree add --detach /tmp/qa-wt main; git read-tree phase-7/live; scripts/redaction-check.sh   # clean; worktree removed
git diff b83d31a..f89a5c9                                              # F1-F14 spot-checked
```

Cleanup: `/tmp/qa-probe`, `/tmp/qa-bin`, `/tmp/qa-*.log`, `/tmp/qa-authtoken.err`, and the `/tmp/qa-wt` worktree removed; `git worktree list` shows only the main checkout; `/tmp/relbin` left as found. Tree: only this file is untracked.
