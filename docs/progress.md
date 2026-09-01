# Progress

Living status doc. Read first, update at every phase boundary. Last updated: 2026-09-01 (Phase 2 shipped).

## Current state

- **Phase 2 (lib resources) shipped 2026-09-01**, four sequential `--no-ff` merges on `main` (a: Invoices; b: Clients+Estimates+Expenses+Taxes; c: Projects+TimeTracking+MyTeam+Settings; d: Accounting+Reports+Webhooks+Uploader+Tokenization). **Inventory: implemented 213, todo 0, ignored 0** -- every Postman request is claimed by exactly one lib method, duplicates handled via stacked `// inventory:` comments. Coverage at ship: freshbooks module ~91.8% (gate floor 90%).
- The `freshbooks` module now has all 36 services implemented across ~30 resource files, plus transport additions from Phase 2: multipart upload (`doMultipart`, bounded at 10MB, replayable on retry, base-filename only), host override for card tokenization (`doOnHost`, https forced regardless of base URL), raw fetch with Accept override (`fetchRaw`, used by invoice PDF and report CSV), one shared retry core (`attemptLoop`), `pathSegment`/`noTraversal` input validation on every caller-supplied path segment, and shared seams `listOpts`/`newPage`/`pageSize`/`softDelete`. List convention package-wide: `List(ctx, id, opts *XListOptions, extra ...RequestOption)` + `All` iterators with a 100/page default. PCI-sensitive tokenization structs carry redacting `String()` (as do `Application`/`ApplicationUpdateRequest`).
- **Phase 1 (lib core) shipped 2026-08-23** (`98ea08c`): client, three-family transport, errors, `Page[T]`/`All`, types, `auth/` (PKCE, rotation write-back, stores), Identity. **Phase 0 (scaffold) shipped 2026-08-22** (`b3063ba`): go.work, inventory tool + parity check, mise gate, CI/release, public repo.
- Spec section 3/5.1 now carry `STATE AS OF 2026-09-01` callouts from all four batches: the stage-1 duplicate-key re-cut; payment-options body (docs win: entity_type/entity_id sent); invoice delete verb (PUT kept over docs' DELETE); business-family bare `field=value` filters (docs-confirmed); business-family sort direction discrepancy (`-field` vs our `_desc` -- unresolved, needs an owner); envelope families events=accounting, uploads/payments=business (docs-confirmed); the `ledger` family misclassification found and fixed; bill quantity/vendor-balance/attachment-id wire shapes; webhook `callback_id` body conflict; and the docs' successor endpoints (journal entries, chart-of-accounts list, business_uuid-scoped reports) deliberately not implemented because parity is against the collection.
- **Everything confirmed this phase is docs-confirmed, not live-confirmed** (unattended run, no sandbox). The live-conformance backlog below is the authoritative list.

## Phase ledger

| Phase | Status | Branch / merge | Notes |
|---|---|---|---|
| 0 Scaffold | **SHIPPED 2026-08-22** | `phase-0/scaffold` -> `main` @ `b3063ba` | reports in `docs/phases/0/` |
| 1 Lib core | **SHIPPED 2026-08-23** | `phase-1/lib-core` -> `main` @ `98ea08c` | one converged blocker (`Token.String`); reports in `docs/phases/1/` |
| 2a Invoices (51 keys) | **SHIPPED 2026-09-01** | `phase-2/a` merged | review 6 blocking + QA 2 blocking (transactionid decode, payment-options body) -> 2 fix commits -> PASS |
| 2b Clients+Estimates+Expenses+Taxes (59) | **SHIPPED 2026-09-01** | `phase-2/b` merged | security BLOCK (pathSegment) + QA 3 decode blockers (quantity, vendor balances, attachment ids) -> 2 fix commits -> PASS |
| 2c Projects+TT+MyTeam+Settings (47) | **SHIPPED 2026-09-01** | `phase-2/c` merged | security BLOCK (pathSegment + a vendor-sourced fixture) + QA 2 blockers (Delete Project path, All+extra pin) -> 2 fix commits -> PASS; list-signature convergence approved |
| 2d Accounting+Reports+Webhooks+Uploader+Tokenization (52) | **SHIPPED 2026-09-01** | `phase-2/d` merged | security BLOCK (plaintext card path, PAN redaction, pathSegment) + QA 5 additive blockers + 1 QA-self-error regression -> 3 fix commits -> PASS. Closes 213/213 |
| 3 MCP | not started | `phase-3/mcp` | next target |
| 4 CLI | not started | | |
| 5 Release | not started | | carries the CI/goreleaser backlog + Node 20 deprecation warning on actions/checkout@v4 |

## Discoveries (Phase 2)

- 2026-09-01: the Postman collection duplicates 25 requests across folders (same method+URL); resolved by whole-group batch ownership + stacked inventory comments. Also: `Invoices/Single Invoice w/ Payment Gateway` POSTs to a `{invoiceId}` URL with a create body (copy-paste artifact, folded into Create); `Expenses/Delete Expense`'s example sends `vis_state: 0` (authoring mistake, we send 1); the `Settings/Developer/Get all applications` example is a mislabeled Identity Info body.
- 2026-09-01: `familyForPath` had a latent bug -- ledger paths (`/accounting/businesses/.../ledger_accounts`, `/accounting/ledger_accounts/`) fell into the general accounting case and would have double-unwrapped their flat `{"data": ...}` bodies. Fixed in batch d before any caller existed.
- 2026-09-01: three docs pages have moved past the collection (journal entries `manualJournalEntry` API, chart-of-accounts list under `/reports/chart_of_accounts`, business_uuid-scoped profit-loss/trial-balance). The lib implements the collection's endpoints per the parity contract; spec 5.1 records the successors.
- 2026-09-01: `paid.freshbooks.com` tokenization endpoints carry the account bearer by design (vendor collection declares oauth2 on them); the lib forces https on that path regardless of `WithBaseURL`.

## Phase-close backlog (convergence + live conformance)

Cross-batch items deferred by triage, in rough priority:

1. **Business-family sort direction**: `Sort()` emits `field_desc`; docs + one Postman capture say `-field` for business endpoints. Family-switch `Sort()` (or document the workaround permanently). Owner: whoever next touches `types.go`/`options.go` (Phase 3 pre-work or Phase 5).
2. **`scripts/check.sh` dirty-tree guard vs the QA lane's uncommitted report**: teach the guard to ignore `docs/phases/*/reports/` or the QA lane keeps stashing its report to /tmp for the gate run.
3. **`PageMeta` drops `meta.sort`** (captured on Projects list). Model or document.
4. **`StaffService.List` discards the sibling business fields**; no accounting list-staff endpoint exists in the collection (docs document one). Live-check then decide.
5. **Full-fixture sweep**: one verbatim captured-response fixture per resource (batch b QA's ADV-7); most fixtures are still trimmed.
6. **Live-conformance pass** (attended, needs a sandbox): every `STATE AS OF 2026-09-01` docs-only fact; plus checkout-link response shape, `EnablePaymentOptions` response (currently discarded), tokenization shapes (no docs at all), ledger taxonomy endpoints (`json.RawMessage`, zero evidence), webhook `callback_id` body, invoice delete verb, quoted-ID writes (bill `vendorid`/`categoryid`), `Expenses/Create Custom Expense Category` (docs say unsupported).
7. **govulncheck locally**: not installed, so read-only security lanes reason around it (`mise run vuln` covers the gate). Consider adding to mise tools.

## Next action

Run `/goal complete everything in @GOAL.md` in a fresh session. It targets Phase 3 (MCP server): single branch `phase-3/mcp`, sonnet implementer, opus reviewers, go-sdk v1.7.0+ stateless streamable HTTP + stdio, one tool per lib method, parity against the same inventory, `docs/mcp.md`. Unattended. Note for the work order: MCP tool output must strip `client_secret` (`Application`) and never render tokenization card fields -- both constraints are written on the types.

## How to resume in a fresh session

1. Read this file, then `GOAL.md`, then `CLAUDE.md`.
2. `git status --porcelain` must be empty and `git log --oneline -5` should match the ledger above. If not, reconcile before starting.
3. Read only the spec sections the current phase names.
4. Start the goal.
