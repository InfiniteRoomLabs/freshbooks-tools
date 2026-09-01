# Phase 2 batch a -- QA / reality-check lane

**Verdict: NEEDS WORK** (2 BLOCKING, 8 ADVISORY)

Subject: branch `phase-2/a` in `.worktrees/a`, 5 commits ahead of `main` (`ef05323`, `eb209c7`, `08af234`, `df10186`, `27e9b77`). Oracle: spec sections 3 and 5.1, `GOAL.md` stage 2, `docs/phases/2/plan-a.md`, and the FreshBooks docs pages `/api/invoices`, `/api/items`, `/api/payments`, `/api/online-payments` (fetched and quoted directly, not from memory).

The gate is green and the fix commit is complete. Both blockers are wire-fidelity defects against the official docs that no existing test can catch, because every captured example that would expose them is `null`.

---

## 1. Gate and clean tree -- PASS

`mise run check` from inside the worktree, exit 0. Tail:

```
== inventory-check: freshbooks ==
implemented 55, ignored 0, todo 158, uncovered 0, double-covered 0, stale 0, unknown 0
coverage-gate: .../freshbooks/coverage.out total = 92.4% (floor 90%)   PASS
coverage-gate: .../mcp/coverage.out total = 100.0% (floor 90%)         PASS
coverage-gate: .../cli/coverage.out total = 100.0% (floor 90%)         PASS
== vuln: freshbooks/mcp/cli ==  No vulnerabilities found.
check.sh: all OK
```

Per-package: `freshbooks` 92.2%, `freshbooks/auth` 93.8%, `internal/inventory` 92.2%. `git status --porcelain` empty before and after my pass (I built a throwaway fidelity probe in `freshbooks/` and deleted it; re-verified empty). `scripts/redaction-check.sh`: clean. `go test -tags integration -race ./...` inside `freshbooks/`: ok.

**Inventory arithmetic checks out:** 55 implemented = 4 (Phase 1 Identity) + 51 (batch a). `uncovered 0, double-covered 0` means every stacked `// inventory:` comment resolves. `ignored 0` -- the ignore list contains no `ignore` directives at all, only 158 honest `todo` lines for batches b/c/d, so "ignore entries carry a written reason" is vacuously satisfied. `git diff main..HEAD -- .../ignore.list` removes exactly 51 lines.

## 2. GOAL.md stage 2 deliverables -- all met

| Deliverable | Met | Evidence |
|---|---|---|
| One `<resource>.go` + `_test.go` per service | Yes | invoices, invoice_profiles, items, payments, retainers -- 10 files |
| Spec 5.1 method vocabulary on pre-declared services | Yes | List/All/Get/Create/Update/Delete + Send, PDF, ShareLink, EnablePaymentOptions, Undelete. `client.go` untouched (services were pre-wired) |
| `// inventory:` comment per request | Yes | inventory-check `uncovered 0, double-covered 0` |
| `ignore.list` -- own lines only | Yes | 51 lines removed, no reordering, no other edits |
| Fixtures from docs examples, synthetic IDs | Yes | 9 fixtures; redaction-check clean; IDs are `ACM123`/`8675309`-style |
| Typed models with `json` tags | Yes | but see ADV-1/ADV-2 |
| Sad-path tests per family | Yes | see section 5 |
| Coverage >= 90% module-wide | Yes | 92.4% |
| `mise run check` green in worktree | Yes | section 1 |
| Conventional commits, no push/merge | Yes | `feat(freshbooks): ...` x2, `docs(phase-2): ...` x2, `fix(lib-resources-a): ...` |

## 3. The 17 triage fixes -- all 17 landed

Verified each in the tree, not just in the commit message. F1 `omitzero` on all six fields with per-family absence assertions (`invoices_test.go:150,154`, `items_test.go:142`, `payments_test.go:112`, `invoice_profiles_test.go:113`). F2 `entity_type`/`entity_id` asserted at `payments_test.go:301`. F3 explicit-`false` assertion at `invoices_test.go:402`. F4 `Sort` field gone (grep clean) + CHANGELOG line 53. F5 `*bool` + `[edge]` partial-update test, `retainers_test.go:169`. F6 `fetchRaw`+`attemptLoop` in `transport.go`, `Accept: application/pdf` and `%PDF-` magic-byte rejection asserted. F7/F8 `listOpts`/`newPage` in `page.go`, all five sites delegate. F9 `softDelete`, four callers. F10-F12 doc comments fixed. F13 `json.Number`. F14 `decodeCheckoutLink` errors instead of echoing, tested. F15 `Page`/`PerPage` gone + CHANGELOG line 55. F16 `pathSegment`/`noTraversal` + table tests (`transport_test.go:576,599`, `invoices_test.go:424`). F17 credential-equivalence doc comment on `InvoiceShareLink`.

**F16 is deliberately partial** -- see ADV-6.

---

## BLOCKING findings

### B-1. `Payment.TransactionID` is `*string`; FreshBooks documents `transactionid` as `int`

`freshbooks/payments.go:31`

Verbatim from https://www.freshbooks.com/api/payments Field Descriptions table:

```
transactionid
 | int
 | deprecated
```

Code: `TransactionID *string \`json:"transactionid,omitempty"\``

**Failure scenario:** a gateway-processed payment returns `"transactionid": 4457812`. `json.Unmarshal` fails with `cannot unmarshal number into Go struct field Payment.transactionid of type string`, and the error surfaces from `decodeBody` as `freshbooks: decoding the response: ...`. This takes down `Payments.Get`, `Payments.List`, `Payments.All`, `Payments.Create`, and `Payments.Update` -- the whole service -- for any account that has ever taken an online payment.

**Why no test catches it:** `transactionid` is `null` in all four captured examples (Postman single/create/update, docs single), so the fixtures round-trip through a nil pointer and prove nothing. This is exactly the "test that passes vacuously" the QA template asks about.

Expected: `*int64` (or `json.Number` if FreshBooks is inconsistent). Observed: `*string`.

Same class, worth checking while you are in there: `Payment.OrderID *string` and `Payment.Gateway *string` are also `null` in every example and untyped in the docs table (`gateway | string | the payment processor used, if any` -- so `Gateway` is fine; `orderid` has no table row at all, so it stays INFERRED).

### B-2. `Invoices.EnablePaymentOptions` omits `entity_type`/`entity_id`, which the docs require -- and F2 added them to the sibling endpoint

`freshbooks/invoices.go` (`EnablePaymentOptions`, `PaymentOptionsRequest`)

The **only** documented request body on https://www.freshbooks.com/api/online-payments for `POST /payments/account/<accountid>/invoice/<invoiceid>/payment_options` is, verbatim:

```json
{
 "gateway_name":"stripe",
 "entity_id": 2168250,
 "entity_type":"invoice",
 "has_credit_card":true
}
```

The code sends only `PaymentOptionsRequest` -- `gateway_name` plus three booleans, no entity fields.

This is not merely a gap, it is an **inconsistency introduced by the fix commit**. Triage F2 added exactly `entity_type`/`entity_id` to `UpdateCheckoutLinkGateway` (`checkoutLinkGatewayRequest`, `payments.go`) on the strength of one Postman example. The docs document the same two fields for the *invoice* variant of the same endpoint family, and that call site was left alone. The two sibling calls now disagree about the endpoint's contract.

The Postman example for `Invoices/Enable Payment Options On Invoice` does **not** carry them (`{"has_credit_card": true, "has_ach_transfer": true, "allow_partial_payments": true, "gateway_name": "fbpay"}`), so docs and Postman genuinely conflict here. Per `docs/phases/2/plan-a.md` ("If the docs disagree with the spec, the docs win") and `CLAUDE.md` ("the API wins ... add a `> **STATE AS OF YYYY-MM-DD**` callout in the affected spec section in the same commit"), this needed either the fields or a recorded callout. Neither happened: `git diff --name-only main..HEAD -- docs/superpowers/` is empty.

Same applies to `InvoiceProfiles.EnablePaymentOptions`, which shares `PaymentOptionsRequest`.

**Recommended fix:** carry `entity_type`/`entity_id` on both invoice and invoice-profile payment-options calls, built from the path argument exactly as F2 does for checkout links (sent as an int for invoices -- the docs send `2168250` unquoted -- and note the response returns it quoted as `"5370347"`, so a read model must not reuse the write type). Plus one `STATE AS OF 2026-09-01` line recording the docs-vs-Postman conflict.

---

## ADVISORY findings

### ADV-1. `Invoice` silently drops 12 documented response fields, including `uuid` and `version`

Measured, not eyeballed: I decoded FreshBooks' own captured responses through the library's types and compared every response key against the struct's `json` tags (throwaway reflection probe, since deleted).

```
invoice(DOCS)     : 12 of 65 response keys have no struct field:
    autobill_status basecampid deposit_percentage deposit_status dispute_status
    ext_archive gmail last_order_status net_paid_amount sentid uuid version
invoice(postman)  :  9 of 62   (same minus net_paid_amount, uuid, version)
invoice_profile   :  2 of 44   [ext_archive total_accrued_revenue]
item(postman)     :  0 of 12   -- complete
payment(postman)  :  0 of 17   -- complete
retainer(postman) :  0 of 13   -- complete
```

Items, payments and retainers are clean, which is good work. For invoices, `version` (`"2021-07-05 08:17:47.399872"`) and `uuid` are the ones that matter -- `version` looks like an optimistic-concurrency token, and without it a caller cannot do a safe conditional update. `deposit_status`, `deposit_percentage` and `dispute_status` are user-visible invoice state. Silent drops are invisible at compile time and survive every round-trip test.

### ADV-2. `Item.Tax1`/`Tax2` are `float64`; the docs type them `int`, and they are tax **IDs**, not rates

`freshbooks/items.go`. Docs Field Descriptions, verbatim: `tax1 | int | id of default tax for the item`. The example value is `58730`. Modelling an identifier as a binary float is the same class of mistake F13 fixed for retainer money. Decoding happens to work; the type is wrong and invites arithmetic on an id.

### ADV-3. `PaymentOptionsRequest` is missing three documented booleans

The docs field table for payment options types six booleans: `has_credit_card`, `has_ach_transfer`, `has_bacs_debit`, `has_sepa_debit`, `has_paypal_smart_checkout`, `allow_partial_payments`. The struct carries three. Given F3's own reasoning ("an omitted field means leave it as it is"), omitting the other three is safe rather than destructive -- but UK/EU debit and PayPal cannot be enabled through this library at all.

### ADV-4. `EnablePaymentOptions` discards the response body

Both invoice and invoice-profile variants pass `nil` as `out`. The docs show a flat `{"payment_options": {...}}` carrying the applied state plus a `gateway_info` object. Callers get no confirmation of what was actually enabled. Returning a typed result would also give B-2's `entity_id` string-vs-int asymmetry a place to be handled honestly.

### ADV-5. Two more unrecorded docs-vs-Postman conflicts

The implementer report's section "Spec discrepancies" is thorough and its reasoning for deferring the `/payments/` envelope callout to batch d (avoiding two batches racing on one spec section) is sound -- I am not re-litigating that. But two conflicts it had the evidence for went unrecorded:

1. **Invoice delete verb.** https://www.freshbooks.com/api/invoices "Delete Invoice" is `curl -L -X DELETE '.../invoices/invoices/<invoiceId>'` carrying `{"invoice":{"vis_state":1}}`. The Postman inventory says `PUT` with the identical body, and the code uses `PUT` via `softDelete`. Items and payments both genuinely use `PUT`; invoices are documented as the outlier. I lean toward `PUT` being right (a DELETE with a body is odd, and Postman is captured traffic), so this is advisory rather than blocking -- but it is a live docs conflict on a destructive operation and belongs in a `STATE AS OF` note or the live-conformance backlog.
2. **`entity_type` singular vs plural.** The docs field table describes it as `Eg. "invoices"` while every request, response, and query example on the same page uses singular `"invoice"`. F2 hard-codes `"checkout_link"` singular, which matches the examples. Fine, worth a comment.

### ADV-6. F16 was applied partially, by a defensible reading, but the deviation is unrecorded

Triage F16 asked for a `resolve` guard rejecting `.`/`..` segments **"or whose `ref.RawQuery` is non-empty when the path was built from caller-supplied IDs"**. Only the traversal half exists (`noTraversal`). The RawQuery half was correctly *not* implemented -- `Invoices.ShareLink` deliberately folds `?share_method=share_link` into the path string, and I verified `resolve` preserves it (`q := ref.Query()` then merges option values before `u.RawQuery = q.Encode()`), so a blanket rejection would have broken that method.

The security property still holds by a different route: `pathSegment` rejects `/`, `?`, `#`, and `..` in every caller-supplied string before interpolation, so a caller-supplied ID can never introduce a query in the first place. I traced every path builder in the batch and found no gap (retainers interpolate `BusinessID`, an int64). This is the right call -- it just needs a line saying so, otherwise the next reader sees F16 as half-done.

### ADV-7. The retainer list fixture invents a `meta` block the API is not known to send

`freshbooks/testdata/retainers/list.json` ends with `"meta": {"page":1,"pages":1,"per_page":15,"total":2}`. The captured Postman response for `Invoices/Retainers/Get all retainers` has no `meta` block -- I checked the raw response body, and its absence is precisely the stated justification for F15 dropping `Page`/`PerPage`. `RetainersService.List`'s own doc comment says "Meta on the returned page is therefore zero-valued unless the live API sends it", which the fixture then contradicts. The test asserting the meta block decodes is asserting behaviour no evidence supports. Either drop the block from the fixture, or keep it and say in the comment that it is forward-looking.

### ADV-8. Nothing proves the PDF path actually retries

F6's substance -- `fetchRaw` sharing `attemptLoop` with `do` -- is structurally correct and I verified the shared code path by reading it. But `TestInvoicesPDF` covers only happy, 404, and non-PDF-body. No test drives a 429-then-200 through `Invoices.PDF`. The refactor is the kind that a later "simplify" pass could quietly unpick. One test asserting two attempts and a honoured `Retry-After` on the PDF path would lock it in.

**Also worth a line to the lead:** the implementer report at `docs/phases/2/reports/a-implementer.md:74` is now stale -- it says `CreateCheckoutLink`/`UpdateCheckoutLink` "fall back to echoing the request back to the caller". F14 changed exactly that to return an error. Expected (the report predates the fix commit), but do not act on that sentence.

---

## 4. Seams -- adequate for this batch, with a caveat

`go test -tags integration -race ./...` inside `freshbooks/` passes. The three integration tests (`TestExpiryRefreshWriteBackRetry`, `TestFileStoreSurvivesProcessRestart`, `TestAllAcrossPagesWithMidStreamRateLimit`) are all Phase 1 seams; batch a added none and its work order promised none, so this is not a miss. `TestAllAcrossPagesWithMidStreamRateLimit` covers the generic `All` iterator including error-stops-iteration, which is the seam the five new `All` wrappers depend on; each new wrapper additionally has its own `[happy] iterates every X once` unit test.

## 5. Test quality -- genuinely good

93 subtests across the five new files plus 31 in `transport_test.go`. No `t.Skip` outside `live_test.go`'s env guard (which is correct and documented). No committed `-run` filters. `-race` throughout.

The sad-path coverage is real rather than decorative: 12 nil-request guards, 9 `ErrNotFound` propagations, a 422 with field errors (`[sad] an invalid create_date is a validation error`), a business-family bare-string 404 (`[sad] a 404 is the business family's bare error string` -- correctly distinguishing the two error shapes), a 422 on an unsent invoice, and `[sad] a non-2xx status decodes as an API error, not raw bytes` on the PDF path. Transport-level 401/429-with-`Retry-After`/malformed-JSON/cancelled-context are covered in `transport_test.go` from Phase 1 and still pass.

The `[edge]` tests are the tell that the fix commit was understood rather than pattern-matched: `an unset UnitCost is omitted, not sent as an empty Money`, `a partial update (Active unset) does not carry active: false`, `a false toggle survives into the body, not dropped by omitempty`, `a nil request still marks the invoice sent`. These assert the *absence* of keys, which is the only way to test `omitzero` honestly.

Fixture values spot-checked against source: retainer `fee`/`excess_rate` are quoted decimal strings on read (`"600.00"`) while the write path sends bare JSON numbers -- I confirmed this asymmetry is real in FreshBooks' own captured responses, so `Retainer.Fee string` + `RetainerCreateRequest.Fee json.Number` is correct, not a mistake. `Money` as `{amount, code}` is confirmed by both docs and Postman for invoice totals, line items, item unit costs, and payments. Item `qty`/`inventory` as decimal strings match the docs type column.

## 6. Fidelity spot-checks -- 8 inventory entries hand-verified against the docs

| Inventory key | Expected (docs/Postman) | Observed (code) | |
|---|---|---|---|
| `Invoices/List Invoices` | `GET /accounting/account/{id}/invoices/invoices` | `invoicesPath` + `do(GET, FamilyAccounting)` | OK |
| `Invoices/Single Invoice w/ Logo` | same URL + `include[]=presentation` | `Get` + `Include("presentation")`, asserted same call | OK |
| `Invoices/Delete  Invoice` | body `{"invoice":{"vis_state":1}}` | `softDelete(path,"invoice")` | OK, verb per ADV-5 |
| `Invoices/Send Invoice by Email` | `"action_email": true` + `email_recipients` + `invoice_customized_email` | `invoiceSendBody{ActionEmail:true}` | OK -- `action_email` confirmed required by docs prose |
| `Invoices/Invoice Links/Downloads/Share Link` | `GET .../share_link?share_method=share_link` | path + raw query; `resolve` preserves it | OK |
| `Invoices/Items and Services/List Items Filtered by SKU` | `search[sku]={sku}` | `Search{"sku":...}` via accounting encoding | OK |
| `Invoices/Payments/Make Payment` | `{"payment":{"invoiceid":..,"amount":{"amount":".."},"date":..,"type":..}}` | `paymentWriteEnvelope` + `PaymentCreateRequest` | OK |
| `Invoices/Enable Payment Options On Invoice` | docs body carries `entity_id`+`entity_type` | omitted | **B-2** |

Plus the machine-checked decode of five captured responses in ADV-1.

---

## Verdict

**NEEDS WORK.** The gate is green, all 17 triage fixes landed with real tests, the inventory balances, and the test suite is honest work -- this is a good batch. But B-1 is a latent decode failure that takes out the entire `PaymentsService` for any account with a real online payment, and it is invisible to every existing test because the field is `null` in all four captured examples. B-2 leaves two sibling calls to the same endpoint family disagreeing about that endpoint's contract, in the same commit that created the disagreement.

Both are small fixes. Suggested second fix commit: type `transactionid` correctly, carry `entity_type`/`entity_id` on both `EnablePaymentOptions` methods, and add the `STATE AS OF 2026-09-01` line covering the payment-options body conflict and the invoice delete-verb conflict. ADV-1 (`uuid`/`version` at minimum), ADV-2, and ADV-7 are cheap enough to fold in; the rest can ride to the live-conformance pass.

## Commands run

```
mise run check                                    # exit 0, from .worktrees/a
git status --porcelain                            # empty, before and after
scripts/redaction-check.sh                        # clean
mise exec -- go test -tags integration -race ./...  # ok (from freshbooks/)
mise exec -- go test -run TestZZQAFidelity -v .   # throwaway probe, file deleted
curl the four /api/ docs pages; grep the raw HTML for transactionid, Delete Invoice,
  payment_options bodies, field-description type columns
python3 walks of internal/inventory/testdata/inventory.json for the 51 in-scope keys
  (method, host, pathTemplate, query, body, captured responses)
```

---

# Re-verification -- 2026-09-01, after `9032def`

**Verdict: PASS.**

Subject: `9032def fix(lib-resources-a): apply the QA findings`, the sixth commit on `phase-2/a`. Focused delta pass per team-lead: both blockers, the new spec callout, one gate run. I did not redo the full fidelity sweep.

## Both blockers closed

**B-1 -- CLOSED.** `freshbooks/payments.go:36` is now `TransactionID *int64` with a doc comment recording that it was INFERRED `*string` from the all-`null` examples until the docs contradicted it. The test that was missing now exists: `[happy] a non-null numeric transactionid decodes as *int64` feeds `"transactionid": 4457812` through `Payments.Get` and asserts `*p.TransactionID == 4457812`. That is precisely the input that would have failed before, so the fix is pinned rather than merely applied.

**B-2 -- CLOSED, and better than I asked for.** A new unexported `paymentOptionsBody` embeds `PaymentOptionsRequest` and adds `entity_type` + `entity_id`; both `Invoices.EnablePaymentOptions` (`entity_type: "invoice"`) and `InvoiceProfiles.EnablePaymentOptions` (`entity_type: "invoice_profile"`) build it from the path argument. `entity_id` is `int64`, matching the docs' bare-number example, and the doc comment explicitly warns that the response echoes it back quoted so a read model must not reuse the write type -- the asymmetry I flagged is recorded at the point of use, not just in the spec. The invoice-profile `entity_type` value is correctly marked INFERRED by analogy (no docs or Postman evidence exists for it).

Tests assert the body on both paths: `gotBody["entity_type"] != "invoice" || gotBody["entity_id"] != float64(90001)` and the `"invoice_profile"` / `700` equivalent.

The inconsistency that made B-2 blocking is also resolved from the other side: `checkoutLinkGatewayRequest` now embeds `PaymentOptionsRequest` too, so the checkout-link gateway call carries the full boolean set instead of a hand-copied subset of four. The two sibling calls now agree on the endpoint's contract, which was the actual defect.

## Spec callout -- good

`docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` section 3 gains a `> **STATE AS OF 2026-09-01 (Phase 2 batch a, QA pass):**` block covering all three conflicts I raised, each with its resolution and reasoning: the payment-options body (docs win, fields now sent), the invoice delete verb (keeps `PUT`, with the argument stated -- a `DELETE` with a body is unusual, Postman is captured traffic, and items/payments agree on `PUT` -- and explicitly deferred to the live-conformance pass rather than changed on docs prose alone), and `entity_type` singular-vs-plural (examples beat the field-table gloss). I disagreed with none of it; the delete-verb call is the one I would have made, and it is now recorded instead of silent.

`docs/phases/2/triage-a.md` also gains an "F16 deviation record" section documenting why the RawQuery half was deliberately not implemented. That closes ADV-6 as a bookkeeping matter.

## Other advisories

| | Status | Evidence |
|---|---|---|
| ADV-1 | Closed | 12 fields added (`uuid`, `version`, `net_paid_amount`, `sentid`, `gmail`, `basecampid`, `ext_archive`, and the four `*_status` + `deposit_percentage`). Re-ran my reflection check: **invoice(DOCS) 0 of 65 unmodelled, invoice(postman) 0 of 62, invoice_profile complete.** Test asserts `UUID` and `Version` decode |
| ADV-2 | Closed | `Item.Tax1`/`Tax2` are `*int64` with a comment saying they are tax ids, not rates |
| ADV-3 | Closed | `PaymentOptionsRequest` carries all six documented booleans; reaches the checkout-link call too via embedding |
| ADV-4 | Deferred | Per team-lead, rides to live-conformance |
| ADV-5 | Closed | Both conflicts in the spec callout |
| ADV-6 | Closed | Deviation recorded in `triage-a.md` |
| ADV-7 | Closed | The invented `meta` block is gone from `testdata/retainers/list.json` |
| ADV-8 | Closed | New test `[happy] a 429 with Retry-After is retried, proving PDF shares do's retry loop` |

## Gate

`mise run check` from the worktree. **Every substantive step green in all three modules:**

```
== lint: freshbooks ==            0 issues.
== test: freshbooks ==            ok  freshbooks 92.2% | auth 93.8% | internal/inventory 92.2%
== cover: freshbooks ==           total = 92.4% (floor 90%)  coverage-gate: PASS
== vuln: freshbooks ==            No vulnerabilities found.
== inventory-check: freshbooks == implemented 55, ignored 0, todo 158, uncovered 0,
                                  double-covered 0, stale 0, unknown 0
== cover: mcp ==   100.0%  PASS      == cover: cli ==  100.0%  PASS
== vuln: mcp/cli ==  No vulnerabilities found.        == actionlint ==  (clean)
```

The run exits 1 on its final step only:

```
DIRTY TREE:
?? docs/phases/2/reports/a-qa.md
[check] ERROR task failed
```

That is this report, the one allowed dirty file, exactly as team-lead predicted. `git status --porcelain` shows nothing else. Coverage and inventory are unchanged from the pre-fix run despite ~220 lines added, so the new code carries its own tests.

**Process note worth recording:** `mise run check 2>&1 | tail -N` reports *`tail`'s* exit code, not `check.sh`'s. My first-pass "exit 0" was genuine (the tree was clean at that point), but the pattern will happily report a green gate over a red one. Use `mise run check > log 2>&1; echo $?`, or set `pipefail`.

## Nit carried forward (not blocking, no action needed now)

Several always-present read-only integers on `Invoice` -- `basecampid`, `ext_archive`, `sentid`, and `deposit_status` -- carry `omitempty` while their doc comments say FreshBooks always sends them. The tag only affects re-marshalling a decoded `Invoice`, which this library never does, so it is cosmetic today. It would become visible if the CLI or MCP server ever round-trips an `Invoice` to JSON output, where a legitimate `0`/`""` would vanish. Worth a look when Phase 3/4 wires the output layer.

## Verdict

**PASS.** Both blockers are fixed at the wire level and pinned by tests that would fail against the old code. Seven of eight advisories are closed, the eighth deferred by agreement. The spec callout is honest about what is CONFIRMED, what is INFERRED, and what is deferred to live conformance. `phase-2/a` is ready to merge.
