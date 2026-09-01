# Phase 2 batch d -- code-review lane

(Report delivered via SendMessage; transcribed to file by the lead. Content verbatim.)

Branch `phase-2/d` (5 commits, `f8e68ac..f89178f`) vs `main`, worktree `.worktrees/d`. Read-only pass: `git`, `grep`, file reads, and one `uv run python` extraction of the Postman collection's captured requests/responses. No gate runs.

Scope reviewed: the hand-resolved `transport.go` merge; `client.go`'s `familyForPath` change; the 9 new resource files (ledger accounts, journal entries, other income, reports, callbacks, attachments, images, gateways, payment options); their tests and fixtures; `services.go` doc edits; the `ignore.list` delta.

**Verdict: REQUEST CHANGES.** Five blocking findings, eleven advisory. The transport merge itself is sound -- no retry, error-decode, or bounded-read guarantee was lost. The blocking findings are in the new resource code: one systemic convention break repeated across all nine files, and four concrete mismatches against the collection's captured evidence.

## Transport merge review

Traced every path. No guarantee lost.

- All four entry points funnel into one retry loop: `do` / `doOnHost` / `doMultipart` -> `send` -> `sendRaw` -> `attemptLoop`; `fetchRaw` -> `attemptLoop` directly. Every caller gets identical 429/502/503/504 handling, identical `Retry-After` parsing, identical context-cancellation exit, and identical single-resolution of the access token before the loop -- the property that keeps a rotating refresh token from being spent twice.
- Replayable bodies survive: `do`/`doOnHost` marshal to `[]byte` once outside the loop; `doMultipart` materializes the multipart body up front and replays its bytes; the `multipart.Writer` is never reused.
- Bounded reads survive on both paths (`maxResponseBytes` with the `+1` overflow probe in `roundTrip`; `maxUploadBytes` uses the same idiom).
- `newRequest`'s new `contentType` param is wired correctly at all four call sites -- but its doc comment is wrong (A-4).
- Dead code: `sendRaw` and `doRaw` are each single-caller pass-throughs (A-5); `doRaw` hides blocking bug B-5.

## BLOCKING

- **B-1. No `pathSegment` validation anywhere in batch d** -- 9 files, ~30 interpolation sites (`ledger_accounts.go:111,124,138,153,204`, `journal_entries.go:116,133,178`, `other_income.go:90`, `callbacks.go:61`, `attachments.go:36`, `images.go:30`, `gateways.go:65`, `payment_options.go:153,174`, `reports.go` x13). `string(acct)`, `string(biz)` (`BusinessUUID` is a string type), `accountUUID`, `downloadToken`, and the sub-type `id` all reach the path unvalidated; `noTraversal` catches only `.`/`..`. Fix: the standard guard everywhere, `(string, error)` builders where shared, `[sad]` tests per service.
- **B-2. `UploadedImage` documents a `Link` field that does not exist**, and the captured response's sibling `link` (the only usable URL for the uploaded asset) is silently discarded by both `Upload` and `UploadWithoutAccount`. Fix: decode and surface `link`; make doc comment and struct agree.
- **B-3. `JournalEntryDetail.SubAccountID` is `int64`; the captured request body sends `"sub_accountid": "635974"` -- a quoted string** (request side; the response's unquoted form is handled by the separate result type correctly). Fix: `string` on the request type; test asserts the quoted form.
- **B-4. `StripeTokenizeRequest` sends `api_key` twice** -- inside `cc_info` (via the struct's json tag) and at the top level (via the wrapper). The capture has it top-level only. Fix: tag the field `json:"-"`.
- **B-5. `DownloadCSV` asks a CSV endpoint for JSON**: `doRaw` hardcodes `accept=""` so `newRequest`'s `Accept: application/json` stands. Fix: delete `doRaw`; call `fetchRaw(..., "text/csv")`.

## ADVISORY

- **A-1** Callbacks `Verify`/`ResendVerification` omit `callback_id`, which both captured bodies carry.
- **A-2** `OtherIncomeUpdateRequest` mixes pointer and non-pointer partial-update fields; four scalars cannot be cleared.
- **A-3** Read models silently drop captured fields: `GatewayPricing` (ach tiers, `default_pricing_tier_id`, `promo_expiry_date`, `max_ach_fee`), `FBPayConnection.bank_info`, `InvoiceDetailsReport.clients`, `TimeEntryDetailEntry` (`timer`, `is_logged`, `pending_*`, `highlight`), `LedgerAccountUpdateRequest.sub_accounts`.
- **A-4** `newRequest`'s contentType doc comment describes behavior the code does not have.
- **A-5** `sendRaw`/`doRaw` are single-caller pass-throughs left from the merge; fold/delete.
- **A-6** `DownloadCSV` is named generically but hardcodes `invoice_details.csv`; rename or parameterize.
- **A-7** Ledger taxonomy methods invent typed shapes with zero evidence, against the batch's own `json.RawMessage` policy; recommend RawMessage + delete `LedgerAccountSubType`.
- **A-8** `doMultipart`'s `fields` parameter has no production caller.
- **A-9** `JournalEntryDetailEntry` nests three anonymous structs on an exported type; extract named types.
- **A-10** `doOnHost` sends the bearer to a hard-coded external host that `WithBaseURL` cannot redirect; document, and guard/route its path handling before anyone parameterizes it.
- **A-11** `CreditCardToken.IsPrimary` is a toggle bool with `omitempty`.

## Checked and clean

`familyForPath` ordering correct (ledger before general `/accounting/`; `/events/` accounting confirmed by capture; `/uploads/`+`/payments/` business confirmed). Inventory mapping spot-checks pass (attachments vs images upload split is real; FBPAY/STRIPE SaveCreditCard collapse correct; Expense Details follows Postman correctly). Multipart field name `content` matches all five upload captures. Report option structs nil-safe; envelope keys verified against captures for every typed report. `Callback.UnmarshalJSON` alias idiom correct. `LedgerAccount.UpdatedAt time.Time` correct (RFC 3339). Nullable ids are pointers matching captured nulls. Ledger create/update flat bodies match captures. Tests substantive, no skips/filters; multipart tests parse the body back properly. `ignore.list` delta exactly 52 lines.
