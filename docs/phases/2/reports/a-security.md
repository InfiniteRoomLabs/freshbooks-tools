# Phase 2 batch a -- security lane report

- Branch: `phase-2/a` (3 commits: `ef05323`, `eb209c7`, `08af234`)
- Worktree: `.worktrees/a`
- Diff reviewed: `git diff main...phase-2/a` -- 23 files, +3136 / -51
- Mode: READ-ONLY. Ran `git`, `grep`, `scripts/redaction-check.sh`. No edits, no commits, no `mise`/test/build runs.
- `govulncheck` is not installed in this worktree, so it could not be run. Moot for this diff: `go.mod`, `go.sum`, and `.github/` are untouched, so the dependency graph is byte-identical to `main`.

## Verdict: PASS

No blocking finding. Four advisories, one of which (A1) is pattern-setting and should get a decision before the CLI and MCP batches start feeding these methods untrusted identifiers.

## Findings

### A1 -- ADVISORY -- caller-supplied string path segments reach the URL unescaped and unvalidated

`freshbooks/invoices.go:435`, `freshbooks/payments.go:259,262-263`, `freshbooks/items.go:173`, `freshbooks/invoice_profiles.go:254`, plus the `fmt.Sprintf` payment-options paths at `invoices.go:415`, `invoice_profiles.go:249`, `payments.go:254`.

Evidence. Every accounting path is built by interpolating `AccountID` (an untyped `string`, `types.go:16`) straight into the path, and `checkoutLinkPath(acct, id string)` does the same for a checkout-link id. `Client.resolve` (`transport.go:104-124`) then does `url.Parse(path)` and `u.Path = base + "/" + ref.Path`, so anything in those strings is interpreted, not escaped:

- `acct = "ACM123?per_page=100"` -> `ref.Query()` picks the pair up and it is merged into the real query string alongside the library's own options.
- `acct = "ACM123/../.."` -> dot segments survive `url.URL.String()` (Go does not clean them client-side) and the server resolves them, retargeting the authenticated request at a different endpoint.

Note that fixing this at the call site does not work: `url.PathEscape` produces `%2F`, `url.Parse` decodes it back into `ref.Path`, and `resolve` rebuilds `u.Path` from the decoded form, so the escape is undone. The guard has to live in `resolve` or in a shared segment validator.

Why advisory, not blocking. Impact is bounded today: the redirect hook strips `Authorization` across hosts (`client.go:186-196`) so no token leaves `api.freshbooks.com`; server-side authorization still governs which account the token may read; and the only current caller is library-internal test code passing literals. It becomes real when the CLI (`-a/--account` flag, config file) and the MCP server (tool inputs authored by a model) start passing these through. `BusinessID` is not affected -- it is `int64` with a `String()` (`types.go:21-24`), so its segments can only be digits.

Fix. Either reject unsafe segments centrally in `resolve` (fail on a `ref.Path` containing a `.` or `..` segment, and on a non-empty `ref.RawQuery` from a path the library built), or add a `pathSegment(string) (string, error)` guard used by every `*Path` helper before interpolation. Whichever is chosen, it is one change in the lib and it should land before batches b/c/d copy the current shape further.

### A2 -- ADVISORY -- the raw PDF fetch asks for JSON and never checks what it got back

`freshbooks/invoices.go:441-467` (`Client.fetchRaw`), with `freshbooks/transport.go:190-196` (`newRequest`).

Evidence. `fetchRaw` reuses `newRequest`, which unconditionally sets `Accept: application/json`. So `Invoices.PDF` requests a PDF while telling the server it wants JSON, and on the way back it returns `raw` with no `Content-Type` or magic-number check. A 200 carrying an HTML interstitial, a login redirect landing page, or a JSON body is handed to the caller as if it were PDF bytes; a caller that writes the result to `invoice.pdf` writes whatever arrived.

Header handling is otherwise correct: `Authorization` is resolved through the same `c.authorization` path as JSON requests and is not logged (`roundTrip` logs `redactPath(endpoint)`, which drops query, fragment, and userinfo -- `transport.go:290-297`).

Fix. Let `newRequest` take an `Accept` value so `PDF` can send `application/pdf`, and have `PDF` reject a response whose `Content-Type` is not a PDF (or that does not start with `%PDF-`) rather than returning it.

### A3 -- ADVISORY -- the raw fetch path skips the retry loop and Retry-After handling

`freshbooks/invoices.go:441-467`.

Evidence. `fetchRaw` calls `c.roundTrip(ctx, req, fam, 1)` once, outside the attempt loop in `do` (`transport.go:65-96`). A 429 with `Retry-After` on a PDF download therefore surfaces to the caller immediately, while the identical 429 on any JSON endpoint is retried with the server's own backoff. Availability and rate-limit-politeness, not confidentiality -- and the error is still decoded correctly (verified by the `[sad]` case at `invoices_test.go:303-308`, which gets `ErrNotFound`, not raw bytes).

Fix. Route `fetchRaw` through the same attempt loop as `do` (the two differ only in `decodeBody`), or state the deviation in the `PDF` doc comment so callers know they own the retry.

### A4 -- ADVISORY -- share links are credential-equivalent and are not marked as such

`freshbooks/invoices.go:369-380` (`InvoiceShareLink`), `invoices.go:382-397` (`ShareLink`).

Evidence. `share_link` is an unauthenticated URL that lets anyone holding it view or download the invoice -- functionally a bearer token with a business's billing data behind it. The library itself handles it correctly (it appears only in a returned struct; nothing logs response bodies). The risk is downstream: the CLI's `table`/`json` output and MCP tool results will echo the field by default, and MCP tool results land in a model transcript.

Fix. No change needed in this batch. Add a doc-comment warning on the field now, and carry a note into the CLI and MCP work orders to treat share links like a token in logs and default output.

## Checklist results

1. **Secrets never leak** -- clean. No token, client secret, or `Authorization` value appears in any new code, error string, fixture, or doc comment. No logging statements were added at all (`grep` for `logger.`/`slog.` over the added lines: 0 hits). `%v` appears only inside `t.Fatalf` on already-decoded response bodies and errors, never on a request, a header, or a token. No `String()` methods were added on token- or request-carrying types.
2. **Credential storage** -- not applicable; this batch touches no file, config, or token-store code.
3. **OAuth flow** -- not applicable; `freshbooks/auth/` is untouched. Every new method resolves its bearer through the existing `c.authorization` seam, so refresh rotation and its write-back are unchanged.
4. **Transport** -- TLS untouched (`DefaultBaseURL` is `https://`, no `InsecureSkipVerify` anywhere). The 30s `http.Client` timeout and the cross-host `Authorization`-stripping `CheckRedirect` (`client.go:110,186-196`) apply to the new PDF path too, since `fetchRaw` uses `c.httpClient` via `roundTrip`. **Bounded reads confirmed:** `fetchRaw` inherits `roundTrip`'s `io.LimitReader(resp.Body, maxResponseBytes+1)` and the explicit over-limit rejection (`transport.go:139-146`), so a hostile or runaway PDF response cannot exhaust memory -- 10 MiB ceiling. `Retry-After` parsing is defensive and unchanged; see A3 for the PDF path not using it.
5. **Trust boundaries** -- every response decodes into a typed struct; no `any`-typed response decoding, no reflection, no `unsafe`, no `exec`, no filesystem access, no `os.Getenv` in the diff (grep: 0 hits). The one gap is A1, string path segments.
6. **Stateless MCP** -- not applicable; no `mcp/` changes.
7. **Supply chain** -- `go.mod`, `go.sum`, and `.github/` are unchanged (`git diff --stat` over those paths is empty). No new dependencies. `govulncheck` unavailable locally; no new attack surface for it to find.
8. **Public-repo hygiene** -- clean. `scripts/redaction-check.sh` -> `redaction-check: clean`. Manual sweep of all 10 new fixtures and the tests found only synthetic data: account `ACM123` (the FreshBooks docs placeholder), ids `90001`/`55001`/`40001`/`5001`/`2200`/`8675309`, organizations "Example Signs Co" and "Trailhead Gear", contact "Alex Example", email `client@example.test`, and share link `https://my.freshbooks.com/#/link/example`. No real FreshBooks account or business ids, no tokens, no internal hostnames or IPs, no vault item names, no personal correspondents. The only URLs in the diff are `semver.org` (CHANGELOG boilerplate) and the `my.freshbooks.com` example.
