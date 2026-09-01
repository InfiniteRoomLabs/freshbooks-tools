# Phase 2 batch b -- code-review lane report

Branch `phase-2/b` (rebased onto main, which carries merged batch a), worktree `.worktrees/b`, diff `git diff main...phase-2/b`, commits `1d0a403..a2d3c9b`. Read-only pass: no files edited, no commits, no `mise`/test/build runs.

Reviewed against spec sections 3 and 5.1, `CLAUDE.md`, the work order `docs/phases/2/plan-b.md`, the implementer report `docs/phases/2/reports/b-implementer.md`, the exemplars (`freshbooks/identity.go` and merged batch a: `invoices.go`, `payments.go`, `items.go`, `transport.go`, `page.go`), and the FreshBooks docs pages for clients, credits, estimates, expenses, expense categories, bills, bill payments, vendors, and taxes.

## Verdict: REQUEST CHANGES

The batch is broad, well-documented, and the inventory/parity bookkeeping looks right. But six of the seven wire-fidelity bug classes batch a's gate fixed are present again here, one of them in every file. Nothing below is a design disagreement; each is the port disagreeing with either the FreshBooks docs, the merged batch a code, or itself.

Numbered findings, BLOCKING first.

---

## BLOCKING

### 1. No `pathSegment` validation anywhere in the batch: 22 unvalidated `AccountID` interpolations

`freshbooks/clients.go:154,158`, `contacts.go:45`, `credit_notes.go:135,139`, `estimates.go:192,196`, `expenses.go:141,145,267,285,347`, `expense_categories.go:85,89`, `taxes.go:88,92`, `bills.go:126,130`, `bill_payments.go:60,64`, `bill_vendors.go:113,117`.

Every path builder in the batch is `func xPath(acct AccountID, ...) string` and interpolates `acct` straight into the path. Batch a's gate introduced `pathSegment` (`transport.go:190-210`) exactly for this, and every batch a builder returns `(string, error)` after calling it -- see `invoices.go:513-520`, `payments.go:333-347`, `items.go:186`.

Why it matters concretely: `resolve` parses the built path with `url.Parse` (`transport.go:154`). An `AccountID` of `ACM123?per_page=500` yields `ref.Path == "/accounting/account/ACM123"` and a silently merged `per_page=500` query -- a different request than the caller asked for, with no error. `#` truncates the path the same way. `noTraversal` (`transport.go:181`) only catches `.`/`..`, and its own doc comment says it is a "defense-in-depth backstop behind pathSegment", not a substitute. These IDs will arrive from CLI flags, config files, and model-authored MCP tool inputs in later phases.

Fix: change all 19 `*Path` helpers to `(string, error)` calling `pathSegment(string(acct))` (plus `pathSegment` on any string id), propagate the error at each of the ~45 call sites, and inline-build the three ad-hoc paths at `expenses.go:267,285,347` through a helper the same way. Add one `[sad]` test per resource asserting a hostile `AccountID` errors before any HTTP round trip, mirroring `transport_test.go:575-590`.

### 2. `omitempty` on struct-typed `Money` write fields: empty money objects on every write

`estimates.go:25,27,30,32` (`EstimateLine.UnitCost/Amount/TaxAmount1/TaxAmount2`), `credit_notes.go:25,28,30` (`CreditNoteLine.UnitCost/TaxAmount1/TaxAmount2`), `bills.go:19,20,81` (`BillLine.UnitCost/Amount`, `BillLineRequest.UnitCost`).

`Money` is a plain struct (`types.go:58-61`) with no `MarshalJSON` and no `IsZero`. `encoding/json`'s `omitempty` never omits a struct, so these fields are always emitted, and a zero `Money` serializes as `{"amount":"","code":""}`.

`EstimateLine` and `CreditNoteLine` are the line types on `EstimateWriteRequest.Lines` (`estimates.go:148`) and `CreditNoteWriteRequest.Lines` (`credit_notes.go:108`), so creating an estimate line that sets only `Name`/`Qty`/`UnitCost` still posts `"amount":{"amount":"","code":""}`, `"taxAmount1":{"amount":"","code":""}`, `"taxAmount2":{...}` -- server-computed read-only fields, sent empty. This is the identical class batch a fixed: `InvoiceLine.UnitCost` and `.Amount` carry `omitzero`, not `omitempty` (`invoices.go:30,40`), and `InvoiceCreateRequest.CreateDate` is `Date` + `omitzero` (`invoices.go:265`).

Fix: `omitzero` on every struct-typed field in these three types (Go 1.24+ tag; the module requires >= 1.26, and batch a already relies on it). Same for the `Date` value fields that are optional on a write path -- `bills.go:39` `Bill.DueDate`, `expenses.go:299,300` `ExpenseProfile.EndDate/NextIssueDate`.

### 3. `Is1099 bool` with `omitempty` on a shared create/update request: `false` is unsendable

`bill_vendors.go:390` (`BillVendorRequest.Is1099`), used by both `Create` (`:455`) and `Update` (`:472`).

`BillVendorRequest` is the partial-update payload for `PUT .../bill_vendors/{id}`. With `omitempty`, `Is1099: false` is indistinguishable from unset, so a caller can turn 1099 tracking **on** but can never turn it **off**. The FreshBooks vendors docs page sends `"is_1099": false` explicitly in its own create example, so the API expects the field to be sendable as false. This is batch a's toggle-bool class verbatim.

Same shape, lower blast radius, in the nested line structs: `bills.go:87` `BillLineRequest.CompoundedTax`, `credit_notes.go:31` `CreditNoteLine.CompoundedTax`.

Fix: `Is1099 *bool` and `CompoundedTax *bool` on the request types. The batch already uses this convention correctly on its top-level write structs (`ExpenseWriteRequest.IsCOGS *bool`, `TaxUpdateRequest.Compound *bool`, `EstimateWriteRequest.RichProposal *bool`), so this is an inconsistency inside the batch, not a policy question.

### 4. `categoryid` sent as a JSON string; the docs field table and the docs example both say int

`expenses.go:101` (`ExpenseWriteRequest.CategoryID string`) and `expenses.go:324` (`ExpenseProfileCreateRequest.CategoryID string`).

The doc comment at `expenses.go:95-97` justifies this from a Postman example. The FreshBooks expenses docs page contradicts it on both counts: the field table lists `categoryid` as **int, writable**, and the page's own create-expense example body sends `"categoryid": 93993004` unquoted. The batch's own read model uses `CategoryID int64` (`expenses.go:33`), so the type flips across the same field name in the same file.

`CLAUDE.md` and the work order both say docs beat the Postman collection when they disagree ("If the docs disagree with the spec, the docs win"; "Inferred vs confirmed... the API wins"). This one risks a hard 422 on every `Expenses.Create` that files an expense under a category.

Fix: `CategoryID *int64` on both write structs, and replace the doc comment's justification with a note that the Postman example quotes the value while the docs do not.

Related, same page, same class (fold into the same fix): `ExpenseWriteRequest.TaxPercent1/TaxPercent2/MarkupPercent` are `*float64` (`expenses.go:110-112`) while the docs field table types all three as **string**, and the batch's own `Expense` read model types them `string` (`expenses.go:62,63,66`).

### 5. `ClientWriteRequest` cannot set 12 documented-writable client fields

`clients.go:103-124`.

The FreshBooks clients docs page marks all of these writable; none appear on the request struct: `s_street`, `s_street2`, `s_city`, `s_province`, `s_code`, `s_country` (the entire shipping address), `pref_email`, `pref_gmail`, `allow_late_fees`, `allow_late_notifications`, `company_industry`, `company_size`. The `Customer` read model has every one of them (`clients.go:64-76,53-55`), so the asymmetry is visible in the same file.

A caller of this library cannot set a client's shipping address at all. Clients is a headline resource; this is a functional gap, not a nicety.

Fix: add the 12 fields to `ClientWriteRequest`, with the four bools as `*bool` (see finding 3), and extend `clients_create.json` / `clients_update.json` and the Create test to exercise a shipping address.

### 6. `BillVendor.TaxDefaults []string` will fail to decode; `outstanding_balance` is missing

`bill_vendors.go:355` and the `BillVendor` struct generally.

The vendors docs page types `tax_defaults` as an **object array**, not a string array. Decoding a vendor that actually has tax defaults will fail with `json: cannot unmarshal object into Go value of type string` and surface as an opaque error from `List`/`Get`. The fixture `testdata/accounting/bill_vendors_list.json` sends `"tax_defaults": []`, so the test suite can never catch it.

Also missing from the read model: `outstanding_balance`, a documented read-only `{amount, code}` field on every vendor -- the class-5 "documented response field silently dropped".

Fix: introduce `BillVendorTaxDefault` (`taxid`/`name`/`amount` per the docs) and type `TaxDefaults []BillVendorTaxDefault`; add `OutstandingBalance *Money \`json:"outstanding_balance,omitempty"\``; put a non-empty `tax_defaults` entry in the list fixture so the decode is exercised.

Same class, one line lower blast radius: `BillLine` (`bills.go:16-27`) omits the documented read-only `tax_amount1`/`tax_amount2` objects the bills docs page lists on every line.

### 7. Every timestamp in the batch is `string`; the library's `DateTime` type exists precisely for these

`clients.go:79-82` (`SignupDate`, `LastLogin`, `LastActivity`, `Updated`), `estimates.go:51,53` (`CreatedAt`, `Updated`) and `estimates.go:17` (`EstimateLine.Updated`), `expenses.go:70` (`Updated`), `expense_categories.go:223,224`, `taxes.go:33`, `bills.go:61,62`, `bill_vendors.go:357,358`. `grep -c DateTime` returns 0 for all ten new files.

Batch a decodes `created_at` and `updated` as `DateTime` (`invoices.go:142,143`) and, where it deliberately kept a `string`, said so and why in the doc comment (`invoices.go:120-125`, for `version`, whose layout `DateTime` cannot parse). The FreshBooks clients docs page types `signup_date`, `updated`, and `last_login` as **datetime**, and the batch's own fixture values (`"2026-08-22 04:32:55"`) parse cleanly under `DateTimeLayout`, so nothing here is blocked by an unparseable layout.

This is a public API-shape decision that is cheap now and a breaking change after the first tagged release.

Fix: `DateTime` for every one of these fields. If any specific field turns out to carry a layout `DateTime` rejects, keep it a `string` and document the reason on the field, matching the `invoices.go:120-125` precedent.

---

## ADVISORY

### 8. No Create/Update test asserts the serialized request body

`bill_vendors_test.go:52-60` ("posts the vendor payload") checks only `r.Method`. `estimates_test.go:95-108` builds an `EstimateLine` with `UnitCost` set and asserts only the method and the decoded response id. `clients_test.go`, `credit_notes_test.go` (Create), `expense_categories_test.go`, `bill_payments_test.go`, `taxes_test.go` follow the same shape. Only the vis_state soft-delete paths read the body back (`bills_test.go:102-115`, `estimates_test.go:150-166`, `expenses_test.go:156-168`, `credit_notes_test.go:131-147`).

That gap is why findings 2, 3, 4, and 7 are invisible to a green gate: no test can see what actually goes on the wire for a write. Suggest one `[happy]` body-assertion subtest per write method -- decode the request body and assert both the fields that must be present and the fields that must be **absent** (e.g. no `amount` key on an estimate line the caller did not price).

### 9. Batch a's shared helpers are re-implemented per file instead of reused

- `listOpts` (`page.go:38`) and `newPage` (`page.go:56`) exist and are used by every batch a service. Batch b hand-rolls the same option loop eight times (`clients.go:134`, `credit_notes.go:118`, `estimates.go:172`, `expenses.go:124`, `expense_categories.go:252`, `taxes.go:71`, `bills.go:109`, `bill_vendors.go:401`) and constructs `&Page[T]{Items: ..., Page: env.Page, ...}` by hand at each `List` (e.g. `estimates.go:208`, `taxes.go:106`). `listOpts` does not cover `Include`, so the two resources that need it (`clients.go`, `estimates.go`) can wrap it rather than replace it.
- `(*Client).softDelete` (`transport.go:146`) was added by batch a for exactly the accounting vis_state-PUT pattern and is unused here: `expenses.go:238`, `estimates.go:283`, `bill_vendors.go:186`, and `credit_notes.go` each hand-roll `map[string]any{"<key>": map[string]any{"vis_state": VisStateDeleted}}`. `bills.go:179` needs its own variant only because `Archive` decodes the response.

Mostly the simplification lane's call, but the divergence also means a future fix to the shared helper silently skips this batch.

### 10. Smaller docs/field gaps, grouped

- `TaxCreateRequest` (`taxes.go:47-51`) has no `Compound` field, though `TaxUpdateRequest` and the `Tax` read model both do -- a compound tax cannot be created in one call.
- `ExpenseWriteRequest` (`expenses.go:98-115`) omits `projectid`, documented writable on the expenses page.
- `Expense.Attachment`/`ExpenseAttachmentRequest` and `Expenses.Vendors`' `{"vendors": [...]}` string-list shape are flagged INFERRED in the doc comments, which is the right handling -- worth carrying into the Phase 2 live-check list rather than changing now.

### 11. `Estimates.Delete`'s doc comment states a docs fact that does not hold

`estimates.go:273-279` says the FreshBooks docs page "list[s] the verb as DELETE". The estimates docs page's "Delete Single Estimate" section shows `PUT .../estimates/estimates/<id>`, agreeing with the Postman example and with the implementation. The code is right; the comment (and discrepancy #2 in `b-implementer.md`, and the matching line in the section 3 `STATE AS OF 2026-09-01` callout) records a conflict that is not there. Trim it so the spec callout stays trustworthy.

### 12. Inconsistent optional-field and option-parameter conventions

- `ClientWriteRequest` (`clients.go:103-124`) makes `VatName`, `Note`, and `HomePhone` `*string` while `Organization`, `Email`, `MobilePhone`, `BusinessPhone`, and `Fax` are plain `string` with `omitempty`. On a partial-update PUT that means three fields can be cleared and the rest cannot, with no stated rule. Pick one (pointers throughout, given it is the Update payload) and say so in the type doc comment.
- `Get` takes `opts ...RequestOption` on `Clients` (`clients.go:192`) and `Estimates` (`estimates.go:230`) but not on `Expenses` (`expenses.go:178`), `ExpenseCategories` (`:306`), or `Taxes` (`taxes.go:129`); `List` takes no `extra ...RequestOption` anywhere, where batch a's `Invoices.List` does (`invoices.go:222`). Worth settling before the CLI and MCP layers bind to these signatures.

### 13. Two thin tests

`bill_vendors_test.go:36-49` is named "auto-paginates until a short page" but serves a single one-page fixture, so the loop terminates on the first response and the pagination logic is never exercised; the sibling `All` tests share the shape. `TestBillVendorsCreate`'s `[happy]` case asserts only the method (see finding 8).

---

## What looked right

Inventory comments and duplicate-key stacking match the work order and the `identity.go:86-87` precedent; the tax triple-stacking is correct. Envelope keys, HTTP verbs, and URL templates check out against the docs pages for every resource I spot-checked (bills, bill_vendors, credit_notes, estimates, expenses, taxes). Sad paths cover 404/422/429 through `errors.Is` sentinels consistently. `Delete` returning only `error`, `Bills.Archive` keeping its `(*Bill, error)`, the `Customer`-not-`Client` rename, and the choice to give `ContactsService` real methods are all well-reasoned and well-documented in the doc comments. Fixture IDs are synthetic and I found no redaction issues.

## Suggested fix order

1. Finding 1 (mechanical, touches every file, changes signatures -- do it first so the others rebase onto it).
2. Findings 2, 3, 7 (struct tags and field types; one pass per file).
3. Findings 4, 5, 6 (docs-driven type and field corrections).
4. Finding 8 (body assertions), which should be written so that it would have failed before 2/3/4 landed.
5. Advisories 9-13 as the lead sees fit; 9 overlaps the simplification lane.
