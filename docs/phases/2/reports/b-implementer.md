# Phase 2 batch b implementer report -- Clients + Estimates + Expenses + Taxes

Worktree: `.worktrees/b`, branch `phase-2/b`. Not pushed, not merged.

## Files created

Resource pairs (`<resource>.go` + `<resource>_test.go`), all under `freshbooks/`:

- `taxes.go` / `taxes_test.go` -- `TaxesService` (List, All, Get, Create, Update, Delete)
- `bills.go` / `bills_test.go` -- `BillsService` (List, All, Create, Archive, Delete)
- `bill_payments.go` / `bill_payments_test.go` -- `BillPaymentsService` (Create, Update)
- `bill_vendors.go` / `bill_vendors_test.go` -- `BillVendorsService` (List, All, Create, Update, Delete)
- `expenses.go` / `expenses_test.go` -- `ExpensesService` (List, All, Get, Create, Update, Delete, Summaries, Vendors, CreateRecurring)
- `expense_categories.go` / `expense_categories_test.go` -- `ExpenseCategoriesService` (List, All, Get, Create)
- `estimates.go` / `estimates_test.go` -- `EstimatesService` (List, All, Get, Create, Update, Delete, Accept, Send)
- `credit_notes.go` / `credit_notes_test.go` -- `CreditNotesService` (List, All, Create, Update, Delete)
- `clients.go` / `clients_test.go` -- `ClientsService` (List, All, Get, Create, Update, RemoveAllSecondaryContacts)
- `contacts.go` / `contacts_test.go` -- `ContactsService` (Update, Delete)

Fixtures: 34 new files under `freshbooks/testdata/accounting/` (one per happy-path response shape), reusing the existing generic `error_404.json` / `error_422.json` / `error_429.json` fixtures for sad paths, matching the Phase 1 precedent in `identity_test.go`.

Files changed:

- `freshbooks/services.go` -- removed the 10 stub types this batch implements for real (`ClientsService`, `ContactsService`, `ExpensesService`, `ExpenseCategoriesService`, `EstimatesService`, `TaxesService`, `BillsService`, `BillPaymentsService`, `BillVendorsService`, `CreditNotesService`). `client.go` was already fully wired to all 36 service fields from Phase 1 scaffolding, so no change there.
- `freshbooks/internal/inventory/testdata/ignore.list` -- removed exactly the 59 `//go:inventory-todo` lines this batch owns; touched no other line.
- `freshbooks/CHANGELOG.md` -- `[Unreleased]` entries for the 9 new services.
- `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` -- one new `STATE AS OF 2026-09-01` callout in section 3 recording four discrepancies (below).

## Test counts, coverage, gate

- 52 top-level `Test*` functions across the 10 new test files, 101 passing subtests (`[happy]`/`[sad]`/`[edge]` tagged), all green under `-race`.
- Module-wide coverage (`freshbooks`): **93.8%** on `mise run cover` (floor 90%); `go test` reports 94.5% for the package alone.
- `mise run check` tail (full repo, all three modules):

```
check.sh: all OK
```

Every step passed: fmt-check, vet, lint (0 issues), test (`-race` + `-tags integration`), cover (93.8% >= 90%), vuln (no vulnerabilities), inventory-check, actionlint, build (12 cross-compiled artifacts), dirty-tree banner clean.

- `mise run inventory-check`: `implemented 63, ignored 0, todo 150, uncovered 0, double-covered 0, stale 0, unknown 0`. 63 = 4 (Phase 1 `Authorization`) + 59 (this batch). 150 = 209 remaining minus this batch's 59.

## Inventory keys covered

**59**, exactly the work order's scope: Clients 13 (incl. `Clients/Credits` 6), Estimates 8, Expenses 28 (incl. the 5 `Expenses/*Tax*` spellings), plus the 10 duplicate tax keys from `Accounting/Taxes/*` (5) and `Settings/Items and Services/*` (5) stacked onto `TaxesService`'s five methods. `Expenses/Upload Expense Receipt Image/Upload Receipt Image` was left untouched (batch d owns it) -- its `ignore.list` line was not removed and not touched.

## git status / git log

```
$ git status --porcelain
(empty)

$ git log --oneline main..phase-2/b
1d0a403 docs(phase-2): batch b changelog entry and spec discrepancy callouts
ed7d4ea feat(freshbooks): add Clients and Contacts services
87a2eb2 feat(freshbooks): add CreditNotesService
2a5224c feat(freshbooks): add EstimatesService
5853643 feat(freshbooks): add Expenses and ExpenseCategories services
fcfd671 feat(freshbooks): add Bills, BillPayments, and BillVendors services
31cef66 feat(freshbooks): add TaxesService covering the tax-rate duplicate keys
```

Each commit was verified independently green (`go build`, `go vet`, `gofmt -l`, `go test`, `mise run inventory-check`) by staging its files, `git stash push --keep-index -u`-ing the rest of the WIP out of the working tree, running the checks, then committing and popping the stash back -- so the sequence really is bisectable, not just the final state.

## Design decisions and ambiguities resolved

1. **`Customer`, not `Client`, as the Go type name.** The FreshBooks resource is called "client" everywhere in the API and docs, but `*Client` already names this library's own API client type (`client.go`). Named the resource type `Customer` instead, tracking the API's own `customerid` field name used throughout the rest of the accounting family (estimates, credit notes reference `customerid`/`clientid`). `ClientsService`, `ClientListOptions`, `ClientWriteRequest` keep the "Client" name since there's no collision at that level.

2. **`ContactsService` does get real methods**, contrary to the work order's hedge that it "may legitimately end Phase 2 with no methods." The inventory has two standalone contact endpoints (`Clients/Edit Secondary Contact ID`, `Clients/Delete Secondary  Contact ID`) hitting `/accounting/account/{accountId}/users/contacts/{contactId}` directly -- a distinct resource path from `/users/clients/{customerId}`. `ClientsService.RemoveAllSecondaryContacts` (PUT with an empty `contacts` array on the client payload) and `ContactsService.{Update,Delete}` (addressing one contact by its own ID) are therefore two different operations on two different paths, matching the work order's own `Clients.Update` vs `Clients.RemoveAllSecondaryContacts` distinction.

3. **Duplicate-key stacking**, per the work order's rule and the `identity.go:86-87` precedent:
   - Taxes: one method per operation, 3 stacked keys each (`Expenses/*`, `Accounting/Taxes/*`, `Settings/Items and Services/*`).
   - `Expenses.Create` stacks "Create Expense" + "Create Expense with Receipt"; `Expenses.Update` stacks the two "...with Receipt" pairs the same way.
   - `Estimates.Create` stacks "Create Single Proposal w/ Sections, Logos, and E-signature" + "Single Estimate With Estimate Lines" (both POST, same URL, differ only in which optional fields the example sets).
   - `CreditNotes.Create` stacks "Create Credit Note" + "Create Prepayment Credit"; `CreditNotes.Update` stacks the "Update Credit Note" + "Update Prepayment Credit" pair the same way. `CreditType` on the shared request distinguishes goodwill/prepayment/overpayment.
   - Per the work order's explicit list, `Estimates.Delete`/`Accept`/`Send`/`Update` and `Bills.Archive`/`Delete` each got their own method despite sharing a URL with other estimate/bill operations, since they are genuinely different operations (different bodies, different intents), not example-only duplicates.

4. **`Delete` returns only `error`, not the updated resource**, for every soft-deleting method (`Expenses.Delete`, `Estimates.Delete`, `CreditNotes.Delete`, `BillVendors.Delete`, `Bills.Delete`, `Contacts.Delete`) -- matching the spec's literal vocabulary example (`err = c.Invoices.Delete(ctx, acct, 1234)`). `Bills.Archive` keeps returning `(*Bill, error)` since it's a resource-specific verb, not part of the `Delete` vocabulary.

5. **Every soft-delete except `Taxes.Delete` and `Contacts.Delete`** is a `PUT` with `{"<resource>": {"vis_state": 1}}`, per the CLAUDE.md gotcha ("Expense/Bill delete and archive are PUT with vis_state values. Model as distinct methods."), applied consistently to Estimates and CreditNotes too since their Postman examples show the same shape. `Taxes.Delete` and `Contacts.Delete` are real HTTP `DELETE`s -- the Postman collection is explicit about the method for both.

6. **`ExpenseCategoryCreateRequest`/`ExpenseCategoriesService.Create` implemented despite the docs saying otherwise** -- see spec discrepancy #3 below. Left in per the parity contract (every assigned inventory key needs a method); flagged rather than silently dropped.

## Spec discrepancies (all recorded as a single `STATE AS OF 2026-09-01` callout in section 3; none live-verified -- no sandbox account this phase)

1. **`Expenses/Delete Expense`'s Postman example body sends `vis_state: 0`** (active), not `1`, contradicting every other soft-delete in the family and the FreshBooks docs page for expenses itself. Treated as a Postman authoring mistake; `ExpensesService.Delete` sends `vis_state: 1`.
2. **`Estimates/Delete Estimate`'s FreshBooks docs page lists the verb as `DELETE`**, but the concrete Postman example sends `PUT` + `vis_state: 1`. Trusted the concrete example over the docs page's verb, consistent with the rest of the family.
3. **`Expenses/Create Custom Expense Category` is in the Postman collection, but the FreshBooks docs page for expense categories states creating/updating/deleting categories is unsupported.** Implemented per the inventory parity contract; INFERRED, contradicts the docs.
4. **No Postman example response** for `Clients/Edit Secondary Contact ID`, `Clients/Delete Secondary  Contact ID`, `Expenses/Expense Vendors`, and `Expenses/Create Recurring Expense`. Response shapes are INFERRED from this API family's otherwise-uniform conventions (a single-resource envelope named after the resource, or a bare array for the vendors list).

## Blockers

None. Ready for the code-review / simplification / security gate, then QA.
