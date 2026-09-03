# Phase 7 security lane -- `phase-7/live` (2026-09-03)

Read-only review of `git diff main...phase-7/live` (13 commits, `9c76dfc..ad1c1b4`, 50 files). No files other than this report were touched; no tests, builds, or FreshBooks calls were made. The branch is on no remote (`git for-each-ref refs/remotes` has no `phase-7/live`), so nothing below needs a history scrub of a published ref.

## Verdict: **BLOCK** (one blocking finding, a two-minute fix; everything else advisory)

The block is on the repo's own rule ("fixture IDs are synthetic", `CLAUDE.md`), not on exposure risk: the values in A1 are internal row ids that grant nothing without an authenticated session on that account. It is still a real value from the production account in a public-repo capture, and the implementer's own sweep missed it because the checklist only named the top-level ids.

## Findings

### A1 -- BLOCKING -- six system-assigned ids in the ledger capture are not synthetic

- `freshbooks/testdata/seed/ledger_accounts/list.json:19`, `:37`, `:53`, `:69`, `:70`, `:86` -- the `jea_id` (two 7-digit values, 2 apart) and `jesa_id` (four 8-digit values, odd, 2-12 apart) keys carry what look like FreshBooks' real journal-entry-account row ids for this business. Shape evidence: the Phase 1 seeds (`users_me.json`, `accounting_clients_list.json`) and every other Phase 7 capture use the `700NN` / `8675309` / `4242424` / `0000...-4000-8000-...000N` conventions; these six are the only integers in the whole branch diff outside that set (regex sweep of every added line, 6-8 digit runs, excluding the fractional-seconds parts of timestamps). Sequential odd spacing is what a live allocator produces, not what a hand-written fixture produces.
- Cross-check: the six values appear nowhere else in the tree (no Go test, no doc, no unit fixture) -- `grep -rn` over the working tree hits only that file. They are not used by any assertion (`TestLiveLedgerAccounts` checks `uuid`/`type`/`updated_at` only; `LedgerAccount` in `ledger_accounts.go` does not decode them), so the fix cannot break anything.
- Why the implementer's sweep missed it: `implementer.md` line 63 lists the greps run (account id, business id, business uuid, business name, identity uuid, member email, Stripe account id, publishable key). Secondary row ids were not on the list.
- **Fix:** rewrite the six values into the `700NN` range (e.g. `70020`..`70025`), keeping `null` where it is `null`. One fix commit is sufficient; the branch has never been pushed, so no remote history contains them. If the lead wants zero trace in `main`'s history after the `--no-ff` merge, the local-only branch can be rewritten before merge (it is unshared, so the "never rewrite shared history" rule does not apply); given the values' low sensitivity I would not bother.
- **Process fix (same commit or a follow-up):** add "every integer and uuid in a capture, not just the top-level ids" to the Phase 7 capture checklist in `docs/phases/7/plan.md` deliverables, and/or a one-line `grep -oE '[0-9]{6,}'` sanity sweep in the redaction script.

### A2 -- ADVISORY -- capture timestamps are real to the second (or microsecond)

- `freshbooks/testdata/seed/gateways/get.json` and `freshbooks/testdata/gateways/get_stripe_unified.json`: `stripe_tos_accepted_date` and `charges_first_enabled_at`/`payouts_first_enabled_at` are ~2 minutes apart at seconds precision (a real onboarding session), and `stripe_account_updated_at` is a probe-day (`2026-09-02`) stamp.
- `freshbooks/testdata/seed/ledger_accounts/list.json:17,34,50,67,83`: microsecond-precision `updated_at` values; `seed/expenses/list.json:48,50`: a seconds-precision `updated` and a microsecond `version` (the same instant).
- The spec's fact-Q callout quotes one real `invoices[].version` example (`2026-08-24 12:14:28.7...`).
- Phase 1 already set the precedent of keeping the real account-creation date (`accounting_clients_list.json` `signup_date` is `2026-08-22 04:32:55`), so this is consistent with existing practice, and none of it identifies a person. It does time-stamp when the company onboarded Stripe and entered its first expense.
- **Fix (optional):** round to `T00:00:00Z` / `00:00:00` where no test depends on the layout. Keep fractional seconds on the ledger `updated_at` (the live test and spec callout are about the fractional-seconds layout) but zero the fraction (`.000000Z`). Replace the spec's quoted example with a synthetic one.

### A3 -- ADVISORY -- the gateway capture publishes the real Stripe account configuration

- `seed/gateways/get.json` / `testdata/gateways/get_stripe_unified.json`: identifiers are synthetic (32 zeros, `acct_` + zeros, `owner@example.com`), but the payouts schedule (`daily`, 7-day delay), the eight capability flags, `country`, `account_status`, and `onboarding_completion_percent` are the account's real settings. This is configuration about the company that already publishes its name here, not PII, and it is exactly the shape the fix needed. Noting it so the decision is deliberate rather than accidental.
- **Fix (optional):** flip one or two capability booleans and the delay-days value; nothing asserts on them (`TestLiveGateways` checks presence only; `gateways_test.go` -- verify it does not pin `7`/`daily` before changing).

### A4 -- ADVISORY -- `pk_live_` prefix on a synthetic key will trip some secret scanners

- `seed/gateways/get.json`, `testdata/gateways/get_stripe_unified.json`: `publishable_key` is `pk_live_examplepublishablekey000000`. Publishable keys are not secrets, and the value is obviously fake, but several scanners (and GitHub push-protection custom patterns) key on `pk_live_`. A false positive on a public repo costs a triage each time.
- **Fix:** use `pk_test_example...` in both files, or keep `pk_live_` and accept the noise. `TestLiveGateways` only checks non-empty.

### A5 -- ADVISORY -- `701cdcf` (`auth token` refresh): correct; one message gap

- Verified: `cli/internal/auth/status.go` `Token()` -- `store.Load` -> if valid and not `--refresh`, print stored token -> else `cfg.Refresh(ctx, tok.RefreshToken)` -> `store.Save(ctx, next)` -> only then return `next.AccessToken`. A `Save` failure returns an error and prints nothing (`auth_cmd.go:220-222`: the only `Fprintln` of `tok` is after the error check, to `cmd.OutOrStdout()` only). Nothing in `freshbooks/auth/` logs (`grep log\.|Printf|slog` over non-test files: zero hits); `classifyAuthError` wraps the error text, which for the refresh path is the OAuth error string or the store error, never a token value.
- Store atomicity/mode unchanged and confirmed: `freshbooks/auth/store.go:101-149` -- `CreateTemp` in the target dir, `Chmod 0o600`, write, `Sync`, `Rename` over the target.
- Tests cover the seam: `status_test.go` "[happy] --refresh rotates and persists before returning", "[sad] an expired token refreshes instead of printing the dead one" (asserts the rotated refresh token is what `store.Load` returns), "[edge] a token expiring inside the skew is refreshed too", "[sad] a save failure after a successful refresh is returned" (via `brokenSaveStore`).
- Gap: on that save failure the OAuth server has already consumed the old refresh token (one-time use) and the new pair was never persisted, so the user is now locked out until `auth login`, but the error only says "saving the refreshed token". Pre-existing on the `--refresh` path; the fix makes it reachable without the flag. **Fix:** append "; the rotated token could not be stored, run 'freshbooks auth login' again" to that error.

### A6 -- ADVISORY -- `fb5302c` (default scopes): exactly the grantable set, nothing widened; `uploads:write` enables three file writes by default

- `cli/internal/auth/scopes.go`: 20 read/write objects (the 19 documented minus `notifications`/`profile`/`reports`, plus `uploads`) x 2 + 3 read-only = 43. That is the lead's recorded 45-scope grant (`lead-sandbox.md` line 13) minus `account:read`/`account:write`; `riskhub` and `mcp:*` are excluded with a comment naming them. No scope outside the lead's list was added. `TestDefaultScopes` pins the count, the three absent `:write` strings, the presence of both `uploads` scopes, and the absence of `account:`/`riskhub:` prefixes.
- What `user:uploads:write` lets a default-scope CLI token do that the shipped 44-set could not (it never got a token at all): `freshbooks attachments upload-expense-receipt` (`POST /uploads/account/{id}/attachments`), `freshbooks images upload` (`POST /uploads/account/{id}/images`, logo/proposal image), and `freshbooks images upload-without-account` (`POST /uploads/images`) -- i.e. write arbitrary files into the account's attachment/image store. `user:uploads:read` has no lib consumer yet (`/uploads/` GETs are not implemented). Net: the default token is a full read/write token for 20 object families; that was already the documented design, and `--scopes` narrows it.

### A7 -- ADVISORY -- live tests: gated, read-only, no token writes by the tests themselves

- Gate: `live_test.go:23-27` `liveClient` skips unless `FRESHBOOKS_LIVE=1`; both files carry `//go:build live`. `live_conformance_test.go` reuses `liveClient` in every test.
- Calls, all GET: `Identity.Me`, `Expenses.Vendors`, `Expenses.List`, `Clients.List`, `Gateways.Get`, `LedgerAccounts.List/Types/SubTypes/SubType`, `Staff.List`, `TimeEntries.List`, `Projects.List`, `Callbacks.List`, and three `c.Do(ctx, http.MethodGet, ...)` probes (`:260`, `:300`). No POST/PUT/PATCH/DELETE anywhere in the file. The "bad filter" probes send `notadate` and `sort=no_such_field_desc`, harmless.
- Token: never written, printed, or logged by the tests. Note (pre-existing Phase 1 design, not this branch): when `FRESHBOOKS_ACCESS_TOKEN` is unset, `liveClient` builds a refreshing `NewTokenSource` over `NewFileStore(DefaultTokenPath())`, so a refresh mid-suite persists the rotated pair to the lib token file (0600, atomic). Designed behaviour; the plan's documented invocation uses the env var and never hits it.
- Output at default verbosity: `t.Logf` lines print counts, layout names, and the echoed `sort` value only. Failure-path `t.Fatalf("...: %v", err)` prints `(*Error).Error()`, which is `status family: message (errno N, field F)` -- no URL (so no real business id), no raw body (`errors.go:55-81`). `:181` and `:191` print `LedgerAccountSubType` structs on failure, which is the global taxonomy, not account data. Nothing prints a member, client, expense, or gateway record.

### A8 -- PASS -- lead reports and the dev-app client id

- `docs/phases/7/reports/lead-stage1.md` and `lead-sandbox.md`: no account/business ids, no uuids, no email, no token or JWT fragment, no client id. The only host/port is the loopback MCP probe (`127.0.0.1:18081`). The "45 scopes granted" and "12h expiry" statements are shape, not values.
- Dev-app `client_id`: regex `\b[0-9a-f]{64}\b` over every added line of the branch diff and over every intermediate commit (`git log -p main..phase-7/live`): zero hits. `fnox.toml` is gitignored (`.gitignore:18`) and not in the diff.

### A9 -- PASS -- redaction check and full-branch sweep

- `scripts/redaction-check.sh` reads the *index* (`git diff --cached --name-only` + `git show :file`), so it cannot be run read-only against a branch diff. I reproduced its exact matching logic (same resolver, same 8-char short-term word-boundary rule) against `git show phase-7/live:<file>` for all 50 changed files: 18 terms loaded, **0 hits**, `git status --porcelain` empty before and after. Suggestion (not this phase): give the script an optional `<base>..<head>` mode for gate lanes.
- Added-line sweep over the final diff and per commit: emails -> only `owner@example.com` (3x); uuids -> all `00000000-0000-4000-...`; Stripe -> only the two synthetic values; `eyJ`/64-hex/`Bearer <x>`/`token=`/`code=`/URLs with query strings -> none; phone -> `5555550100` only; amounts -> `"100.00"` only. No value was added in an intermediate commit and removed later.
- Changed unit fixtures (`testdata/accounting/expenses_vendors.json`, `testdata/gateways/get_stripe_unified.json`, `testdata/ledger_accounts/*.json`, `testdata/staff/list.json`): synthetic throughout; the staff fixture only gained `identity_uuid` (synthetic) and `language`.

## Summary for triage

| Id | Tag | File | Fix |
|---|---|---|---|
| A1 | BLOCKING | `freshbooks/testdata/seed/ledger_accounts/list.json:19,37,53,69,70,86` | rewrite six `jea_id`/`jesa_id` values into the `700NN` range; extend the capture checklist to all integer ids |
| A2 | ADVISORY | gateway/ledger/expense captures, spec fact-Q example | round timestamps where no layout test depends on them |
| A3 | ADVISORY | gateway captures | optionally perturb the Stripe config booleans/delay |
| A4 | ADVISORY | gateway captures | `pk_test_` prefix to dodge scanner noise |
| A5 | ADVISORY | `cli/internal/auth/status.go:~135` | error text on save-after-refresh failure should say re-login |
| A6 | ADVISORY | `cli/internal/auth/scopes.go` | none; recorded what `uploads:write` enables |
| A7 | ADVISORY | `freshbooks/live_conformance_test.go` | none; note the file-store refresh path is pre-existing |
| A8 | PASS | lead reports | none |
| A9 | PASS | redaction + sweep | optional `--range` mode for the script |

History scrub: **not needed** -- the branch is local-only and the only non-synthetic values (A1) are non-secret row ids. Fix A1 in the one fix commit, re-gate.
