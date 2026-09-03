# Phase 8 gate -- code-review lane

Branch `phase-8/converge` @ `abbe407`, 5 commits over `main` (`ccccafc..abbe407`). Read-only pass: `git`, `grep`, `jq`, reads. No gate run, no network, no edits outside this file.

## Verdict: REQUEST CHANGES

Two blocking findings (R1, R2), ten advisory. The engineering is sound -- D1's one-request decode is correct against the capture, D2's 17 field types all match the capture's JSON types and nullability, D5's "additive only" holds (no exported signature changed, no write body changed, no new dependency), and the MCP/CLI ripple is exactly one tool and one command with no other manifest movement. Both blockers are cheap: a stale user-facing count and a missing struct tag that becomes a breaking change the moment `freshbooks/v0.3.0` ships.

## Blocking

### R1 [BLOCKING] `docs/mcp.md:3` still advertises 168 tools

`docs/mcp.md:3` -- "a stateless Model Context Protocol server: 168 tools, one per `freshbooks` client-library method". The branch bumped `168 -> 169` in `mcp/internal/tools/{doc,registry}.go`, all four MCP test files, `docs/phases/3/tools.md`, and the CLI's equivalents, but missed the MCP's primary user-facing document. Nothing guards it: `docs/cli.md` has a docs-drift test (`mise run docs` regenerates it, and it was regenerated here); `docs/mcp.md` is hand-written and no Go test parses it (`grep -rn "mcp.md" --include=*.go` finds only doc-comment cross-references).

Evidence: `grep -rn '\b168\b' --include=*.md .` -- the only live (non-historical) 168 outside the release-frozen `mcp/CHANGELOG.md:23` and `cli/CHANGELOG.md:30` entries is this line. Root `CHANGELOG.md:20`, `GOAL.md:64,94,109,110`, the spec's Phase 3 callout and the phase 2/3/5 reports are historical records and are correctly untouched.

Fix: bump to 169 and carry the same caveat `docs/phases/3/tools.md`'s totals line now carries -- the 169th tool is keyless, wrapping the same wire endpoint as `time_entries_list`.

### R2 [BLOCKING] `TimeEntriesPage.Totals` ships with no JSON tag

`freshbooks/time_entries.go:89-92`:

```go
type TimeEntriesPage struct {
	Page[TimeEntry]
	Totals TimeEntryTotals
}
```

The embedded `Page[TimeEntry]` promotes its own tagged fields, so the value marshals as `{"items":[...],"page":1,"pages":1,"per_page":15,"total":1,"Totals":{...}}`. `Totals` is the one field in the repo's serialized surface with a Go-cased JSON key. Every other field on `Page`, `PageMeta`, `TimeEntry`, `TimeEntryTotals` itself, and all 45 tags on `Expense` is snake_case.

Failure scenario: this value is the return of both new wrappers. `mcp/internal/tools/tools_time_entries.go:82` returns it as `any`, and per the spec's Phase 3 callout the lib value goes out as `StructuredContent` plus JSON text, so an MCP client reading the totals must key on `"Totals"`. `cli/internal/cmd/commands_time_entries.go:81` returns it to `-o json`/`-o yaml`, same key. Once `freshbooks/v0.3.0` and the `mcp`/`cli` bumps are tagged -- which `docs/progress.md`'s "Next action" schedules immediately after this merge -- renaming it is a breaking change to a published wire shape.

I checked for precedent: an AWK sweep of every exported field without a `json:` tag in `freshbooks/*.go` returns only `*ListOptions` structs and `Client`'s service pointers, none of which is ever serialized. `TimeEntriesPage` is the only serialized return type with an untagged exported field.

Fix: `Totals TimeEntryTotals \`json:"totals"\``. Leave the embedded `Page[TimeEntry]` untagged -- flat promotion is what keeps `cli/internal/output/output.go:189`'s `items` probe working.

## Advisory

### R3 [ADVISORY] `scripts/redaction-check.sh:52` -- the `^700[0-9]{2}$` allowlist branch is dead

`seed_number_allowed` is called from exactly one place (`scripts/redaction-check.sh:104`), always with a `([0-9]{6,})` capture, so `$n` is always >= 6 digits. `^700[0-9]{2}$` requires exactly 5. The branch can never return 0.

This is not a hole -- a 5-digit `700NN` is invisible to a 6+-digit sweep either way -- but the allowlist reads as covering the repo's synthetic-id convention when it does not, and it is the one allowlist entry the work order named by regex. I swept the corpus by hand (`for f in $(git ls-files 'freshbooks/testdata/seed/*'); do sed -E '<uuid strip>' "$f" | grep -oE '[0-9]{6,}'; done`): the only 6+-digit runs present are `4242424` (x4), `8675309` (x3), `999999999`, `5555550100`, `5550100100`, `1111111` and all-zero runs -- every one an explicit literal or the `^0+$` branch. The sweep is doing real work; this one branch is not.

Fix: delete it, or keep it with a comment saying it is a deliberate no-op guarding a future threshold change.

### R4 [ADVISORY] `scripts/redaction-check.sh:103` -- the UUID strip is unanchored and unconditional

```sh
content=$(printf '%s' "$content" | sed -E 's/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}//g')
```

Two consequences worth recording:

1. A real FreshBooks `business_uuid` or `identity_uuid` pasted into a future seed capture is exempt from the number sweep by construction. The comment's fallback -- "UUID redaction is a separate concern from this sweep (the term-list scan / manual review)" -- only holds if that exact uuid is a configured redaction term, which for a newly captured account it will not be. `CLAUDE.md` names "real FreshBooks account/business IDs" as a hygiene target, and the ledger-account family is uuid-scoped, so this is the family most likely to leak one.
2. The substitution has no token boundary. A leaked 8-digit id immediately followed by a uuid-shaped suffix (`12345678-1234-1234-1234-123456789012`) is consumed whole and never reaches the digit sweep. Contrived, but the anchor costs nothing.

Fix: anchor both sides (`(^|[^0-9a-fA-F-])` / `([^0-9a-fA-F-]|$)`), and if uuid leakage is in scope, invert the check -- fail any uuid that does not match the repo's own `00000000-0000-4000-8000-0000000000..` convention rather than exempting all of them.

### R5 [ADVISORY] the sweep's path filter excludes the fixtures this phase edited

`scripts/redaction-check.sh:91` fires only on `freshbooks/testdata/seed/*`. D2 re-seeded `freshbooks/testdata/accounting/expenses_{list,get}.json` and `freshbooks/testdata/ledger_accounts/{list,get,create,update}.json` **from the captures** -- exactly the copy-from-live path the sweep exists to police -- and none of those files is swept. The values landed are fine (`ACM123`, `70020`-`70024`, zeros and nulls), but the guard did not check them; I did, by hand.

Per D3's letter this is correct behaviour and not a defect in the implementation. Flagging so a later phase can widen the filter to `freshbooks/testdata/`.

### R6 [ADVISORY] `docs/progress.md:7` records a finding that is not true

The report and the living status doc both state that "the plan's `5555550100` phone-number placeholder does not match the actual pre-existing capture value `5550100100` (`freshbooks/testdata/seed/users_me.json:33`)". Both values are in the corpus: `freshbooks/testdata/seed/staff/list.json:1` carries `"phone_number":"5555550100"`, `users_me.json:33` carries `"phone_number": "5550100100"`. The plan named a real placeholder; the implementer found a second one.

The allowlist is right (both are listed). The narrative in a doc whose whole job is to be trusted at the next phase boundary is wrong. Fix: "the corpus carries both placeholders; both are allowlisted."

### R7 [ADVISORY] `docs/phases/3/tools.md:172-176` -- five stale line references

D1 inserted ~44 lines above `TimeEntriesService.List`, so rows 164-168 still cite `time_entries.go:108` (List), `:119` (Search), `:144` (Create), `:171` (Update), `:187` (Delete). The methods are now at 152, 179, 204, 231, 247. Row 169's `time_entries.go:164` is correct.

`parseToolsMD`'s regex (`mcp/internal/tools/parity_test.go:30`) matches `\([^)]*\)` and discards it, so no test catches this -- the file is "frozen" by test only for name, service, method, annotation and keys. Fix: bump the five rows in the same edit that fixes R1.

### R8 [ADVISORY] three spellings of the URL that is the sole evidence for two untyped public fields

`PerTeamMember` and `PerClient` stay `json.RawMessage` on the strength of one claim: the FreshBooks docs show no populated example. That claim cites `https://www.freshbooks.com/api/time_entries` in `freshbooks/time_entries.go:72`, in the spec 5.1 callout, and in `docs/progress.md:63`. That path appears nowhere else in the repo. Every other time-tracking citation -- 6 of them, including `time_entries_list_with_totals`'s own tool description added in the same commit (`mcp/internal/tools/tools_time_entries.go:78`) -- uses `.../api/time_tracking`. The implementer report says it checked `.../api/timetracking`, a third spelling.

I cannot resolve which URL was actually fetched (no network in this lane). The decision to leave the fields raw is defensible either way, but the audit trail for an INFERRED public API shape should name a URL that exists and matches the repo's own convention. Fix: use `time_tracking` in all three places, or state the URL actually fetched.

### R9 [ADVISORY] the default output format hides the feature

`time-entries list-with-totals` is registered `List: true` (`cli/internal/cmd/commands_time_entries.go:78`). `cli/internal/output/output.go:189` unwraps any object carrying `items` into rows, so `-o table` -- the default on a TTY -- renders exactly the same table as `time-entries list` and drops `Totals` entirely. The only way to see the totals is `-o json` or `-o yaml`.

The command's entire reason to exist is invisible in its default rendering. Fix: at minimum say so in the `Short` string and `docs/cli.md`; better, print the totals as a trailer line in table mode.

### R10 [ADVISORY] the new live ledger-account subtest cannot fail

`freshbooks/live_conformance_test.go:177-193` -- "category_id/jea_id/jesa_id decode without dropping a populated value" counts non-nil pointers and `t.Logf`s the counts. Past the `List` error check it has no assertion; it passes identically whether the server sends all three populated, all three null, or the keys absent. The subtest's own comment explains why a value assertion is impossible, which is fair, but a falsifiable check is still available.

`TestLiveExpenseFields` is better -- it asserts `AccountingSystemID != ""` per row -- though its other four fields are logged, not asserted, for the same defensible reason.

Fix: assert something that can fail, e.g. at least one of the three is non-nil (with `t.Skip` when the account genuinely has none), or decode the raw body alongside and assert the count of non-null `jea_id` keys equals the count of non-nil `JEAID` pointers -- which is the actual "did not silently drop" property the subtest name claims.

### R11 [ADVISORY] `3f2b1d3` is scoped `feat(freshbooks)` but changes three modules

`3f2b1d3 feat(freshbooks): surface time-entry totals via ListWithTotals` touches 7 files under `mcp/` and 6 under `cli/` alongside 4 under `freshbooks/`. `CLAUDE.md` scopes commits per module (`feat(freshbooks)`, `feat(mcp)`, `feat(cli)`).

Splitting it would be worse -- the parity tests are cross-module by design, so any split leaves a red commit in history. The fix is the scope, not the split: `feat: surface time-entry totals via ListWithTotals`. History-only; not worth a rewrite if the branch is already shared.

### R12 [ADVISORY] `ListWithTotals`'s signature diverges from every sibling list method

`ListWithTotals(ctx, businessID, opts ...RequestOption)` against `List(ctx, businessID, opts *TimeEntryListOptions, extra ...RequestOption)`. The doc comment owns the choice and D5 forbids touching `List`, so this is defensible. Noting it because it forced both wrappers onto `reqOpts()`/`ReqOpts()` -- helpers whose doc comments still say they exist "for the handful of methods that take raw variadic RequestOptions (JournalEntries.Details, JournalEntryAccounts.List)" (`mcp/internal/tools/shapes.go:40-42`, `cli/internal/cmd/invocation.go:145-148`). Those two comments are now incomplete. If a second resource ever gets a `ListWithTotals`, settle the convention then.

## What I verified clean

- **D1 decode.** `timeEntriesListWithTotalsResponse.Meta` embeds `PageMeta` and `TimeEntryTotals` at the same depth; no Go field name and no JSON tag collides (`page/pages/per_page/total/sort` vs `total_logged/total_unbilled/total_logged_per_team_member/total_logged_per_client`), so one decode produces the flat `meta` shape the capture shows. `newPage(resp.TimeEntries, resp.Meta.PageMeta)` is the same constructor `list` uses and carries `Sort` through. One request, `FamilyBusiness`, same path as `List`.
- **`List` untouched.** The only change to `List`'s hunk is the doc comment above `timeEntriesListResponse` ("Reach them with (*Client).Do" -> "ListWithTotals is the way to reach them"). Zero behavioural bytes.
- **`ListWithTotals` tests.** `TestTimeEntriesListWithTotals` covers `[happy]` path + totals + both raw lists, `[edge]` empty `[]` lists, `[happy]` option pass-through asserting `per_page=5` and `billed=0` in the query, `[sad]` 401 -> `ErrUnauthorized`. Path asserted exactly.
- **MCP/CLI wiring.** Same path (`roundtrip_test.go` `wantPath` entry matches `time-entries/list` exactly), business scope on both, `hintRO` (`ReadOnlyHint: true`) on the tool, `ClassRO` on the command. `inv.ReqOpts()` includes `SortOpt()`, so the registered `HasSort: true` / `--sort` flag is actually honoured -- not a dead flag.
- **Keyless allowed-set change is narrow.** Both `keylessTools` and `keylessCommands` are literal two-entry maps naming the exact tool/command; no wildcard, no prefix match. Both tests still assert `keylessGot == keylessWant == len(map)` and the 212-key total, so a third keyless entry appearing by accident still fails.
- **Manifest is otherwise unchanged.** `git diff main...HEAD -- mcp/internal/tools cli/internal/cmd | grep -E '^[+-]\s*(newSpec|Group:)'` returns exactly two added lines and zero removed. No other `tools_*.go` or `commands_*.go` file is touched.
- **`168 -> 169` completeness.** Complete except R1: `mcp/internal/tools/{doc.go,registry.go,parity_test.go,roundtrip_test.go,unit_test.go}`, `mcp/cmd/freshbooks-mcp/run_test.go`, `cli/internal/cmd/{registry.go,root.go,parity_test.go,roundtrip_test.go}`, `docs/phases/3/tools.md`, `docs/phases/4/commands.md`, `docs/cli.md` (regenerated, entry inserted in sorted order with the full inherited-flag block). Historical 168s in `CHANGELOG.md:20`, `mcp/CHANGELOG.md:23`, `cli/CHANGELOG.md:30`, `GOAL.md`, the spec's Phase 3 callout and the phase 2/3/5 reports are correctly frozen.
- **D2 types against the capture.** Dumped `freshbooks/testdata/seed/expenses/list.json` `expenses[0]` (43 keys) and `ledger_accounts/list.json` `data[0]` (15 keys) with `jq` and checked each against the struct tags. All 14 + 3 new fields match the capture's JSON type and nullability: `accounting_systemid`/`account_name`/`bank_name`/`version` present strings -> `string`; `billable`/`potential_bill_payment` present booleans -> `bool`; `ext_invoiceid`/`ext_systemid` present `0` -> `int64`; `bill_matches` present `[]` -> `[]json.RawMessage`; `accountid`/`ext_accountid` null -> `*string`; `background_jobid`/`converse_projectid`/`modern_projectid` null -> `*int64`; `jea_id` non-null across 2 of 5 rows and `jesa_id` across 3 -> `*int64` CONFIRMED; `category_id` null in all 5 rows -> `*int64` INFERRED. Zero capture keys remain unmodelled on either type.
- **No write-body change.** `Expense` is a read model only: `Create`/`Update` take `ExpenseWriteRequest` (`freshbooks/expenses.go:161-192`), `LedgerAccounts` take `LedgerAccountCreateRequest`/`UpdateRequest`. Neither type is an MCP or CLI input (MCP input schemas are input-only, `Out` is `any`). So no zero-valued new field can reach the wire, and the `Billable bool` + `omitempty` pattern -- which would be the `Timer.IsRunning` trap if `Expense` were a request body -- is safe here. `omitempty` rather than the plan's `omitzero` is correct for these types (no new `Money`/struct field) and matches the file's existing bool/string/int convention (`has_receipt`, `include_receipt`, `is_cogs`).
- **`LegacyAccountID`.** Naming the `accountid` key `LegacyAccountID` beside the existing `AccountingSystemID` reads right: the capture shows `accounting_systemid: "ACM123"` (the account slug) and `accountid: null`, two genuinely different identifiers, and `AccountID` is already a package-level type. The doc comment says so.
- **`CategoryID` INFERRED flagging.** Called out in the field comment, the `freshbooks/CHANGELOG.md` entry, the spec 5.1 callout, and re-cut as `docs/progress.md` backlog item 16. Honest.
- **Fixtures re-seeded with synthetic ids only.** `ACM123`, `70020`-`70024`, `2026-08-22 04:32:55.000000`, zeros and nulls. No real value.
- **`--range` mode.** Reads added lines only (`grep -E '^\+' | grep -v '^\+\+\+' | sed 's/^\+//'`), file list from `git diff <range> --name-only`, `found=1` -> `exit 1` on a hit in both modes. Index mode's term scan is byte-identical in behaviour to `main` (whole staged content via `git show ":$file"`, same `continue` on a deleted file, same short-term word-boundary threshold). `added_lines_with_numbers` tracks new-file line numbers correctly off the `-U0` hunk headers (`-` lines do not advance the counter; consecutive `+` lines do), pure bash, no awk dependency. The digit-strip loop cannot spin: `$n` is digits only, so `${content/$n/}` is a literal removal that always shrinks the string.
- **`usage` conversion.** `#USAGE` with no space (matching jdx usage 6.0 and every other script in `scripts/`), `flag "--range <range>" help="..."` matches `scripts/branch-protection.sh`'s pattern exactly, and the script reads the value as `${usage_range:-}` with an unset-safe guard in all four call sites. `usage = "6.0.0"` is pinned in `mise.toml`, five of the seven scripts already carry the shebang, and nothing in `.github/workflows` or `mise.toml` invokes `redaction-check.sh`, so requiring `usage` on `PATH` breaks no automated path. The one soft spot: the header comment's "optional for outside contributors ... exits 0 with a notice" promise now depends on `usage` being installed before the script can start at all. Consistent with the rest of the repo; noting, not filing.
- **Changelogs.** All four additive, all under `[Unreleased]`, `### Added` for the three module-level entries and `### Changed` for D3's script work in the root -- matching D5. The root's extra `### Added` rollup follows the established per-phase precedent in that file.
- **`docs/progress.md`.** Items 9, 12, 14 closed with their resolution inline; 15 and 16 re-cut from 12's and 14's residue with the evidence each needs; ledger row 9 added with the numbering quirk explained; "Next action" correctly rewritten to the review gate.
- **Hygiene.** `git diff main...HEAD | grep -P '[^\x00-\x7F]'` is empty -- ASCII only, no smart quotes or em dashes, `--` and `->` throughout. No new markdown paragraph is hard-wrapped. Zero `// inventory:` comments added, removed, or moved (`git diff -- freshbooks/ | grep -c 'inventory:'` = 0), consistent with the implementer's unchanged `implemented 213, todo 0`. No vault names, internal hostnames, real ids or tokens anywhere in the diff.
