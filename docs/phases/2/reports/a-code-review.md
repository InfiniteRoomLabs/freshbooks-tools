# Phase 2 batch a -- code-review lane

Branch `phase-2/a`, worktree `.worktrees/a`, diff `git diff main...phase-2/a` (commits `ef05323`, `eb209c7`).
Work order: `docs/phases/2/plan-a.md`. Spec sections 3 and 5.1. Exemplar: `freshbooks/identity.go`.
Read-only pass: no builds, tests, or `mise run check` were run.

## Verdict: REQUEST CHANGES

Six blocking findings, six advisory. The structural work is right -- every URL, HTTP method, ID family,
and response envelope in the batch matches the Postman inventory, the 51-key scope is exact, and the
test suite makes real assertions. The blocking findings are all in the *request bodies*: several write
paths serialize fields the caller never set, and two cannot express a documented request at all.

## What I verified as correct

- **Inventory scope is exact.** The `ignore.list` diff removes exactly 51 `//go:inventory-todo` lines:
  45 `Invoices/*` (the 4 `Other Income` + 1 `Upload Logo` lines batch d owns are untouched) plus the
  6 `Settings/Items and Services/*` item lines. No reordering, no comment churn.
- **Every path template matches `inventory.json`**, including the two easy-to-get-wrong splits:
  checkout-link CRUD is `/payments/account/{acct}/checkout-links` (hyphen) while its payment-options
  sibling is `/payments/account/{acct}/checkout_link/{id}/payment_options` (underscore); retainers are
  `/comments/business/{id}/retainers` for list/create and `/comments/business/{id}/retainer/{id}` for
  single. Both are reproduced faithfully.
- **ID families are right.** Retainers take `BusinessID`, everything else `AccountID`. No conversions.
- **Envelopes are right.** Accounting write bodies wrap in the singular resource key; the retainer
  business-family body wraps in `retainer` (confirmed against the Postman body, which does wrap --
  this is the exception to "business family is flat" and the batch got it right); checkout-link
  bodies are flat, matching Postman.
- **Duplicate-key stacking follows the exemplar.** `identity.go:86-87`'s stacked-comment shape is
  reproduced across `Items` (4 keys on `List`, 2 each on the rest), `Invoices.Create` (4), and
  `Invoices.ShareLink` (2). `Share PDF` really does carry `share_method=share_link` in Postman, so
  folding it into `ShareLink` is correct, not a guess.
- **Fixtures are synthetic and clean.** Only `ACM123` / `8675309`; no emails, URLs, or real IDs.
  `scripts/redaction-check.sh` reports clean.
- **Tests are not vacuous.** Real path/query/body assertions, `errors.Is` sentinel checks, no `t.Skip`,
  no committed `-run` filters, `[happy]/[sad]/[edge]` tags applied consistently.
- **Implementer ambiguities 1, 2, 3 are resolved correctly.** I checked each against `inventory.json`:
  `Single Invoice w/ Payment Gateway` is indeed a `POST` to the `{invoiceId}` URL carrying a full
  create body -- reading it as a Postman copy-paste artifact and folding it into `Create` is the
  right call. `Update`/`Update w/ Expense`/`Toggle Online Payments` are genuinely one operation.
  Retainer `Delete`/`Undelete` as separate methods matches the work order's Delete/Send precedent.

---

## BLOCKING

### 1. `omitempty` on struct-typed write fields is a no-op -- five write paths send junk

`freshbooks/invoices.go:29`, `:40`, `:243`; `freshbooks/items.go:120`; `freshbooks/payments.go:114`;
`freshbooks/invoice_profiles.go:174`

`encoding/json`'s `omitempty` only omits false, 0, nil pointers/interfaces, and empty
strings/slices/maps. It **never** omits a struct. Every one of these fields is a `Money` or `Date`
value (not a pointer), so the tag does nothing and the field is always serialized:

| Field | What is actually sent when the caller leaves it unset |
|---|---|
| `InvoiceCreateRequest.CreateDate` (`invoices.go:243`) | `"create_date": null` |
| `InvoiceProfileCreateRequest.CreateDate` (`invoice_profiles.go:174`) | `"create_date": null` |
| `PaymentCreateRequest.Date` (`payments.go:114`) | `"date": null` |
| `InvoiceLine.UnitCost` (`invoices.go:29`) | `"unit_cost": {"amount":"","code":""}` |
| `InvoiceLine.Amount` (`invoices.go:40`) | `"amount": {"amount":"","code":""}` |
| `ItemCreateRequest.UnitCost` (`items.go:120`) | `"unit_cost": {"amount":"","code":""}` |

`InvoiceLine.Amount`'s own doc comment says "the server fills it, callers never send it" -- but it is
sent on every single create and update, on every line.

Failure scenario: `c.Payments.Create(ctx, acct, &PaymentCreateRequest{InvoiceID: 90001, Amount:
Money{Amount:"10.00", Code:"USD"}, Type:"Check"})` posts
`{"payment":{"invoiceid":90001,"amount":{...},"date":null,"type":"Check"}}`. FreshBooks either
rejects the explicit `null` or coerces it -- neither is what the caller asked for, and this is a
money-recording path. Same shape for `Invoices.Create` with no `CreateDate`.

The existing tests cannot catch this: `payments_test.go`'s create case asserts only
`envelope["invoiceid"]`, and `invoices_test.go`'s asserts only `customerid`/`lines` -- neither asserts
the *absence* of an unset field.

Fix: switch these six tags to `omitzero` (Go 1.24+; this module targets Go >= 1.26 and
`freshbooks/auth/token.go:34` already uses it). `omitzero` honors `time.Time.IsZero` through the
embedded field for `Date`/`DateTime`, and `Money` is a comparable struct so it compares against the
zero value. Add one assertion per create test that the unset field key is absent from the body.

The same tag appears on read-only response structs (`items.go:31`, `invoice_profiles.go:65`,
`payments.go:35`, `invoices.go:111`, `:115`, `:116`) where it is harmless -- `omitempty` is
irrelevant when decoding. Only the six write-path fields above need changing. Pointer fields
(`*Money`, `*Date`) are already correct.

### 2. `UpdateCheckoutLinkGateway` cannot send the documented request body

`freshbooks/payments.go:250-256`

The only captured example for `Invoices/Payments/Update Checkout Link Payment Gateway` is:

```json
{"has_credit_card":true,"gateway_name":"stripe","entity_type":"checkout_link","entity_id":"9e963fcaf173401db907649f43ba77a4"}
```

The method reuses `PaymentOptionsRequest`, which has no `entity_type` or `entity_id` field, so those
two are never sent. The path already carries the link id, but `entity_id`/`entity_type` are the only
body fields FreshBooks documents for this endpoint and there is no evidence they are optional.

Failure scenario: `UpdateCheckoutLinkGateway(ctx, acct, "cl_1", &PaymentOptionsRequest{GatewayName:
"stripe", HasCreditCard: true})` posts a body missing both fields; the gateway is not attached and
the caller gets no error to distinguish that from success.

The test at `payments_test.go:267` asserts only `gotBody["gateway_name"] == "stripe"`, so it passes
either way -- a change-detector that cannot fail on this defect.

Fix: give the checkout-link gateway call its own request type (or add the two fields to a dedicated
struct) that sets `entity_type: "checkout_link"` and `entity_id: id` from the path argument, and
assert both in the test.

### 3. `PaymentOptionsRequest`'s bool fields cannot be turned off

`freshbooks/invoices.go:400-405`

```go
HasCreditCard        bool `json:"has_credit_card,omitempty"`
HasACHTransfer       bool `json:"has_ach_transfer,omitempty"`
AllowPartialPayments bool `json:"allow_partial_payments,omitempty"`
```

`omitempty` on a `bool` omits `false`. This is a settings-toggle endpoint: a caller who wants to
*disable* credit-card payments sets `HasCreditCard: false`, the field is dropped from the body, and
the server sees no instruction to change it. Every documented Postman example sends all three
explicitly.

Failure scenario: `EnablePaymentOptions(ctx, acct, id, &PaymentOptionsRequest{GatewayName:"fbpay",
HasACHTransfer:true})` intending ACH-only posts `{"gateway_name":"fbpay","has_ach_transfer":true}` --
credit card stays however it was, silently.

Fix: drop `omitempty` from the three bools so they always serialize (matching the Postman examples),
or make them `*bool` if tri-state is wanted. Add a test asserting a `false` survives into the body.

### 4. `InvoiceListOptions.Sort` is a public field that does nothing

`freshbooks/invoices.go:181`, `:186-201`, `:220`

`Sort` is declared on the options struct and copied through `All` at line 220, but `opts()` never
translates it into a `Sort(...)` request option -- only `Search`, `Page`, and `PerPage` are emitted.
Setting it has no effect on the request.

Failure scenario: `c.Invoices.List(ctx, acct, &InvoiceListOptions{Sort: "invoice_date"})` returns
server-default ordering with no error and no warning. Spec 5.1 lists `Sort(field, dir)` as a request
option "applied uniformly".

Compounding it: none of `ItemListOptions`, `PaymentListOptions`, `InvoiceProfileListOptions`, or
`RetainerListOptions` has a `Sort` field at all, so the public API is inconsistent across five
sibling types in the same batch.

Fix: pick one and apply it to all five -- either wire `Sort` through `opts()` (note `Sort(field,dir)`
takes a direction, so the field probably wants to be a `field`+`dir` pair or a pre-suffixed string)
and add it to the other four structs, or delete it from `InvoiceListOptions` and let callers pass
`Sort(...)` via the `extra ...RequestOption` variadic that every `List` already accepts. Deleting is
the smaller change and the variadic already covers the use case.

### 5. `RetainerUpdateRequest` silently deactivates a retainer on a partial update

`freshbooks/retainers.go:393-404`

```go
Fee                   float64 `json:"fee,omitempty"`
Active                bool    `json:"active"`     // no omitempty, not a pointer
```

`Active` has no `omitempty` and is not a pointer, so it is always sent -- as `false` unless the
caller remembers to set it. Every other field carries `omitempty`, which signals partial-update
semantics to the caller.

Failure scenario: `c.Retainers.Update(ctx, biz, id, &RetainerUpdateRequest{Fee: 750})` -- a caller
raising the fee -- posts `{"retainer":{"fee":750,"active":false}}` and deactivates the retainer.
That is silent data loss on a billing arrangement.

Every sibling write struct in this batch gets this right with pointers (`InvoiceUpdateRequest`,
`ItemUpdateRequest`, `PaymentUpdateRequest`, `InvoiceProfileUpdateRequest`), so this is also
convention drift within the batch.

Fix: make `Active *bool` (and drop `omitempty` from it, which a pointer makes redundant), matching
the four sibling update structs. If the endpoint is genuinely full-replace -- the doc comment says
"replaces a retainer's terms", and Postman does send every field -- then remove `omitempty` from
*all* fields instead and document it as a replace. Either is fine; the current mix of the two is the
bug. Add a test that a partial update does not carry `"active": false`.

### 6. `fetchRaw` drops the retry policy, and its doc comment does not say so

`freshbooks/invoices.go:446-467`

`fetchRaw` calls `c.roundTrip(ctx, req, fam, 1)` exactly once. It never loops, never consults
`c.retry`, and never honors `Retry-After`. `Invoices.PDF` is the only caller, so PDF downloads are
the one public method in the package with no retry on 429/502/503/504.

Spec 5.1's transport bullet is explicit that the client "retries 429/502/503/504 with exponential
backoff + jitter (default 3 attempts)" and "Honors `Retry-After`". PDF rendering is exactly the kind
of endpoint that returns 429 or 502 under load.

Failure scenario: a 429 with `Retry-After: 2` on `Invoices.PDF` returns `ErrRateLimited` immediately
instead of waiting and retrying, while the identical status on `Invoices.Get` retries transparently.
Callers cannot tell from the API surface that the two behave differently.

The doc comment claims it "shares the client's authorization, request building, and error decoding
with the JSON path, but skips `decodeBody`" -- accurate about what it shares, silent about the retry
loop it also skips. A reader takes it as a complete list.

Fix: rather than duplicating the retry loop, thread the decode step through the existing `do()` --
e.g. give `do` a variant that takes `out any` as either a decode target or a `*[]byte` raw sink, so
there stays a single request path as spec 5.1 requires. Failing that, at minimum reuse the same
retry loop and correct the doc comment to name every guarantee it drops.

Related, non-blocking: `fetchRaw` is a `*Client` transport method living in `invoices.go`. If it
survives as a separate helper it belongs in `transport.go` next to `do`, `resolve`, and `roundTrip`.

---

## ADVISORY

### 7. Doc comment references a method that does not exist

`freshbooks/invoices.go:162` -- "populated when `Include("allowed_gateways")` is used, or after
`Invoices.ToggleGateways` / `Invoices.EnablePaymentOptions`". There is no `ToggleGateways` method;
that operation was folded into `Update` (correctly). Fix: say `Invoices.Update` with
`AllowedGatewayIDs`.

### 8. Doc comment references a `doc.go` note that was never written

`freshbooks/payments.go:182-183` -- "see the package doc note in `doc.go` about INFERRED
payments-family facts". `freshbooks/doc.go` contains no such note. Either add it or drop the pointer;
a dangling cross-reference is worse than none, because a reader assumes the detail exists elsewhere.

### 9. Retainer money is carried as `float64`, against the package's own convention

`freshbooks/retainers.go:368-369`, `:397-398` -- `Fee float64`, `ExcessRate float64` on both write
structs. The package deliberately models money as `Money{Amount string}` precisely to avoid binary
floating point on currency, and the *response* type reads them back as `string` (`retainers.go:289`,
`:290`). Postman does send them as JSON numbers, so this is defensible for the wire, but a caller
computing a fee in `float64` before assigning is the classic precision bug this package's design
avoids everywhere else. Fix: take `string` (or `json.Number`) on the write structs and let callers
hand over an exact decimal; the wire form stays a JSON number either way if you use `json.Number`.

### 10. Checkout-link create/update silently echo the caller's own request

`freshbooks/payments.go:214-217`, `:231-234` -- when the response has no `checkout_link` key, the
methods return the *request* pointer. If the live API answers with a flat object (which is plausible;
there is no captured example either way), the decode into `checkoutLinkEnvelope` succeeds with a nil
inner pointer, and the caller receives their own input back with `ID` still empty -- indistinguishable
from success, and they can never learn the created link's id. The behavior is documented and tested,
so this is a deliberate choice rather than an oversight, but the failure mode is silent. Fix: decode
into both shapes (try the envelope, then the flat object) before falling back, or return a distinct
error when neither yields an `ID`.

### 11. Spec 5.1's `/payments/` envelope callout is unaddressed, and this batch is the one it names

Spec 5.1 states: "`/payments/` and `/uploads/` fall through to `business` and are **unverified in
either direction**. **The Phase 2 batch that implements each of these must confirm its envelope
live** and reclassify if needed." This batch implements six `/payments/` endpoints
(`Invoices.EnablePaymentOptions`, `InvoiceProfiles.EnablePaymentOptions`, and the four checkout-link
methods). Batch d owns `/uploads/`, not `/payments/`.

The implementer's report item 4 does the research honestly -- the `/api/online-payments` docs page
shows a flat `{"payment_options": {...}}`, consistent with the current `FamilyBusiness` default, so
no code change is needed -- but declines to write the callout to avoid racing batch d on the same
spec section. That is a reasonable coordination instinct, and I am flagging it for the lead rather
than the implementer: the finding (docs-level partial confirmation, still not live-CONFIRMED) needs
to land in spec 5.1 before Phase 2 closes, or it will be lost. Lead should assign it.

Same category, correctly handled: the business-family `field=value` filter encoding. Spec 5.1 says
"Phase 2's first business-scoped list endpoint must confirm it", and `Retainers.List` is that
endpoint. The new test asserts the query is `active=true` rather than `search[active]=true`, but that
asserts what the library does, not what FreshBooks accepts -- the Postman `Get all retainers` entry
carries `query: []`, so there is no evidence either way. Leaving the callout INFERRED is the right
call and the report says so plainly. No action beyond keeping the callout open.

### 12. `RetainersService` has no `All`, but `RetainerListOptions` offers `Page`/`PerPage`

`freshbooks/retainers.go:314-318`, `:343`. Every other service in the batch pairs `List` with `All`.
Retainers omits `All` because the Postman example shows no `meta` block -- which is documented and
reasonable -- yet the options struct still exposes pagination fields that then have nothing to
iterate. Fix: either drop `Page`/`PerPage` from `RetainerListOptions` until pagination is confirmed,
or add `All` for symmetry. Low stakes; pick whichever the lead prefers for consistency.

---

## Summary

| # | Severity | Finding | Location |
|---|---|---|---|
| 1 | BLOCKING | `omitempty` no-op on struct write fields sends `null` / empty `Money` | `invoices.go:29,40,243`; `items.go:120`; `payments.go:114`; `invoice_profiles.go:174` |
| 2 | BLOCKING | `UpdateCheckoutLinkGateway` omits required `entity_type`/`entity_id` | `payments.go:250-256` |
| 3 | BLOCKING | `PaymentOptionsRequest` bools cannot be set to false | `invoices.go:400-405` |
| 4 | BLOCKING | `InvoiceListOptions.Sort` is never encoded | `invoices.go:181,186-201` |
| 5 | BLOCKING | `RetainerUpdateRequest.Active` deactivates on partial update | `retainers.go:393-404` |
| 6 | BLOCKING | `fetchRaw` skips the retry policy; doc comment omits it | `invoices.go:446-467` |
| 7 | ADVISORY | Doc references nonexistent `Invoices.ToggleGateways` | `invoices.go:162` |
| 8 | ADVISORY | Doc references a `doc.go` note that does not exist | `payments.go:182-183` |
| 9 | ADVISORY | Retainer money as `float64` | `retainers.go:368-369,397-398` |
| 10 | ADVISORY | Checkout-link methods silently echo the request | `payments.go:214-217,231-234` |
| 11 | ADVISORY | Spec 5.1 `/payments/` envelope callout unwritten (lead to assign) | spec 5.1 |
| 12 | ADVISORY | `Retainers` has no `All` but exposes pagination options | `retainers.go:314-318` |

Findings 1, 3, and 5 share one root cause -- JSON tag semantics on optional write fields -- and are
worth fixing as a single pass with a body-shape assertion added to each affected create/update test.
