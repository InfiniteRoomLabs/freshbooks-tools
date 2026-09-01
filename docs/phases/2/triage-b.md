# Phase 2 batch b -- gate triage

Lead triage of the three read-only lane reports (`b-code-review.md` REQUEST CHANGES 7 blocking / 6 advisory; `b-simplify.md` 3 apply-recommended / 2 optional; `b-security.md` BLOCK, S1 + 2 advisory). Security S1 and review 1 are the same finding. One fix commit: `fix(lib-resources-b): apply the review-gate findings`. QA runs after the fix commit.

## Fix list (apply ALL, one commit)

- **F1** (review 1 + security S1): every `*Path` builder in the batch becomes `(string, error)` guarded by `pathSegment(string(acct))` (and any string id), mirroring `invoicesPath`; propagate errors at all call sites; fold the three inline `fmt.Sprintf` paths in `expenses.go` into guarded builders. One `[sad]` test per resource asserting a hostile `AccountID` (`"a/b"`, `"a?b"`, `"a#b"`) errors with no HTTP request.
- **F2** (review 2): `omitzero` (not `omitempty`) on every struct-typed write field: `EstimateLine.{UnitCost,Amount,TaxAmount1,TaxAmount2}`, `CreditNoteLine.{UnitCost,TaxAmount1,TaxAmount2}`, `BillLine.{UnitCost,Amount}`, `BillLineRequest.UnitCost`, `Bill.DueDate`, `ExpenseProfile.{EndDate,NextIssueDate}`.
- **F3** (review 3): `BillVendorRequest.Is1099` -> `*bool`; `BillLineRequest.CompoundedTax` and `CreditNoteLine.CompoundedTax` -> `*bool`.
- **F4** (review 4): `ExpenseWriteRequest.CategoryID` and `ExpenseProfileCreateRequest.CategoryID` -> `*int64` (docs field table + docs example; Postman quotes it -- update the doc comment to say so). `ExpenseWriteRequest.{TaxPercent1,TaxPercent2,MarkupPercent}` -> `*string` matching the docs type column and the batch's own read model.
- **F5** (review 5): add the 12 documented-writable fields to `ClientWriteRequest` (full shipping address `s_*`, `pref_email`, `pref_gmail`, `allow_late_fees`, `allow_late_notifications`, `company_industry`, `company_size`); bools as `*bool`; extend the create/update fixtures and the Create test to exercise a shipping address.
- **F6** (review 6): `BillVendorTaxDefault` struct (`taxid`, `name`, `amount` per docs) with `BillVendor.TaxDefaults []BillVendorTaxDefault`; add `BillVendor.OutstandingBalance *Money`; non-empty `tax_defaults` in the list fixture so decode is exercised; add `BillLine.{TaxAmount1,TaxAmount2}` read fields.
- **F7** (review 7): every timestamp field currently `string` becomes `DateTime` (clients x4, estimates x3, expenses `Updated`, expense_categories x2, taxes x1, bills x2, bill_vendors x2). If a specific field's wire layout defeats `DateTime`, keep `string` and document why on the field (the `invoices.go` `version` precedent).
- **F8** (review 8 + review 13): one `[happy]` body-assertion subtest per write method: decode the serialized request and assert required keys present AND unset keys absent (write them so they would have failed before F2/F3/F4). Fix the vacuous `bill_vendors` `All` pagination test to serve two pages.
- **F9** (simplify 1): the eight `requestOptions()` bodies delegate to `listOpts` and are renamed `opts()` (the two with `Include` wrap it).
- **F10** (simplify 2): the eight `Page[T]{...}` literals become `newPage(env.X, env.PageMeta)`.
- **F11** (simplify 3): the four hand-rolled vis_state soft-delete bodies (`bill_vendors`, `credit_notes`, `expenses`, `estimates`) delegate to `(*Client).softDelete`. `bills.go`'s `visStatePut` stays (simplify 7's reasoning: `Archive` needs a different state and decodes the response).
- **F12** (simplify 4): add `defaultPerPage`/`pageSize` beside `listOpts`; the eight `All` bodies use it.
- **F13** (review 11): the estimates delete-verb "conflict" is not real (the docs page shows PUT, agreeing with Postman and the code). Trim the claim from `estimates.go`'s doc comment AND drop that bullet from the section 3 `STATE AS OF 2026-09-01 (Phase 2, batch b ...)` callout. The implementer report stays as written (historical).
- **F14** (review 12): `ClientWriteRequest` optional-field convention: pointers throughout (`*string`) with the rule stated in the type doc comment. Signature consistency with merged batch a: `List` gains `extra ...RequestOption` and `Get` gains `opts ...RequestOption` across the batch's services (matches `Invoices.List`/`Get`).
- **F15** (review 10, docs-backed parts): `TaxCreateRequest.Compound *bool` (a compound tax must be creatable) and `ExpenseWriteRequest.ProjectID *int64` (documented writable).

## Explicit non-applies (do NOT do)

- simplify 5 (shared request-capture test helper): deferred -- doing it b-only splits the idiom; package-wide sweep belongs to a later phase.
- simplify 8 (generic `All` helper): agreed DO-NOT-APPLY.
- simplify 9's `All` page-size divergence: keep b's default-100 behavior for now; harmonizing a-vs-b `All` semantics is a phase-close convergence item (lead's ship checklist), not a b fix.
- review 10's INFERRED items (`Expense.Attachment`, `Expenses.Vendors` shape): correctly flagged INFERRED; ride to the live-conformance pass.
- security S2 (govulncheck missing locally): the gate's `mise run vuln` covers it in QA/CI; no action in this batch.

## Lane verdict reconciliation

Security S1 == review 1 (one fix, F1). Review 11 contradicts the implementer's discrepancy #2 -- the reviewer re-fetched the docs page and is right; resolved in favor of review (F13). Simplify's three delegations were anticipated by the lanes' briefing (b predates batch a's helpers); all land here so the library converges on one idiom before c and d gate.
