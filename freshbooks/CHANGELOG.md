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
- The client's redirect cap returns a real error instead of handing back the
  final 3xx as a response.
