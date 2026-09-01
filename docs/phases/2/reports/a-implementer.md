# Phase 2 batch a implementer report (Invoices)

Branch `phase-2/a`, worktree `.worktrees/a`. Work order: `docs/phases/2/plan-a.md`.

## Files created / changed

- `freshbooks/invoices.go` + `freshbooks/invoices_test.go` -- `InvoicesService`: `List`, `All`, `Get`, `Create`, `Update`, `Delete`, `Send`, `PDF`, `ShareLink`, `EnablePaymentOptions`, `InvoicePresentationDefaults`. Also adds `(*Client).fetchRaw`, an unexported transport helper (in this file, no `transport.go` change) for the PDF endpoint, which answers with raw bytes rather than JSON.
- `freshbooks/invoice_profiles.go` + `freshbooks/invoice_profiles_test.go` -- `InvoiceProfilesService`: `List`, `All`, `Get`, `Create`, `Update`, `Delete`, `EnablePaymentOptions`.
- `freshbooks/items.go` + `freshbooks/items_test.go` -- `ItemsService`: `List`, `All`, `Get`, `Create`, `Update`, `Delete`. Shared by the `Invoices/Items and Services` and `Settings/Items and Services` Postman folders (12 stacked inventory keys across 5 methods).
- `freshbooks/payments.go` + `freshbooks/payments_test.go` -- `PaymentsService`: `List`, `All`, `Get`, `Create`, `Update`, `Delete` (accounting payments) plus `CreateCheckoutLink`, `UpdateCheckoutLink`, `DeleteCheckoutLink`, `UpdateCheckoutLinkGateway` (FreshBooks Payments checkout links, same Postman subfolder).
- `freshbooks/retainers.go` + `freshbooks/retainers_test.go` -- `RetainersService`: `List`, `Get`, `Create`, `Update`, `Delete`, `Undelete` (business-scoped, `BusinessID`).
- `freshbooks/testdata/{invoices,invoice_profiles,items,payments,retainers}/*.json` -- 10 new fixtures, synthetic IDs (account `ACM123`, business `8675309`), seeded from the Postman collection's captured examples. Sad-path tests reuse the existing generic `testdata/accounting/error_{404,422}.json` and `testdata/projects/error_404.json` fixtures rather than duplicating them.
- `freshbooks/internal/inventory/testdata/ignore.list` -- removed exactly the 51 `//go:inventory-todo` lines this batch owns (45 `Invoices/*` minus the 4 Other Income + 1 Upload Logo lines batch d owns, plus the 6 `Settings/Items and Services/*` item lines). Touched no other line.
- `freshbooks/CHANGELOG.md` -- `[Unreleased]` entry for the five new services.
- Did **not** touch `client.go` or `services.go`: all five service structs and their `Client` field wiring were already pre-declared in Phase 1 (`services.go`, `client.go`'s `registerServices`).

## Inventory coverage

51 keys, exactly the batch's scope (45 `Invoices/*` + 6 `Settings/Items and Services/*`). `mise run inventory-check`:

```
implemented 55, ignored 0, todo 158, uncovered 0, double-covered 0, stale 0, unknown 0
```

(55 = 4 from Phase 1's `IdentityService` + this batch's 51.)

## Tests

`go test -race .` in `freshbooks/`: 69 top-level test functions, 275 `t.Run` subtests (whole package, including Phase 1's), all pass. New test files alone: `invoices_test.go` (11 top-level funcs), `invoice_profiles_test.go` (7), `items_test.go` (6), `payments_test.go` (7, including the checkout-link group), `retainers_test.go` (5) -- 36 top-level funcs covering `[happy]`, `[sad]`, `[edge]`, and `[corner]` cases per method family (nil-request guards, 404/422/429 decoding, query encoding, envelope shape).

## Coverage

Module-wide (`mise run cover`, floor 90%): **94.1%** (`freshbooks` package alone 95.4%, `auth` 93.8%, `internal/inventory` 92.2% -- both untouched by this batch).

## `mise run check` tail (clean tree, exit 0)

```
== inventory-check: freshbooks ==
implemented 55, ignored 0, todo 158, uncovered 0, double-covered 0, stale 0, unknown 0
...
== cover: cli ==
coverage-gate: .../cli/coverage.out total = 100.0% (floor 90%)
coverage-gate: PASS
== vuln: cli ==
No vulnerabilities found.
== inventory-check: cli (skipped -- only freshbooks has an inventory) ==
== actionlint ==
== build ==
build: mcp ./cmd/freshbooks-mcp -> .../dist/freshbooks-mcp_linux_amd64
... (11 more cross-compile targets)
build: done, artifacts in .../dist
check.sh: all OK
```

Full log covers all three modules (freshbooks, mcp, cli); freshbooks' own `fmt-check`, `vet`, `lint` (0 issues), `test`, `cover`, `vuln`, and `inventory-check` steps all passed before this tail.

## `git log --oneline main..phase-2/a`

```
eb209c7 feat(freshbooks): add items, payments, and retainers resources
ef05323 feat(freshbooks): add invoices and invoice profile resources
```

## `git status --porcelain`

Empty (clean tree) after both commits.

## Spec discrepancies and ambiguities, and how they were resolved

1. **`Single Invoice w/ Payment Gateway`'s Postman path carries a literal invoice id, not a placeholder.** Its pathTemplate normalizes to `POST .../invoices/invoices/{invoiceId}`, unlike the other three "create invoice" duplicates which POST to `.../invoices/invoices` with no id. Its request body, though, is unmistakably a full invoice-creation payload (`customerid`, `lines`, `presentation`) -- the same shape as `Create Invoice with Expense` / `Single Invoice w/ Line Items` / `Single Invoice w/ Logo and styles`. Per `CLAUDE.md`'s documented Postman gotcha ("a few hard-coded example account IDs instead of variables"), I read this as a copy-paste artifact (an example invoice id left in the URL) rather than a real second create-with-id endpoint, and folded its inventory key into `Invoices.Create` alongside the other three. No live/docs evidence contradicts this; flagging here per the work order's "if genuinely ambiguous, report it" instruction rather than guessing silently.
2. **`Update Invoice`, `Update Invoice w/ Expense`, and `Toggle Online Payments on Invoice`** are the same `PUT .../invoices/invoices/{invoiceId}` with different example bodies (status change, line-item expense edit, `allowed_gatewayids` toggle) -- genuinely the same operation (field update) by the work order's duplicate-key test, so they share one `Invoices.Update(ctx, acct, id, *InvoiceUpdateRequest)` with `AllowedGatewayIDs` as one of the optional fields. `Delete Invoice` (`vis_state: 1`) and `Send Invoice by Email` (`action_email` + recipients) stayed separate methods, matching the work order's own Delete/Send precedent.
3. **Retainers' `Delete`/`Undelete`/`Update`** all `PUT` the same `.../retainer/{retainerId}` URL. `Delete` (`active: false`) and `Undelete` (`active: true`) are the soft-delete/restore pair, parallel to the accounting family's `vis_state` pattern, so each kept its own method (mirroring the work order's stated Delete/Send precedent for "genuinely different operations" on one URL) rather than folding into `Update`.
4. **`/payments/` family envelope**, INFERRED per spec 5.1's callout and this batch's own gotcha list. FreshBooks' `/api/online-payments` docs page confirms the response shape for `payment_options`: flat `{"payment_options": {...}}`, no `{"response": {"result": ...}}` wrapping -- consistent with `familyForPath`'s existing default (`/payments/` falls through to `FamilyBusiness`, which does not unwrap). No code change was needed; this is a partial confirmation (docs, not live) of the callout in spec section 5.1, worth a `STATE AS OF` note if a later live-conformance pass wants to upgrade it from INFERRED to CONFIRMED. I did not add a spec callout myself since this batch's gotchas list assigns full resolution of the payments/uploads envelope question to batch d, and I did not want two batches racing to edit the same spec section; flagging the finding here for whichever batch (or the lead) writes that callout.
5. **`CheckoutLink`'s response shape has no Postman example** (`Single Checkout Link`, `Update Checkout Link`, `Update Checkout Link Payment Gateway` all have empty `responses: []`), and the FreshBooks docs page for online payments did not document checkout-link fields either (only `payment_options`). `CheckoutLink` is modeled from the request-body fields FreshBooks' own examples show (`item_id`, `amount`, `currency`, `note`, `is_active`, `send_admin_receipt`, `created_at`, `taxes`, `item_name`) and decoded defensively: `CreateCheckoutLink`/`UpdateCheckoutLink` fall back to echoing the request back to the caller if the response has no `checkout_link` key (tested in `payments_test.go`'s `[edge]` case). This is INFERRED, not CONFIRMED; a live-conformance pass should verify the actual response envelope and field names.
6. **Business-family filter encoding** (`field=value`, not `search[field]=value`): this batch's `Retainers.List` is the first business-scoped list endpoint the library implements, per spec 5.1's callout asking Phase 2 to confirm it. I implemented it per the existing (INFERRED, from docs) encoding already in `types.go`'s `values()` and added a test (`TestRetainersList/[happy]_a_business-family_search_filter_is_a_bare_field=value...`) asserting the query string is `active=true`, not `search%5Bactive%5D=true`. This is still INFERRED -- no live call was made -- so the spec callout's INFERRED status should stay until a live-conformance pass; I did not upgrade it.

No blockers. All 51 keys implemented, gate green on a clean tree.
