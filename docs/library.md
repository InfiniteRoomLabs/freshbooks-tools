# Library

`github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks` is a handcrafted, stdlib-only client for the FreshBooks REST API. The API reference lives on pkg.go.dev; this page is the orientation.

## Client construction

```go
client, err := freshbooks.NewClient(
    freshbooks.WithTokenSource(src),
    freshbooks.WithUserAgent("my-app/1.0"),
)
```

| Option | Purpose |
|---|---|
| `WithTokenSource(auth.TokenSource)` | Where access tokens come from. Without it the client is unauthenticated, which is only useful against a fixture server. |
| `WithHTTPClient(*http.Client)` | Replaces the transport. The client is shallow copied, so your value is never mutated. |
| `WithBaseURL(string)` | Points at a sandbox or a fixture server. A path prefix is preserved. |
| `WithUserAgent(string)` | Identify your product. |
| `WithLogger(*slog.Logger)` | Debug-level request/response logging. Never logs headers or bodies. |
| `WithRetry(RetryPolicy)` | Replaces the retry policy; `WithRetry(freshbooks.NoRetry)` disables retrying. |
| `WithClock(func() time.Time)` | Replaces the clock used for backoff and `Retry-After`. Tests use it for determinism. |

A `*Client` is safe for concurrent use.

## Services and ID types

Every resource is an exported field on `*Client` -- `client.Invoices`, `client.Projects`, `client.TimeEntries`, and so on: 36 services covering all 213 documented FreshBooks endpoints.

The FreshBooks API is two API families that never share an identifier, so the library gives each its own type:

| Type | Addresses | Comes from |
|---|---|---|
| `AccountID` (string) | `/accounting/account/{account_id}/...` | `Membership.AccountID` |
| `BusinessID` (int64) | `/projects/business/{business_id}/...`, timetracking, comments | `Membership.BusinessID` |
| `BusinessUUID` (string) | `/accounting/businesses/{business_uuid}/ledger_accounts/...` | `Membership.BusinessUUID` |

Passing the wrong one is a compile error, not a 404 three hours later. All three come from one call:

```go
memberships, err := client.Identity.Me(ctx)
```

`Identity.Whoami` returns the same memberships plus the identity's own id, email, and name.

## Values on the wire

- `Money{Amount, Code}` keeps the amount as the string the API sent, so no precision is lost. `Money.Rat()` parses it into an exact `*big.Rat`.
- `Date` is `YYYY-MM-DD`. `DateTime` accepts all three formats FreshBooks uses -- RFC 3339, `YYYY-MM-DD HH:MM:SS`, and a bare date -- and *remembers which one it decoded from*, so a value read from one family and written back to it round-trips in that family's spelling. `NewDateTime` defaults to RFC 3339; `InLayout` overrides it.
- `VisState` names the visibility states (`active`, `deleted`, `archived`).

## Requests, pagination, and errors

Request options apply uniformly and the transport encodes them per family -- the accounting API spells filters `search[field]=value`, the business-scoped API spells them `field=value`:

```go
client.Invoices.List(ctx, acct,
    freshbooks.Include("lines"),
    freshbooks.Search{"status": "paid"},
    freshbooks.Sort("invoice_date", freshbooks.SortDesc),
    freshbooks.PageNumber(2),
    freshbooks.PerPage(50),
)
```

`Search` is both a request option and the type of the `Search` field on the per-resource list-option structs, so the same literal works in either position.

> The design spec calls the page-selecting option `Page`, which cannot coexist in Go with the `Page[T]` pagination type. The type keeps the short name because it appears in every `List` signature; the option is `PageNumber`.

> The accounting family's `search[field]=value` spelling is confirmed against the live API; the business-scoped family's bare `field=value` is taken from the FreshBooks documentation and has not been exercised live.

`List` returns `Page[T]{Items, Page, Pages, PerPage, Total}`. `All` is the auto-paginating iterator:

```go
for inv, err := range client.Invoices.All(ctx, acct, nil) {
    if err != nil {
        return err
    }
    // ...
}
```

It stops at the first error -- yielding it once, with a zero item -- and at context cancellation, and it stops fetching as soon as you `break`.

Failures are `*Error`:

```go
if errors.Is(err, freshbooks.ErrNotFound) { ... }

var apiErr *freshbooks.Error
if errors.As(err, &apiErr) {
    apiErr.StatusCode  // HTTP status
    apiErr.Code        // the accounting family's errno, 0 elsewhere
    apiErr.Field       // the offending field, when the API names one
    apiErr.Raw         // the undecoded body
    apiErr.RetryAfter() // non-zero on a rate limit that carried the header
}
```

The sentinels are `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrValidation`, and `ErrRateLimited`. An error message never renders the raw body, so a failing call cannot spill a credential into a log.

## Retries

By default: three attempts, 500ms base delay doubling each time, capped at 30 seconds, with full jitter, on 429, 502, 503, 504, and transport failures. A `Retry-After` header wins over the computed backoff but is still capped by `MaxDelay` -- a client should not block for an arbitrary period because a header said so. A cancelled context stops the loop immediately.

### Retries make every call at-least-once

This matters for writes. A 502, a 504, or a network timeout can all arrive *after* the server has already processed the request; the retry replays the body, so a `POST` that creates an invoice or a payment can create two. With retrying enabled, every method is at-least-once, not exactly-once.

The library does not yet gate retries by idempotency. Until it does, a write you cannot afford to duplicate should go through a client built with `freshbooks.WithRetry(freshbooks.NoRetry)`, handling the transient statuses yourself:

```go
writer, err := freshbooks.NewClient(
    freshbooks.WithTokenSource(src),
    freshbooks.WithRetry(freshbooks.NoRetry),
)
```

Reads are unaffected -- replaying a `GET` costs a round trip and nothing else.

## The escape hatch

```go
var out map[string]any
err := client.Do(ctx, http.MethodGet,
    "/accounting/account/"+string(acct)+"/systems/systems/1", nil, &out)
```

`Do` uses the same transport as every generated method, so an unmodelled endpoint still gets authentication, retry, envelope unwrapping, and family-specific error decoding. The family is inferred from the path prefix.

## Examples

The package `Example` and `ExampleAll` in `example_test.go` are runnable and appear on pkg.go.dev.
