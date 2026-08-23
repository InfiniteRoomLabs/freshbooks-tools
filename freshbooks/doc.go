// Package freshbooks is a handcrafted client library for the FreshBooks
// REST API (https://www.freshbooks.com/api/start).
//
// The FreshBooks API is really two API families living behind one base URL
// (https://api.freshbooks.com):
//
//   - Accounting: clients, invoices, expenses, estimates, payments, items,
//     taxes, bills, credit notes, and reports, scoped by a string
//     "account_id" and rooted at /accounting/account/{account_id}/.... These
//     endpoints wrap responses as {"response": {"result": {...}}} and errors
//     as {"response": {"errors": [...]}}.
//   - Business-scoped: projects, time entries, services, and team members,
//     scoped by an integer "business_id" and rooted at paths like
//     /projects/business/{business_id}/.... These endpoints return plain
//     JSON bodies with a "meta" pagination object.
//
// Because the two ID spaces are never interchangeable, the library exposes
// distinct types for each: AccountID (string) and BusinessID (int64), plus
// BusinessUUID (string) for the ledger-accounts endpoints. Passing the wrong
// type to a method is a compile error, not a runtime one.
//
// # Getting started
//
// Build a token source, build a client, and resolve the identifiers every
// other call needs:
//
//	src := auth.NewTokenSource(cfg, auth.NewFileStore(path))
//	client, err := freshbooks.NewClient(freshbooks.WithTokenSource(src))
//	memberships, err := client.Identity.Me(ctx)
//
// See the package Example for a runnable version. Client.Do is the escape
// hatch for endpoints this library does not model yet; it uses the same
// transport, so it gets authentication, retry, envelope unwrapping, and
// error decoding for free.
//
// # Errors and retries
//
// Every failing call returns an *Error carrying the HTTP status, the
// accounting family's errno and field where present, and the raw body.
// Match it with errors.Is against ErrUnauthorized, ErrForbidden,
// ErrNotFound, ErrValidation, or ErrRateLimited. The transport retries 429,
// 502, 503, and 504 with jittered exponential backoff, honouring
// Retry-After up to the policy's MaxDelay; WithRetry(NoRetry) turns that
// off.
//
// Nothing in this package logs a token, a request body, or a response body:
// error strings carry the status and the server's message only, and the
// debug log records method, URL, status, and attempt number.
//
// See docs/library.md in the repository root for the programmer-facing
// walkthrough, and docs/authentication.md for the OAuth2 flow and token
// lifecycle.
package freshbooks

// Version is the current release version of the freshbooks module. It is
// overwritten by the release tooling at build time; the value here is the
// development placeholder.
const Version = "0.0.0-dev"
