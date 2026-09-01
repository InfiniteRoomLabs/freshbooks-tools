# Work order: Phase 2 batch b implementer (Clients + Estimates + Expenses + Taxes)

Dispatch: `Agent(subagent_type: "general-purpose", model: "sonnet", name: "phase-2b-impl")`.

---

You are implementing **Phase 2 batch b (Clients + Estimates + Expenses + Taxes)** of `freshbooks-tools`, a public MIT Go monorepo. Work ONLY inside `<repo root>/.worktrees/b` on branch `phase-2/b` (already created, clean). Do not touch other branches or worktrees. All `git` and `mise` commands run from inside the worktree.

## Read first (pointers, not pasted)

1. The oracle: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections 3 (every `STATE AS OF` callout; the 2026-08-23 ones are live-CONFIRMED ground truth) and 5.1. Section 2 is locked; do not redesign.
2. Conventions: `CLAUDE.md`, `GOAL.md` stage 2.
3. **FreshBooks docs before coding** (they carry response examples the Postman collection lacks): https://www.freshbooks.com/api/clients, /api/credits, /api/estimates, /api/expenses, /api/expense_categories, /api/bills, /api/bill_payments, /api/vendors, /api/taxes, /api/update-existing-client-with-secondary.
4. Inventory entries: `freshbooks/internal/inventory/testdata/inventory.json`, folders `Clients`, `Estimates`, `Expenses`, EXCEPT the exclusions below, PLUS 10 tax keys from other folders. Every request you implement gets a `// inventory: <full key>` comment on its method.
5. Exemplar to mirror in shape and style: `freshbooks/identity.go` + `freshbooks/identity_test.go` (service methods, fixtures, stacked inventory comments), `freshbooks/transport.go` (use the existing transport helpers, never raw http).

## Scope: exactly which inventory keys

You own every key under `Clients/`, `Estimates/`, `Expenses/` **plus** all 10 tax duplicates from other folders, **minus** 1 upload key owned by batch d. Total 59 keys.

**You own ALL taxes.** The collection carries the same 5 tax operations in three folders (same method+URL). Implement ONE `TaxesService` method per operation and stack all three keys as `// inventory:` comment lines on it, exactly like `freshbooks/identity.go:86-87`:

| Operation | Your key | Stacked duplicate keys (you own these too) |
|---|---|---|
| List | `Expenses/List Taxes` | `Accounting/Taxes/List Taxes`, `Settings/Items and Services/List Taxes` |
| Create | `Expenses/Create Single Tax` | `Accounting/Taxes/Create Single Tax`, `Settings/Items and Services/Create Single Tax` |
| Get | `Expenses/Single Tax (GET)` | `Accounting/Taxes/Get Single Tax`, `Settings/Items and Services/Single Tax (GET)` |
| Update | `Expenses/Update Tax` | `Accounting/Taxes/Update Single Tax`, `Settings/Items and Services/Update Tax` |
| Delete | `Expenses/Single Tax (DELETE)` | `Accounting/Taxes/Delete Single Tax`, `Settings/Items and Services/Single Tax (DELETE)` |

**Removed from your batch** (do NOT implement, do NOT touch its ignore.list line -- batch d owns all `/uploads/` endpoints):

- `Expenses/Upload Expense Receipt Image/Upload Receipt Image`

**Duplicate-key rule** (applies inside your batch too, e.g. `Create Expense` vs `Create Expense with Receipt`, `Update Credit Note` vs `Update Prepayment Credit`, the estimate `PUT` variants): when two keys are the same method+URL+operation differing only in example body, implement ONE method and stack their comments. When the same method+URL carries genuinely different operations (FreshBooks deletes, accepts, and sends via `PUT` with different bodies), give each operation its own method with its own key (`Estimates.Delete`, `Estimates.Accept`, `Estimates.Send`, `Estimates.Update`; `Bills.Archive` vs `Bills.Delete`; `Clients.Update` vs `Clients.RemoveAllSecondaryContacts`).

## Services you own (files you create)

`ClientsService` (clients.go, incl. secondary contacts -- check whether spec 5.1's `ContactsService` gets its own methods or rides on clients; the inventory keys decide: contacts operations live inside client payloads, so `ContactsService` may legitimately end Phase 2 with no methods -- note it in your report if so), `CreditNotesService` (credit_notes.go, the `Clients/Credits` subfolder), `EstimatesService` (estimates.go), `ExpensesService` (expenses.go), `ExpenseCategoriesService` (expense_categories.go, if the Expenses folder carries category endpoints -- the inventory decides), `TaxesService` (taxes.go), `BillsService` (bills.go), `BillPaymentsService` (bill_payments.go), `BillVendorsService` (bill_vendors.go). All accounting family (`AccountID` string, enveloped). Wire fields in `client.go` only if not already wired; add exactly your services' lines.

## ignore.list

Remove EXACTLY the `//go:inventory-todo` lines for the 59 keys you implement (Clients 13 + Estimates 8 + Expenses 28 + the 5 `Accounting/Taxes/*` lines + the 5 `Settings/Items and Services` tax lines). Touch no other line.

## Deliverables

- One `<resource>.go` + `<resource>_test.go` pair per service above, typed models with `json` tags (pointer fields for optionals on write structs), request/list options per spec 5.1.
- Fixtures under `freshbooks/testdata/<resource>/*.json` seeded from the docs pages' examples with synthetic IDs (never real account IDs, tokens, names, or emails).
- Tests: table-driven `httptest` per method family; sad paths per family; `t.Run` tags `[happy] [sad] [edge] [corner]` where they aid triage.
- Coverage >= 90% module-wide inside the worktree (`mise run cover`).
- Doc comments on every exported identifier.
- `freshbooks/CHANGELOG.md` `[Unreleased]` entry in the existing style.
- `mise run check` green on a clean tree (run from the worktree; includes inventory-check -- your parity must balance).
- Conventional commits `feat(freshbooks): ...`, TDD-sized, each green. **Stage and commit in separate Bash calls.** Do NOT push, do NOT merge.

## Gotchas

- Everything in your batch is accounting family: string `account_id`, enveloped responses (the transport un-peels; your code never sees the envelope), `search[field]=value` filters via the existing `Search` option type.
- Bills and Vendors are Beta endpoints -- mark any fact only evidenced by Postman as INFERRED in doc comments.
- Expense/Bill "delete" and "archive" are `PUT` with `vis_state` values. Model as distinct methods.
- Docs are ASCII-only, no hard wraps. Fixture IDs synthetic.
- If the docs disagree with the spec, the docs win: add a `> **STATE AS OF 2026-09-01**` callout in the affected spec section in the same commit and list it in your report.
- Run `scripts/redaction-check.sh` before each commit.

## Reporting (both channels)

When done (check green, committed, `git status --porcelain` empty in the worktree): write the report to `docs/phases/2/reports/b-implementer.md` (commit it in the worktree), send the same report with `SendMessage` to `team-lead` (full report in `message`, not `summary`), AND return it as your final text. Report: files created/changed, test counts, module coverage, the exact `mise run check` tail, `git log --oneline main..phase-2/b`, `git status --porcelain` output, inventory keys covered (count), and every spec discrepancy or ambiguity you hit and how you resolved it. If genuinely blocked, report the blocker the same way instead of guessing.
