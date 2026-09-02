# freshbooks-tools design

Status: Approved 2026-08-22 (brainstormed with Wes; sections A-E' signed off in chat).

## 1. Purpose

A Go monorepo with three MIT-licensed subprojects for the FreshBooks REST API (https://www.freshbooks.com/api/start):

1. `freshbooks` -- a handcrafted, canonical-Go client library covering the full API surface.
2. `freshbooks-mcp` -- a stateless MCP server (stdio + streamable HTTP) exposing the library 1:1.
3. `freshbooks` CLI -- a kubectl-style cobra CLI exposing the library 1:1.

Why we build instead of adopt: nothing in Go targets the current OAuth2 REST API (the only Go client is for the dead classic API); existing MCPs are TypeScript/Python and stateful; and Infinite Room Labs wants portfolio-grade code it owns to run its own books.

## 2. Decisions locked (do not re-litigate)

| Decision | Choice | Why |
|---|---|---|
| Topology | One monorepo, Go workspace, three modules | Lib version bumps are one PR; "upstream lib CI" is a job dependency, not a cross-repo poll |
| Module paths | `github.com/InfiniteRoomLabs/freshbooks-tools/{freshbooks,mcp,cli}` | Org precedent (`push-it`); lib imports as `freshbooks` |
| Tags | `freshbooks/vX.Y.Z`, `mcp/vX.Y.Z`, `cli/vX.Y.Z` | Go multi-module convention; each module releases independently |
| Binaries | `freshbooks` (CLI), `freshbooks-mcp` | |
| Codegen | None. Postman collection -> inventory tool -> agent work orders -> handcrafted code | Collection has no response schemas; generated code would be type-thin and still need full rewrite |
| Docs | `docs/*.md` + pkg.go.dev | Zero tooling; Docusaurus is Future Work |
| jsonpath output | Not implemented | Tarpit; `-o json \| jq` covers it |
| Token ownership | Lib `TokenSource` + `TokenStore`; CLI owns login + credential file; MCP is a token consumer only | Refresh tokens are one-time-use; stateless MCP cannot own rotation |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` v1.7.0+ | Required; has `StreamableHTTPOptions{Stateless: true}` |
| Process | GOAL.md treadmill + four-lane review gate (from hoyle-re) | Proven; see section 9 |
| License | MIT, every module | |
| Visibility | Public from the first commit | Portfolio piece; public-repo hygiene applies to every commit |

## 3. FreshBooks API facts the design depends on

Verified 2026-08-22 against https://www.freshbooks.com/api/authentication, the Postman collection (130 requests, 14 folders), and `https://auth.freshbooks.com/.well-known/oauth-authorization-server`.

> **STATE AS OF 2026-08-22 (Phase 0, stage 1):** the "130 requests" figure counted each of the collection's 22 nested subfolders as a single request. The collection holds **213 leaf requests** in 14 top-level folders. Subfolders: `Clients/Credits`; `Invoices/{Items and Services, Invoice Recurring Template, Invoice Links/Downloads, Upload Logo, Retainers, Other Income, Payments}`; `Expenses/{Upload Expense Receipt Image, Bills (Beta), Vendors (Beta)}`; `Projects/Tasks`; `Accounting/{Accounts, Journal Entries, Other Income, Taxes}`; `Settings/{Abilities, Businesses, Gateways, Systems, Items and Services, Developer}`. Leaf counts per folder: Authorization 4, Clients 13, Invoices 50, Expenses 29, Estimates 8, Time Tracking 7, Projects 19, My Team 7, Reports 15, Accounting 19, Uploader 3, Webhooks 5, Settings 28, Tokenization 6. Inventory keys are therefore the full path `<Folder>/<Subfolder>/<Request name>` with every segment trimmed (`My Team ` carries a trailing space in the source). 211 keys are unique; `Expenses/Single Tax` and `Settings/Items and Services/Single Tax` each occur twice. Phase 2's batch split must be re-cut against these counts (Payments, Retainers and Other Income live under Invoices; Taxes and Journal Entries under Accounting).
>
> **STATE AS OF 2026-08-22 (Phase 0, inventory tool):** the "identical method and URL" claim above is wrong -- each `Single Tax` pair is a GET and a DELETE that share the same Postman request name and URL, not two copies of the same request. That is a genuine name collision, not an exact duplicate, so the inventory tool cannot silently collapse it into one entry without losing the DELETE operation. `Normalize` disambiguates by suffixing *both* sides of the collision (`Single Tax (GET)`, `Single Tax (DELETE)`); the plain, unsuffixed key never survives a collision, so which request "owns" it never depends on collection order. The real collection therefore normalizes to **213 distinct inventory keys**, not 211; "211" remains correct only as the count of distinct Postman *request names* before disambiguation.

- Base URL `https://api.freshbooks.com`. Required headers: `Authorization: Bearer <token>`, `Content-Type: application/json`.
- OAuth2 authorization-code flow. Documented endpoints: authorize `https://auth.freshbooks.com/oauth/authorize/`, token `https://api.freshbooks.com/auth/oauth/token`, revoke `.../auth/oauth/revoke`. The RFC 8414 metadata additionally advertises `https://auth.freshbooks.com/service/auth/oauth/{authorize,token,revoke,introspect}` with PKCE `S256`. Phase 1 verifies which endpoints accept our app and prefers the metadata ones with PKCE; the documented ones are the fallback.

> **STATE AS OF 2026-08-23 (Phase 1, stage 1, live OAuth check):** the full authorization-code flow was run twice against the registered dev app, once per endpoint set, with PKCE S256 and redirect `https://localhost:8765/callback`. CONFIRMED facts:
>
> - **Both endpoint sets accept the app end-to-end** and behave identically (same token response shape, same rotation, same revoke semantics). Default for the lib is the RFC 8414 set (`https://auth.freshbooks.com/service/auth/oauth/{authorize,token,revoke}`); the documented set (authorize `https://auth.freshbooks.com/oauth/authorize/`, token + revoke on `https://api.freshbooks.com/auth/oauth/{token,revoke}`) is the named fallback.
> - **PKCE S256 is accepted** by both authorize endpoints and both exchanges succeeded with `code_verifier`. Whether a missing or wrong verifier is rejected was NOT tested (one consent per run); treat PKCE as accepted-but-possibly-unenforced (INFERRED). `client_secret` is always required (`token_endpoint_auth_methods_supported: client_secret_basic, client_secret_post`; form-encoded `client_secret_post` observed working).
> - **Token response** (identical on both sets): `access_token` (JWT, ~1006 chars), `token_type` `"Bearer"`, `expires_in` `43200` (12h), `created_at` (unix seconds), `refresh_token` (64-char hex, one-time-use), `scope` (space-separated string), plus an undocumented `direct_buy_tokens: {}`.
> - **Refresh rotation**: `grant_type=refresh_token` returns a full new pair; reusing the old refresh token -> `400 {"error": "invalid_grant", "error_description": "..."}`. In one observed refresh the response `created_at` equaled the original exchange's `created_at` while `expires_in` was 43199, i.e. expiry is `created_at + expires_in`, not `now + expires_in` -- compute expiry from `created_at`.
> - **Revoke**: form POST `client_id`+`client_secret`+`token` -> `200 {}` on both sets; the access token is dead immediately after (users/me -> 401).
> - **`GET /auth/api/v1/users/me`**: top level is `{"response": {...}}` with NO `result` layer (auth family differs from accounting). `response.business_memberships[].business.{id (int), account_id (string), business_uuid (uuid), name}` confirmed, role at `membership.role`; also `business_statuses` and a large `permissions` map keyed by `account_id`.
> - **Accounting envelope**: `{"response": {"result": {"<plural>": [...], "page", "pages", "per_page", "total"}}}` confirmed; error shape `{"response": {"errors": [{"errno", "field", "message", "object", "value"}]}}` -- note the `value` field, which section 3's original list omitted.
> - **Business-scoped family**: flat body with `meta {page, pages, per_page, sort [], total}` confirmed; 404 error is `{"error": "Requested resource could not be found."}` (string, no errno); 401 (both families, bad token) is `{"error": "unauthenticated", "error_description": "..."}`.
> - **Dates observed**: accounting `"YYYY-MM-DD HH:MM:SS"` (`signup_date`, `updated`), auth family RFC 3339 (`"2026-08-22T04:31:37Z"`).
> - Sanitized captures (synthetic IDs) are committed under `freshbooks/testdata/seed/` as the fixture source of truth for Phase 1.
- Access tokens live ~12h. Refresh tokens never expire but are **one-time-use**: every refresh returns a new refresh token and invalidates the old one. Whoever refreshes must persist the new token immediately.
- Redirect URIs must be HTTPS.

> **STATE AS OF 2026-08-22:** the developer portal's form rejects `http://localhost:...` outright ("Redirect URIs must be HTTPS/SSL URIs"), contradicting third-party guides. The dev app uses `https://localhost:8765/callback`. The CLI loopback listener therefore serves TLS with an ephemeral in-memory self-signed certificate (browser shows a one-time warning), and always offers a paste-the-redirected-URL fallback that needs no listener at all.
- Scopes: `user:{object}:{read|write}` over ~22 objects; `user:profile:read` is always granted. (`mcp:*` scopes also exist for an unreleased first-party FreshBooks MCP -- irrelevant to us, noted in Future Work.)

> **STATE AS OF 2026-09-02 (Phase 7, live):** the scope bullet above is CORRECTED. The developer portal's scope picker -- not https://www.freshbooks.com/api/scopes -- is the authority on what exists, and the two lists disagree in both directions:
>
> - **Three of the documented objects are read-only**: `profile`, `notifications`, and `reports` have a `:read` scope and NO `:write` scope. Requesting a nonexistent scope does not degrade to a partial grant; it rejects the whole consent with "The requested scope is invalid, unknown, or malformed" after login, with no indication of which scope was at fault. The CLI's shipped default set requested all three writes and so could never complete a login.
> - **The portal offers objects the docs page omits**: `uploads` (read/write -- the three upload endpoints need it), `account` (read/write) and `riskhub` (read/write), plus the `mcp:*` family already noted in Future Work.
> - **A scope must also be enabled on the app**, and the failure is indistinguishable from the above. The dev app was registered in Phase 1 with two scopes; widening it in the portal was required before the wider consent succeeded. A bogus scope name is NOT rejected at the authorize URL, so no unattended probe can enumerate an app's enabled scopes.
>
> Granted live on 2026-09-02: 45 scopes (all 22 documented objects read, write for the 19 that have one, plus `account` and `uploads` read/write). The CLI's default set is now the same minus `account`, 43 scopes (`cli/internal/auth/scopes.go`).
- Identity: `GET /auth/api/v1/users/me` returns `business_memberships[]`, each with `business.id` (integer `business_id`) and `business.account_id` (string `account_id`). Both are needed.
- Two API families with different URL roots, ID types, envelopes, and error shapes:
  - **Accounting** (`/accounting/account/{account_id}/...`): clients, invoices, expenses, estimates, payments, items, taxes, bills, credit notes, other income, journal entries, reports, systems, staff. Responses are wrapped `{"response": {"result": {...}}}`; errors `{"response": {"errors": [{"errno", "field", "message", "object"}]}}`. Pagination `page`, `per_page`, `pages`, `total` inside `result`. Search via `search[field]=value`, includes via `include[]=x`.
  - **Business-scoped** (`/projects/business/{business_id}/...`, `/timetracking/business/{business_id}/...`, `/comments/business/{business_id}/...`, `/auth/api/v1/businesses/{business_id}/...`): projects, time entries, services, service rates, team members, retainers. Plain JSON bodies with a `meta` pagination object; errors `{"error": ..., "errno": ...}` or `{"message": ...}` per endpoint.
  - Plus `/events/account/{account_id}/events/callbacks` (webhooks), `/uploads/account/{account_id}/{attachments,images}` (multipart), `/payments/account/{account_id}/...` (gateways, checkout links, payment options), `/accounting/businesses/{business_uuid}/ledger_accounts/...`.
  > **STATE AS OF 2026-08-22 (Phase 0):** the collection also has a `/accounting/ledger_accounts/{types,sub_types}` type taxonomy for the ledger accounts above -- no `business_uuid` in the path, so it doesn't match the `/accounting/businesses/` prefix literally, but it is plainly ledger-family (it lives in the `Accounting/Accounts` folder and enumerates the types `/accounting/businesses/{business_uuid}/ledger_accounts/` accounts use). The inventory tool's family classifier treats `/accounting/ledger_accounts/` as `ledger` too.
- Money is `{"amount": "100.00", "code": "USD"}` (string decimal). Dates appear as `YYYY-MM-DD`, `YYYY-MM-DD HH:MM:SS` (account-local, no zone) in accounting, and RFC 3339 in business-scoped APIs.
- Rate limits are undocumented; 429 carries `Retry-After`.

> **STATE AS OF 2026-09-01 (Phase 2 batch a, QA pass):** three docs-vs-Postman conflicts found while implementing the Invoices/Payments/Retainers batch, all resolved docs-wins per this section's own rule:
>
> - **Payment-options body.** The `/api/online-payments` docs page's only worked example for `POST /payments/account/{account_id}/invoice/{invoiceId}/payment_options` sends `entity_id` (a bare JSON number, the invoice id) and `entity_type` (`"invoice"`) alongside the gateway booleans; the Postman example for the same endpoint (`Invoices/Enable Payment Options On Invoice`) omits both. The library now always sends them, built from the path argument, for both the invoice and invoice-profile variants (the latter's `entity_type` value, `"invoice_profile"`, has no docs or Postman example and is INFERRED by analogy). The response echoes `entity_id` back quoted as a string, so a caller must not reuse the request type to decode it.
> - **Invoice delete verb.** The `/api/invoices` docs page's "Delete Invoice" example is `DELETE` with a `{"invoice":{"vis_state":1}}` body; the Postman collection captures `PUT` with the identical body. The library keeps `PUT`: a `DELETE` carrying a body is unusual, Postman is captured live traffic rather than prose, and `Items`/`Payments` (documented and captured consistently) both genuinely use `PUT` for the same soft-delete shape. Flagged for the live-conformance pass rather than changed on docs-prose alone.
> - **`entity_type` singular vs. the field table's plural.** The `/api/online-payments` field-description table glosses `entity_type` as `Eg. "invoices"` (plural), but every request, response, and query example on the same page uses the singular `"invoice"`. The library sends the singular form, matching the examples over the table gloss.

> **STATE AS OF 2026-09-01 (Phase 2, batch b -- Clients, Estimates, Expenses, Taxes):** three discrepancies found reading the Postman collection against the FreshBooks docs pages, none live-verified (no sandbox account in this phase):
>
> - **`Expenses/Delete Expense`'s Postman example body sends `vis_state: 0`** (active), not `1` (deleted), contradicting every other soft-delete in the accounting family (bills, vendors, credit notes, estimates all delete via `vis_state: 1`) and the FreshBooks docs page for expenses, which itself documents `vis_state: 1` as deleted. Treated as a Postman authoring mistake; `ExpensesService.Delete` sends `vis_state: 1`. A live check should confirm.
> - **`Expenses/Create Custom Expense Category` is in the Postman collection, but the FreshBooks docs page for expense categories states plainly that creating, updating, and deleting categories is not supported by the API.** Implemented per the inventory (the parity contract requires every assigned key to map to a method) since a custom-category feature exists in the FreshBooks UI, so a write endpoint plausibly does too -- but this is INFERRED from Postman only and contradicts the docs. A live check should confirm whether this endpoint actually works.
> - **No Postman example response exists for four endpoints batch b owns**: `Clients/Edit Secondary Contact ID`, `Clients/Delete Secondary  Contact ID`, `Expenses/Expense Vendors`, and `Expenses/Create Recurring Expense`. Response shapes for these (`ContactsService.Update`'s `contact` envelope, `ExpensesService.Vendors`'s bare `vendors` string array, `ExpensesService.CreateRecurring`'s `expense_profile` envelope) are INFERRED from this API family's otherwise-uniform response conventions, not confirmed live.
>
> (The batch b implementer report originally recorded a fourth discrepancy claiming the FreshBooks docs page lists `Estimates/Delete Estimate`'s verb as `DELETE`. The batch b code-review lane re-checked the docs page during the gate: its "Delete Single Estimate" section shows `PUT`, agreeing with the Postman example and the implementation. No conflict exists; the claim is not carried forward here. The implementer report itself is left as written, as a historical record of what was believed at merge time.)
>
> **Addendum, batch b QA pass:** QA decoded every captured batch-b response in `freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json` against the library's own structs rather than against hand-authored fixtures, and found three more conflicts the fix-commit fixtures had been masking:
>
> - **`Expenses/Bills (Beta)` line `quantity` is a bare int on write but a quoted string on every captured and documented read** (`"quantity": "1"`). `BillLine.Quantity` (read) is now `string`; `BillLineRequest.Quantity` (write) stays `int` -- a genuine, evidenced asymmetry, not unified.
> - **`Expenses/Vendors (Beta)` `outstanding_balance`/`overdue_balance` are arrays of `{"amount": {"amount", "code"}}` wrappers, not a bare `{"amount", "code"}` object**, contradicting the `/api/vendors` docs field table (which calls the field a plain object) but confirmed by every captured response and every docs example row. `BillVendor.OutstandingBalance`/`OverdueBalance` are now `[]VendorBalance`.
> - **`Expenses` attachment ids (`attachmentid`, `id`) are bare JSON numbers**, not strings, in every captured expense response. `ExpenseAttachment.AttachmentID`/`ID` are now `int64`.
>
> Two evidence conflicts inside this same batch were resolved in opposite directions, each for a stated reason: `TaxCreateRequest.Amount` follows the Postman example (`"amount": 13`, unquoted) over the docs, matching the retainer-fee precedent from batch a where a live-observed write shape wins; `ExpenseWriteRequest.TaxPercent1`/`TaxPercent2`/`MarkupPercent` follow the FreshBooks docs field table (`string`) over the Postman example's unquoted number, because the read model already types the same fields `string` in every captured response -- Postman's create-request example is the outlier there, not the response shape.

Anything above marked as inferred from examples must be confirmed against the live API during phase 1 and recorded with a `STATE AS OF` callout if wrong (section 9.6).

## 4. Repository layout

```
freshbooks-tools/
  go.work
  LICENSE                      MIT
  README.md                    overview, module links, contributor agent-setup note
  CLAUDE.md                    conventions, gotchas, locked decisions, process pointers
  GOAL.md                      current autonomous phase goal (section 9)
  CHANGELOG.md                 repo-level (process/infra); per-module changelogs below
  mise.toml                    tool pins + tasks (check, test, lint, cover, build, docs)
  .golangci.yml
  .github/workflows/ci.yml, release.yml
  scripts/                     coverage gate, branch-protection setup, changelog section extractor
  docs/
    getting-started.md  building.md  authentication.md  library.md  mcp.md  cli.md
    agentic-transformation.md  progress.md  phases/<n>/{plan.md,reports/}
    superpowers/specs/, superpowers/plans/
    freshbooks.postman_collection.json   (source inventory; moves under freshbooks/internal/inventory/testdata)
  freshbooks/                  module .../freshbooks   (library)
    CHANGELOG.md  doc.go  client.go  options.go  transport.go  errors.go  page.go  types.go
    auth/          oauth.go  token.go  store.go  store_file.go  store_memory.go
    <resource>.go  <resource>_test.go   (one pair per resource)
    internal/inventory/        postman -> inventory.json tool (+ testdata)
    testdata/                  httptest fixtures per resource
  mcp/                         module .../mcp
    CHANGELOG.md  cmd/freshbooks-mcp/main.go  internal/{config,server,tools}/
  cli/                         module .../cli
    CHANGELOG.md  cmd/freshbooks/main.go  internal/{cmd,config,output,auth}/
```

`go.work` lists all three modules; `mcp` and `cli` `require` the lib by module path and, during development, resolve it through the workspace. Releases of `mcp`/`cli` pin a released `freshbooks/vX.Y.Z`.

## 5. Library design (`freshbooks`)

### 5.1 Programmer-facing API

```go
ts := auth.NewTokenSource(cfg, auth.NewFileStore(path))          // refresh + rotation write-back
c, err := freshbooks.NewClient(freshbooks.WithTokenSource(ts))
me, err := c.Identity.Me(ctx)                                    // AccountID + BusinessID per membership
inv, err := c.Invoices.Get(ctx, acct, 1234, freshbooks.Include("lines"))
page, err := c.Invoices.List(ctx, acct, &freshbooks.InvoiceListOptions{Page: 2, Search: freshbooks.Search{"status": "paid"}})
for inv, err := range c.Invoices.All(ctx, acct, nil) { ... }     // range-over-func iterator, auto-pagination
err = c.Invoices.Delete(ctx, acct, 1234)
raw, err := c.Do(ctx, http.MethodGet, "/accounting/account/"+string(acct)+"/systems/systems/1", nil, &out)
```

- **Flat services** as exported fields on `*Client`: `Identity`, `Clients`, `Contacts`, `Invoices`, `InvoiceProfiles`, `Expenses`, `ExpenseCategories`, `Estimates`, `Payments`, `Items`, `Taxes`, `Bills`, `BillPayments`, `BillVendors`, `BillableItems`, `CreditNotes`, `OtherIncome`, `JournalEntries`, `JournalEntryAccounts`, `LedgerAccounts`, `Reports`, `Systems`, `Staff`, `Tasks`, `Projects`, `TimeEntries`, `Services`, `ServiceRates`, `TeamMembers`, `Retainers`, `Callbacks` (webhooks), `Attachments`, `Images`, `Gateways`, `CheckoutLinks`, `PaymentOptions`. Names mirror the official Python/Node SDKs. The inventory tool is the authority on the final list; every Postman request must map to exactly one method.
- **Method vocabulary:** `List`, `All`, `Get`, `Create`, `Update`, `Delete`, plus resource-specific verbs (`Invoices.Send`, `Invoices.PDF`, `Invoices.ShareLink`, `TimeEntries.Search`, `Reports.ProfitLoss`...). Every method takes `ctx` first, then the scope ID, then the resource ID, then a request struct or variadic `RequestOption`.
- **Distinct ID types:** `type AccountID string`, `type BusinessID int64`, `type BusinessUUID string`. Passing the wrong family is a compile error. `Identity.Me` returns `Membership{AccountID, BusinessID, BusinessUUID, Name, Role}`.
- **Typed models** per resource with `json` tags, pointer fields for optionals on write structs, value fields on read structs where the API always returns them. `Money{Amount string; Code string}` with `(Money).Rat() (*big.Rat, error)`; `Date` and `DateTime` implementing `json.Marshaler`/`Unmarshaler` for all three wire formats; `VisState` and status enums as typed ints/strings with `String()`.
- **Pagination:** `Page[T]{Items []T; Page, Pages, PerPage, Total int}` returned by `List`; `All` is `iter.Seq2[T, error]` (Go 1.23+) that stops on the first error or `ctx.Done()`.
- **Request options:** `Include(names ...string)`, `Search(map)`, `Sort(field, dir)`, `PerPage(n)`, `Page(n)` -- applied uniformly; the transport knows each family's query encoding.

> **STATE AS OF 2026-08-23 (Phase 1, lib core):** this bullet and the `Page[T]` bullet above name the same identifier, which Go cannot have twice in one package. Resolved in favour of the type: `Page[T]` keeps the short name because it appears in every `List` signature, and the request option ships as `PageNumber(n)`. `Search` ships as a named map type (`type Search map[string]string`) that also implements `RequestOption`, so the spec's own `freshbooks.Search{"status": "paid"}` literal works both as a list-options field value and as a variadic request option. The two families' query encodings differ only in filters: accounting spells them `search[field]=value`, business-scoped spells them `field=value`; `include[]`, `sort=<field>_<asc|desc>`, `page`, and `per_page` are common to both.
>
> Also settled here: the transport treats the auth family as its own envelope case. Section 3's live callout records that `/auth/api/v1/...` returns `{"response": {...}}` with no `result` layer, so `Family` has three values (`accounting`, `business`, `auth`), not two. `Retry-After` is honoured but capped by `RetryPolicy.MaxDelay`: a client should not block for an arbitrary period because a header said so.
>
> Two encoding facts in this section are **INFERRED**, not CONFIRMED, and are flagged as such in the code:
>
> - **Business-family filter encoding.** The accounting family's `search[field]=value` is CONFIRMED (live, 2026-08-23). The business-scoped family's bare `field=value` is INFERRED from the FreshBooks docs pages only; no live business-scoped list call was made with a filter. **Phase 2's first business-scoped list endpoint must confirm it** and correct this callout if the API disagrees.
> - **Envelope shape outside the three named families.** The transport's three `Family` values are envelope shapes, not a one-to-one map of the inventory tool's classifier, which also names `events`, `uploads`, `payments`, and `ledger`. `/events/` (webhook callbacks) is classified `accounting` because it is `account_id`-scoped and the collection's example response is the accounting envelope -- INFERRED from that Postman example, not observed live. `/payments/` and `/uploads/` fall through to `business` and are **unverified in either direction**. **The Phase 2 batch that implements each of these must confirm its envelope live** and reclassify if needed; a wrong classification means `(*Client).Do` hands back an un-peeled envelope.
>
> **STATE AS OF 2026-09-01 (Phase 2, batch c):** the business-family filter encoding above is now **CONFIRMED against the FreshBooks docs** (docs-only; this phase made no live calls). `https://www.freshbooks.com/api/parameters` states plainly: accounting endpoints filter via `?search[key]=value`; "Business/Project/Time Tracking Endpoints" (the business-scoped family) filter via bare fields, e.g. `?complete=true`, `?updated_since=...`, `?billed=true`. `TimeEntriesService.List` (`time_entries.go`) is the first business-scoped list endpoint and confirms this: its tests assert `updated_since` and `include_deleted` land as bare query keys, never `search[updated_since]`. Live confirmation against a real account remains pending (no attended step this phase).
>
> The same docs page also surfaces a **second, previously undocumented encoding difference** this callout did not originally flag: **sort direction**. The bullet above and the `Sort()` doc comment both say `sort=<field>_<asc|desc>` is common to both families. The docs disagree for the business-scoped family: "Project/time tracking endpoints: `?sort=field_name` (ascending) or `?sort=-field_name` (descending)" -- a leading-minus prefix, not a `_desc` suffix. The Postman example for `Time Tracking/Time Entries For Employee on Specific Project` corroborates this live-observed-in-the-wild: its query carries `sort=-started_at`. `Sort()` and `requestOptions.values()` (`types.go`) were **not** changed to family-switch this -- doing so would flip behavior for every existing call site built against the current suffix format (including Phase 1's own `types_test.go` assertions), which is a design decision for whoever owns `types.go`/`options.go` next, not a batch-c-scoped fix. `TimeEntriesService.Search` and `List` therefore still emit `<field>_asc`/`<field>_desc` for a `Sort()` option; a caller who needs the `-field` form the live API actually expects should pass it directly via `Search{"sort": "-started_at"}` until this is resolved. Flagged for the review gate and for whichever phase next touches `Sort()`.

> **STATE AS OF 2026-09-01 (Phase 2, batch d):** all three families named above are now docs-confirmed (Postman example + FreshBooks docs page), though still not live -- this phase runs unattended. `events` stays `accounting`: the "List Webhook Callbacks" example is the full `{"response":{"result":{"callbacks":[...],"page":...}}}` envelope. `uploads` and `payments` are both `business` (flat, no envelope): the "Upload Logo or Proposal Image" example is `{"image": {...}}`, the FreshBooks expense-attachments docs page confirms `{"attachment": {...}}` for the sibling endpoint, and the "Get Publishable Key"/"Create Setup Intent" examples are `{"gateway_connections": [...]}` and `{"credit_card": {...}}` -- no case needed either family to move off its Phase 1 default, so `(*Client).Do` was already correct for them.
>
> One family this bullet did not name turned out to be wrong: `ledger` (the chart-of-accounts endpoints under `/accounting/businesses/{businessUuid}/ledger_accounts/...` and `/accounting/ledger_accounts/{types,sub_types}`) was falling into the general `/accounting/` prefix match and getting classified `accounting` -- double envelope unwrap. Its actual responses are flat `{"data": ...}` (Postman "List Accounts", "Single Account", "Create Account", "Update Account" examples all agree). `familyForPath` now matches the ledger paths as `business` *before* the general `/accounting/` case. The two taxonomy endpoints (`/accounting/ledger_accounts/types`, `/.../sub_types[/{id}]`) carry no Postman example and no public docs page at all; their result shape (`LedgerAccountsService.Types`/`SubTypes`/`SubType`) is INFERRED from the sibling ledger-account fields (`type`, `sub_type` strings) rather than any observed response, and is flagged as such in the code.
>
> **A body-shape conflict the QA pass surfaced:** `CallbacksService.Verify` and `ResendVerification` (`callbacks.go`) send `callback_id` in the request body. Both captured Postman requests carry it (`{"callback":{"callback_id":{{callbackId}},"verifier":"..."}}` and `{"callback":{"callback_id":2001,"resend":true}}`); the live https://www.freshbooks.com/api/webhooks docs page shows both bodies **without** it (`{"callback":{"verifier":"..."}}`, `{"callback":{"resend":true}}`). The Postman capture is kept -- per this project's own inferred-vs-confirmed rule the collection is the parity contract -- and the redundant field is low-risk (the id is already in the URL path), but the disagreement is real and worth a reader knowing about; both method doc comments now note it.
>
> **Docs pages that have moved past the Postman collection's endpoints, for three of batch d's resources.** The library implements what the collection captures, which is correct -- that collection is the parity contract this whole repo is built against -- but a reader comparing the code to the *current* public docs will see a mismatch and should not assume it is a bug:
>
> - **Journal entries**: https://www.freshbooks.com/api/journal-entries now documents `POST /accounting/businesses/{business_uuid}/journal_entries` with a `manualJournalEntry` wrapper and per-line `{accountId (uuid), amount{}, type: TYPE_DEBIT|TYPE_CREDIT}` -- a different API, on a different scope id, from the implemented `POST /accounting/account/{account_id}/journal_entries/journal_entries` with `{sub_accountid, debit, credit}` lines (`JournalEntries.Create`).
> - **Chart of accounts, list operation only**: https://www.freshbooks.com/api/chart-of-accounts documents the list as `GET /accounting/businesses/{business_uuid}/reports/chart_of_accounts` (wrapper `response.result.journal_entry_accounts`); the implemented list is the Postman-captured `GET /accounting/businesses/{business_uuid}/ledger_accounts/accounts` (wrapper `data`), which the captured body confirms (`LedgerAccountsService.List`). Create and get-single match the current docs exactly.
> - **Reports, profit-and-loss and trial balance**: https://www.freshbooks.com/api/reports documents both under `/accounting/businesses/{business_uuid}/reports/...` (wrappers `profit_and_loss` / `trial_balance`); the implemented paths are the Postman-captured `/accounting/account/{account_id}/reports/accounting/profitloss_entity` and `.../trial_balance` (`ReportsService.ProfitLoss`, `.TrialBalance`).
>
> Each affected method's doc comment now points back here.
- **Errors:** `*Error{StatusCode int; Code int; Message string; Field string; Family Family; Raw json.RawMessage}` implementing `error` and `Unwrap`; sentinels `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrValidation`, `ErrRateLimited` matched via `errors.Is`; `RetryAfter() time.Duration` on rate-limit errors.
- **Transport:** single `do()` path that sets headers, encodes body, unwraps the accounting envelope, decodes family-specific errors, and retries 429/502/503/504 with exponential backoff + jitter (default 3 attempts, `WithRetry(RetryPolicy)`, `WithRetry(NoRetry)` to disable). Honors `Retry-After`. `WithHTTPClient`, `WithBaseURL` (tests/sandbox), `WithUserAgent`, `WithLogger(*slog.Logger)` (never logs tokens or bodies above debug), `WithClock(func() time.Time)` (tests).
- **Auth package `freshbooks/auth`:** `Config{ClientID, ClientSecret, RedirectURL, Scopes, Endpoints}`, `AuthCodeURL(state, opts...)` with PKCE, `Exchange(ctx, code, verifier)`, `Refresh(ctx, refreshToken)`, `Revoke`, `Token{AccessToken, RefreshToken, Expiry, Scopes}`, `TokenSource` interface (`Token(ctx) (*Token, error)`), `StaticTokenSource(access)`, `NewTokenSource(cfg, store)` which refreshes when expiring within a skew and writes the rotated token back through `TokenStore` (`Load/Save`; `FileStore` 0600 with atomic rename; `MemoryStore`). Concurrency-safe: one refresh in flight per source.
- **Escape hatch:** `(*Client).Do(ctx, method, path string, body, out any) error` using the same transport so untyped endpoints still get auth, retry, and error decoding.
- **Dependencies:** stdlib only. Tests use `github.com/stretchr/testify`.
- **Doc comments** on every exported identifier; `doc.go` carries the package overview with a runnable `Example`. Lint enforces presence (`revive` exported rule).

### 5.2 Inventory tool (`freshbooks/internal/inventory`)

`go run ./internal/inventory -in <postman.json> -out inventory.json` walks the collection and emits, per request: folder, name, method, normalized path template (variables lower-camel-cased, hard-coded IDs replaced, `my.freshbooks.com/service/api` rewritten to the public path, stray whitespace stripped), query params, example request body, example responses, and the inferred ID family. It also prints a coverage report when given a package (`-check ./...`): every inventory entry must be referenced by a `// inventory: <folder>/<name>` comment on exactly one method, and vice versa. This comment is the parity contract used by the lib, MCP, and CLI parity tests.

## 6. MCP server design (`freshbooks-mcp`)

- `freshbooks-mcp serve --transport stdio|http [--addr :8080] [--path /mcp]`; `freshbooks-mcp version`; `freshbooks-mcp tools` (prints the tool manifest as JSON, used by docs and parity tests).
- **Stateless** per the 2026-07-28 spec: HTTP mode uses `mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})`. `getServer` builds a per-request `*mcp.Server` whose tool handlers use a lib client bound to the bearer token from that request's `Authorization` header. No sessions, no caches keyed by client, no disk writes. `GET /healthz` returns 200.
- **Stdio mode** reads `FRESHBOOKS_ACCESS_TOKEN`, or `FRESHBOOKS_REFRESH_TOKEN` + `FRESHBOOKS_CLIENT_ID` + `FRESHBOOKS_CLIENT_SECRET` + `FRESHBOOKS_TOKEN_FILE` to use the lib's `FileStore` for rotation. A single-user process owning its own file is still stateless from the protocol's point of view.
- **Tools:** one tool per lib method, named `{service_snake}_{verb}` (`invoices_list`, `invoices_get`, `time_entries_create`, `reports_profit_loss`). Input schemas are derived by the go-sdk's `jsonschema` reflection from a per-tool input struct that embeds the lib's request/option structs plus `account_id`/`business_id`; output is the lib's typed result marshaled as JSON. Tool descriptions are one sentence plus a link to the FreshBooks docs page for that resource. Annotations: `ReadOnlyHint` on `list/get/all/search/reports`, `DestructiveHint` on `delete`, `IdempotentHint` on `update`.
- **Default scope IDs:** `FRESHBOOKS_ACCOUNT_ID` / `FRESHBOOKS_BUSINESS_ID` may pre-fill the scope so tools can omit them; an `identity_me` tool is always present so a client can discover them.
- **Config:** cobra flags with `FRESHBOOKS_MCP_*` env twins (flag > env), no config file. `--log-level`, `--log-format json|text` via `log/slog`. Secrets are redacted from logs and error strings.
- **Errors:** lib `*Error` maps to MCP tool errors (`isError: true`) with `{code, message, field, status}` in the content; transport/auth failures map to JSON-RPC errors.
- **Dependencies:** go-sdk, cobra, lib.


> **STATE AS OF 2026-09-01** (Phase 3 stage 1, unattended): go-sdk v1.7.0 is the current release, so the pin holds. Three corrections to the paragraphs above, all decided in `docs/phases/3/plan.md`: (1) there are no `_all` tools -- the 17 lib `All` iterators are conveniences over `List`, and an unbounded page walk is the wrong shape for a model context, so `ReadOnlyHint` applies to `list/get/search/reports`; (2) go-sdk's `jsonschema` reflection cannot derive schemas for 54 of the 169 lib types because `Date`/`DateTime` embed `time.Time` and `ProfitLossLine` is recursive, so input schemas are built once with `jsonschema.ForType` plus three `TypeSchemas` overrides (verified 169/169) and set on `Tool.InputSchema` explicitly, and the `Out` type is `any` (no output schema; the lib value goes out as `StructuredContent` + JSON text); (3) the tool surface is 168 tools carrying 212 of the 213 inventory keys -- `Authorization/Revoke Refresh Token` lives on `auth.Config.Revoke`, and the MCP is a token consumer, so it is not a tool. The definitive list is `docs/phases/3/tools.md`.

## 7. CLI design (`freshbooks`)

```
freshbooks auth login [--scopes ...] [--callback-port 8765] [--no-browser]
    # loopback PKCE flow on https://localhost:<port>/callback (ephemeral self-signed TLS,
    # listener bound to 127.0.0.1); --no-browser prints the auth URL and accepts the
    # redirected URL (or bare code) pasted on stdin -- no listener needed
freshbooks auth status | logout | token [--refresh]
freshbooks config view | contexts | use-context <name> | set-context <name> --account X --business Y
freshbooks identity me
freshbooks <resource> list [--all] [--page N] [--per-page N] [--search k=v]... [--include x]... [--sort field[:asc|desc]]
freshbooks <resource> get <id>
freshbooks <resource> create -f file.json|-        # stdin
freshbooks <resource> update <id> -f file.json|-
freshbooks <resource> delete <id> [--yes]
freshbooks invoices send <id> ...  freshbooks invoices pdf <id> -o out.pdf   (resource-specific verbs)
freshbooks api <METHOD> <path> [-f body.json] [-q k=v]...   # raw escape hatch
freshbooks completion bash|fish|zsh|powershell
freshbooks version
```

- **cobra**; every flag has a `FRESHBOOKS_*` env twin; precedence flag > env > `$XDG_CONFIG_HOME/freshbooks/config.yaml` (`--config` to override). Global flags: `-o/--output json|yaml|table|name`, `--account`, `--business`, `--context`, `--no-headers`, `-q/--quiet`, `--dry-run` (prints method, URL, body; sends nothing), `--timeout`, `--log-level`.
- **Stateless:** no daemon, no cache. Persisted state is exactly `credentials.json` (0600, lib `FileStore`, keyed by context) and `config.yaml`.
- **Automation:** machine-readable errors on stderr as JSON when `-o json`; exit codes 0 ok, 1 API/runtime error, 2 usage, 3 auth (no/expired token), 4 not found; `--yes` for non-interactive deletes; `-f -` reads stdin; table output is the default on a TTY, json when stdout is not a TTY.
- **Command tree is generated from the inventory contract:** a table in `cli/internal/cmd/registry.go` maps lib methods to `{group, verb, flags}`; the parity test asserts every inventory entry is reachable by exactly one command.
- **Dependencies:** cobra, pflag, `gopkg.in/yaml.v3`, `golang.org/x/term` (TTY detection), lib.

> **STATE AS OF 2026-09-02** (Phase 4, CLI, unattended): the surface above holds with these corrections, all decided in `docs/phases/4/plan.md` and the Phase 4 gate. (1) `systems get` takes no positional id: `--account` and `--business` together address the resource (`SystemsService.Get(ctx, AccountID, BusinessID)`). (2) `api --query` has no `-q` shorthand; the global `-q/--quiet` owns it (pflag panics on a same-shorthand collision). (3) `--sort field[:asc|desc]` is implemented on the 21 list commands whose lib method takes `...RequestOption`, with a documented caveat: the direction encoding for business-scoped resources is unconfirmed against the live API (`docs/progress.md` backlog). (4) Report commands take their filter options as an optional `-f file|-` JSON document decoded into the report's options struct, not as per-report flags. (5) The boolean global flags (`--no-headers`, `--dry-run`, `--yes`, `--force`) have no env twin, and only the scope flags and `--context` consult `config.yaml`; the other global flags resolve flag > env > default. (6) `--yes` is required for destructive commands only when stdin is a TTY, so pipelines are not forced to pass it; destructive commands carry a `(destructive: requires --yes on a TTY)` suffix in their help. (7) `--dry-run` is rejected (exit 2) by the `auth` and `config` commands rather than silently ignored. (8) Credentials are one lib `FileStore` per context at `$XDG_CONFIG_HOME/freshbooks/credentials/<context>.json`; context names are restricted to `[A-Za-z0-9._-]`.

## 8. Testing, CI, release, docs

### 8.1 Testing

- **Unit** (all modules): table-driven, `httptest.Server` replaying fixtures from `testdata/` (seeded from Postman examples and the per-resource pages on freshbooks.com/api). Every resource file has a `_test.go` covering list/get/create/update/delete, error decoding, and option encoding.
- **Integration** (`//go:build integration`, run in CI): cross-package seams -- token expiry -> refresh -> store write-back -> retried request; `All()` across multiple pages with a mid-stream 429; MCP tool handler -> lib -> fixture server for every tool (generated from the manifest); CLI command -> lib -> fixture server for every command, asserting output formats and exit codes.
- **Parity**: inventory `-check` across lib/mcp/cli fails the build if any request is uncovered or double-covered.
- **Live** (`//go:build live`, `FRESHBOOKS_LIVE=1` + creds): read-only smoke against a real account; never in CI by default.
- **Green rule:** no `t.Skip` without an issue link, no `-run` filters committed, race detector always on, lint warnings are errors. Coverage floor **90%** per module, enforced by `scripts/coverage-gate.sh` (reads `go tool cover -func`, fails below threshold). Tests are tagged in `t.Run` names with `[happy] [sad] [edge] [corner] [parity]` where it aids triage.
- Determinism: fake clocks (`WithClock`) for token expiry and backoff; seeded jitter in tests.

### 8.2 `mise.toml` tasks

`check` = `fmt-check -> vet -> lint -> test (race, coverprofile) -> coverage-gate -> inventory-check -> build (cross matrix)`, printing a dirty-tree banner (`git status --porcelain` non-empty) at the end. `test`, `lint`, `cover`, `build`, `docs` individually. Tool pins: `go`, `golangci-lint`, `goreleaser` (no `latest`).

### 8.3 CI (`.github/workflows/ci.yml`)

- Triggers: `pull_request`, `push` to `main`, `workflow_call` (used by release).
- Jobs: `lib` -> `mcp` and `cli` (`needs: lib`, so a lib failure fails them). Each job runs `mise run check` for its module. Cross-build matrix `{linux,darwin,windows} x {amd64,arm64}` via `GOOS/GOARCH` on one runner.
- Branch protection on `main`: PRs only, required checks `lib`, `mcp`, `cli`, linear history. Applied by `scripts/branch-protection.sh` (uses `gh api`), documented in `building.md`; GitHub does not read this from the repo.
- Dependabot for Go modules and Actions, weekly.

### 8.4 Release (`.github/workflows/release.yml`)

- Trigger: tag push matching `freshbooks/v*`, `mcp/v*`, `cli/v*`.
- Guards: strict semver after the prefix (`^v\d+\.\d+\.\d+$`), tag commit is an ancestor of `origin/main` (`git merge-base --is-ancestor`), and the module's `CHANGELOG.md` has a `## [X.Y.Z]` section. Any failure aborts before building.
- Runs `ci.yml` via `workflow_call`; tests + coverage gate block the release.
- `mcp`/`cli`: goreleaser builds the 6-target matrix, archives, checksums, SBOM, and creates the GitHub release with the changelog section as body (`scripts/changelog-section.sh`). `freshbooks`: GitHub release with changelog body only; the Go proxy picks up the tag.
- Changelog format: Keep a Changelog per module with `[Unreleased]` on top; enforced by the agent-ops `changelog-guard` hook (README lists agent-ops as a contributor setup requirement).

### 8.5 Docs (`docs/`)

`getting-started.md` (create a FreshBooks app, scopes, first call from lib/CLI/MCP), `authentication.md` (flow, token lifetimes, rotation, where each component keeps tokens, links to freshbooks.com/api/authentication), `library.md`, `mcp.md` (per-transport setup incl. Claude Desktop / claude.ai connector / curl examples), `cli.md` (command reference generated by `cobra/doc`), `building.md` (mise, check, release, branch protection), `agentic-transformation.md` (how the Postman collection was pulled, the inventory tool, the work orders, the gate; links to section 9), `progress.md`. README links to pkg.go.dev for API reference. All docs ASCII-only, no hard wraps.

## 9. Process (from hoyle-re, adapted)

### 9.1 Phases

Each phase is one GOAL.md block, one branch, one review gate, one `--no-ff` merge.

| # | Phase | Implementer model | Reviewer model | Notes |
|---|---|---|---|---|
| 0 | Scaffold: go.work, modules, LICENSE, mise, CI/release skeleton, docs skeleton, inventory tool, parity-check plumbing | sonnet | opus | Written work order from this spec |
| 1 | Lib core: client, transport, options, errors, page/iter, types (ID/Money/Date), auth package, Identity service | opus | fable | Judgment-heavy; lead designs, implementer builds |
| 2 | Lib resources, 4 batches by Postman folder: (a) Clients+Invoices+Estimates, (b) Expenses+Payments+Accounting+Reports, (c) Projects+TimeTracking+MyTeam+Settings, (d) Webhooks+Uploader+Tokenization+remaining | sonnet x4 in parallel worktrees | opus | Service fields pre-declared in phase 1; batches only add files |
| 3 | MCP server | sonnet | opus | |
| 4 | CLI | sonnet | opus | |
| 5 | Release hardening, docs pass, first tags `freshbooks/v0.1.0`, `mcp/v0.1.0`, `cli/v0.1.0` | sonnet | opus | |

Rules: reviewer tier is always at least one above the implementer (sonnet -> opus, opus -> fable). Haiku is never used where a permission prompt could occur (it has no auto mode); read-only sweeps only. `model:` is passed explicitly on every dispatch; check the agent's `tools:` list before writing its prompt (`general-purpose` has everything; most agent-ops agents lack Bash or SendMessage).

### 9.2 Gate per phase

```
implement (TDD, commits each green, runs `mise run check` itself, no push/merge)
  -> parallel, read-only: code review | simplification | security review
  -> QA lane (the ONLY lane that runs `mise run check`; dirty tree = fail)
  -> lead triages all four reports -> ONE fix commit `fix(<phase>): apply the review-gate findings`
  -> re-run gate -> merge --no-ff (body summarises gate results) -> push
  -> update CHANGELOGs, docs/progress.md, GOAL.md self-advance
```

Verdict vocabulary: QA `PASS | NEEDS WORK` (default NEEDS WORK), code review `APPROVE | REQUEST CHANGES`, simplification `APPLY-RECOMMENDED | OPTIONAL | DO-NOT-APPLY`, security `PASS | BLOCK` with findings rated by impact. Every finding carries `file:line` and evidence. The lead may override a finding; overrides are recorded in the merge commit body. Simplifications that change observable behavior or wire encoding are refused.

Security lane checklist: tokens/secrets never in logs, errors, panics, or `--dry-run` output; credential files 0600 and written atomically; TLS not disabled; redirect/loopback listener binds localhost only and validates `state`; input validation at trust boundaries (CLI args, MCP tool inputs, API responses decoded into typed structs); no shell-outs with user input; `govulncheck` clean; dependency diff reviewed.

### 9.3 Work orders

Five templates live in `docs/phases/_templates/`: implementer, code-review, simplify, security, qa -- adapted from hoyle-re's (read-first pointers not pasted text; deliverables; gotchas; reporting via `SendMessage` to `team-lead` with the report in `message`, plus the same report written to `docs/phases/<n>/reports/<lane>.md`). Every template's read-first list includes https://www.freshbooks.com/api/start and the per-resource pages (e.g. https://www.freshbooks.com/api/invoices) because they carry response examples the Postman collection lacks, and the inventory entries for the phase.

### 9.4 GOAL.md

Same shape as hoyle-re's: purpose paragraph; "Shipped so far"; one `## Goal` blockquote with numbered stages (stage 1 is always "verify the premise": re-read the inventory + docs pages for this phase, diff against the spec, record corrections with a `STATE AS OF` callout before coding); `Done when:` (gate green, merged, pushed, changelogs + progress updated, **and GOAL.md retargeted to the next phase**); `Stop only` for a genuine decision an advisor skill cannot make; `### Self-advance`; `## Lessons from prior runs` (append-only); `## Reference`; `## Retarget` table with effort ratings. Driven by `/goal complete @GOAL.md`. Commit + push at every checkpoint is explicitly authorized inside a GOAL run.

### 9.5 Commits and branches

Conventional commits, imperative, scoped (`feat(freshbooks): ...`, `feat(mcp): ...`, `feat(cli): ...`, `chore(ci): ...`, `docs: ...`). Stage and commit in separate Bash calls (agent-ops guards). Branches `phase-<n>/<slug>`; phase 2 batches `phase-2/<batch>` in worktrees under `.worktrees/` (gitignored). Never `--no-verify`. Co-author trailer per harness rules.

### 9.6 Knowledge capture

`docs/progress.md` is the handoff artifact (current state, next action, how to resume). Invalidated assumptions get a `> **STATE AS OF YYYY-MM-DD**` callout in place. Process traps are triple-recorded: project memory, GOAL.md Lessons, CLAUDE.md Gotchas. Facts about the API are marked CONFIRMED (observed live or in official docs) / INFERRED (from examples) / TODO.

## 10. Public-repo hygiene

No operator-specific strings in any commit: no vault item names, internal IPs/domains, real account/business IDs, personal correspondents. Fixture IDs are synthetic. `scripts/redaction-check.sh` greps staged content against the agent-ops redaction term list before commits (documented; also run in CI).

## 11. Future work (not in scope)

- Docusaurus site for the guides.
- jsonpath output (`-o jsonpath=...`) via a library if someone needs it.
- OS keyring `TokenStore` for the CLI.
- Probe `mcp.freshbooks.com` and the `mcp:*` scopes once FreshBooks ships its first-party MCP; compare.
- Webhook receiver helper (signature verification) in the lib.
- Plane work-item tracking per phase, if wanted.
