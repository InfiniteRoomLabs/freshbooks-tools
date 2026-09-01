# Phase 2 batch b -- security lane report

- **Branch:** `phase-2/b` (rebased onto `main` containing merged batch a)
- **Worktree:** `.worktrees/b`
- **Diff:** `git diff main...phase-2/b` -- 60 files, +4609 / -89
- **Commits:** 6 feature + 2 docs (`1d1bf6e`..`a2d3c9b`)
- **Tree state:** `git status --porcelain` empty before and after this review
- **Verdict: BLOCK** -- 1 BLOCKING finding, 2 ADVISORY

Batch b adds ten accounting resource services plus fixtures. It touches no auth, transport, credential-storage, OAuth, or MCP code, so template checks 2, 3, 4, and 6 have no diff surface. Checks 1, 5, 7, and 8 were exercised and are reported below.

---

## BLOCKING

### S1 -- All 22 path builders interpolate an unvalidated caller-supplied `AccountID` (batch a finding A1, recurring at full batch scale)

**File:line (all 22 sites):**

| File | Lines |
|---|---|
| `freshbooks/clients.go` | 155, 159 |
| `freshbooks/contacts.go` | 46 |
| `freshbooks/credit_notes.go` | 136, 140 |
| `freshbooks/estimates.go` | 193, 197 |
| `freshbooks/expenses.go` | 142, 146, 267, 285, 347 |
| `freshbooks/expense_categories.go` | 86, 90 |
| `freshbooks/taxes.go` | 89, 93 |
| `freshbooks/bills.go` | 127, 131 |
| `freshbooks/bill_payments.go` | 61, 65 |
| `freshbooks/bill_vendors.go` | 114, 118 |

**Evidence.** `grep -c pathSegment` across all ten new `.go` files returns **0**. Every builder is the unchecked shape:

```go
func clientsPath(acct AccountID) string {
	return fmt.Sprintf("/accounting/account/%s/users/clients", acct)
}
```

`main` already carries the helper and the convention batch a landed for exactly this (`freshbooks/transport.go:190-209`, applied at `invoices.go:514,527,541`, `items.go:186`, `payments.go:179,321,333`, `invoice_profiles.go:268,277`). The established form returns an error:

```go
func itemsPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/items/items", acct), nil
}
```

Batch b was written before A1 landed and never picked it up. **52 exported methods** across the ten services take `acct AccountID` and reach an unvalidated builder (clients 6, contacts 2, credit_notes 5, estimates 8, expenses 9, expense_categories 4, taxes 6, bills 5, bill_payments 2, bill_vendors 5).

**Reproduced, not theorized.** I compiled a temporary probe against `Client.resolve` with `clientsPath` (file created and removed; tree verified clean afterward):

```
acct="AB?x=1"               -> https://api.example.com/accounting/account/AB?x=1%2Fusers%2Fclients   err=<nil>
acct="AB#frag"              -> https://api.example.com/accounting/account/AB                          err=<nil>
acct="AB/expenses/expenses" -> https://api.example.com/accounting/account/AB/expenses/expenses/users/clients  err=<nil>
acct="AB%2f..%2f"           -> ""   err=freshbooks: request path ".../AB/..//users/clients" contains a directory traversal segment
```

Three of four hostile segments pass silently:

1. **`#` truncates the request path entirely.** `GET .../users/clients` becomes `GET /accounting/account/AB` -- a different endpoint, no error, and the typed envelope decode is what fails (or worse, succeeds against unintended data).
2. **`?` truncates the path and injects attacker-chosen query parameters** into an authenticated request.
3. **`/` injects arbitrary path segments**, letting a crafted account id steer an authenticated, token-bearing request at a different accounting endpoint than the method name promises.

The `noTraversal` backstop (`transport.go:179-186`) catches only case 4 (`.`/`..`). Its own doc comment states it is a "defense-in-depth backstop behind `pathSegment`" and assumes "every `*Path` builder validates its caller-supplied identifiers" -- an invariant batch b breaks for 22 of them.

**Why this is BLOCKING rather than advisory.** It is the identical defect the gate already ruled blocking in batch a (A1), so letting it merge would both reintroduce the vulnerability across ten more services and split the codebase into two contradictory conventions. `AccountID` is not a trusted internal value: per `pathSegment`'s own comment it will arrive from CLI flags, config files, and model-authored MCP tool inputs in phases 3 and 4. Fixing it after those consumers exist is strictly harder.

**Fix.** Mirror `itemsPath`/`invoicesPath` exactly: change all 19 named builders to `(string, error)`, guard with `pathSegment(string(acct))`, and propagate at the 52 call sites. Fold the 3 inline `fmt.Sprintf` paths (`expenses.go:267,285,347`) into guarded builders rather than adding bare inline guards. Add a `[sad]` table case per service asserting a rejected account id (`"a/b"`, `"a?b"`, `"a#b"`) returns an error and issues no HTTP request -- `transport_test.go:580` is the existing pattern.

---

## ADVISORY

### S2 -- `govulncheck` is not installed, so check 7's vulnerability scan could not run

`command -v govulncheck` finds nothing on this machine. Batch b adds **no dependencies** -- `git diff main...phase-2/b --stat -- '*go.mod' '*go.sum'` is empty, and the module stays stdlib + testify -- so residual supply-chain risk from this diff is minimal and this does not gate the merge. Worth installing via `mise` so the gate's security lane can run it for the phases that do add dependencies (`mcp` pulls go-sdk + cobra).

### S3 -- Soft-delete `vis_state` divergence is documented but unverified

The spec callout added at `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` records that `ExpensesService.Delete` sends `vis_state: 1` against a Postman example showing `0`. The reasoning is sound and correctly marked INFERRED, and it is a correctness matter for the code-review lane rather than a security one. Flagging only because a delete verb that silently un-deletes (or vice versa) is a data-integrity risk on real books -- it belongs on the live-conformance checklist, which the callout already says.

---

## Checks that passed, with evidence

**1. Secrets never leak -- PASS.** `grep -nE 'slog|log\.|fmt\.Print|Authorization|Bearer|token|secret|unsafe'` across all ten new `.go` files returns **zero matches**. The new code does no logging, handles no credentials, and adds no `String()` methods. Error strings wrap resource nouns and ids only; no request/response struct is `%v`-formatted. Fixtures carry no tokens or `Authorization` headers.

**5. Trust boundaries -- PARTIAL, see S1.** Positives: every response decodes into a typed envelope struct (`customerListEnvelope`, etc.) -- no `map[string]any`, no `interface{}` escape hatches. Nil-request guards are present across all ten services (`bills` 3, `contacts` 2, `expense_categories` 3, `bill_payments` 4, `clients`/`taxes`/`credit_notes`/`bill_vendors` 5, `estimates` 6, `expenses` 7), so a nil body returns an error instead of panicking. No manual query-string assembly anywhere: every list method routes options through `opts.requestOptions()` into the shared `RequestOption` machinery, which encodes via `url.Values.Encode()` in `resolve` -- so `search[field]=value` filter values are properly escaped and cannot inject query structure. No shell-outs, no file-path handling, no `unsafe`. The one boundary that is unguarded is the path segment itself (S1).

**7. Supply chain -- PASS (scan caveat in S2).** No `go.mod`/`go.sum`/workflow changes in the diff. No new dependencies to justify.

**8. Public-repo hygiene -- PASS.** `scripts/redaction-check.sh` exits 0 ("redaction-check: clean"). An independent sweep of the full diff against the configured redaction-term list (internal addresses, internal domains, mirror hostnames, operator paths and identifiers, vault folder names) returns **zero matches**.

**Fixture hygiene -- PASS.** All 35 new fixtures audited:

- Emails are all RFC 2606 reserved: `client@example.com`, `newclient@example.com`, `secondary@example.com`, `vendor@example.com`.
- The only account identifier is `"accounting_systemid": "ACM123"` -- synthetic.
- Phone `4155550100` sits in the reserved fictional 555-0100..555-0199 range.
- Names and orgs are generic placeholders (`Alex`, `Jordan`, `Alexandra Example`, `Secondary Contact`, `Example Signs Co`, `New Client Co`, `Example Supplies Co`), addresses are `1 Example Ave.` / `Example City` / `94000`. No real correspondents, no employer-derived data.

**`ignore.list` -- clean.** The delta is 59 deletions and **0 additions**, matching the 59 assigned inventory keys. No entry was silently added to mask an uncovered endpoint.

---

## Method note

Read-only throughout, with one disclosed exception: to move S1 from "looks exploitable" to reproduced evidence I wrote a temporary probe test into `freshbooks/`, ran `go test -run TestSegInject` on it, and deleted it. `git status --porcelain` was empty afterward and is empty now. No other file was created or modified, nothing was committed, and no `mise run check`, build, or full test run was invoked. This report file is written but not committed, per the work order.

## Verdict

**BLOCK** on S1. The fix is mechanical -- it is the same change batch a already made in four files, applied to ten more -- but it must land before merge so the `pathSegment` invariant holds uniformly across the library and `noTraversal`'s documented assumption stays true. S2 and S3 are advisory and do not gate.
