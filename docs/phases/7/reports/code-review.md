# Phase 7 code-review lane -- `phase-7/live` (2026-09-03)

Scope: `git diff main...phase-7/live`, 13 commits `9c76dfc..ad1c1b4`. Read-only; no gate, tests, builds, or live calls were run. Every lib type was checked by hand against its capture under `<repo root>/freshbooks/testdata/seed/`.

## Verdict: REQUEST CHANGES

One blocking item (a mis-filed breaking change in the library changelog), six advisories. Every lib fix decodes its capture; nothing in the code needs to change to merge.

## Findings

### R1 -- BLOCKING -- the `LedgerAccounts` return-type change is not recorded as breaking

`freshbooks/CHANGELOG.md:13`. `Types`/`SubTypes`/`SubType` went from `json.RawMessage` to `[]LedgerAccountType` / `[]LedgerAccountSubType` / `*LedgerAccountSubType` (`a96e9f7`). That is a compile break for any consumer of `freshbooks/v0.1.0`, and the implementer report itself says it "needs a release-note line". The entry sits under `### Fixed`, contains no word to that effect, and a reader skimming for compatibility would miss it. The trailing sentence ("Callers that unmarshalled the raw message themselves must switch") is the only hint.

Fix: move the entry to a `### Changed` heading above `### Fixed` and open it with `**Breaking:**`. The shape itself is right (see the confirmations below); only the filing is wrong.

### R2 -- ADVISORY -- five new `t.Skip` calls in the live suite, one of which skips exactly when there is data to assert

`freshbooks/live_conformance_test.go:62, 91, 365, 380, 420`. `CLAUDE.md` says no `t.Skip` without an issue link, and the lane brief said none beyond the existing `FRESHBOOKS_LIVE` gate. Four are data-absence skips (no vendors, no gateway, no expenses, no clients) -- defensible in a live suite, but a `t.Log` and `return` says the same thing without a skip count. Line 420 is different: `TestLiveDateTimeFormats` skips when the account *has* projects, with a message telling a future reader to write the assertion then. That inverts the point of the test.

Fix: at line 420, when `projects.Total != 0`, assert the layout of the first project's `created_at`/`updated_at` (whichever `Project` carries) and log it -- fact Q then closes itself the day the account gets a project. For the other four, replace `t.Skip` with `t.Log(...); return`, or keep the skips and point each at backlog item 13 in `docs/progress.md`.

### R3 -- ADVISORY -- fact Q's spec callout cites invoice producers no capture or test backs

Spec section 5.1, the `STATE AS OF 2026-09-03 (Phase 7, live)` bullet list under "Timestamp layouts, by live producer": `invoices[].create_date`, `invoices[].due_date`, `invoices[].created_at`, `invoices[].updated`, `invoices[].version`, and `clients[].signup_date`. There is no `freshbooks/testdata/seed/invoices/` directory, `TestLiveDateTimeFormats` never lists invoices, and the lead's S2 probe was `clients list`, not `invoices list`. The plan's rule is one capture per observed row. Clients is covered by the Phase 1 `seed/accounting_clients_list.json`; invoices is not covered anywhere.

Fix: either add a redacted `seed/invoices/list.json` (per_page 1 is enough) and extend the "accounting reads" subtest to assert an invoice's `Updated.Layout()`, or reword those rows as "observed in an uncaptured probe" so the callout does not overclaim.

### R4 -- ADVISORY -- two observation dates for the same facts

`freshbooks/expenses.go:327` and `:340` say "CONFIRMED live 2026-09-02"; `freshbooks/gateways.go:88` says "CONFIRMED live 2026-09-02 (Phase 7)". The changelog entries, the spec callouts, and the implementer report date the same two facts 2026-09-03. The report explains why (the vendors/gateways work was resumed from the previous run's tree), but a reader of the code and the spec sees a contradiction.

Fix: pick the real observation date per fact and align the code comment, the changelog line, and the spec callout. If the probes ran on 09-02, the spec bullets for J1 and E should say so.

### R5 -- ADVISORY -- "43 -- every scope the developer portal offers" is not what `scopes.go` does

`cli/internal/cmd/auth_cmd.go:147` (flag help), `cli/internal/docsgen/docsgen.go:80` and the generated `docs/cli.md:61`, and the subtest name at `cli/internal/auth/paths_test.go:67`. `scopes.go` deliberately leaves out `account`, `riskhub`, and `mcp:*`, all of which the portal offers, and says so in its own comment. The user-facing wording claims the default is the portal's full list; `docs/authentication.md` and the spec callout get it right ("43 ... `account` and `riskhub` are left out deliberately").

Fix: "the 43 grantable user:* scopes this toolset's endpoints use (each must be enabled on the app)" in the flag help and the docsgen header; regenerate `docs/cli.md`; rename the subtest.

### R6 -- ADVISORY (hand to the security lane) -- `jea_id`/`jesa_id` in the ledger capture look live, not synthetic

`freshbooks/testdata/seed/ledger_accounts/list.json`: `jea_id` 4550415 / 4550417 and `jesa_id` 12108003 / 12108005 / 12108007 / 12108015. Every other id in the nine new captures follows the synthetic pattern (`ACM123`, `8675309`, `70001..70011`, zero-filled UUIDs, `acct_0000...`); these six do not. They are internal journal-entry-account row ids, so the exposure is low, and `redaction-check.sh` only knows the configured term list, which would not catch them. Flagging because I tripped over it; the security lane owns the call.

Fix if the security lane agrees: replace with a monotonic synthetic series (e.g. `700201..`) in the same commit that touches the capture, and re-run `scripts/redaction-check.sh`.

### R7 -- ADVISORY -- no root `CHANGELOG.md` line for Phase 7

Every previous phase has an `[Unreleased]` entry in the root changelog; this branch adds none, although it changes repo-level docs (`docs/authentication.md` gained a Scopes section, `docs/getting-started.md` step 3 was rewritten) and adds the live suite plus nine capture directories. The agent-ops changelog guard will demand a staged root changelog at the `--no-ff` merge on `main` anyway.

Fix: one `### Added` line in the merge or fix commit: Phase 7 shipped, live suite, captures, the two doc changes, pointer to `docs/phases/7/`.

## Confirmations (what was checked and found right)

**Lib fixes against their captures**

- `f095cde` Vendors: `expenseVendorsEnvelope` (embedded `PageMeta` + `[]struct{Vendor}`) decodes `seed/expenses/vendors.json` key for key. The walk passes `PageNumber(page)` through `do(..., opts ...RequestOption)` (`transport.go:52`), and terminates on `len(env.Vendors)==0 || env.Pages <= page`, so `pages: 0` and an empty page both stop at one request. The three new unit subtests cover two pages, an empty page, and a 401.
- `21e431b` Gateways: `StripeUnifiedConnection` has all 21 keys of `seed/gateways/get.json` with matching JSON types (`int` for `stripe_payouts_schedule_delay_days`/`onboarding_completion_percent`, `[]string` for `available_onboarding_strategies`, `[]StripeCapability{capability,is_active}`). The four timestamps are `DateTime`, whose `UnmarshalJSON` (`types.go:169`) accepts `null` and `""`, so an account still mid-onboarding (null `charges_first_enabled_at`) decodes. The two members seen only as `null`/`{}` stay `json.RawMessage`. `paypal` untouched. `testdata/gateways/get_stripe_unified.json` is byte-identical to the seed capture.
- `a96e9f7` Ledger taxonomy: `{"name":...}` objects, numeric `id`, `base_number` string -- all three fixtures and all three seed captures decode into the new types. `income` vs `revenue`: the fixture, the unit test (`slices.Contains(names, "income")`), and the live test (all five names) agree on `income`; the only remaining `revenue` in the lib is the unrelated Reports `RevenueByClient`. MCP (`tools_ledger_accounts.go:66,73,80`) and CLI (`commands_ledger_accounts.go:72,82,92`) closures return `any` and the result is `json.Marshal`ed, so for live data the wire output has the same key set as before (the raw bytes were those objects already). The MCP roundtrip's `fakeUpstream` serves `{}` for these paths; `SubType` now yields a zero struct instead of `null` there, which no test asserts on. No generated manifest exists (`docs/phases/3/tools.md` is a phase artifact); `mcp/` is untouched.
- `70d38a8` Staff: `identity_uuid`/`language` match `seed/staff/list.json` members. The seventeen sibling business keys listed in the `staffListResponse` comment match the capture's top-level key set exactly. The unit fixture was re-seeded with both keys.
- `04bd77a` Page/Sort: `Sort []string json:"sort,omitempty"` on both `Page[T]` and `PageMeta`; `newPage` copies it, and all 20 list decoders go through `newPage` (no hand-built `Page[T]{}` elsewhere), so every business list surfaces it. `omitempty` drops nil and the empty `"sort": []` the projects capture shows, so accounting output is byte-identical. CLI `table`/`name` read only `items` (`output.go:175`); the sole exact-bytes CLI assertion is the binary-output one; MCP wire tests assert request-side probes, not response bytes. The only unit fixtures carrying `sort` are `testdata/projects/list*.json`, which now decode it -- correct.

**CLI fixes**

- `fb5302c` scopes: 20 read/write objects x 2 + 3 read-only = 43, matching the lead-sandbox arithmetic (45 granted minus `account:read/write`). `TestDefaultScopes` pins the literal 43, asserts the three `:write` strings absent, `uploads:read/write` present, `account`/`riskhub` absent. The old 44-set fails the count and the three-absent subtests.
- `701cdcf` `auth token`: refresh path is `!refresh && tok.Valid(now, DefaultExpirySkew)` -> print, else `cfg.Refresh` then `store.Save` *before* returning (one-time refresh tokens honoured). Clock injected, `nil` -> `time.Now`; the command passes `nil`. `--refresh` on a valid token still rotates. Distinct error for expired-with-no-refresh-token; `classifyAuthError` routes it like the pre-existing no-refresh-token error. Of the four new tests, three fail on the old code (expired, inside-skew, expired-no-refresh); the "still-valid prints without a network call" one passes on both and is the guard against over-refreshing -- fine.
- `5411fce`: `ErrorLog: log.New(io.Discard, "", 0)` on the loopback server only; the paste-fallback prompt is unchanged.

**Live tests** (`-tags live`, `FRESHBOOKS_LIVE=1` gate via `liveClient`): each `TestLive*` asserts the captured shape, not just no-error -- envelope peeled (`Page==1`, `PerPage==3`) for D; verbatim `meta.sort` echo x3 plus non-empty `Page.Sort` for C/O; 422 naming the field vs 200 for B; ids/uuid/language for P; five type names, non-zero `id`/`base_number`, list-vs-single equality for F; populated `stripe_unified` fields for E. No `-run` filter committed. Assertions never print account values.

**Spec callouts**: every confirmed or corrected row (scopes; J1; B, C, O, Q; P; F; E, D) carries a `STATE AS OF 2026-09-0x (Phase 7, live)` line. Fact C is stated as unresolved with the reason (no validation, no data); fact Q as three of four layouts with the zoneless one still INFERRED, and the unnamed fifth format recorded. Write rows are named DEFERRED with the sandbox pointer. `Invoice.Version` is indeed a `string` and `Expense` has no `Version`, as the Q callout says.

**Conventions**: `// inventory:` counts unchanged in every touched service file (expenses 10, ledger 7, gateways 3, staff 4). Commit scopes are conventional and per-module (`fix(cli)`, `fix(freshbooks)`, `test(freshbooks)`, `docs(phase-7)`); spec callouts land in the same commit as their code change. `docs/progress.md` backlog items 1, 4, 5, 7 re-cut and items 12, 13 added, consistent with the implementer report's fact table. Docs are ASCII, unwrapped. No absolute paths or internal hostnames in tracked files.
