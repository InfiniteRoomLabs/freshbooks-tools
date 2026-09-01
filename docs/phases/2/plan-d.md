# Work order: Phase 2 batch d implementer (Accounting + Reports + Webhooks + Uploader + Tokenization)

Dispatch: `Agent(subagent_type: "general-purpose", model: "sonnet", name: "phase-2d-impl")`.

---

You are implementing **Phase 2 batch d (Accounting + Reports + Webhooks + Uploader + Tokenization + payments-family strays)** of `freshbooks-tools`, a public MIT Go monorepo. Work ONLY inside `<repo root>/.worktrees/d` on branch `phase-2/d` (already created, clean). Do not touch other branches or worktrees. All `git` and `mise` commands run from inside the worktree.

## Read first (pointers, not pasted)

1. The oracle: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections 3 (every `STATE AS OF` callout; the 2026-08-23 ones are live-CONFIRMED ground truth) and 5.1 -- **especially the INFERRED envelope-shape callout in 5.1 (events/payments/uploads families), which YOUR batch resolves.** Section 2 is locked; do not redesign.
2. Conventions: `CLAUDE.md`, `GOAL.md` stage 2.
3. **FreshBooks docs before coding** (they carry response examples the Postman collection lacks): https://www.freshbooks.com/api/chart-of-accounts, /api/journal-entries, /api/other_income, /api/reports plus the per-report pages linked from it (/api/account-aging-report, /api/balance-sheet-report, /api/cash-flow-report, /api/chart-of-accounts-report, /api/expense-details-report, /api/general-ledger-report, /api/accounting-api-for-reporting-other-income), /api/webhooks, /api/expense-attachments, /api/invoice_presentation_attachments, /api/gateways, /api/online-payments, /api/enabling-online-payments. Tokenization has no public docs page; Postman is the only evidence -- mark those facts INFERRED.
4. Inventory entries: `freshbooks/internal/inventory/testdata/inventory.json`, folders `Accounting`, `Reports`, `Webhooks`, `Uploader`, `Tokenization`, PLUS the cross-folder keys listed below, MINUS the 5 tax keys batch b owns. Every request you implement gets a `// inventory: <full key>` comment on its method.
5. Exemplar to mirror in shape and style: `freshbooks/identity.go` + `freshbooks/identity_test.go` (service methods, fixtures, stacked inventory comments), `freshbooks/transport.go` (you will EXTEND this file -- read it fully).

## Scope: exactly which inventory keys

You own `Accounting/` minus taxes, plus `Reports/`, `Webhooks/`, `Uploader/`, `Tokenization/`, plus 9 duplicate keys from other folders. Total 52 keys.

**Removed from your batch** (batch b owns all taxes; do NOT implement, do NOT touch their ignore.list lines):

- `Accounting/Taxes/Create Single Tax`, `.../Delete Single Tax`, `.../Get Single Tax`, `.../List Taxes`, `.../Update Single Tax` (5)

**Added to your batch** (same method+URL duplicates of endpoints you own -- implement ONE method per operation and stack every duplicate key as its own `// inventory:` comment line, exactly like `freshbooks/identity.go:86-87`):

Other income (duplicates of `Accounting/Other Income/*`):
- `Invoices/Other Income/Create Single Other Income`, `.../Delete Single Other Income`, `.../List Other Income`, `.../Update Single Other Income` (4)

Uploads (duplicates of the `Uploader/` endpoints):
- `Invoices/Upload Logo/Upload Logo`, `Expenses/Upload Expense Receipt Image/Upload Receipt Image` (both = POST `/uploads/account/{accountId}/images`, same as `Uploader/Upload Logo or Proposal Image`)
- `Settings/Developer/Upload App Logo` (= POST `/uploads/images`, same as `Uploader/Upload Image Without AccountId`)

Gateways (duplicates of `Tokenization/1a. [STRIPE] -  Get Publishable Key`, GET `/payments/account/{accountId}/gateway`):
- `Settings/Businesses/Gateway Details`, `Settings/Gateways/List Gateways` (2)

**Duplicate-key rule** (applies inside your batch too, e.g. `Accounting/Journal Entries/Accounts` and `Reports/General Ledger` share GET `/accounting/.../journal_entry_accounts`): same method+URL+operation -> ONE method, stacked comments. Same URL, different operation semantics -> distinct methods, one key each.

## Services you own (files you create)

`LedgerAccountsService` (ledger_accounts.go -- `Accounting/Accounts`, business_uuid-scoped chart of accounts + the `/accounting/ledger_accounts/{types,sub_types}` taxonomy), `JournalEntriesService` (journal_entries.go), `JournalEntryAccountsService` (journal_entry_accounts.go), `OtherIncomeService` (other_income.go), `ReportsService` (reports.go -- one verb method per report, e.g. `ProfitLoss`, `AccountAging`, `BalanceSheet`...), `CallbacksService` (callbacks.go -- webhooks), `AttachmentsService` (attachments.go), `ImagesService` (images.go), `GatewaysService` (gateways.go), `CheckoutLinksService`/`PaymentOptionsService` only if your keys map there (check `pathTemplate`; Tokenization's fbpay/stripe endpoints may fit `PaymentOptionsService` -- if none fits, note it in your report and use the closest pre-declared service; flag for the gate). Wire fields in `client.go` only if not already wired; add exactly your services' lines.

## Transport work (the judgment-heavy part -- do it FIRST, TDD)

1. **Multipart upload support.** The transport is JSON-only today. Add multipart/form-data request support for the `/uploads/` endpoints (file part + optional fields), following the existing `do()` path's conventions (auth header, retry, error decode, bounded reads). Keep the public surface minimal: an unexported transport helper + exported service methods like `Images.Upload(ctx, acct, r io.Reader, opts...)`. Downloads (attachment GET) return content honestly (reader or bytes -- follow the spec's method vocabulary and note your choice).
2. **Envelope confirmation (spec 5.1 INFERRED callout).** `/events/` is classified accounting (INFERRED from Postman); `/payments/` and `/uploads/` fall through to business and are unverified. Gather the docs pages' response examples for webhooks, uploads, and gateways; set each family's envelope classification to what the evidence shows; update the spec 5.1 callout with a `> **STATE AS OF 2026-09-01**` note (docs-confirmed level; live confirmation pending -- this phase is unattended). If `(*Client).Do` would hand back a wrong envelope for any of these, fix the classification in the transport in the same commit.
3. `Tokenization/1. [FBPAY]` and `[STRIPE] Create Payment Method` POST to `paid.freshbooks.com` -- a different host. The transport is single-base-URL; implement these with an explicit host override (unexported transport support), doc-comment the host, and mark semantics INFERRED (no public docs).

## ignore.list

Remove EXACTLY the `//go:inventory-todo` lines for the 52 keys you implement (Accounting 14 + Reports 15 + Webhooks 5 + Uploader 3 + Tokenization 6 + the 9 cross-folder keys above). Touch no other line.

## Deliverables

- Transport multipart + host-override support with tests (happy, sad, size-bound, retry interaction).
- One `<resource>.go` + `<resource>_test.go` pair per service above, typed models with `json` tags, request/list options per spec 5.1.
- Fixtures under `freshbooks/testdata/<resource>/*.json` seeded from the docs pages' examples with synthetic IDs (never real account IDs, tokens, names, or emails).
- Tests: table-driven `httptest` per method family; sad paths per family; `t.Run` tags `[happy] [sad] [edge] [corner]` where they aid triage.
- Coverage >= 90% module-wide inside the worktree (`mise run cover`).
- Doc comments on every exported identifier.
- `freshbooks/CHANGELOG.md` `[Unreleased]` entry in the existing style.
- The spec 5.1 envelope callout updated (see above).
- `mise run check` green on a clean tree (run from the worktree; includes inventory-check -- your parity must balance).
- Conventional commits `feat(freshbooks): ...`, TDD-sized, each green. **Stage and commit in separate Bash calls.** Do NOT push, do NOT merge.

## Gotchas

- Ledger accounts use `BusinessUUID` (distinct type), NOT `AccountID`/`BusinessID`. The taxonomy endpoints (`/accounting/ledger_accounts/{types,sub_types}`) have no scope ID at all.
- One `my.freshbooks.com`-sourced entry lives in your folders (the collection's only create-journal-entry request); the inventory tool already rewrote it to the public path. Implement against the public path, mark INFERRED in the doc comment, note in your report.
- Reports are read-only GETs with report-specific query options; model each report's result struct from its docs page, not from guesswork.
- Webhooks: `Resend Verification Code` and `Verify Webhook Callback` share PUT on the callback -- distinct operations, distinct methods, one key each.
- Docs are ASCII-only, no hard wraps. Fixture IDs synthetic.
- If the docs disagree with the spec, the docs win: `> **STATE AS OF 2026-09-01**` callout in the same commit + report it.
- Run `scripts/redaction-check.sh` before each commit.

## Reporting (both channels)

When done (check green, committed, `git status --porcelain` empty in the worktree): write the report to `docs/phases/2/reports/d-implementer.md` (commit it in the worktree), send the same report with `SendMessage` to `team-lead` (full report in `message`, not `summary`), AND return it as your final text. Report: files created/changed, test counts, module coverage, the exact `mise run check` tail, `git log --oneline main..phase-2/d`, `git status --porcelain` output, inventory keys covered (count), the envelope-callout resolutions, and every spec discrepancy or ambiguity you hit and how you resolved it. If genuinely blocked, report the blocker the same way instead of guessing.
