# Progress

Living status doc. Read first, update at every phase boundary. Last updated: 2026-09-01 (Phase 3 shipped).

## Current state

- **Phase 3 (MCP server) shipped 2026-09-01**, `phase-3/mcp` -> `main` @ `7ed5891` (`--no-ff`). `freshbooks-mcp`: 168 tools, one per exported lib service method minus the 17 `All` iterators, carrying 212 of the 213 inventory keys (`Authorization/Revoke Refresh Token` lives on `auth.Config.Revoke`; the MCP is a token consumer). Data-driven registry (`mcp/internal/tools`, one `Spec` per tool, schemas computed once at init with three `jsonschema` type overrides and shared via `mcp.SchemaCache`), config with `FRESHBOOKS_MCP_*` flag/env twins plus the lib-wide token/scope env (stdio only), stdio and stateless streamable HTTP transports (per-request client from the request bearer, 401 before JSON-RPC parsing, `/healthz`), `serve`/`version`/`tools` commands, `docs/mcp.md` with Claude Desktop / Claude Code / curl setup. Coverage at ship: mcp 91.9%, freshbooks 91.8%, cli 100% (scaffold).
- **Gate outcome:** code review REQUEST CHANGES (the 168-row round-trip asserted nothing about the request sent), security BLOCK (go-sdk's pre-handler schema validation echoed raw card input into `IsError` content, bypassing the module's error mapping), simplify 6 APPLY-RECOMMENDED; one fix commit `0280faa` (F1-F15 in `docs/phases/3/triage.md`); QA PASS with 6 advisories (below). Tool manifest byte-identical across the fix commit.
- **Phase 2 (lib resources) shipped 2026-09-01**, four sequential merges; inventory `implemented 213, todo 0`. **Phase 1 (lib core) shipped 2026-08-23** (`98ea08c`). **Phase 0 (scaffold) shipped 2026-08-22** (`b3063ba`).
- Spec callouts: section 6 now carries a `STATE AS OF 2026-09-01` (go-sdk v1.7.0 current; no `_all` tools; `Date`/`DateTime`/`ProfitLossLine` schema overrides with `Out = any`; 168 tools / 212 keys). Sections 3/5.1 carry the Phase 2 docs-confirmed callouts. **Everything API-facing is still docs-confirmed, not live-confirmed** (no sandbox in any unattended run).

## Phase ledger

| Phase | Status | Branch / merge | Notes |
|---|---|---|---|
| 0 Scaffold | **SHIPPED 2026-08-22** | `phase-0/scaffold` -> `main` @ `b3063ba` | reports in `docs/phases/0/` |
| 1 Lib core | **SHIPPED 2026-08-23** | `phase-1/lib-core` -> `main` @ `98ea08c` | one converged blocker (`Token.String`); reports in `docs/phases/1/` |
| 2a-2d Lib resources | **SHIPPED 2026-09-01** | `phase-2/{a,b,c,d}` merged sequentially | 213/213 keys; reports in `docs/phases/2/` |
| 3 MCP | **SHIPPED 2026-09-01** | `phase-3/mcp` -> `main` @ `7ed5891` | review 1 blocking + security 1 blocking (independent) -> 1 fix commit -> QA PASS (6 advisory); reports in `docs/phases/3/` |
| 4 CLI | not started | `phase-4/cli` | next target |
| 5 Release | not started | | carries the CI/goreleaser backlog + Node 20 deprecation warning on actions/checkout@v4 |

## Discoveries (Phase 3)

- 2026-09-01: go-sdk v1.7.0's `AddTool` schema reflection (`jsonschema-go` v0.4.3) fails on 54 of the 169 lib request/option/result types: `Date`/`DateTime` embed `time.Time` ("custom schema for embedded struct must have type object") and `ProfitLossLine` is recursive. Fix in the MCP: explicit `Tool.InputSchema` from `jsonschema.ForType` with three `TypeSchemas` overrides, `Out = any`. A lib-side alternative (stop embedding `time.Time`) would be an API change; not taken.
- 2026-09-01: go-sdk validates `arguments` against the input schema BEFORE the typed handler runs and quotes the offending value in the `IsError` text (`jsonschema-go validate.go` `type: %v has type %q`). The module's own error mapping never sees that path. Sensitive tools (four `payment_options_*`, `identity_update_application`) now use an untyped handler that validates itself and returns a name-only error.
- 2026-09-01: go-sdk decodes tool input with `segmentio/encoding/json`, which rejects a quoted string for a `json.Number` field, while the inferred schema types `json.Number` as `"string"`. `RetainerCreateRequest`/`UpdateRequest` `Fee`/`ExcessRate` are exposed as plain strings on the tool input and converted in the closure.
- 2026-09-01: FreshBooks' projects API is itself inconsistent -- `GET .../projects/{id}` (plural) vs `POST/PUT .../project` (singular); the lib mirrors the Postman capture; QA's own expectation was the wrong one.
- 2026-09-01: bare `go` on the dev machine was version-skewed against mise's toolchain (`compile: version "go1.26.6" does not match go tool version "go1.26.7"`); every probe must run through `mise exec -- go`.

## Phase-close backlog (convergence + live conformance)

Cross-phase items deferred by triage, in rough priority. Phase 4 may fold in items 1-4 if its gate agrees; the rest wait for Phase 5 or an attended run.

1. **`Page[T]` and `User`/`Membership` carry no `json` tags** (QA Q1): 23 MCP tools (every `*_list`, `identity_whoami`, `identity_register`, `time_entries_search`) emit Go-cased keys (`Items`, `PerPage`, `Memberships`) in `structuredContent` while their inputs are snake_case, and `docs/mcp.md` documents no output envelope. Lib change (API-visible, pre-v0.1.0 so cheap now): add tags to `freshbooks/page.go` and `freshbooks/identity.go`, document the list envelope in `docs/mcp.md`. Owner: Phase 4 (the CLI's table output wants the same tags) or Phase 5.
2. **Business-family sort direction**: `Sort()` emits `field_desc`; docs + one Postman capture say `-field` for business endpoints. Family-switch `Sort()` (or document the workaround permanently). Owner: whoever next touches `types.go`/`options.go`.
3. **MCP test hardening** (QA Q2, Q3, Q5, Q6): the schema-invalid redaction rows assert against an empty log; `payment_options_save_credit_card`'s invalid row carries no sensitive value; `assertProbesInQuery` matches bare digits not `key=value`; the well-typed sensitive rows never assert success. All cheap; none a coverage hole.
4. **MCP `Bearer` scheme match is case-sensitive** (QA Q4): RFC 7235 says the scheme token is case-insensitive; one-line `strings.EqualFold`. Fail-closed today.
5. **`scripts/check.sh` dirty-tree guard vs the QA lane's uncommitted report**: teach the guard to ignore `docs/phases/*/reports/` or QA keeps running the gate before writing its report.
6. **`PageMeta` drops `meta.sort`** (captured on Projects list). Model or document.
7. **`StaffService.List` discards the sibling business fields**; no accounting list-staff endpoint exists in the collection (docs document one). Live-check then decide.
8. **Full-fixture sweep**: one verbatim captured-response fixture per resource; most fixtures are still trimmed.
9. **`golang.org/x/sys` GO-2026-5024** (Windows-only, reachability verified negative on every GOOS): pick up in the Phase 5 dependency refresh.
10. **`mcp/CHANGELOG.md` is hard-wrapped** (Phase 0 scaffold style) while the docs rule says never hard-wrap; reflow when the file is next rewritten for the v0.1.0 section.
11. **Live-conformance pass** (attended, needs a sandbox): every `STATE AS OF 2026-09-01` docs-only fact; plus checkout-link response shape, `EnablePaymentOptions` response, tokenization shapes (no docs at all), ledger taxonomy endpoints, webhook `callback_id` body, invoice delete verb, quoted-ID writes, `Expenses/Create Custom Expense Category`; and the MCP against a real token end to end.
12. **govulncheck locally**: not installed, so read-only security lanes reason around it (`mise run vuln` covers the gate). Consider adding to mise tools.

## Next action

Run `/goal complete everything in @GOAL.md` in a fresh session. It targets Phase 4 (CLI): single branch `phase-4/cli`, sonnet implementer, opus reviewers, cobra tree generated from a registry table mirroring `docs/phases/3/tools.md` (plus `--all` on list commands to cover the 17 `All` iterators), loopback PKCE login with the self-signed-TLS gotcha, contexts, `json|yaml|table|name` outputs, parity test, `docs/cli.md` via `cobra/doc`. Unattended. Backlog items 1-4 above are candidates to fold in.

## How to resume in a fresh session

1. Read this file, then `GOAL.md`, then `CLAUDE.md`.
2. `git status --porcelain` must be empty and `git log --oneline -5` should match the ledger above. If not, reconcile before starting.
3. Read only the spec sections the current phase names.
4. Start the goal.
