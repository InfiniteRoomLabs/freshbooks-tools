# Phase 2 batch c -- security lane report

- Branch: `phase-2/c` (rebased onto main with batches a+b merged)
- Worktree: `.worktrees/c`
- Diff reviewed: `git diff main...phase-2/c` (58 files, +3595/-50)
- Mode: READ-ONLY. No edits, no commits, no `mise run check`/test/build runs.
- Tooling run: `scripts/redaction-check.sh` (clean). `govulncheck` is **not installed** on this machine -- supply-chain vuln scan NOT performed; see finding 4.

## Verdict: **BLOCK**

Two blocking findings, three advisory. Both blockers are mechanical, single-pattern fixes -- neither is a design problem.

---

## 1. BLOCKING -- `pathSegment` validation is missing on every caller-supplied string path segment in the batch

**Evidence.** `freshbooks/transport.go:190-208` defines `pathSegment(s string) error`, and its own doc comment states the threat model plainly: *"These values come from callers today, and once the CLI and MCP server exist will also arrive from flags, config files, and model-authored tool inputs -- so a segment carrying a slash, a query/fragment delimiter, or a `..` traversal must never reach `url.Parse` silently."*

Batches a and b honour this at 24 call sites (`clients.go:160`, `invoices.go:514,527,541`, `expenses.go:154,298,321,391`, `payments.go:179,321,324,333,344`, `taxes.go:82`, `estimates.go:184`, `items.go:186`, `bills.go:137`, `bill_vendors.go:139`, `bill_payments.go:61`, `credit_notes.go:133`, `expense_categories.go:76`, `contacts.go:46`, `invoice_profiles.go:268,277`).

Batch c calls it **zero times**. `grep -rn "pathSegment" freshbooks/*.go` returns no batch-c file. Fifteen unvalidated interpolations of a caller-supplied string:

| file:line | method | unvalidated segment |
|---|---|---|
| `freshbooks/settings.go:81` | `IdentityService.DeleteBusinessSubscription` | `string(accountID)` -- **on a DELETE** |
| `freshbooks/settings.go:112` | `IdentityService.ProvisionPayments` | `string(accountID)` |
| `freshbooks/settings.go:195` | `IdentityService.UpdateApplication` | raw `clientID string` |
| `freshbooks/staff.go:98` | `StaffService.Get` | `accountID` via `%s` |
| `freshbooks/staff.go:122` | `StaffService.Update` | `accountID` via `%s` |
| `freshbooks/staff.go:136` | `StaffService.Delete` | `accountID` via `%s` |
| `freshbooks/systems.go:44` | `SystemsService.Get` | `string(accountID)` |
| `freshbooks/tasks.go:44,57,77,114,127` | `TasksService.{List,Get,Create,Update,Delete}` | `accountID` |
| `freshbooks/services_svc.go:107,122` | `ServicesService.{Create,GetBillableItem}` | `accountID` |
| `freshbooks/team_members.go:80` | `TeamMembersService.Get` | `teamMemberUUID string` |

`int64` `BusinessID` interpolations (`businessID.String()`, `%d` ids) are inherently safe and are **not** flagged.

**Why `noTraversal` does not cover this.** `transport.go:177-188` only rejects a literal `.` or `..` segment, and its own comment calls itself "a defense-in-depth backstop **behind** `pathSegment`". Reading `resolve` (`transport.go:152-173`): the assembled path goes through `url.Parse`, then `u.Path` is rebuilt from `ref.Path` and `ref.Query()` is merged into the outgoing query. Consequences for an unvalidated segment:

- `accountID = "ACM123?x=1"` -> `url.Parse` splits at `?`: `ref.Path` becomes `/accounting/account/ACM123`, the rest becomes query. The request **silently targets a different endpoint** than the method's contract, with attacker-influenced query parameters merged in. `noTraversal` sees nothing wrong.
- `accountID = "ACM123/users/staffs/1"` -> no `.`/`..` segment, so `noTraversal` passes; the request lands on an unrelated resource under a method whose signature promised otherwise.
- `accountID = ""` -> `noTraversal` allows the empty segment; a malformed `/account//systems/...` request is issued. `pathSegment` rejects empty explicitly, so batch a/b returns a clear error where batch c issues the request.
- `#` truncates identically via the fragment split.

This is a trust-boundary regression against a control the repo already built and already applies everywhere else, and it lands on a destructive `DELETE` (`DeleteBusinessSubscription`) and on the Developer app-management surface. It is also exactly the surface Phase 3/4 will expose to CLI flags and model-authored MCP tool inputs.

**Fix.** Add the established guard at each of the 15 sites, matching the batch a/b shape, e.g. in `staff.go:98`:

```go
if err := pathSegment(string(accountID)); err != nil {
    return nil, err
}
```

and in `settings.go:195` validate `clientID` (a caller-supplied string id) the same way. `team_members.go:80` validates `teamMemberUUID`. Void-returning methods (`StaffService.Delete`, `DeleteBusinessSubscription`, `ProvisionPayments`) return the bare `err`.

---

## 2. BLOCKING -- fixture carries verbatim third-party account identifiers and a personal name

**Evidence.** `freshbooks/testdata/settings/business_added.json`:

```json
"id": 1986590,
"name": "Kevin's Corral",
"account_id": "LWY0VG",
"address": { "id": 2448988, ... }
```

Every one of these values is copied from the vendor Postman collection in-tree (`freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json` contains `"id": 1986590, "name": "Kevin's Corral", "account_id": "lwy0vG", ... "id": 2448988`, and elsewhere the person name `Kevin Melling`). The fixture merely uppercased the account id.

Every other fixture in the repo uses the synthetic `ACM123`: `grep -rho '"account_id": *"[^"]*"|"accountid": *"[^"]*"' freshbooks/testdata/` yields `ACM123` x9 and this one `LWY0VG`. The batch's own sibling fixtures (`staff/single.json`, `systems/get.json`, `team_members/*.json`) are correctly synthetic (`ACM123`, `Example Business LLC`, `*@example.com`) -- this file is the single outlier.

`CLAUDE.md` public-repo hygiene: *"no vault item names, internal IPs/domains, real FreshBooks account/business IDs, tokens, or personal correspondents. Fixture IDs are synthetic."* This violates the account-ID clause and the personal-name clause. `scripts/redaction-check.sh` passes because it only knows its configured term list; it cannot catch a vendor-sourced account id.

**Honest severity note for triage:** confidentiality impact is effectively nil -- these values already sit in the vendor's own publicly published collection, which is already committed to this repo. This is a stated-rule violation with a one-line fix, not a leak. Flagging it as blocking because the rule is explicit and the gate exists to enforce it; downgrade to advisory is a defensible team-lead call.

**Fix.** Replace with synthetic values consistent with the rest of the corpus, e.g. `"id": 1001, "name": "Example Business LLC", "account_id": "ACM123", "address": {"id": 2001, ...}`, and update the matching assertions in `freshbooks/settings_test.go`.

---

## 3. ADVISORY -- `Application` carries an OAuth `client_secret` with no redacted rendering

**Evidence.** `freshbooks/settings.go:120-131` (`Application`) and `:167-176` (`ApplicationUpdateRequest`) both expose `ClientSecret string \`json:"client_secret"\``. Neither type implements a redacting `String()` or `slog.LogValuer`.

The repo has an established precedent for exactly this: `freshbooks/auth/token.go:18,37-45` -- *"Token deliberately implements fmt.Stringer with a redacted rendering"* -- rendering `AccessToken: redacted, RefreshToken: redacted`.

`CreateApplication` returns a live client secret; `UpdateApplication` both sends and receives one. Any `fmt.Printf("%v", app)`, `slog` attribute, or error string built from these structs writes an OAuth application secret in the clear. This matters most downstream: per the locked design the CLI renders results as `json|yaml|table`, and the MCP server is stateless and model-facing -- an MCP tool returning `Application` would place a client secret into model conversation context.

No secret leak exists in this diff today (batch c contains no `slog`/`fmt.Errorf` call that formats these structs; `grep` for `Authorization|token|Bearer` across all nine new service files returns nothing, and the transport's `redactPath` handling is untouched). This is a latent hazard, not a live one.

**Fix.** Mirror `auth.Token`: add `func (a Application) String() string` (and one on `ApplicationUpdateRequest`) rendering `ClientSecret: redacted`, with a doc comment stating why. Additionally, note in `Application`'s doc comment that MCP tool output must strip `client_secret`, so the Phase 3 MCP work inherits the constraint explicitly rather than by luck.

---

## 4. ADVISORY -- supply-chain scan not performed; no dependency or workflow surface changed

**Evidence.** `git diff main...phase-2/c -- go.sum go.work go.work.sum freshbooks/go.mod freshbooks/go.sum .github/` is **empty**. No new dependency, no `go.sum` change, no CI/release workflow change -- so template item 7's dependency-justification and workflow-permissions checks have no surface in this batch, and the pinning/least-privilege posture established earlier is unchanged.

`govulncheck` is not installed on this host (`command -v govulncheck` -> not found), so the known-vulnerability scan the template calls for did **not** run. Recording that as an unverified item rather than silently claiming clean. Given zero dependency delta, batch c cannot have introduced a new vulnerable dependency; the residual risk is a newly *disclosed* vuln in an existing dep, which is unrelated to this branch.

**Fix.** None required for this batch. Suggest the QA lane (which is permitted to run tooling) or CI install `govulncheck` so the security lane can execute item 7 for real on future batches.

---

## 5. ADVISORY -- destructive internal-host-derived endpoints: doc comments are honest, gating is deferred

Per the work order's item 2, I examined both endpoints rewritten from internal `my.freshbooks.com` Postman entries.

**Accidental triggering: no risk found.** Both require an explicit call with explicit identifiers; neither is reachable as a side effect of a read path, neither is invoked by any `List`/`Get`/`All` helper, and neither has a default or zero-value trigger. `ProjectsService.Delete` (`projects.go:161-165`) sends a soft-delete `vis_state` body rather than a hard delete. `IdentityService.DeleteBusiness` (`settings.go:64-67`) takes an `int64` `BusinessID` and, per its own comment, is refused by the API with a 422 while a subscription is active -- an accidental single call cannot destroy a business without the operator first having called `DeleteBusinessSubscription`.

**Doc-comment honesty: satisfied.** `projects.go:151-160` and `settings.go:69-79` each name the `my.freshbooks.com` provenance, state that the inventory tool rewrote the host, give the exact rewritten path, and label the behavior `INFERRED from that Postman example alone, never confirmed live`. `projects.go` additionally flags that the path root differs from its own `Get`/`Update` siblings. This is the standard the CLAUDE.md inferred-vs-confirmed rule asks for, and it is met.

**The gap, deferred not blocking.** Neither comment states that these are *irreversible account-level destructive operations that a later surface must gate behind explicit confirmation*. The library is correctly unopinionated about that -- gating belongs to the CLI and MCP layers -- but nothing currently carries the requirement forward to those phases. Combined with finding 1 (`DeleteBusinessSubscription` takes an unvalidated string `accountID`), a mistyped or injected account id on a destructive, never-live-verified endpoint is the worst-case shape here.

**Fix.** Non-blocking. Add one sentence to each doc comment ("destructive and irreversible; CLI and MCP surfaces must require explicit confirmation and must not expose this as an unattended tool"), and fix finding 1 so the id cannot be malformed. Consider a `docs/` note listing the destructive method set for the Phase 3/4 work orders.

---

## Checklist coverage

| Template item | Result |
|---|---|
| 1. Secrets never leak (logs, errors, fixtures, doc examples) | Pass in-diff; latent hazard -> finding 3. No `Authorization`/token/Bearer handling anywhere in the nine new service files; transport redaction untouched. |
| 2. Credential storage (0600, atomic, XDG) | No surface in this batch (no credential-storage code touched). |
| 3. OAuth flow (PKCE, state, loopback, rotation, single-flight) | No surface. `freshbooks/auth/` untouched. The Developer *application-management* endpoints are API CRUD, not the OAuth flow itself. |
| 4. Transport (TLS, timeouts, redirects, bounded bodies, Retry-After) | Unchanged; `transport.go` is not modified by this batch. Existing `maxResponseBytes` bound and `redactPath` remain in force for all new calls. |
| 5. Trust boundaries (validation, path traversal, no shell-out, no `unsafe`) | **FAIL -> finding 1.** No shell-outs, no `unsafe`, all responses decode into typed structs (the three undecoded `map[string]any` discussion-thread returns are evidence-driven and safe -- `encoding/json` into `map[string]any` has no injection surface). |
| 6. Stateless MCP | No surface. `mcp/` untouched. |
| 7. Supply chain (`go.sum`, new deps, `govulncheck`, workflow pinning) | No dependency or workflow delta; `govulncheck` unavailable -> finding 4. |
| 8. Public-repo hygiene | **FAIL -> finding 2.** `redaction-check.sh` clean; one fixture carries vendor-sourced real identifiers + a personal name. Remaining ~30 new fixtures are correctly synthetic. |

## Required to clear the gate

1. Apply `pathSegment` at the 15 sites in finding 1.
2. Synthesize `freshbooks/testdata/settings/business_added.json` and its `settings_test.go` assertions.

Findings 3, 4 and 5 are advisory and do not gate the merge.
