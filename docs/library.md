# Library

## Client construction

`freshbooks.NewClient` and its `With*` options (`WithTokenSource`, `WithHTTPClient`, `WithBaseURL`, `WithUserAgent`, `WithLogger`, `WithRetry`, `WithClock`). Lands in Phase 1.

## Services and ID types

Why `AccountID`, `BusinessID`, and `BusinessUUID` are distinct types instead of one stringly-typed ID, and the full list of resource services as fields on `*Client`. Lands in Phase 1 (types pre-declared) and Phase 2 (implementations, in four parallel batches by Postman folder).

## Requests, pagination, and errors

The `List`/`All`/`Get`/`Create`/`Update`/`Delete` method vocabulary, `Page[T]` vs. the `iter.Seq2` auto-paginating `All`, request options (`Include`, `Search`, `Sort`, `Page`, `PerPage`), and the `*Error` type with its `errors.Is`-matchable sentinels. Lands in Phase 1.

## The escape hatch

`(*Client).Do` for untyped calls that still get auth, retry, and error decoding for free. Lands in Phase 1.

## Examples

Runnable `Example` functions per resource once Phase 2 lands; linked from pkg.go.dev automatically.
