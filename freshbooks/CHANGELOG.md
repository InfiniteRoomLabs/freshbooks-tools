# Changelog

All notable changes to the `freshbooks` module are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `InvoicesService` (`List`, `All`, `Get`, `Create`, `Update`, `Delete`,
  `Send`, `PDF` (raw bytes over the shared retry/backoff transport, not a
  separate one-shot fetch), `ShareLink`, `EnablePaymentOptions`,
  `InvoicePresentationDefaults`), `InvoiceProfilesService` (recurring invoice
  templates: `List`, `All`, `Get`, `Create`, `Update`, `Delete`,
  `EnablePaymentOptions`), `ItemsService` (catalogue items, shared by the
  `Invoices/Items and Services` and `Settings/Items and Services` Postman
  folders), `PaymentsService` (invoice payments plus FreshBooks Payments
  checkout links: `CreateCheckoutLink`, `UpdateCheckoutLink`,
  `DeleteCheckoutLink`, `UpdateCheckoutLinkGateway`), and `RetainersService`
  (business-scoped recurring minimum-fee arrangements: `List`, `Get`,
  `Create`, `Update`, `Delete`, `Undelete`) -- Phase 2 batch a, 51 inventory
  entries.
- Transport: `(*Client).softDelete` for the accounting family's
  vis_state-PUT soft delete, and a `pathSegment`/`noTraversal` guard that
  rejects a caller-supplied path segment (an `AccountID`, a checkout-link
  id) carrying a slash, `?`, `#`, or a `.`/`..` traversal before it reaches
  the request path.
- `ClientsService` (`List`, `Get`, `Create`, `Update`,
  `RemoveAllSecondaryContacts`) and `ContactsService` (`Update`, `Delete`)
  covering all 13 `Clients/*` inventory keys, with the client resource named
  `Customer` in Go to avoid colliding with the library's own `*Client` type.
- `CreditNotesService` (`List`, `Create`, `Update`, `Delete`) covering the
  `Clients/Credits/*` subfolder's 6 keys, `CreditType` distinguishing
  goodwill, prepayment, and overpayment credits.
- `EstimatesService` (`List`, `Get`, `Create`, `Update`, `Delete`, `Accept`,
  `Send`) covering all 8 `Estimates/*` inventory keys.
- `ExpensesService` (`List`, `Get`, `Create`, `Update`, `Delete`,
  `Summaries`, `Vendors`, `CreateRecurring`) and `ExpenseCategoriesService`
  (`List`, `Get`, `Create`) covering 13 of the `Expenses/*` inventory keys.
- `TaxesService` (`List`, `Get`, `Create`, `Update`, `Delete`) covering the
  tax-rate operations duplicated across `Expenses/*`, `Accounting/Taxes/*`,
  and `Settings/Items and Services/*` -- one method per operation with all
  three inventory keys stacked, following the `IdentityService.Me`
  precedent. `Delete` is a real HTTP `DELETE`, unlike every other
  soft-deleting resource in this batch.
- `BillsService` (`List`, `Create`, `Archive`, `Delete`), `BillPaymentsService`
  (`Create`, `Update`), and `BillVendorsService` (`List`, `Create`, `Update`,
  `Delete`) covering the Beta `Bills (Beta)` and `Vendors (Beta)` subfolders
  (14 keys).
- `LedgerAccountsService` (chart of accounts + type taxonomy,
  `BusinessUUID`-scoped: `Create`, `List`, `Get`, `Update`, `Types`,
  `SubTypes`, `SubType`), `JournalEntriesService` (`Create`, `Details`),
  `JournalEntryAccountsService.List` (the shared Accounting/Journal
  Entries/Accounts + Reports/General Ledger endpoint), and
  `OtherIncomeService` (`Create`, `List`, `Update`, `Delete`), covering the
  14 non-tax `Accounting` inventory entries plus their 4 `Invoices/Other
  Income` duplicates.
- `ReportsService`: all 13 accounting reports (`AccountsAging`,
  `BalanceSheet`, `BankReconciliationSummary`, `ClientAccountStatement`,
  `DownloadCSV`, `ExpenseDetails`, `InvoiceDetails`, `ItemSales`,
  `PaymentsCollected`, `ProfitLoss`, `RevenueByClient`, `SalesTaxSummary`,
  `TrialBalance`) plus the business-scoped `TimeEntryDetails`, covering all
  15 `Reports` inventory entries. Reports with no Postman example and no
  matching public docs page return `json.RawMessage` rather than a guessed
  struct.
- `CallbacksService` (webhook subscriptions: `Register`, `List`, `Delete`,
  `Verify`, `ResendVerification`), covering all 5 `Webhooks` entries.
- `AttachmentsService.UploadExpenseReceipt` and `ImagesService` (`Upload`,
  `UploadWithoutAccount`), covering all 3 `Uploader` entries plus their 3
  cross-folder duplicates (`Invoices/Upload Logo`, `Expenses/Upload
  Expense Receipt Image`, `Settings/Developer/Upload App Logo`).
- `GatewaysService.Get` and `PaymentOptionsService` (`FBPayTokenize`,
  `StripeTokenize`, `StripeCreateSetupIntent`, `SaveCreditCard`), covering
  all 6 `Tokenization` entries plus their 2 `Settings` gateway duplicates.
- Transport: `doMultipart` for the `/uploads/` endpoints (multipart/form-data
  with a 10MB upload bound), `doOnHost` for FreshBooks' card-tokenization
  host (`paid.freshbooks.com`, distinct from the API base URL), and `doRaw`
  for endpoints that answer a file instead of JSON (`Reports.DownloadCSV`).
- Core client: `Client` with all 36 resource services declared as fields,
  `NewClient` and the `With*` options (`WithTokenSource`, `WithHTTPClient`,
  `WithBaseURL`, `WithUserAgent`, `WithLogger`, `WithRetry`, `WithClock`),
  and `(*Client).Do` as the escape hatch for unmodelled endpoints.
- Transport: a single `do()` path with family-aware query encoding, accounting
  and auth envelope unwrapping, family-specific error decoding, jittered
  exponential backoff on 429/502/503/504 honouring `Retry-After`, context
  cancellation during backoff, a 10MB response cap, and `Authorization`
  stripped on cross-host redirects.
- Types: `AccountID`, `BusinessID`, `BusinessUUID`, `Money` with `Rat()`,
  `Date` and `DateTime` covering all three FreshBooks wire formats,
  `VisState`, and the `Include` / `Search` / `Sort` / `PageNumber` / `PerPage`
  request options.
- Errors: `*Error` with `errors.Is` sentinels (`ErrUnauthorized`,
  `ErrForbidden`, `ErrNotFound`, `ErrValidation`, `ErrRateLimited`) and
  `RetryAfter()`.
- Pagination: `Page[T]`, `PageMeta`, and the `All` iterator (`iter.Seq2`).
- `IdentityService` with `Me`, `Whoami`, and `Register`, covering the four
  `Authorization` inventory entries together with `auth.Config.Revoke`.
- `freshbooks/auth`: the PKCE authorization-code flow (`AuthCodeURL`,
  `Exchange`, `Refresh`, `Revoke`), both live-verified endpoint sets,
  `Token` / `TokenSource` / `StaticTokenSource`, `NewTokenSource` with expiry
  skew, single-flight refresh, and rotation write-back before return, plus
  `FileStore` (0600 in a 0700 directory, temp + rename) and `MemoryStore`.
- Repository scaffold: module skeleton, doc.go package overview, and the
  `freshbooks/internal/inventory` tool that normalizes the FreshBooks
  Postman collection into a parity contract for future phases.
- Phase 2 batch c (Projects + Time Tracking + My Team + Settings, 47
  inventory keys): `ProjectsService` (projects, abilities, discussion
  threads and comments), `TasksService`, `TimeEntriesService` (including the
  free-text `Search` endpoint), `ServicesService` and `ServiceRatesService`
  (spanning the business-family service catalogue and the accounting-family
  billable-items resource the same folder name conflates), `TeamMembersService`
  (team members, invitation and per-identity rates, invites),
  `StaffService` (the deprecated Staff resource plus the auth-family
  business-group member list), `SystemsService.Get`, and
  `IdentityService.{AddBusiness,DeleteBusiness,DeleteBusinessSubscription,
  ProvisionPayments,CreateApplication,Applications,UpdateApplication}` for
  the Settings/Businesses and Settings/Developer keys, which have no
  dedicated pre-declared service.
- `DateTime` now accepts a fourth wire format, a zoneless
  `"YYYY-MM-DDTHH:MM:SS"` timestamp observed in the Projects/Time Tracking
  Postman examples (e.g. Projects' `created_at`/`updated_at`), alongside the
  three documented formats.

### Fixed

- `InvoiceListOptions` no longer declares a `Sort` field that `List`/`All`
  never encoded; pass `Sort(field, dir)` through the `extra ...RequestOption`
  variadic instead. `RetainerListOptions` no longer declares `Page`/`PerPage`:
  FreshBooks' captured example response for that endpoint carries no `meta`
  pagination block to page against.
- Six write-path struct fields (`Money`/`Date` values on `InvoiceLine`,
  `InvoiceCreateRequest`, `InvoiceProfileCreateRequest`,
  `PaymentCreateRequest`, `ItemCreateRequest`) used `omitempty`, which is a
  no-op on a struct; an unset field serialized as `null` or an empty
  `Money{}` instead of being omitted. Switched to `omitzero`.
  `PaymentOptionsRequest`'s three booleans lost their `omitempty` for the
  opposite reason: it silently dropped an explicit `false`, so a caller
  could not turn a payment method off.
- `RetainerUpdateRequest.Active` is now `*bool`; a partial update that only
  set `Fee` was posting an implicit `active: false` and deactivating the
  retainer.
- `Payments.UpdateCheckoutLinkGateway` now sends `entity_type`/`entity_id`,
  the only body fields FreshBooks documents for that endpoint, derived from
  the id argument instead of being silently omitted.
- `Payments.CreateCheckoutLink`/`UpdateCheckoutLink` decode the enveloped
  shape, then a flat object, and return an error if neither yields an id,
  instead of echoing the caller's own request back as if it were the
  server's answer.
- `auth.Token.String` now takes a value receiver, so `%v` on a `Token` value
  (or on a struct that embeds one) redacts the credentials instead of
  printing them.
- A `TokenStore.Save` failure after a successful refresh no longer discards
  the rotated pair. The source keeps it and retries the save on the next
  `Token` call, so a transient store failure is recoverable rather than a
  forced re-authentication.
- The `auth` package no longer falls back to `http.DefaultClient`: its
  default has a 30s timeout and refuses to follow redirects, which would
  otherwise replay the client secret and refresh token to the redirect
  target.
- `*Error.Family` is now the family the request was built for rather than one
  re-derived from the request path, which disagreed under a `WithBaseURL`
  path prefix.
- Webhook callback paths (`/events/`) are classified as the accounting family
  so their envelope is unwrapped.
- Ledger-account paths (`/accounting/businesses/.../ledger_accounts/...`,
  `/accounting/ledger_accounts/{types,sub_types}`) are now classified as the
  business (flat, no envelope) family instead of falling into the general
  `/accounting/` accounting-enveloped case. They were never reachable before
  this phase, so this is not a behavior change for released code, but the
  double-unwrap would have discarded their actual `{"data": ...}` bodies.
- The client's redirect cap returns a real error instead of handing back the
  final 3xx as a response.
