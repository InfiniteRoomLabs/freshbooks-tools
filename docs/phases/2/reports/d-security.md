# Phase 2 batch d -- security lane

- Branch: `phase-2/d` (rebased onto main containing a+b+c)
- Path: `.worktrees/d`
- Diff: `git diff main...phase-2/d` -- 65 files, +4414/-89
- Scope: transport (multipart upload, host override, raw fetch), payment-card tokenization, webhook callbacks, ledger accounts, journal entries, other income, reports, uploads
- Read-only lane. `scripts/redaction-check.sh` run (clean). `govulncheck` is not installed on this machine; no `go.mod`/`go.sum` changes in the diff, so the dependency surface is unchanged from the last clean run.

## Verdict: BLOCK

Three blocking findings. The batch is well built -- the upload bound, the replayable multipart body, and the host-override design are all correct -- but two credential-handling gaps sit directly on the card path and one library-wide input-validation convention was dropped across every new file.

---

## BLOCKING

### 1. `doOnHost` inherits the scheme from the caller's base URL, so raw card data can leave over plaintext HTTP to a real production host

`freshbooks/transport.go:72-89`

```go
u := *c.baseURL
u.Host = host          // hard-coded "paid.freshbooks.com"
u.Path = path
endpoint := u.String()  // scheme comes from c.baseURL, unchecked
```

`WithBaseURL` (`freshbooks/options.go:47-60`) accepts any absolute URL and only rejects an empty scheme or host -- `http://` passes. The host is then overridden to the hard-coded real host, but the scheme is not. A client configured against an `http://` base URL (a local mock, a dev proxy, a plaintext staging endpoint -- and the batch's own test does exactly this at `payment_options_test.go:30`, `WithBaseURL("http://api.freshbooks.test")`) will send `FBPayTokenize` / `StripeTokenize` to `http://paid.freshbooks.com/...` -- carrying the full PAN, CVV, expiry, and the account's OAuth bearer token in cleartext to a host that is on the public internet regardless of what the operator pointed the rest of the client at.

This is the one call path in the package where a base-URL override does not keep the request inside the operator's chosen environment. Every other request follows `c.baseURL` wholesale, so a plaintext base URL only ever reaches a plaintext host the operator picked.

The vendor's own collection hard-codes `https://paid.freshbooks.com/gateway/fbpay/tokenize` and `.../gateway/stripe/payment-method`. The implementation should not be more permissive than the source it is modelled on.

Fix: force the scheme in `doOnHost`, since the host is already forced:

```go
u := *c.baseURL
u.Scheme = "https"
u.Host = host
```

If the test needs plaintext, give the test a `httptest.NewTLSServer` or an unexported seam -- not the production code path.

### 2. Raw-card request structs have no redacting `String()`, so a single `%+v` or `slog.Any` prints a full PAN and CVV

`freshbooks/payment_options.go:16-38`

```go
type FBPayTokenizeRequest struct {
    Name        string `json:"name"`
    CardNumber  string `json:"card_number"`
    ...
    CVV         string `json:"cvv"`
    ...
}
type StripeTokenizeRequest struct {
    CardNumber  string `json:"card_number"`
    APIKey      string `json:"api_key"`
    ...
}
```

The library already established the countermeasure for secret-bearing structs in an earlier batch -- `Application.String()` at `freshbooks/settings.go:161-164` and `ApplicationUpdateRequest.String()` at `settings.go:222-225` both render `ClientSecret: redacted`. Batch d introduces two structs holding data strictly more sensitive than a client secret and gives neither a `String()`.

Consequence: `fmt.Errorf("tokenize failed for %+v: %w", req, err)`, `log.Printf("%v", req)`, `slog.Any("request", req)`, or any `%v` on a slice/map containing one dumps card number + CVV into logs. Downstream consumers (the CLI and MCP server, both in this repo's roadmap) are the likely offenders, and by then the struct is public API and the omission is baked in.

Fix: mirror the existing precedent.

```go
// String renders the tokenize request with the card data redacted.
func (r FBPayTokenizeRequest) String() string {
    return fmt.Sprintf("freshbooks.FBPayTokenizeRequest{Name: %q, CardNumber: redacted, CVV: redacted, ExpiryMonth: %q, ExpiryYear: %q}",
        r.Name, r.ExpiryMonth, r.ExpiryYear)
}
```

Same for `StripeTokenizeRequest` (redact `CardNumber` and `APIKey`). Add a test asserting the rendered string does not contain the fixture PAN -- the `settings.go` precedent should already have one to copy.

### 3. `pathSegment` is not called anywhere in batch d -- zero calls across nine new files that interpolate caller-supplied strings into request paths

`freshbooks/transport.go:314` defines `pathSegment`, and the rest of the library calls it on every caller-supplied string segment -- 30+ call sites across `bills.go`, `clients.go`, `expenses.go`, `invoices.go`, `payments.go`, `settings.go`, `team_members.go`, and others. Batch d has **zero**:

| file:line | unvalidated segment |
|---|---|
| `attachments.go:36` | `string(acct)` |
| `callbacks.go:59` | `string(acct)` |
| `images.go:29`, `images.go:44` | `string(acct)` |
| `gateways.go:66` | `string(acct)` |
| `payment_options.go:157`, `:175` | `string(acct)` |
| `ledger_accounts.go:111`, `:124` | `string(biz)` (`BusinessUUID` is a string type) |
| `ledger_accounts.go:138`, `:153` | `string(biz)` **and** `accountUUID` |
| `ledger_accounts.go:201` | `id` |
| `journal_entries.go:116`, `:133`, `:178` | `string(acct)` |
| `other_income.go:90` | `string(acct)` |
| `reports.go` (13 methods) | `string(acct)` |
| `reports.go:247` | `string(acct)` **and** `downloadToken` |

`resolve()` does catch the worst case -- `url.Parse` rejects an absolute path and `noTraversal` rejects `..` -- so this is not host escape. What it does allow: a `?` or `#` in a segment silently converts the rest of the path into a query string or fragment (reshaping which endpoint is hit and which query parameters the server sees), and an empty `AccountID` produces a malformed path like `/events/account//events/callbacks` that fails confusingly at the server instead of clearly at the client. `doOnHost` is worse -- see advisory 5.

`BusinessUUID`, `accountUUID`, sub-type `id`, and `downloadToken` are the sharpest edges: all bare `string`, all fed straight from an API response or caller input.

Fix: add the standard guard at the top of each method, matching e.g. `payments.go:321-325`:

```go
if err := pathSegment(string(acct)); err != nil {
    return nil, err
}
```

`inventory-check` and the coverage gate will not catch this class -- it needs the explicit sweep.

---

## ADVISORY

### 4. The raw-card structs are not doc-commented as PCI-sensitive

`freshbooks/payment_options.go:15-38`. The doc comments say only "is the payload for PaymentOptionsService.FBPayTokenize". Nothing warns a caller that these fields carry an unmasked PAN and CVV, that the values must never be logged, persisted, or placed in an error, and that the struct exists only to be handed straight to `FBPayTokenize` and discarded. The method comments do note the card "never [goes] through the regular FreshBooks API host", which is the right instinct -- extend it to the structs. Pair this with finding 2; the redacting `String()` is the enforcement and the doc comment is the notice.

### 5. `doOnHost` bypasses `resolve()`, so it skips `noTraversal` and inherits the base URL's query string

`freshbooks/transport.go:72-89` builds the URL by hand rather than going through `c.resolve()`. Two consequences: the path never sees `noTraversal()`, and `u := *c.baseURL` carries over `c.baseURL.RawQuery` (`u.Path = path` sets only the path), so a base URL configured with a trailing query would silently append those parameters to a tokenization request. Both call sites pass a compile-time constant path today, so neither is live -- but this is the same class as finding 3 and the function is now a general-purpose seam. Add `if err := noTraversal(path); err != nil { return err }` and `u.RawQuery = ""`.

### 6. Tests use non-standard, real-looking card numbers instead of the canonical synthetic PANs

`freshbooks/payment_options_test.go:56` uses `4500123456789012` and `:98` uses `450001234567809012` (18 digits). Both are copied verbatim from the vendor's published Postman collection, so this is not a leak of anything private -- but `4500...` is a plausible live Visa BIN, and a repo-wide card-number scanner or a DLP tool pointed at this public repo will flag it. `payment_options_test.go:98` also uses `pk_live_example` as a Stripe publishable key placeholder; publishable keys are not secrets, but the `pk_live_` prefix invites the same false positive.

Fix: use the universally recognised test PANs -- `4111111111111111` (Visa) or `4242424242424242` (Stripe) -- and `pk_test_example`. Fixtures elsewhere in the repo already follow the synthetic-ID convention (`8675309`, `4242424`, `00000000-0000-4000-8000-...`), so this is consistency as much as hygiene.

### 7. The multipart filename is not reduced to its base name

`freshbooks/transport.go:106` passes the caller's `filename` straight to `multipart.Writer.CreateFormFile`. I checked Go 1.26's `quoteEscaper` (`$GOROOT/src/mime/multipart/writer.go:128`) -- it escapes `\`, `"`, `\r`, and `\n`, so **header injection is not possible** and the earlier-suspected CRLF vector does not exist. What remains: directory components pass through unmodified, so `Images.Upload(ctx, acct, "../../etc/passwd", r)` sends that literal string as the part's filename. Exploitability is entirely FreshBooks-side and unknowable from here, but the client should not be the one shipping a traversal string. One line: `filename = filepath.Base(filename)` in `buildMultipartBody`, or reject a filename containing a separator.

### 8. `govulncheck` could not be run

Not installed on this machine. Mitigating: the diff changes no `go.mod` or `go.sum`, adds no dependency, and the library remains stdlib-only. The dependency risk is identical to the last batch that did get a clean scan. Worth installing it via `mise` so this lane can actually execute the check rather than reasoning around it.

---

## Checks that passed

- **Host override intent (priority check 1).** Confirmed. The vendor's Postman collection declares `oauth2` bearer auth on both `paid.freshbooks.com` requests, so sending the account's `Authorization` header to that second host is the documented, intended behaviour -- not an accidental credential spill. `tokenizationHost` is a package-level `const` (`payment_options.go:12`), `doOnHost` is unexported, and both call sites pass that constant; a caller cannot steer the bearer token to a host of their choosing.
- **Cross-host redirect protection from the override host.** `stripAuthOnCrossHostRedirect` (`client.go:190-201`) compares the next hop against `via[len(via)-1].URL.Host`, not against `c.baseURL` -- so a redirect away from `paid.freshbooks.com` strips the `Authorization` header exactly as one away from the API root does. The redirect budget (10) is enforced. The batch did not weaken this.
- **Upload bound (priority check 2).** `maxUploadBytes = 10 << 20`, enforced with the correct `io.LimitReader(r, max+1)` + `n > max` idiom (`transport.go:100-108`) -- no off-by-one, no silent truncation.
- **Retry replay.** The multipart body is materialised into a `[]byte` once, up front, and `newRequest` wraps it in a fresh `bytes.NewReader` per attempt (`transport.go:383-386`). A retried upload replays byte-identical; the non-rewindable `multipart.Writer` is never reused. `Content-Length` is set explicitly. This is the correct design and the doc comment explains why.
- **No content sniffing.** `CreateFormFile` sets `application/octet-stream`; the client never inspects or guesses the uploaded bytes' type.
- **No card data in logs or errors.** The package's only logger is the injectable `slog.Logger` (`client.go:25`, defaulting to `slog.DiscardHandler`); no `log.`, `fmt.Print`, or `os.Stderr` write exists in non-test code. No error string in `payment_options.go` interpolates a request struct or a card field. (Finding 2 is about what a *caller* can trivially do, not about what this package does.)
- **Webhook verifier codes.** `Verify` (`callbacks.go:113-127`) places the verifier in the request body only -- never in the path, never in a query string, never in an error message. The test's verifier is the obvious placeholder `the-verifier-code`. Nothing leaks.
- **TLS.** Nothing in the diff disables verification, sets `InsecureSkipVerify`, or constructs a `tls.Config`. (Finding 1 is a scheme-selection gap, not a verification gap.)
- **Supply chain.** No `go.mod`/`go.sum` change, no new dependency, no `unsafe`, no shell-out. Library remains stdlib-only.
- **Public-repo hygiene.** `scripts/redaction-check.sh` reports clean. I read every added fixture: account IDs are the synthetic `ACM123`/`8675309`/`4242424` family, UUIDs are the `00000000-0000-4000-8000-...` pattern, hosts are `example.com`/`.test`, and names are placeholders. No internal hostname, IP, vault item name, real account or business ID, token, or personal correspondent appears anywhere in the diff. The only hygiene nit is advisory 6.
- **`familyForPath` additions.** The new `/uploads/`, `/payments/`, and ledger-accounts cases are ordered correctly (the specific ledger case precedes the general `/accounting/` one) and each carries a dated INFERRED/CONFIRMED provenance comment naming its source, per the repo's inferred-vs-confirmed rule.

## Re-gate

Findings 1, 2, and 3 must land before merge. 1 and 2 are a few lines each; 3 is a mechanical sweep across nine files plus the matching sad-path tests. Advisories 4-7 are cheap and I would take them in the same fix commit; 8 is tooling, not code.
