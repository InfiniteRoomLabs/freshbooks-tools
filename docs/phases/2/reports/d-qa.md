# Phase 2 batch d -- QA / reality-check lane

**FINAL VERDICT (round 3, 2026-09-01, at `e34b7fc`): PASS.** See section 11.

**Verdict (round 2, at `d100999`): NEEDS WORK** -- one BLOCKING regression, everything else cleared. See section 10.

**Verdict (round 1, at `a624cf9`): NEEDS WORK** (5 BLOCKING, 4 ADVISORY) -- the record below is left as written.

Subject: branch `phase-2/d` at `a624cf9` (`fix(lib-resources-d): apply the review-gate findings`) in `.worktrees/d`. Oracle: spec sections 3 and 5.1, `GOAL.md` stage 2, `docs/phases/2/plan-d.md`, `docs/phases/2/triage-d.md`. Docs consulted live: /api/chart-of-accounts, /api/journal-entries, /api/other_income, /api/reports, /api/webhooks, /api/expense-attachments.

The gate is green and every one of the twenty triage fixes landed. The blockers below are all in one class: **captured response fields that the library silently discards**, plus documented query options that no method exposes. This is the same class the batch-b and batch-c QA passes blocked on, and the same class the code-review lane's own B-2 (`link` dropped on image upload) was filed under -- the F16 sweep simply did not go all the way through the report structs and the gateway structs.

---

## 1. Gate (run once, from the worktree, on the clean HEAD, exit code captured directly)

```
$ mise run check > /tmp/qa-d-check.log 2>&1; echo "EXIT=$?"
EXIT=0
```

```
== cover: freshbooks ==
coverage-gate: .../.worktrees/d/freshbooks/coverage.out total = 91.7% (floor 90%)
coverage-gate: PASS
== vuln: freshbooks ==
No vulnerabilities found.
== inventory-check: freshbooks ==
implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0
...
== cover: mcp ==
coverage-gate: ... total = 100.0% (floor 90%)   coverage-gate: PASS
== cover: cli ==
coverage-gate: ... total = 100.0% (floor 90%)   coverage-gate: PASS
== actionlint ==
== build ==
build: done, artifacts in .../.worktrees/d/dist
check.sh: all OK
EXIT=0
```

`git status --porcelain` -> empty, both before the gate and after probe cleanup. Only `docs/phases/2/reports/d-qa.md` (this file) is dirty at hand-off.

**Parity: 213 implemented / 0 todo / 0 ignored / 0 uncovered / 0 double-covered.** All 213 inventory keys are now claimed exactly once -- the phase-2 goal is mechanically met. `testdata/ignore.list` carries no `//go:inventory-todo` lines at all, so there is no ignore list to audit for reasons.

## 2. Fix verification: all TWENTY triage items landed

| Fix | Evidence |
|---|---|
| F1 pathSegment sweep | `pathSegment(` present in all nine files (ledger 3, journal 2, other_income 1, reports 3, callbacks 1, gateways 1, images 1, attachments 1, payment_options 2) |
| F2 doOnHost forces https | `transport.go:83-87` `u.Scheme = "https"` + `noTraversal(path)` + `u.RawQuery = ""`; `options.go:48` documents that tokenization ignores `WithBaseURL`. **Probe-verified** (section 4b) |
| F3 redacting String() | `payment_options.go:35,59`. **Probe-verified** (section 4c) |
| F4 synthetic PANs | `4111111111111111` / `4242424242424242` / `pk_test_example` in tests and `testdata/gateways/get.json`; no `pk_live` outside the vendored Postman collection and generated `inventory.json` (FreshBooks' own published data, untouched by this batch) |
| F5 reports get()/setNonEmpty | `reports.go:32` `get`; 10 `setNonEmpty`/`FormatBool` hits; `boolQuery` gone |
| F6 doRaw/sendRaw deleted, CSV Accept | zero `doRaw`/`sendRaw` in the package; `reports.go:261` `fetchRaw(..., FamilyAccounting, "text/csv")`; `newRequest` doc comment (`transport.go:370-373`) matches the code |
| F7 image `link` surfaced | `images.go:20-37` `imageUploadResponse.result()`. **Probe-verified** (section 4a) |
| F8 quoted sub_accountid | `journal_entries.go:11` `SubAccountID string`; `JournalEntryDetailResult.SubAccountID` stays `int64`. **Probe-verified** (section 4d) |
| F9 StripeTokenizeRequest.APIKey | `json:"-"`, sent once at body top level (`payment_options.go:55,165-168`) |
| F10 newPage | present in `other_income.go` and `callbacks.go` |
| F11 FamilyBusiness constants | `familyForPath(path)` now only in `transport.go` (`Do`) and one test; the nine resource call sites pass the constant |
| F12 TimeEntryAbility deleted | zero hits package-wide |
| F13 filepath.Base + fields param dropped | `transport.go:143`; `doMultipart` signature has no `fields`. **Probe-verified** (section 4a: `dir/sub/logo.png` -> `filename="logo.png"`) |
| F14 callback_id in body | `callbacks.go:169-176, 194-200`. Matches both Postman captures (see ADVISORY-3) |
| F15 OtherIncomeUpdateRequest pointers | `other_income.go:73-82`: `*string` for CategoryName/Date/PaymentType/Source, each doc-commented |
| F16 captured-but-dropped fields | GatewayPricing ach tiers/default_pricing_tier_id/promo_expiry_date/max_ach_fee, `FBPayConnection.BankInfo`, `InvoiceDetailsReport.Clients`, `TimeEntryDetailEntry.{Timer,IsLogged,Pending*,Highlight}`, `LedgerAccountUpdateRequest.SubAccounts` all present. **Incomplete -- see BLOCKING-1..4** |
| F17 DownloadInvoiceDetailsCSV | renamed; URL still hard-codes `invoice_details.csv` (matches Postman) |
| F18 taxonomy json.RawMessage | `Types`/`SubTypes`/`SubType` return `json.RawMessage`; `LedgerAccountSubType` gone |
| F19 named nested types | `JournalEntryDetailAccount` / `...SubAccount` / `...Source` |
| F20 approved signature changes | `CreditCardToken.IsPrimary bool` without `omitempty`; `OtherIncomeListOptions`/`CallbackListOptions` + `opts()` + `List(ctx, id, opts *XListOptions, extra ...RequestOption)` + `All` iterators on both |

F19 is the recorded override of simplify's DO-NOT-APPLY 16, and F13 the recorded override of simplify 9 -- both applied as the triage directed.

## 3. Mandatory acceptance test: decode every captured batch-d response

Throwaway probe (`freshbooks/zzqaprobe_test.go`, deleted; tree verified clean) walked `freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json`, pulled every non-empty captured response body in batch d's scope (Accounting minus Taxes, Reports, Webhooks, Uploader, Tokenization, plus the nine cross-folder duplicate keys d owns), served each from `httptest`, and drove it through the real service method -- envelope classification, family error decoding and all. Tokenization cases went through a host-rewriting `RoundTripper` so the forced-https `doOnHost` path was exercised for real.

```
PROBE SUMMARY: 38 cases, 0 decode failures
--- PASS: TestQAProbeDecodeEveryCapturedBatchDResponse (0.09s)
```

**38 / 38 captured bodies decode. Zero decode failures.** That covers every batch-d Postman entry that carries a body; the entries with none (Uploader/Upload Expense Receipt, Uploader/Upload Image Without AccountId, Webhooks Register/Verify/Resend/Delete, the three ledger taxonomy endpoints, Reports/Download CSV, Reports/Client Account Statement, Reports/Expense Details and Bank Reconciliation Summary -- which have a response entry with an empty body -- and the three STRIPE tokenization steps) have nothing to decode against, which is why those methods correctly return `json.RawMessage` or a shape marked INFERRED.

**Silent-drop counts.** Per case, the probe diffed the captured body's key names against the re-marshalled decoded value. 30 of 38 cases drop nothing but the envelope/wrapper key. The remainder, hand-inspected against the raw capture, are the findings in section 5. Two real drops (`ItemSalesReport.total*` and `StripeConnection.max_ach_fee`) are invisible to a name-only diff because a sibling struct happens to use the same key name -- both were caught by reading the raw bodies, and are reported below.

## 4. Request-side verification (all four required checks pass)

Throwaway probe `freshbooks/zzqareq_test.go`, also deleted.

**(a) Multipart body replay on retry** -- 429 then 200, `WithRetry(testRetry(3))`:
```
OK: 2 attempts, byte-identical 254-byte multipart bodies, base filename only, link surfaced
```
Both attempts carry the file bytes, both carry `filename="logo.png"` (the `dir/sub/` prefix passed in is stripped, per F13), both carry a `multipart/form-data; boundary=` Content-Type, and the sibling `link` is surfaced on the result (F7). The build-once-into-`[]byte` design is correct: `newRequest` makes a fresh `bytes.Reader` per attempt, so replay cannot be defeated by a consumed reader.

**(b) Forced-https card path** -- client configured with a plaintext base URL carrying a query string:
```
OK: base "http://127.0.0.1:43945/root?leak=secret" -> tokenization URL "https://paid.freshbooks.com/gateway/fbpay/tokenize"
OK: stripe tokenization URL "https://paid.freshbooks.com/gateway/stripe/payment-method"
```
Scheme forced to https, host forced to `paid.freshbooks.com`, the base URL's path and its `leak=secret` query both dropped. Security finding 1 is genuinely closed for both tokenization endpoints, not just the one the fix commit's test covers.

**(c) Redacting String()** -- five render paths each (`%v`, `%+v`, `%s`, `.String()`, `%v` on the pointer):
```
OK: fbpay=freshbooks.FBPayTokenizeRequest{Name: "Sam Owner", CardNumber: redacted, CVV: redacted, ExpiryMonth: "9", ExpiryYear: "2029"}
OK: stripe=freshbooks.StripeTokenizeRequest{Name: "Sam Owner", CardNumber: redacted, ExpiryMonth: "9", ExpiryYear: "2029", APIKey: redacted}
```
No render leaks the fixture PAN, the CVV, or the API key. The `%#v` verb does still expose the PAN -- that is inherent to Go and is exactly what both types' doc comments warn about, so it is documented rather than a defect.

**(d) JournalEntryDetail quoted sub_accountid** -- the actual outgoing request body:
```
{"journal_entry":{"details":[{"sub_accountid":"635972","debit":"100.00"},{"sub_accountid":"636000","credit":"100.00"}],...}}
```
Quoted on the request side, `int64` on the response side. F8 is correct and asymmetric-by-evidence, as intended.

## 5. Findings

### BLOCKING-1 -- `ItemSalesReport` discards the report's own totals
`freshbooks/reports.go:371-379`

The captured `Reports/Item Sales` body's `item_sales` object has 12 keys. `ItemSalesReport` models 7. Dropped, with real values in the capture:

| key | captured value | in struct? |
|---|---|---|
| `total` | `{"amount":"23320.00","code":"USD"}` | no |
| `total_discount` | `{"amount":"0.00","code":"USD"}` | no |
| `total_qty` | `20` (bare number -- note `ItemSale.TotalQty` is a **string** `"2"` at the line level, so this needs its own type, not a copy of that field) | no |
| `start_date` | `"2019-01-01"` | no (`end_date` **is** modelled) |
| `statusids` | `[]` | no (`clientids` and `item_names` **are** modelled) |

`total`/`total_discount`/`total_qty` are the headline numbers of a sales report. A caller asking for item sales gets per-item rows and has to re-sum them. `start_date`/`statusids` are the filter echoes whose siblings were already kept, so their omission is an inconsistency inside one struct.

### BLOCKING-2 -- `TrialBalanceReport` discards `download_token`, `start_date`, `end_date`
`freshbooks/reports.go:592-597`

Captured `trial_balance` keys: `company_name, currency_code, data, download_token, end_date, start_date`. The struct models the first three. `download_token` is a real JWT in the capture and is the handle every other report in the file exposes (`ItemSalesReport.DownloadToken`, `PaymentsCollectedReport.DownloadToken`, ...); trial balance is the only report struct that throws it away, so the trial-balance CSV is unreachable from the library. `start_date`/`end_date` are populated (`2019-01-01` / `2019-12-31`).

### BLOCKING-3 -- `StripeConnection` drops `max_ach_fee`, which F16 explicitly named
`freshbooks/gateways.go:63-72`

F16's list is "GatewayPricing (ach tiers, `default_pricing_tier_id`, `promo_expiry_date`, `max_ach_fee`)". Those four were added to `GatewayPricing`, where the capture has them all `null`. But the `Tokenization/1a. [STRIPE] - Get Publishable Key` capture also carries `max_ach_fee` on the **stripe** object, populated: `"max_ach_fee": 5000`. `StripeConnection` has no such field, so the one place the value actually appears is the one place it is discarded. (Note the type: a bare number there, versus `*string` on `GatewayPricing`.)

### BLOCKING-4 -- `GatewayConnection` drops the `paypal` connection entirely
`freshbooks/gateways.go:78-81`

The same capture's `gateway_connections[0]` has three keys: `fbpay`, `paypal`, `stripe`. `GatewayConnection` models two. `paypal` is `null` in this capture, so its shape is unknown -- but this batch's own policy for an always-null captured key, applied five times in this very fix commit (`TimeEntryDetailEntry.Timer`, `.PendingClient`, `.PendingProject`, `.PendingTask`, `.Highlight`), is to keep the key as `json.RawMessage` with a doc comment saying the populated shape is unconfirmed. Dropping it instead means an account with PayPal connected gets a `GatewayConnection` that gives no hint PayPal exists. One field, same treatment as its siblings.

### BLOCKING-5 -- three reports expose no query options at all, including two whose parameters the docs list
`freshbooks/reports.go:91` (`AccountsAging`), `:310` (`InvoiceDetails`), `:410` (`PaymentsCollected`), `:450` (`ProfitLoss`), `:273` (`ExpenseDetails`)

`plan-d.md` promised "report-specific query options"; seven of the twelve reports have an options struct. Five take neither an options struct nor a variadic `...RequestOption`, so there is **no way at all** to set a date range on them. https://www.freshbooks.com/api/reports documents the parameters for two of those five against the exact paths the library uses:

- `invoice_details`: `start_date`, `end_date`, `currency_code`, `clientids`, `statusids`, `date_type`
- `payments_collected`: `start_date`, `end_date`, `currency_code`, `locale`, `clientids`, `payment_methods`

(and lists `start_date`/`end_date`/`currency_code`/`locale` for profit-and-loss, though against a different, newer path -- see ADVISORY-4). An un-date-rangeable P&L or invoice-details report is not usable for the stated purpose of running the company's books. This is additive-only: an `XOptions` struct plus one `opts.values()` call each, matching the seven that already have one.

### ADVISORY-1 -- `FBPayConnection` drops `action_reasons`
`freshbooks/gateways.go:45-59`. Present as `[]` in all three gateway captures. Empty in every example, so shape unknown -- same `json.RawMessage` treatment as BLOCKING-4 would settle both together.

### ADVISORY-2 -- `TimeEntryDetailsReport` drops `download_token` and `aggregations`; its `meta` carries two fields `PageMeta` does not
`freshbooks/reports.go:655-660`, `freshbooks/page.go:27-32`. The captured body's top level is `abilities, aggregations, download_token, meta, time_entries`; the struct models three of five. Its `meta` is `{"total":9,"per_page":30,"page":1,"pages":1,"total_logged":0,"total_unbilled":0}` -- `total_logged`/`total_unbilled` are report figures that fall on the floor because the shared `PageMeta` has no place for them. `aggregations` is `null` in the capture (RawMessage treatment again). Do **not** widen the shared `PageMeta` for one report's extras; a report-local meta type is the right shape.

### ADVISORY-3 -- `callback_id` in the verify/resend bodies is Postman-backed but contradicts the docs page
`freshbooks/callbacks.go:169-176, 194-200`. F14 is correct as evidence: both Postman requests do send it (`{"callback":{"callback_id":{{callbackId}},"verifier":"..."}}` and `{"callback":{"callback_id":2001,"resend":true}}`). But https://www.freshbooks.com/api/webhooks shows both bodies **without** it (`{"callback":{"verifier":"..."}}`, `{"callback":{"resend":true}}`). Sending a redundant field the docs omit is low-risk and the id is already in the path, but per this project's own "inferred vs confirmed" rule the disagreement belongs in a `> **STATE AS OF 2026-09-01**` line in spec 5.1 alongside the other batch-d docs-vs-Postman conflicts, and in the method doc comments. Everything else on that docs page matches the implementation exactly -- all five paths, both verbs, the register body.

### ADVISORY-4 -- the docs now describe successor endpoints for journal entries, chart-of-accounts list, profit-loss and trial balance; nothing records that
The library implements the Postman-captured endpoints (correct: the parity contract is the collection). But the live docs pages have moved on, and a reader of the code will not know:

- **/api/journal-entries** documents `POST /accounting/businesses/{business_uuid}/journal_entries` with a `manualJournalEntry` wrapper and `{accountId (uuid), amount{}, type: TYPE_DEBIT|TYPE_CREDIT}` details -- a different API from the implemented `POST /accounting/account/{acct}/journal_entries` with `{sub_accountid, debit, credit}`. The `Create` doc comment flags the *host* as INFERRED (the `my.freshbooks.com` rewrite) but says nothing about the successor API.
- **/api/chart-of-accounts** documents the account **list** as `GET /accounting/businesses/{uuid}/reports/chart_of_accounts` (wrapper `response.result.journal_entry_accounts`). The implemented list is the Postman one, `GET /accounting/businesses/{uuid}/ledger_accounts/accounts` (wrapper `data`) -- which the captured body confirms. Create and get-single match the docs exactly.
- **/api/reports** documents profit-and-loss and trial-balance under `/accounting/businesses/{business_uuid}/reports/...` with wrappers `profit_and_loss` / `trial_balance`; the implemented paths are the Postman `/accounting/account/{acct}/reports/accounting/profitloss_entity` and `.../trial_balance`.

One paragraph in the spec 5.1 batch-d callout plus a line in each affected doc comment closes this. The batch-d callout is otherwise excellent -- it correctly records the envelope resolutions **and** self-reports the `ledger` family misclassification the batch found and fixed, which is exactly the honesty the process asks for.

## 6. Fidelity spot-check (6 docs pages, request by request)

| Endpoint | Docs (method + path + shape) | Code | Verdict |
|---|---|---|---|
| Expense receipt upload | `POST /uploads/account/<accountId>/attachments`, multipart field `content`, response `{attachment:{filename, public_id, jwt, media_type, uuid}}` | `attachments.go:38-45`, field `"content"`, `UploadedAttachment` has all five | **exact match** |
| Webhook register | `POST /events/account/<id>/events/callbacks`, `{"callback":{"event","uri"}}` | `callbacks.go:67`, same | **match** |
| Webhook verify / resend / delete / list | `PUT`/`DELETE`/`GET` on `.../events/callbacks[/<callback_id>]` | same verbs, same paths | **match** (bodies: ADVISORY-3) |
| Other income list/create/update/delete | `GET/POST/PUT/DELETE /accounting/account/<accountid>/other_incomes/other_incomes[/<id>]`; fields `incomeid, amount, category_name, date, note, payment_type, source, sourceid, taxes, created_at, updated_at, vis_state, userid` | `other_income.go:103`, same paths and verbs; every documented field decodes (captured body -> 0 drops beyond the wrapper) | **match** (docs' `POST .../<id>` for create is a docs typo; the capture and the code both POST to the collection) |
| Chart of accounts create / get single | `POST /accounting/businesses/<uuid>/ledger_accounts/accounts`, `GET .../accounts/<account_uuid>`, wrapper `data` | `ledger_accounts.go:97,110`, wrapper `data`, scoped by `BusinessUUID` | **match** (list: ADVISORY-4) |
| Reports: tax summary | `GET /accounting/account/<id>/reports/accounting/taxsummary`, params `start_date,end_date,currency_code,cash_based`, wrapper `taxsummary` | `reports.go:557` name `"taxsummary"`, `SalesTaxSummaryOptions`, wrapper `taxsummary` | **match** |
| Reports: all 12 paths vs the collection | -- | every `s.get` name matches its Postman URL segment exactly, including the easy-to-miss `profitloss_entity` (URL) vs `profitloss` (wrapper) and the hard-coded `invoice_details.csv` download path | **match** |

## 7. Test quality

- No `t.Skip` anywhere except `live_test.go:26`, which is the intended `FRESHBOOKS_LIVE` guard. No committed `-run` filters. `-race` on via the gate.
- Fixture values spot-checked against the captures they were seeded from; `gateways/get.json` correctly carries `pk_test_example` and the F16 `bank_info`/ach-tier fields, and `gateways_test.go` asserts real values out of them rather than just non-nil.
- Sad paths present per family (403 on gateways, 429 + `Retry-After` in `transport_upload_test.go`, size-bound rejection, hostile path segments per F1). No vacuous or line-touching-only tests spotted in the batch-d files.
- Per-function coverage in the batch-d files bottoms out at 71.4% (`LedgerAccounts.List`) with the uncovered lines being the `if err != nil { return }` legs -- normal, and comfortably inside the 91.7% module figure.

## 8. Commands run

```
mise run check > /tmp/qa-d-check.log 2>&1; echo "EXIT=$?"        # EXIT=0
git status --porcelain                                            # empty
mise exec -- go test -run TestQAProbeDecodeEveryCapturedBatchDResponse -v .
mise exec -- go test -run 'TestQAProbe(Multipart|ForcedHTTPS|Redacting|JournalEntry)' -v .
mise exec -- go tool cover -func=freshbooks/coverage.out
rm -f freshbooks/zzqaprobe_test.go freshbooks/zzqareq_test.go     # tree clean
```

## 9. Recommendation

Every blocker is additive and mechanical -- five struct-field/options additions with a fixture line each, no design decisions, no signature breaks except adding an `opts` parameter to three unreleased report methods. One fix commit, re-run the gate, merge. The security posture, the transport work, the parity contract, and the envelope resolution are all sound; this is a completeness gap in the F16 sweep, not a defect in the batch's design.

---

# 10. Re-verification -- 2026-09-01, round 2

Subject: `d100999 fix(lib-resources-d): apply the QA findings` (G1-G7 answering round 1's B-1..B-5 and ADV-1..4), on top of `a624cf9`. Scope per the lead's tight-pass brief: the touched read models, the reseeded fixtures, the five new report options structs, the spec 5.1 additions, and one gate run.

**Verdict: NEEDS WORK -- 1 BLOCKING (a regression this commit introduced, from an error in round 1's own report). All nine round-1 findings are otherwise correctly closed.**

## 10.1 Gate (round 2, run once on the clean tree, exit captured directly)

```
$ mise run check > /tmp/qa-d-check2.log 2>&1; echo "EXIT=$?"
EXIT=0
```
```
coverage-gate: .../freshbooks/coverage.out total = 91.8% (floor 90%)   PASS
coverage-gate: .../mcp/coverage.out       total = 100.0%               PASS
coverage-gate: .../cli/coverage.out       total = 100.0%               PASS
implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0
No vulnerabilities found.  (x3)
check.sh: all OK
EXIT=0
```
Coverage up 91.7% -> 91.8%. Parity unchanged at 213/213. `git status --porcelain` empty for the gate run (this report was moved aside and restored afterwards); it is the one dirty file at hand-off.

## 10.2 BLOCKING R-1 -- `ItemSalesReport.TotalQty` is typed `int`, but the capture sends a quoted string. Item Sales no longer decodes at all.

`freshbooks/reports.go:455-458`, `freshbooks/testdata/reports/item_sales.json:14`

```
DECODE FAILURE: freshbooks: decoding the response: json: cannot unmarshal
string into Go struct field ItemSalesReport.item_sales.total_qty of type int
```

The captured `Reports/Item Sales` body sends `"total_qty": "20"` -- a quoted decimal string, exactly the same shape as the per-item `ItemSale.TotalQty` (`"2"`). The new field is `TotalQty int`, and the reseeded fixture was hand-authored as an unquoted `20`, so the unit test passes while the real captured body fails.

**This one is on me.** Round 1's BLOCKING-1 said "`total_qty` = `20` (bare number -- note `ItemSale.TotalQty` is a **string** `"2"` at line level, so this needs its own type)". That parenthetical was wrong: my inspection script printed the value through `print()`, which strips the quotes off a string, and I read `20` as a number. The implementer implemented the report as written and reseeded the fixture to match it, which is the correct thing to do with a QA finding. The acceptance probe -- run against the collection rather than against the fixtures -- is what caught it, which is exactly why that probe is mandatory.

Fix, verified against the real captured body before filing:
- `TotalQty string \`json:"total_qty"\`` (`total` and `total_discount` as `*Money` are correct as shipped)
- `testdata/reports/item_sales.json`: `"total_qty": 20` -> `"total_qty": "20"`
- invert the field's doc-comment rationale: it *is* the same shape as `ItemSale.TotalQty`, not a distinct one

```
PROPOSED FIX VERIFIED: total=23320.00 USD, total_discount=0.00 USD, total_qty="20"
```

## 10.3 Round-1 findings: closed / not closed

Decode probe re-run against the **captured collection bodies** (not the fixtures) for every touched read model, diffing each captured object's top-level keys against the re-marshalled decoded value:

| Round-1 finding | Model | Result |
|---|---|---|
| B-1 ItemSalesReport | `Reports/Item Sales` | **FAIL -- see R-1.** Fields are all present and correctly named; only `TotalQty`'s Go type is wrong |
| B-2 TrialBalanceReport | `Reports/Trial Balance` | `DROPS 0` -- `download_token`, `start_date`, `end_date` all land |
| B-3 StripeConnection.max_ach_fee | Tokenization/1a capture | `stripe DROPS 0 (9 captured keys all modeled)`; `MaxACHFee = 5000` asserted against the captured value |
| B-4 GatewayConnection.paypal | Tokenization/1a capture | `DROPS 0`; `PayPal json.RawMessage` preserves the captured `null` verbatim |
| B-5 report options | 5 new structs | **all pass -- see 10.4** |
| ADV-1 action_reasons | Tokenization/1a capture | `fbpay DROPS 0 (13 captured keys all modeled)` |
| ADV-2 TimeEntryDetails | `Reports/Time Entry Details` | `DROPS 0`; `download_token` + `aggregations` modeled, and `TimeEntryDetailsMeta` embeds `PageMeta` and adds `TotalLogged`/`TotalUnbilled` **without** widening the shared `PageMeta` -- the shape I recommended |
| ADV-3 callback_id vs docs | `callbacks.go` + spec 5.1 | closed -- see 10.5 |
| ADV-4 successor endpoints | 3 doc comments + spec 5.1 | closed -- see 10.5 |

The four report methods whose signatures changed but whose models did not (`InvoiceDetails`, `PaymentsCollected`, `ProfitLoss`, `AccountsAging`) were re-run against their captures as a regression check: `DROPS 0` on all four. **9 of 10 re-checked captures decode clean; Item Sales is the sole failure.**

Nested-object diffs were run separately for `fbpay` and `stripe`, because an array-element diff only sees the three gateway keys and would have missed exactly the kind of nested drop B-3 was.

## 10.4 The five new report options structs encode the documented parameters

Each struct driven at full occupancy through `httptest`, asserting the literal query string, then re-called with a `nil` options pointer:

```
InvoiceDetails    clientids=1%2C2&currency_code=USD&date_type=creation&end_date=2026-12-31&start_date=2026-01-01&statusids=3%2C4
PaymentsCollected clientids=1%2C2&currency_code=USD&end_date=2026-12-31&locale=en&payment_methods=visa&start_date=2026-01-01
ProfitLoss        currency_code=USD&end_date=2026-12-31&start_date=2026-01-01
AccountsAging     currency_code=USD&end_date=2026-12-31&start_date=2026-01-01
ExpenseDetails    currency_code=USD&end_date=2026-12-31&start_date=2026-01-01
```

`InvoiceDetails` and `PaymentsCollected` carry exactly the six parameters https://www.freshbooks.com/api/reports lists for each, no more and no fewer, with `clientids`/`statusids`/`payment_methods` comma-separated per `ItemSalesOptions`' existing convention. The three reports the docs do not cover took the conservative `start_date`/`end_date`/`currency_code` triple every sibling uses, and each says so in its doc comment rather than guessing wider. `ExpenseDetailsOptions` explicitly declines the docs page's fuller list (`group_by`, `exclude_personal`, ...) because that page documents a different, business_uuid-scoped endpoint -- correct call, correctly explained. `nil` options sends an empty query on all five and does not panic.

## 10.5 Fixtures, spec, and doc comments

**Fixtures match the captures**, with two deliberate and correct divergences:
- `testdata/reports/time_entry_details.json` sets `download_token: "tok_time_entry_details"` and `meta.total_logged/total_unbilled` to `3`/`2`, where the capture has `null`/`0`/`0`. Seeding the capture's zero values would have let a missing struct field pass vacuously -- populating them is the right anti-vacuous move, and the *decode* correctness of the null/zero case is covered by the probe running the real capture.
- `testdata/gateways/get.json` gains `action_reasons: []` and `max_ach_fee: 5000`, matching the capture; `gateways_test.go` asserts the value rather than non-nil.
- `item_sales.json` and `trial_balance.json` gain the previously-missing keys with the captured values -- correct except `total_qty`'s quoting (R-1).

**Spec 5.1** gains two new blocks under the batch-d callout: the `callback_id` body conflict (Postman carries it, the docs page does not; Postman kept per the parity contract, and the redundancy called out as low-risk since the id is already in the path), and a three-bullet block naming each docs page that has moved past the collection -- journal entries, chart-of-accounts list only, and profit-and-loss/trial-balance -- with both the documented and the implemented path for each. Accurate against what I read on those pages, and it correctly notes that chart-of-accounts create and get-single still match the docs exactly.

**Doc comments** back-reference the callout from `Callbacks.Verify`, `Callbacks.ResendVerification`, `JournalEntries.Create`, and `LedgerAccounts.List`. `ledger_accounts.go`'s addition also volunteers that Create and Get match the docs -- the useful half a reader needs.

## 10.6 Note, not a finding

`StripeConnection.MaxACHFee` is a non-pointer `int`, and the key is absent (not null) from the other two gateway captures, so "absent" and "zero" are indistinguishable. Every sibling scalar on that struct behaves the same way, so this is package convention, not a defect -- worth a pointer only if a zero ACH fee ever becomes meaningful.

## 10.7 Commands run (round 2)

```
mise run check > /tmp/qa-d-check2.log 2>&1; echo "EXIT=$?"   # EXIT=0
mise exec -- go test -run TestQARecheckTouchedReadModels -v ./freshbooks/
mise exec -- go test -run TestQARecheckGatewayNestedObjects -v ./freshbooks/
mise exec -- go test -run TestQARecheckReportOptionsEncoding -v ./freshbooks/
mise exec -- go test -run TestQATotalQtyString -v ./freshbooks/    # proposed-fix check
rm -f freshbooks/zzqare_test.go freshbooks/zzqatq_test.go          # tree clean
```

## 10.8 Recommendation

One field's type and one fixture character. Re-run the gate, re-run the Item Sales case against the capture, merge -- that closes batch d and all 213 keys. Nothing else in `d100999` needs to change: it answered all nine findings correctly, chose the report-local meta type over widening `PageMeta`, and declined to over-model the endpoints where the docs describe a different API. The one regression traces to a factual error in my round-1 report, not to the implementer's work.

---

# 11. Final confirmation -- 2026-09-01, round 3

Subject: `e34b7fc fix(lib-resources-d): correct ItemSalesReport.TotalQty to the captured string type`, on top of `d100999`. Single item: does R-1 close?

**FINAL VERDICT: PASS.**

## 11.1 The fix is exactly the one filed, and nothing else moved

`git diff d100999..e34b7fc` touches three files and nine lines:

- `reports.go`: `TotalQty int` -> `TotalQty string`, and the doc comment inverted to say what is actually true ("a decimal string in the capture -- the same shape as `ItemSale.TotalQty`, the per-item count"). The wrong rationale my round-1 report introduced is gone rather than left to mislead the next reader.
- `reports_test.go`: `got.TotalQty != 20` -> `got.TotalQty != "20"`, with the failure message corrected to "want the captured decimal string".
- `testdata/reports/item_sales.json`: `"total_qty": 20` -> `"total_qty": "20"`.

No other field, fixture, signature, or doc comment changed. `Total`/`TotalDiscount` stay `*Money`, `ItemSale.TotalQty` stays the per-item string.

## 11.2 Item Sales decodes from the capture, with zero drops

Throwaway probe (deleted; tree clean) drove the **captured** `Reports/Item Sales` body -- not the fixture -- through `ReportsService.ItemSales`, after first asserting the capture really does still carry the quoted `"total_qty":"20"` so the check cannot pass vacuously:

```
DROPS 0 -- all 12 captured item_sales keys survive
OK: total=23320.00 USD discount=0.00 total_qty="20" items=9 per-item qty="2"
--- PASS: TestQAFinalItemSalesCapture
```

Decode succeeds. All 12 top-level keys of the captured `item_sales` object survive into the decoded value. Values asserted, not just presence: `total` = 23320.00 USD, `total_discount` = 0.00, `total_qty` = `"20"`, `start_date`/`end_date` = 2019-01-01 / 2019-12-31, `download_token` non-empty, 9 items decoded, and `ItemSale.TotalQty` still `"2"` -- the per-item field was not collaterally changed.

That was the last outstanding case. Combined with round 2's nine other re-checked captures (`DROPS 0` on all nine) and round 1's full sweep, **every captured batch-d response body now decodes through the library's own read models with zero failures and zero silent drops.**

## 11.3 Gate (round 3, run once on the clean tree, exit captured directly)

```
$ mise run check > /tmp/qa-d-check3.log 2>&1; echo "EXIT=$?"
EXIT=0
```
```
coverage-gate: .../freshbooks/coverage.out total = 91.8% (floor 90%)   PASS
coverage-gate: .../mcp/coverage.out       total = 100.0%               PASS
coverage-gate: .../cli/coverage.out       total = 100.0%               PASS
implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0
No vulnerabilities found.  (x3)
check.sh: all OK
EXIT=0
```

`git status --porcelain` empty for the gate run; this report was moved aside and restored afterwards and is the one dirty file at hand-off.

## 11.4 Standing evidence for the PASS

Nothing below was re-run this round -- it is carried forward from sections 1-10, all against commits this one builds on without modifying:

- **Gate:** green on a clean tree, three rounds running. freshbooks 91.8%, mcp 100%, cli 100%.
- **Parity: 213 / 213 keys**, 0 todo, 0 ignored, 0 uncovered, 0 double-covered. Phase 2's goal is mechanically complete; there is no ignore list to audit.
- **Acceptance test:** 38 captured batch-d bodies decoded in round 1 (0 failures), 10 touched/re-signatured models re-checked in round 2 (`DROPS 0` on 9, Item Sales deferred), Item Sales confirmed here. No captured field is silently discarded anywhere in the batch.
- **Request side:** multipart replay byte-identical across a 429 retry with the base filename only; both tokenization endpoints forced to `https://paid.freshbooks.com` from a plaintext base URL with the base query stripped; no PAN, CVV, or API key in any render path of either tokenize request; `sub_accountid` quoted on the request and `int64` on the response.
- **Fidelity:** six docs pages checked request by request -- expense-attachments exact, all five webhook paths and verbs, other-income CRUD and all 13 documented fields, chart-of-accounts create/get-single, tax summary, and all 12 report path segments matching the collection.
- **Process:** all twenty round-1 triage fixes (F1-F20) verified landed, including both recorded overrides; all nine round-2 QA findings verified closed; spec 5.1's batch-d callout records the envelope resolutions, the `ledger` family misclassification the batch self-reported, the `callback_id` docs conflict, and the three docs pages that have moved past the collection.

## 11.5 Verdict

**PASS.** Batch d is ready to merge, and merging it closes Phase 2 at 213/213 inventory keys.

Two notes carried forward, neither blocking and neither for this batch:

- `StripeConnection.MaxACHFee` is a non-pointer `int` on a key that is absent from two of three gateway captures, so absent and zero are indistinguishable. Package convention; revisit only if a zero ACH fee becomes meaningful.
- Everything docs-confirmed this phase remains **docs-confirmed, not live-confirmed** -- Phase 2 ran unattended with no sandbox account. The spec's `STATE AS OF 2026-09-01` callouts say so throughout. The live-conformance pass still owes: the `callback_id` body, the journal-entries and chart-of-accounts-list successor endpoints, the tokenization shapes (no public docs at all), and the three ledger taxonomy endpoints returning `json.RawMessage` on zero evidence.
