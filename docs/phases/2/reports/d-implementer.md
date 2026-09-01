# Phase 2 batch d implementer report

Worktree: `.worktrees/d`, branch `phase-2/d`. Scope: Accounting (minus taxes), Reports, Webhooks, Uploader, Tokenization, plus 9 cross-folder duplicate keys -- 52 inventory keys total. Status: done, gate green, tree clean, not pushed or merged.

## Files created

Transport (extends existing files):

- `freshbooks/transport.go` -- `doMultipart`, `doOnHost`, `doRaw`, `send`/`sendRaw` (shared retry core `do`/`doMultipart`/`doOnHost` now funnel through), `marshalBody`, `buildMultipartBody`, `maxUploadBytes`. `newRequest` gained a `contentType` parameter.
- `freshbooks/client.go` -- `familyForPath` now classifies the ledger, events, uploads, and payments path prefixes explicitly (see "Envelope resolutions" below).
- `freshbooks/services.go` -- doc-comment updates on the service stubs this batch filled in; no type or field changes (client.go's fields and services.go's stub types were already wired for every service this batch owns).

New resource files, each with a `_test.go` pair and `testdata/<resource>/*.json` fixtures:

- `freshbooks/ledger_accounts.go` -- `LedgerAccountsService`: `Create`, `List`, `Get`, `Update`, `Types`, `SubTypes`, `SubType`.
- `freshbooks/journal_entries.go` -- `JournalEntriesService`: `Create`, `Details`; `JournalEntryAccountsService.List`.
- `freshbooks/other_income.go` -- `OtherIncomeService`: `Create`, `List`, `Update`, `Delete`.
- `freshbooks/reports.go` -- `ReportsService`: `AccountsAging`, `BalanceSheet`, `BankReconciliationSummary`, `ClientAccountStatement`, `DownloadCSV`, `ExpenseDetails`, `InvoiceDetails`, `ItemSales`, `PaymentsCollected`, `ProfitLoss`, `RevenueByClient`, `SalesTaxSummary`, `TrialBalance`, `TimeEntryDetails`.
- `freshbooks/callbacks.go` -- `CallbacksService`: `Register`, `List`, `Delete`, `Verify`, `ResendVerification`.
- `freshbooks/attachments.go` -- `AttachmentsService.UploadExpenseReceipt`.
- `freshbooks/images.go` -- `ImagesService`: `Upload`, `UploadWithoutAccount`.
- `freshbooks/gateways.go` -- `GatewaysService.Get`.
- `freshbooks/payment_options.go` -- `PaymentOptionsService`: `FBPayTokenize`, `StripeTokenize`, `StripeCreateSetupIntent`, `SaveCreditCard`.
- `freshbooks/transport_upload_test.go` -- transport-level tests for `doMultipart`/`doOnHost`/`doRaw`.

Docs/process:

- `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` -- new `STATE AS OF 2026-09-01` callout in section 5.1 (envelope confirmation).
- `freshbooks/CHANGELOG.md` -- `[Unreleased]` entries for the new services and the ledger-family classification fix.
- `freshbooks/internal/inventory/testdata/ignore.list` -- the 52 `//go:inventory-todo` lines this batch owns removed; every other line (including batch b's 5 tax lines) untouched.

## Test counts and coverage

- 75 top-level `Test*` functions in `freshbooks/` (pre-existing + this batch's ~24 new ones); 126 `--- PASS` lines counting subtests (`go test ./... -v`).
- `mise run cover` (module-wide, includes Phase 0/1 code): **93.1%** statements, floor 90% -- PASS.
- `-race` clean (`mise run test` runs `go test -race`).

## `mise run check` tail

```
== inventory-check: freshbooks ==
implemented 56, ignored 0, todo 157, uncovered 0, double-covered 0, stale 0, unknown 0
...
== cover: freshbooks ==
coverage-gate: .../freshbooks/coverage.out total = 93.1% (floor 90%)
coverage-gate: PASS
== vuln: freshbooks ==
No vulnerabilities found.
...
== actionlint ==
== build ==
build: done, artifacts in .../dist
check.sh: all OK
```

No dirty-tree banner on the final run (`git status --porcelain` empty).

## `git log --oneline main..phase-2/d`

```
01b1147 chore(freshbooks): mark batch d's 52 inventory keys implemented, add CHANGELOG entry
ae3911e feat(freshbooks): add payment gateways and card tokenization
01a6cdf feat(freshbooks): add webhook callbacks and expense/logo image uploads
c3cea4f feat(freshbooks): add ledger accounts, journal entries, other income, and reports
9eaa221 feat(freshbooks): add multipart upload and host-override transport support
```

## `git status --porcelain`

Empty (clean tree).

## Inventory keys covered: 52 / 52

- Accounting (14, minus the 5 tax keys batch b owns): `LedgerAccountsService` (7), `JournalEntriesService`/`JournalEntryAccountsService` (3), `OtherIncomeService` (4).
- Reports (15): `ReportsService` (14 direct methods) + `JournalEntryAccountsService.List` (the shared `Reports/General Ledger` key).
- Webhooks (5): `CallbacksService` (5 methods, one key each).
- Uploader (3) + its 3 cross-folder duplicates: `ImagesService` (2 methods, 5 keys) + `AttachmentsService` (1 method, 1 key).
- Tokenization (6) + its 2 `Settings` gateway duplicates: `GatewaysService.Get` (3 keys) + `PaymentOptionsService` (4 methods, 5 keys).

Verified via `mise run inventory-check`: `implemented 56` (52 mine + 3 `Authorization` from Phase 1's `identity.go` + 1 from `auth/oauth.go`), `uncovered 0`, `double-covered 0`.

## Envelope-callout resolutions (spec 5.1)

All three families named in the Phase 1 INFERRED callout are now docs-confirmed (Postman example + FreshBooks docs page where Postman was silent), though still not live -- this phase ran unattended:

- **`/events/`** stays `accounting`: the "List Webhook Callbacks" Postman example is the full `{"response":{"result":{"callbacks":[...],"page":...}}}` envelope. No code change needed; confidence upgraded.
- **`/uploads/`** and **`/payments/`** are both `business` (flat, no envelope): confirmed by the "Upload Logo or Proposal Image" example (`{"image": {...}}`), the FreshBooks expense-attachments docs page (`{"attachment": {...}}`), and the "Get Publishable Key"/"Create Setup Intent" examples (`{"gateway_connections": [...]}`, `{"credit_card": {...}}`). No code change needed; both were already correct via the default fallthrough.
- **Not named in the original callout, but wrong:** `ledger` (`/accounting/businesses/.../ledger_accounts/...` and `/accounting/ledger_accounts/{types,sub_types}`) was falling into the general `/accounting/` prefix match and would have double-unwrapped its actual flat `{"data": ...}` body. Fixed by matching the ledger paths as `business` before the general `/accounting/` case in `familyForPath`. This path was unreachable before this phase (no prior batch implemented it), so it is a fix to unshipped code, not a behavior change for released callers.

## Discrepancies and ambiguities hit, and how I resolved them

1. **Ledger-accounts taxonomy (`Types`/`SubTypes`/`SubType`) has zero evidence.** No Postman example, no public FreshBooks docs page (fetched `/api/chart-of-accounts`, which documents a *different* pair of endpoints). Resolved by modeling the smallest honest shape consistent with the sibling ledger-account fields (`type`/`sub_type` strings) and flagging both the method doc comments and `LedgerAccountSubType`'s doc comment as INFERRED/provisional.
2. **`Reports.ExpenseDetails`'s one docs page describes a different endpoint.** `/api/expense-details-report` documents `GET /accounting/businesses/{business_uuid}/reports/expense_details` (business_uuid-scoped), but the Postman inventory key is `GET /accounting/account/{accountId}/reports/accounting/expense_details` (accountId-scoped) -- different path shape entirely, not just a version bump. Followed the Postman inventory (the parity contract) rather than the docs page, and return `json.RawMessage` rather than the docs page's field list, since that field list may not even apply to this endpoint. Noted in the method's doc comment.
3. **`BankReconciliationSummary` and `ClientAccountStatement` have no Postman example and no docs page at all.** Return `json.RawMessage` for the report body; the query-parameter-only `Options` structs are still typed since those *do* have Postman evidence (the `query` array on those Postman requests).
4. **`Accounting/Other Income/Delete Single Other Income` and `.../Update Single Other Income` are the exact same method+URL** (`PUT .../other_incomes/{incomeId}`), differing only in intent (vis_state soft-delete vs. a general field update). Treated as two distinct exported methods per the duplicate-key rule's "different operation semantics" branch: `Delete` is a one-line wrapper around `Update` that supplies `{vis_state: 1}}`.
5. **`Webhooks/Resend Verification Code` and `Webhooks/Verify Webhook Callback` share one PUT endpoint**, as the gotchas note predicted -- implemented as `ResendVerification` and `Verify`, two methods, one key each, same URL helper.
6. **`Tokenization/2. [FBPAY]` and `Tokenization/3. [STRIPE]` are the same operation** (POST `/payments/account/{accountId}/credit-card`, same `credit_card` body shape, differing only in which token field `credit_card_tokens[]` carries). Collapsed into one `SaveCreditCard` method with two stacked inventory comments, per `identity.go`'s established pattern.
7. **Tokenization/PaymentOptions service mapping.** Per the work order's fallback instruction, no key in this batch maps to `CheckoutLinksService` -- every Tokenization endpoint fits either `GatewaysService` (the gateway-connection GET) or `PaymentOptionsService` (everything that tokenizes or saves a card). Flagging per the work order for the gate to confirm this reading of the spec's service-naming intent.
8. **`FBPayTokenize`/`StripeTokenize`'s response shapes.** `FBPayTokenize`'s `{"cc_token": "..."}` has a Postman example and is typed normally. `StripeTokenize`'s response has none; it is INFERRED to mirror Stripe's own create-payment-method object and returned as `json.RawMessage`, with a doc comment pointing at the `"id"` field that `StripeCreateSetupIntent` expects next. `StripeCreateSetupIntent` itself also has no example and returns `json.RawMessage`.
9. **Multipart form field name.** The Postman collection captures no form-data body for any `/uploads/` request (form-data isn't representable as the tool's example-body text). Confirmed via the FreshBooks `invoice_presentation_attachments` docs page: the field name is `content` for both `.../images` and `.../attachments`, with a quoted `curl --form 'content=@"..."'` example.
10. **`Accounting/Journal Entries/Add Journal Entry`'s `my.freshbooks.com`-sourced entry.** As flagged in the work order's gotchas, the inventory tool had already rewritten the path to the public `/accounting/account/{accountId}/journal_entries/journal_entries`; implemented against that public path and marked the host as INFERRED (not live-confirmed) in the method's doc comment.

No case required reopening or re-litigating a locked spec section-2 decision.

## Blockers

None. Work is complete, gated, and committed on `phase-2/d`. Not pushed, not merged, per instructions.
