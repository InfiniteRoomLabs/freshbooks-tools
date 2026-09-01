# Phase 2 batch d -- gate triage

Lead triage of the three read-only lane reports (`d-code-review.md` REQUEST CHANGES 5 blocking / 11 advisory; `d-simplify.md` 8 apply-recommended / 4 optional; `d-security.md` BLOCK 3 blocking / 5 advisory). Convergences: review B-1 == security 3 == simplify 6 (pathSegment); review B-5 == simplify 4 half (doRaw); review A-5 == simplify 4. One fix commit: `fix(lib-resources-d): apply the review-gate findings`. QA runs after.

The transport merge was reviewed clean (no retry/error/bounded-read guarantee lost); its leftover adapters get cleaned up in F6.

## Fix list (apply ALL, one commit; ordering hint at the bottom)

- **F1** (review B-1 + security 3 + simplify 6): `pathSegment` on EVERY caller-supplied string segment across the nine files -- `string(acct)`, `string(biz)` (`BusinessUUID` is a string), `accountUUID`, sub-type `id`, `downloadToken`. Shared builders (`otherIncomePath`, `callbacksPath`, the new `reportPath`, ledger builders) become `(string, error)`; single-use methods guard inline per the `payments.go` shape. One `[sad]` hostile-segment test per affected service.
- **F2** (security 1 + security 5 + review A-10): `doOnHost` forces `u.Scheme = "https"` (the host is already forced -- the scheme must be too; card data + bearer must never go plaintext to the real host), adds `noTraversal(path)` and `u.RawQuery = ""`, and gains a comment noting it bypasses `resolve` deliberately for constant paths. `WithBaseURL`'s doc comment states that the tokenization methods do not honor the configured base URL. Fix the test that relied on plaintext (httptest TLS server or transport seam).
- **F3** (security 2 + security 4): `FBPayTokenizeRequest` and `StripeTokenizeRequest` get redacting `String()` (card number, CVV, api key -> `redacted`), mirroring `Application.String()`; tests assert the fixture PAN does NOT appear in the rendered string; both structs' doc comments state they carry PCI-sensitive data that must never be logged, persisted, or embedded in errors.
- **F4** (security 6): test card numbers become the canonical synthetic PANs (`4111111111111111` / `4242424242424242`), and `pk_live_example` becomes `pk_test_example`.
- **F5** (simplify 1 + 2 + 3): `ReportsService.get` private helper collapses the 12 path/decode preambles; `setNonEmpty` collapses the four option-value chains; `boolQuery` -> `strconv.FormatBool`.
- **F6** (review B-5 + review A-4 + review A-5 + simplify 4): delete `doRaw` and `sendRaw` (fold into `send` taking the `(ctx, authorization)` closure shape); `DownloadCSV` calls `fetchRaw(..., "text/csv")` directly -- a CSV endpoint must not ask for JSON; rewrite `newRequest`'s contentType doc comment to match the code ("nil payload sends no body; non-nil payload always sent; contentType set only when non-empty").
- **F7** (review B-2): surface the captured sibling `link` on image uploads -- add `Link` to the result the caller receives (populate from the sibling key after decode) so the doc comment and struct agree and the only usable URL stops being discarded.
- **F8** (review B-3): `JournalEntryDetail.SubAccountID` (request side) -> `string` per the captured request body; test asserts the quoted form. `JournalEntryDetailResult` stays `int64`.
- **F9** (review B-4): `StripeTokenizeRequest.APIKey` tagged `json:"-"` so it appears only at the body's top level, matching the capture.
- **F10** (simplify 5): `other_income.go` and `callbacks.go` use `newPage`.
- **F11** (simplify 7): the nine `familyForPath(path)` call sites pass the `FamilyBusiness` constant they provably resolve to.
- **F12** (simplify 8): delete `TimeEntryAbility`; use the existing `Ability`.
- **F13** (security 7 + review A-8, overriding simplify 9's lean-keep): `buildMultipartBody` applies `filepath.Base(filename)`; the dead `fields map[string]string` parameter is dropped from `doMultipart` (re-add when an endpoint actually sends form fields).
- **F14** (review A-1): `Callbacks.Verify` and `ResendVerification` include `callback_id` in the body per both captured request bodies.
- **F15** (review A-2): `OtherIncomeUpdateRequest` -- pointers for the remaining clearable scalars (`CategoryName`, `Date`, `PaymentType`, `Source`) for one consistent partial-update convention, and doc comments on the fields.
- **F16** (review A-3): add the captured-but-dropped fields: `GatewayPricing` (ach tiers, `default_pricing_tier_id`, `promo_expiry_date`, `max_ach_fee`), `FBPayConnection.BankInfo`, `InvoiceDetailsReport.Clients`, `TimeEntryDetailEntry.{Timer,IsLogged,PendingClient,PendingProject,PendingTask,Highlight}`, `LedgerAccountUpdateRequest.SubAccounts`. Fixtures extended from the captured shapes so each decodes at least once.
- **F17** (review A-6): rename `DownloadCSV` -> `DownloadInvoiceDetailsCSV` (the URL hardcodes `invoice_details.csv`; the generic name invites wrong usage). Unreleased API; rename is free.
- **F18** (review A-7): ledger taxonomy `Types`/`SubTypes`/`SubType` return `json.RawMessage` per the batch's own zero-evidence policy; delete `LedgerAccountSubType`.
- **F19** (review A-9): extract named types for `JournalEntryDetailEntry`'s three anonymous nested structs.
- **F20** (review A-11 + simplify 11, approved signature change matching batch c's convergence): `CreditCardToken.IsPrimary` loses `omitempty` (or `*bool`); `OtherIncome` and `Callbacks` converge on the package list shape -- `XListOptions` + `opts()` via `listOpts`, `List(ctx, acct, opts *XListOptions, extra ...RequestOption)`, plus `All` iterators (both endpoints return page metadata). Name the signature change in the commit body.

## Explicit non-applies (do NOT do)

- simplify 10 (`OtherIncome.Delete` via `softDelete`): agreed leave -- it returns the updated resource; `softDelete` would drop data.
- simplify 12 (table-driving three report subtests): skip.
- simplify 13-19: agreed DO-NOT-APPLY as filed; do not re-derive.
- security 8 (govulncheck local): `mise run vuln` covers it in QA/CI.

## Lane verdict reconciliation

Fourth consecutive batch where all lanes converge on the pathSegment gap -- the sweep (F1) completes coverage for the whole library at 213/213 keys. Security 1 (plaintext scheme on the card path) is the standout catch of the phase; review's B-2/B-3/B-4/B-5 are all captured-evidence wire defects in the class the prior QA passes established. Simplify's transport cleanup (F6) also deletes the leftover adapters from the lead's rebase merge -- accepted with the review lane's Accept-header fix folded in. No lane-vs-lane conflicts; one lead override recorded (F13 drops the dead multipart fields param against simplify's lean-keep).
