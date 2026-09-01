# Phase 2 batch b -- QA / reality-check lane

**Verdict: PASS** as of the 2026-09-01 re-verification of `9147ade` (see the final section).

First pass (HEAD `e6c65a3`): **NEEDS WORK** (3 BLOCKING, 9 ADVISORY). Everything between here and the
re-verification section is that first pass, left as written.

Subject: branch `phase-2/b` in `.worktrees/b`, HEAD `e6c65a3` (the fix commit answering `docs/phases/2/triage-b.md`), 8 commits ahead of `main`. Oracle: spec sections 3 and 5.1, `GOAL.md` stage 2, `docs/phases/2/plan-b.md`, the FreshBooks docs pages `/api/clients`, `/api/estimates`, `/api/expenses`, `/api/bills`, `/api/vendors`, `/api/taxes` (fetched during this pass, not recalled), and -- the decisive evidence -- FreshBooks' **own captured responses inside this repo** at `freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json`.

The gate is green, all fifteen triage fixes landed, the inventory balances exactly, and the test suite is honest work. But three read models cannot decode FreshBooks' real responses, and I proved it mechanically rather than by eyeballing types: I decoded all 40 captured batch-b responses in the collection through the library's own structs. Four of them fail outright. **One of the three defects was introduced by the fix commit itself (F6).**

---

## 1. Gate and clean tree -- PASS

`mise run check` from inside `.worktrees/b`, run once, exit code captured directly (not through a pipe): **exit 0**. Tail:

```
== inventory-check: freshbooks ==
implemented 114, ignored 0, todo 99, uncovered 0, double-covered 0, stale 0, unknown 0
coverage-gate: .../freshbooks/coverage.out total = 91.3% (floor 90%)   PASS
coverage-gate: .../mcp/coverage.out       total = 100.0% (floor 90%)   PASS
coverage-gate: .../cli/coverage.out       total = 100.0% (floor 90%)   PASS
== vuln: freshbooks / mcp / cli ==   No vulnerabilities found.
== actionlint ==
== build ==   12 artifacts, dist/
check.sh: all OK
EXIT_CODE=0
```

Per-package: `freshbooks` 90.6%, `freshbooks/auth` 93.8%, `internal/inventory` 92.2%.

`git status --porcelain` empty before I started and empty after (I built a throwaway reflection probe at `freshbooks/zz_qa_probe_test.go`, ran it, and deleted it; re-verified empty). `scripts/redaction-check.sh`: `redaction-check: clean`.

**Inventory arithmetic checks out:** 114 implemented = 4 (Phase 1 Identity) + 51 (batch a, merged) + **59 (batch b)**, exactly the plan's number. `uncovered 0, double-covered 0` means every stacked `// inventory:` comment resolves, including the three-way tax stacks. `git diff main..HEAD -- .../ignore.list` removes exactly **59** `//go:inventory-todo` lines and adds **0** lines -- no other line touched, so the file will merge cleanly against c and d. `ignored 0`: the ignore list holds no `ignore` directives at all, only honest `todo` lines, so "ignore entries carry a written reason" is vacuously satisfied.

## 2. GOAL.md stage 2 deliverables -- all met

| Deliverable | Met | Evidence |
|---|---|---|
| One `<resource>.go` + `_test.go` per service | Yes | clients, contacts, credit_notes, estimates, expenses, expense_categories, taxes, bills, bill_payments, bill_vendors -- 20 files |
| Spec 5.1 method vocabulary on pre-declared services | Yes | List/All/Get/Create/Update/Delete plus `Accept`, `Send`, `Archive`, `Summaries`, `Vendors`, `CreateRecurring`, `RemoveAllSecondaryContacts` |
| `// inventory:` comment per request | Yes | inventory-check `uncovered 0, double-covered 0`; 59 keys |
| `ignore.list` -- own lines only | Yes | 59 removals, 0 additions, no reordering |
| Fixtures from docs examples, synthetic IDs | Partly | 39 fixtures, redaction-check clean, IDs synthetic -- **but see B-1/B-2/B-3: three fixtures were authored to match the struct rather than taken from a real response** |
| Typed models with `json` tags, pointers for optional writes | Yes | F3/F4/F5/F14 all landed |
| Sad-path tests per family | Yes | section 5 |
| Coverage >= 90% module-wide | Yes | 91.3% |
| `mise run check` green in worktree, clean tree | Yes | section 1 |
| Conventional commits, no push/merge | Yes | `feat(freshbooks): ...` x5, `docs(phase-2): ...` x2, `fix(lib-resources-b): ...`; `git log origin/main..` shows nothing pushed |

## 3. The fifteen triage fixes -- all fifteen landed

Verified in the tree, not just in the commit message.

- **F1/S1** -- every one of the ten files has a `pathSegment`-guarded builder (`expenses.go` has four, the three inline `fmt.Sprintf` paths folded in as the triage asked). All ten `_test.go` files carry the hostile-`AccountID` table (`[sad] a path separator`, `[sad] a query delimiter`, `[sad] a fragment delimiter`), asserting an error with no HTTP request.
- **F2** -- `omitzero` on every struct-typed write field: credit_notes 5, estimates 6, bills 11, expenses 4.
- **F3** -- `BillVendorRequest.Is1099 *bool` (`bill_vendors.go:96`), `BillLineRequest.CompoundedTax *bool` (`bills.go:91`), `CreditNoteLine.CompoundedTax *bool` (`credit_notes.go:33`).
- **F4/F15** -- `ExpenseWriteRequest.CategoryID *int64` (`expenses.go:105`), `.ProjectID *int64` (`:107`), `.TaxPercent1/2 *string` (`:115-116`), `.MarkupPercent *string` (`:117`), `ExpenseProfileCreateRequest.CategoryID *int64` (`:365`), `TaxCreateRequest.Compound *bool` (`taxes.go:53`).
- **F5** -- all 12 fields present on `ClientWriteRequest` (`clients.go:105-138`), shipping address exercised in the create fixture and test.
- **F6** -- `BillVendorTaxDefault` exists with `taxid`/`name`/`amount`, list fixture has a non-empty `tax_defaults`, `BillLine.TaxAmount1/2` added. **`BillVendor.OutstandingBalance *Money` also added -- and it is wrong; see B-2.**
- **F7** -- exactly the fifteen fields the triage named are now `DateTime`: clients 4, estimates 3, expenses 1, expense_categories 2, taxes 1, bills 2, bill_vendors 2.
- **F8** -- body-assertion subtests present per write method; `bill_vendors_test.go:51` now serves `bill_vendors_list_page2` on the second page, so the `All` test is no longer vacuous.
- **F9/F10/F11/F12** -- `listOpts` / `newPage` / `softDelete` / `defaultPerPage` delegation across all eight list-bearing services; `bills.go` keeps `visStatePut` with the reasoning written into the comment, as the triage decided.
- **F13** -- the estimates delete-verb claim is gone from `estimates.go` (only the honest "models this as a PUT, matching the FreshBooks..." comment remains) and the bullet is gone from the section 3 callout, replaced by an explicit parenthetical recording that the claim was withdrawn. Exactly what F13 asked for.
- **F14** -- `List(..., extra ...RequestOption)` on all eight list services; `Get(..., opts ...RequestOption)` on the five services that have a `Get` (credit notes, bills, bill vendors and bill payments have no single-get inventory key, correctly).

---

## BLOCKING findings

All three are latent decode failures. All three are invisible to every existing test because the fixture that would expose them was **authored to match the struct** instead of taken from FreshBooks' captured response -- the exact `transactionid` class batch a's QA flagged, but worse: here the contradicting evidence is sitting in this repo.

**How I found them.** Throwaway probe (`freshbooks/zz_qa_probe_test.go`, since deleted): extract all 40 captured response bodies for the `Clients`, `Expenses`, `Estimates`, and `Accounting/Taxes` folders out of `freshbooks.postman_collection.json`, unwrap `response.result`, and `json.Unmarshal` each one into the library's own type. Four fail. Reproduce it in ten lines if you want to see it.

### B-1. `BillLine.Quantity` is `int`; FreshBooks returns `quantity` as a **quoted string** in every response

`freshbooks/bills.go:18`

Two independent sources agree:

Captured, `Expenses/Bills (Beta)/Add Bill from Vendor` response, `freshbooks.postman_collection.json`:

```json
{"amount":{"amount":"125.00","code":"USD"}, "description":"Malm Side Table",
 "id":13, "list_index":1, "quantity":"1", "unit_cost":{"amount":"125.00","code":"USD"}}
```

Docs, https://www.freshbooks.com/api/bills, Get Single Bill response:

```json
"lines":[{"description":"Raw material","id":2621,"list_index":1,"quantity":"15", ...}]
```

The `/api/bills` field table types `quantity` as `integer`, and the **request** genuinely sends it unquoted (`"quantity": 40` in the docs' Create example, `"quantity": 1` in Postman's). The **response** is quoted in all five documented examples and both captured ones. Encode and decode disagree, and only the decode side is broken.

**Failure scenario:** any bill that has a line -- i.e. every real bill -- fails to decode. `json.Unmarshal` returns `cannot unmarshal string into Go struct field BillLine.lines.quantity of type int`, surfacing from `decodeBody` as `freshbooks: decoding the response: ...`. That kills `Bills.List`, `Bills.All`, `Bills.Create`, `Bills.Delete`, and `Bills.Archive` -- the whole service.

**Why no test catches it:** `bills_list.json`, `bills_archive.json` and `bills_delete.json` all have `"lines": []`, and the one fixture that has a line, `bills_create.json`, was written with `"quantity": 3` **unquoted**. The read model `BillLine` therefore has effectively zero decode coverage against a real response.

Expected: `Quantity string` on `BillLine` (read). Observed: `int`. `BillLineRequest.Quantity int` (write) is correct as it stands -- do not change it; the asymmetry is real, like the retainer `Fee` asymmetry batch a confirmed.

### B-2. `BillVendor.OutstandingBalance *Money` cannot decode the array FreshBooks actually sends -- and F6 added this field

`freshbooks/bill_vendors.go:59`

Captured, `Expenses/Vendors (Beta)/Add Vendor` response:

```json
"outstanding_balance": [],
"overdue_balance": []
```

Captured, `Expenses/Bills (Beta)/Get Bills` response (a vendor with a balance):

```json
"outstanding_balance": [{"amount": {"amount": "5.00", "code": "CAD"}}]
```

Docs, https://www.freshbooks.com/api/vendors, Get Vendors list -- three vendor rows, same shape:

```json
"outstanding_balance": [{"amount": {"amount": "53885.00", "code": "USD"}}]
```

The docs field table calls it an `object` with `amount`/`code` subfields. It is not. It is an **array**, whose elements are wrapper objects whose own `amount` key is the `{amount, code}` money object -- two levels of nesting the current type does not have.

**Failure scenario:** every read fails, including the empty case. `json.Unmarshal([]byte("[]"), &Money{})` returns `cannot unmarshal array into Go struct field BillVendor.outstanding_balance of type freshbooks.Money`. That takes down `BillVendors.List`, `.All`, `.Create`, `.Update` and `.Delete` -- the entire service -- for **every** vendor, balance or no balance. It is not a rare-data edge; it is unconditional.

**This is a regression introduced by the fix commit.** Before F6 the field did not exist and the service decoded fine (`outstanding_balance` was simply dropped). F6 added it, and the accompanying fixture `bill_vendors_list.json` was written as `"outstanding_balance": {"amount":"375.00","code":"USD"}` -- an object, matching the new struct, contradicting four captured/documented examples the implementer had access to. The fixture is not evidence; it is the assumption restated.

Expected shape:

```go
// OutstandingBalanceEntry wraps one currency's balance.
type VendorBalance struct { Amount Money `json:"amount"` }
OutstandingBalance []VendorBalance `json:"outstanding_balance,omitempty"`
OverdueBalance     []VendorBalance `json:"overdue_balance,omitempty"`
```

Fix the fixture from the captured response at the same time, and add `overdue_balance` while you are there (B-3).

### B-3. `ExpenseAttachment.AttachmentID`/`ID` are `string`; FreshBooks returns numbers

`freshbooks/expenses.go:16-17`

Captured, `Expenses/List Expenses` (and `Create Expense`, and `Create Expense with Receipt`):

```json
"attachment": {"attachmentid": 8668, "id": 8668,
               "jwt": "eyJ0eXAiOiJKV1Qi...", "media_type": "image/png"}
```

Four captured expenses across three requests, every one an unquoted integer.

**Failure scenario:** `cannot unmarshal number into Go struct field ExpenseAttachment.attachment.attachmentid of type string`. Any expense with a receipt kills `Expenses.Get`, and one such expense anywhere in a page kills `Expenses.List`/`.All` for that page. Receipts are a headline FreshBooks feature; this is not a corner.

**Why no test catches it:** not one of the seven expense fixtures has an `attachment` key at all. The struct is entirely unexercised on decode.

Expected: `AttachmentID int64` and `ID int64` (`json:"attachmentid"` / `json:"id"`), with `ExpenseAttachmentRequest` keeping its JWT-only write shape. Add the captured attachment to `expenses_get.json` so the decode is actually exercised.

The triage deferred "review 10's INFERRED items (`Expense.Attachment` ...)" to the live-conformance pass on the grounds that the shape was inferred. It is not inferred -- it is captured, in this repo, and disproven.

---

## ADVISORY findings

### ADV-1. The read models silently drop 40 distinct response fields FreshBooks actually sends

Measured, not eyeballed -- same probe, comparing every key in each captured response against the struct's `json` tags:

```
client        (4 captures) : 7 dropped  [direct_link_token has_retainer level notified
                                         retainer_id statement_token subdomain]
                             +docs adds [allow_email_include_pdf exceeds_client_limit]
credit_note   (2 captures) : 6 dropped  [accounting_systemid current_organization
                                         dispute_status ext_archive last_order_status sentid]
estimate      (6 captures) : 5 dropped  [accounting_systemid address current_organization
                                         ext_archive sentid]   (+docs: accountid, ownerid is modelled)
expense       (4 captures) :14 dropped  [account_name accountid accounting_systemid
                                         background_jobid bank_name category expense_profile
                                         ext_invoiceid ext_systemid from_bulk_import isduplicate
                                         profileid project transactionid]
bill          (5 captures) : 3 dropped  [attachment overall_category overall_description]
bill_vendor   (4 captures) : 1 dropped  [overdue_balance]
tax, expense_category, bill_payment, expense_profile, expense_summary : 0 dropped -- complete
```

Taxes, categories, bill payments and summaries are clean, which is good work. The ones that matter:

- **`Expense.transactionid`** -- the exact field batch a's B-1 was about, dropped rather than mistyped this time. Docs type it `int`. It is the bank-transaction link.
- **`Expense.isduplicate`, `from_bulk_import`, `status`-adjacent `profileid`** -- user-visible expense provenance.
- **`Expense.category` / `Bill line.category`** -- a full nested category object the API sends alongside `categoryid`. `BillLine.CategoryID int64` is worth a look: the captured bill line has **no `categoryid` key at all**, only the nested `category` object, so `BillLine.CategoryID` is always zero on read today.
- **`Bill.attachment`** -- bills can carry a receipt; the library cannot see it.
- **`BillVendor.overdue_balance`** -- same array shape as B-2, sent in every vendor response, unmodelled.
- **`credit_note.dispute_status` / `last_order_status`** -- credit note state.

Silent drops are invisible at compile time and survive every round-trip test. Not blocking (nothing breaks), but this is the same ADV-1 the batch a QA raised, at three times the volume.

### ADV-2. `BillLine.TaxPercent1/2 *int` is unsupported by any evidence and probably wrong

`freshbooks/bills.go:24-25`. The `/api/bills` field table types these `integer` and adds "(2 decimals)" -- which is self-contradictory. Every response example (five in the docs, two captured) has them `null`; the Postman request sends `null`, the docs Create request sends `6`. Nothing in either source shows a populated response value. A 2-decimal percentage in an `int` cannot represent 6.25%. Given the batch's own precedent (`Expense.TaxPercent1` is `string`, matching the docs and the observed `"100"`), `*string` is the safer read type here too. Flag for the live-conformance pass at minimum.

### ADV-3. The same evidence pattern is resolved two different ways inside one batch

- `Tax` write: Postman sends `"amount": 13` unquoted; read is `"amount": "12"` quoted. Code: `TaxCreateRequest.Amount *float64`, `Tax.Amount string`. **Follows Postman.** Correct, and matches the retainer precedent.
- `Expense` write: Postman sends `"taxPercent1": 13` unquoted; docs field table says `string`; read is `"100"` quoted. Code after F4: `ExpenseWriteRequest.TaxPercent1 *string`. **Follows the docs against Postman.**

Both are defensible in isolation, but they are the identical evidence shape resolved opposite ways in one commit, with no `STATE AS OF` note recording either. Per `CLAUDE.md` ("when reality disagrees with the spec, the API wins -- add a callout in the same commit"), the expense-taxPercent conflict at least deserves a line, and the live pass should settle both together.

### ADV-4. Three unrecorded request-side type conflicts (quoted IDs)

Both the docs and Postman send `bill.vendorid` and `bill.lines[].categoryid` as **quoted strings** on create (`"vendorid": "1562"`, `"categoryid": "3696397"`, `"vendorid": "5"`, `"categoryid": "65773"`) while typing them `integer` and returning them unquoted. `BillCreateRequest.VendorID int64` and `BillLineRequest.CategoryID int64` will serialize unquoted. FreshBooks very likely accepts both, but nothing in evidence says so, and this is the only write in the batch where every available example disagrees with what the library sends. One `STATE AS OF` line, or a live check.

### ADV-5. `BillVendorRequest.TaxDefaults []string` is almost certainly the wrong element type

`freshbooks/bill_vendors.go:97`. F6 correctly changed the **read** side to `[]BillVendorTaxDefault`; the **write** side still says `[]string`. Both Postman create/edit examples send `"tax_defaults": []`, so the element type is unevidenced from the request -- but the docs field table describes tax defaults as objects carrying `taxid`, `system_taxid`, `enabled`, `tax_authorityid`, and the response is objects. Sending an array of bare strings has no support anywhere. Either type it `[]BillVendorTaxDefault` (dropping the read-only members) or document why `[]string`.

### ADV-6. `BillVendorTaxDefault` models 3 of the 9 fields FreshBooks sends

Captured (`Get Bills` -> vendor 1) and the docs agree on the full shape:

```json
{"amount":"13.56","created_at":"2020-10-06 14:23:06","enabled":true,"name":"GST",
 "system_taxid":36859,"tax_authorityid":null,"taxid":1,
 "updated_at":"2020-10-06 14:23:06","vendorid":1}
```

The three modelled fields have the right types (`taxid` int, `name` string, `amount` string-decimal) -- F6 got that part right. `enabled` is the one that matters functionally: a caller cannot tell an active default tax from a disabled one.

### ADV-7. Fixtures are trimmed to the struct, so "the fixture decodes" proves almost nothing

Running the probe against `freshbooks/testdata/accounting/*.json` rather than the captured collection: **every fixture except `clients_list.json` drops zero keys**, because every fixture was pared down to exactly the fields the struct declares. `clients_list.json` is the only one seeded from a real docs example, and it is the only one that reveals anything (10 unmodelled keys, ADV-1).

This is why the gate stayed green through three service-killing defects. It also means the batch's fixtures cannot serve as a regression net for the live-conformance pass. Suggestion for the phase-close checklist, not for this fix commit: seed one "full" fixture per resource verbatim from the captured response (synthetic IDs substituted) and assert against it, so the next type mistake fails a test instead of a customer.

### ADV-8. Two fixture values contradict the captured responses they claim to model

Beyond B-1 and B-2's fixtures: `bill_vendors_list.json` also gives `tax_defaults` as `[{"taxid":4001,"name":"HST","amount":"13"}]` -- the right shape, but hand-authored, so ADV-6's six missing fields never show up. And `bills_create.json` line has `"categoryid": 65773` where the captured create response has no `categoryid` key at all, only the nested `category` object. Both are "the fixture asserts the assumption".

### ADV-9. Batch b added no integration-tagged tests

`go test -tags integration -race ./...` inside `freshbooks/` passes; the three seams exercised (`TestExpiryRefreshWriteBackRetry`, `TestFileStoreSurvivesProcessRestart`, `TestAllAcrossPagesWithMidStreamRateLimit`) are all Phase 1's. Batch b's work order promised none, so this is not a miss -- flagging only because the generic `All` seam is now depended on by eight more wrappers, each of which does have its own unit-level `[happy] iterates every X once` test.

---

## 4. Seams -- adequate

`mise exec -- go test -tags integration -race ./...` in `freshbooks/`: all PASS. `TestAllAcrossPagesWithMidStreamRateLimit` covers the generic iterator including error-stops-iteration, which is the seam F12's eight `All` bodies rest on. `TestAll`'s own subtests cover empty first page, no-`Pages` server, nil page, mid-stream error yielded once, cancelled context before first fetch, and break-out-of-range. `bill_vendors_test.go` now serves two real pages (F8), so the one previously vacuous pagination test is honest.

## 5. Test quality -- good, with one systemic hole

**114 subtests** across the ten new test files. No `t.Skip` outside `live_test.go`'s documented env guard. No committed `-run` filters anywhere in `freshbooks/`, `scripts/`, or `mise.toml`. `-race` throughout.

Sad paths are real, not decorative: 19 nil-request guards, 10 `ErrNotFound`, 7 `ErrRateLimited`, 4 `ErrValidation`, 4 `ErrForbidden`, an `ErrUnauthorized`, a duplicate-name 422, and 30 hostile-path-segment cases (`[sad] a path separator` / `a query delimiter` / `a fragment delimiter` x10 resources) that assert the error arrives with **no HTTP request made**. Malformed JSON and cancelled context are covered once, in `transport_test.go`, rather than per-resource -- same as batch a, and correct.

The `[edge]`/body-assertion subtests F8 asked for are present and assert **absence** of keys, which is the only honest way to test `omitzero` and `*bool`. That is the tell the fix commit was understood rather than pattern-matched.

**The systemic hole is the fixtures, not the tests** -- ADV-7. Every one of the three blocking defects would have been caught by a single test that decodes FreshBooks' own captured response instead of a fixture derived from the struct. The tests are not vacuous in the "calls a function to hit lines" sense; they are asserting against evidence the batch authored itself.

## 6. Parity -- PASS

`mise run inventory-check` (inside the gate): `implemented 114, ignored 0, todo 99, uncovered 0, double-covered 0, stale 0, unknown 0`. The three-way tax stacks resolve (`Expenses/*` + `Accounting/Taxes/*` + `Settings/Items and Services/*`, five operations, fifteen keys on five methods). No ignore-list entries exist at all, so the "written reason" rule is vacuously satisfied. `Expenses/Upload Expense Receipt Image/Upload Receipt Image` correctly left to batch d.

## 7. Fidelity spot-checks -- 9 inventory entries hand-verified against the docs pages

| Inventory key | Expected (docs / captured) | Observed (code) | |
|---|---|---|---|
| `Clients/List Clients` | `GET /accounting/account/{id}/users/clients`, `{clients:[],page,pages,per_page,total}` | `clientsPath` + `listOpts` + `newPage` | OK |
| `Clients/Update Client` | `PUT .../users/clients/{id}`, `{"client":{...}}` | `ClientWriteRequest` in a `client` envelope | OK |
| `Clients/Delete Client` | `PUT` + `{"client":{"vis_state":1}}` | `softDelete(path,"client")` | OK |
| `Estimates/Accept Estimate` | `PUT .../estimates/estimates/{id}` | `Accept` distinct from `Update`/`Delete`/`Send` | OK (F13 comment now honest) |
| `Expenses/Delete Expense` | docs `{"expense":{"vis_state":1}}`; Postman wrongly sends `0` | sends `1`, callout recorded | OK |
| `Expenses/Single Expense` | field table: `taxPercent1 string`, `markup_percent string`, `status int` | matches read model exactly | OK |
| `Accounting/Taxes/Delete Single Tax` | **real `DELETE`**, no `vis_state` on Tax at all | `http.MethodDelete`, no `VisState` field, reason in the doc comment | OK -- caught the one exception in the family |
| `Expenses/Bills (Beta)/Archive Bill` | `PUT` + `vis_state: 2`, distinct from delete's `1` | `visStatePut(..., VisStateArchived)` vs `VisStateDeleted` | OK |
| `Expenses/Bills (Beta)/Get Bills` | line `quantity` quoted | `int` | **B-1** |
| `Expenses/Vendors (Beta)/Add Vendor` | `outstanding_balance` an array | `*Money` | **B-2** |
| `Expenses/Create Expense with Receipt` | `attachment.attachmentid` a number | `string` | **B-3** |

Two collection caveats worth knowing, neither a defect: the captured body filed under `Expenses/Bills (Beta)/Get Bills` is actually a **bill_vendors** response (`response.result` has keys `bill_vendors,page,pages,per_page,total`) -- a Postman authoring slip, so `Bills.List`'s `bills` envelope key remains INFERRED with no captured evidence either way. And the docs' own `List Estimates` example is syntactically invalid JSON (a stray `}` before `]`), while the estimate examples carry `estimateid` **twice** in one object -- both FreshBooks doc bugs, not library problems.

---

## Verdict

**NEEDS WORK.** The gate is green, all fifteen triage fixes landed with real tests, the inventory balances at exactly 59, the security fix (F1) is thorough, and the test suite is 114 honest subtests with no skips and no filters. This is a good batch and I want to be clear about that.

But three services -- `Bills`, `BillVendors`, and `Expenses` for any account with receipts -- cannot decode FreshBooks' own responses, and the evidence proving it was already in this repository. `BillVendors` is unconditional: **every** vendor read fails, empty balance or not. And B-2 is a regression the fix commit introduced, with a fixture authored to make it look right.

All three are small, mechanical fixes. Suggested second fix commit:

1. `BillLine.Quantity` -> `string`; `bills_create.json` line quoted, and give `bills_list.json` a real line.
2. `BillVendor.OutstandingBalance`/`OverdueBalance` -> `[]VendorBalance{Amount Money}`; re-seed `bill_vendors_*.json` from the captured response.
3. `ExpenseAttachment.AttachmentID`/`ID` -> `int64`; add the captured attachment to `expenses_get.json`.
4. One `STATE AS OF 2026-09-01` block recording the quantity/balance/attachment conflicts and ADV-3's two-ways resolution.

ADV-1's high-value drops (`transactionid`, `overdue_balance`, `Bill.attachment`, `BillLine.category`), ADV-2, ADV-5 and ADV-6 are cheap enough to fold in; ADV-4 and ADV-7 belong on the phase-close checklist.

## Commands run

```
mise run check                                             # once, exit 0, from .worktrees/b
git status --porcelain                                     # empty, before and after
git diff main..HEAD -- .../ignore.list | grep -c '^-//go:inventory-todo'   # 59
git diff main..HEAD -- .../ignore.list | grep -cE '^\+[^+]'               # 0
mise exec -- go test -tags integration -race ./...         # PASS (freshbooks, auth)
mise exec -- go test -run TestZZProbe ./                   # throwaway probe, deleted
scripts/redaction-check.sh                                 # clean
```

Docs pages fetched this pass: `/api/clients`, `/api/estimates`, `/api/expenses`, `/api/bills`, `/api/vendors`, `/api/taxes`.

---

# Re-verification -- 2026-09-01, HEAD `9147ade`

**Verdict: PASS.** All three blockers are fixed and proven fixed by the probe that found them. Six of the nine advisories were also addressed. No regressions.

Subject: `9147ade fix(lib-resources-b): apply the QA findings`, one commit on top of `e6c65a3`. 18 files, +252/-45: four read models, three test files, ten fixtures, one spec addendum.

## The acceptance test: the captured-response decode probe

Rebuilt the same probe (`freshbooks/zz_qa_probe_test.go`, deleted again afterwards) and re-ran it over all 39 captured batch-b response bodies extracted from `freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json`, decoding each through the library's own structs.

**Before (`e6c65a3`): 4 decode failures. After (`9147ade`): 0 decode failures across all 34 decodable captures.**

| Blocker | Before | After |
|---|---|---|
| B-1 `BillLine.Quantity` | `cannot unmarshal string into ... quantity of type int` on 5 captured bills | `Quantity string`, all 5 decode |
| B-2 `BillVendor.OutstandingBalance` | `cannot unmarshal array into ... of type freshbooks.Money` on 4 captured vendors | `[]VendorBalance`, all 4 decode, `overdue_balance` added |
| B-3 `ExpenseAttachment.AttachmentID`/`ID` | `cannot unmarshal number into ... of type string` on 3 captured expense responses | `int64`, all decode |

Silent-drop counts fell everywhere the fix touched, and every field I named as high-value in ADV-1 is now modelled:

```
                    e6c65a3 -> 9147ade
bill                   3    ->  2    (attachment modelled; overall_category/description remain)
bill_vendor            1    ->  0    (complete)
expense (list)        14    ->  9    (transactionid, category, isduplicate,
                                      from_bulk_import, profileid, expense_profile-adjacent)
expense (single)      11    ->  7
credit_note            6    ->  4    (dispute_status, last_order_status modelled)
client                 7    ->  7    (unchanged -- I did not flag these)
estimate               5    ->  5    (unchanged -- I did not flag these)
tax / expense_category / bill_payment / expense_profile / expense_summary : 0, complete
```

## Fixtures now match the captured shapes, not the structs

This was the root cause of all three blockers, so I checked the reseeded fixtures against the captures directly rather than trusting that the tests pass.

- `bills_list.json` line: `"quantity": "1"` **quoted**, plus the nested `category` object and the four `tax_*` nulls -- the captured `Add Bill from Vendor` line shape. Previously `"lines": []`.
- `bills_create.json` line: `"quantity": "3"` **quoted** (was `3` unquoted, the value that hid B-1).
- `bill_vendors_list.json`: `"outstanding_balance": [{"amount": {"amount": "375.00", "code": "USD"}}]`, `"overdue_balance": []` -- the captured two-level array shape. Previously the bare object that hid B-2. `bill_vendors_create.json` / `_delete.json` / `_list_page2.json` carry the empty-array case, so **both** the empty and populated branches are exercised.
- `expenses_get.json`: `"attachment": {"attachmentid": 8668, "id": 8668, "jwt": "example.synthetic.jwt", "media_type": "image/png"}` -- the captured numeric ids, with the real JWT replaced by a synthetic one (correct; `redaction-check: clean`). Plus `transactionid`, `isduplicate`, `from_bulk_import`, `profileid`, and the nested `category`.
- `credit_notes_*.json`: `dispute_status` / `last_order_status` present as `null`, matching the captures exactly -- honest, since no capture ever shows them populated.

Every one of these is asserted, not just present. The new subtests read values rather than counting them: `line.Quantity != "3"` ("want the quoted string FreshBooks actually returns"), `len(v.OutstandingBalance) != 1 || v.OutstandingBalance[0].Amount.Amount != "375.00"`, `v.OverdueBalance == nil` ("want an empty (non-nil) slice"), `exp.Attachment.AttachmentID != 8668`, `*exp.TransactionID != 900123`, `td[0].Enabled == nil || !*td[0].Enabled`. Subtest count is unchanged at 114 (the fix extended existing subtests rather than adding shallow new ones), and running the probe over `freshbooks/testdata/accounting/*.json` now reports **0 dropped for every fixture except `clients_list.json`**, which is the one seeded verbatim from a docs example and is meant to over-supply.

## Advisories addressed

- **ADV-1** -- `Expense.{TransactionID *int64, IsDuplicate *bool, FromBulkImport *bool, ProfileID *int64, Category *ExpenseCategory}`, `Bill.Attachment`, `BillLine.Category`, `BillVendor.OverdueBalance`, `CreditNote.{DisputeStatus, LastOrderStatus}` -- all the fields I named.
- **ADV-2** -- `BillLine.TaxPercent1/2` -> `*string`, with the reasoning ("an int cannot represent a 2-decimal percentage") on the field. `BillLineRequest.TaxPercent1/2` correctly stays `*int`: the docs Create example sends `6` unquoted, so this is the same evidenced asymmetry as `quantity`, not an oversight.
- **ADV-3** -- both resolutions now stated and justified in the spec addendum (below).
- **ADV-5** -- `BillVendorRequest.TaxDefaults` -> `[]BillVendorTaxDefault`, with a comment saying the element type is unevidenced from a request body and why the object shape wins anyway.
- **ADV-6** -- `BillVendorTaxDefault` gains `Enabled`, `SystemTaxID`, `TaxAuthorityID`, `VendorID`, `CreatedAt`, `UpdatedAt` -- the full 9-field captured shape, nullable ones as pointers.
- **ADV-4 / ADV-7 / ADV-9** deferred, as I recommended.
- **ADV-8** closed by the reseeding above.

`BillLine.CategoryID` was kept (now documented as "0 on real reads today ... kept for parity with the write side") rather than removed. Correct call: the write side needs it and removing a field is the more disruptive option.

## Spec addendum -- accurate

The section 3 callout gains an "Addendum, batch b QA pass" recording all three conflicts with the evidence and the direction each was resolved, plus an explicit paragraph on ADV-3 explaining why `TaxCreateRequest.Amount` follows Postman while `ExpenseWriteRequest.TaxPercent1` follows the docs (the read model already types the same fields `string` in every captured response, so Postman's create example is the outlier). I checked each claim against the sources; all are accurate, and the ADV-3 reasoning is better than "the docs win".

## Gate -- substantively green, exit 1 on my own report file

`mise run check` from inside `.worktrees/b`, run once, exit code captured directly: **exit 1**, and the only failing step is the dirty-tree banner reporting my own uncommitted report:

```
== fmt-check / vet / lint: freshbooks ==   0 issues.
== test: freshbooks ==   ok (all three packages)
== cover: freshbooks ==  total = 91.3% (floor 90%)  PASS
== vuln: freshbooks ==   No vulnerabilities found.
== inventory-check: freshbooks ==
implemented 114, ignored 0, todo 99, uncovered 0, double-covered 0, stale 0, unknown 0
   [mcp and cli: lint 0 issues, coverage 100.0% PASS, vuln clean]
== actionlint ==   (clean)
== build ==        12 artifacts, dist/
DIRTY TREE:
?? docs/phases/2/reports/b-qa.md
[check] ERROR task failed
EXIT_CODE=1
```

Every substantive step passed. The failure is `scripts/check.sh`'s dirty-tree guard firing on `docs/phases/2/reports/b-qa.md`, which the work order designates as the one allowed dirty file. I am not counting it against the batch, but the lead should know: **the "QA leaves its report uncommitted" instruction and the gate's dirty-tree guard are in direct tension.** My first-pass exit 0 was only possible because I ran the gate before writing the report. The fix commit's own message claims a green check with the report "left in place, uncommitted" -- that claim cannot both be true and have the report present, so it was presumably run before writing, same as mine. Worth resolving for batches c and d: either commit the QA report before the final gate, or teach the guard to ignore `docs/phases/*/reports/`.

Because the gate's test step reported `(cached)`, I re-ran the tests uncached to be sure the fix commit's tree is genuinely green:

```
mise exec -- go test -C freshbooks -count=1 -race ./...                   ok (3 pkgs)
mise exec -- go test -C freshbooks -count=1 -tags integration -race ./... ok (3 pkgs)
scripts/redaction-check.sh                                                clean
git status --porcelain                                                    only b-qa.md
```

Parity unchanged and still balanced: `implemented 114, uncovered 0, double-covered 0, stale 0, unknown 0`; the `ignore.list` diff against `main` is still exactly 59 removals and 0 additions.

## Remaining, all non-blocking

1. `Bill.Attachment *ExpenseAttachment` reuses the expense type on the strength of a comment saying "the captured shape is identical" -- but `attachment` is `null` in all five captured bills, so the shape is INFERRED, not confirmed. The comment slightly overstates. One word, or a live check.
2. Still-dropped fields I never flagged: client 7 (`has_retainer`, `retainer_id`, `level`, `notified`, `subdomain`, `direct_link_token`, `statement_token`), estimate 5 (`accounting_systemid`, `address`, `current_organization`, `ext_archive`, `sentid`), credit_note 4 (same family), expense 7-9 (`accountid`, `account_name`, `accounting_systemid`, `bank_name`, `background_jobid`, `ext_invoiceid`, `ext_systemid`, `project`, `expense_profile`), bill 2 (`overall_category`, `overall_description`). Mostly deprecated or internal per the docs field tables. Live-conformance backlog.
3. ADV-4 (quoted `vendorid`/`categoryid` on create) and ADV-7 (one verbatim "full" fixture per resource) remain on the phase-close checklist, as agreed.

## Verdict

**PASS.** Every blocking finding is fixed, and fixed at the root -- the fixtures now carry FreshBooks' real shapes, so the same class of defect fails a test next time instead of shipping. The asymmetries that should have been preserved were preserved and documented (`BillLine.Quantity` string-on-read / int-on-write, `BillLineRequest.TaxPercent *int`), the spec addendum records the evidence honestly including the two-directions ADV-3 resolution, and the new assertions check values rather than shapes. Nothing regressed: 114 subtests, uncached `-race` green, integration green, coverage 91.3%, inventory balanced at 59, redaction clean.

Clear to merge. The one gate caveat is a process artifact of my own report file, not a defect in the batch.
