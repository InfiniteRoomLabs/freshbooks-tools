# Phase 3 implementer report (MCP server)

Branch `phase-3/mcp`, commits `70425ea`..`db0dced` on top of `77a86be`. `mise run check` (all three modules) is green on a clean tree; `git status --porcelain` is empty.

## What shipped

- `mcp/internal/tools`: the 168-tool registry. One `Spec` per `freshbooks` client-library method (minus the 17 `All` iterators and the auth-owned `Authorization/Revoke Refresh Token`), built by a generic `newSpec[In]` constructor and collected into `All` by `registry.go`'s `buildRegistry`. 33 per-resource files (`tools_*.go`, mirroring the lib's file layout) plus shared infrastructure: `registry.go` (`Spec`, `newSpec`, `Register`, `Manifest`), `scope.go` (`Scope`, `AcctScope`/`BizScope`/`UUIDScope`, `resolveScope`), `shapes.go` (`idIn`, `listIn`, `includeIn`, `emptyIn`, `ok`/`void`), `upload.go` (base64 upload/binary-result helpers), `schema.go` (the D3 type overrides and `schemaFor`), `errors.go` (`errResult`, `errorText`).
- `mcp/internal/config`: cobra flags with `FRESHBOOKS_MCP_*` env twins, the lib-wide token/scope environment, `Validate`, `TokenSource` (static + rotating-with-seed), a redacting `String()`/`LogValue()`.
- `mcp/internal/server`: `RunStdio` (one process-lived client) and `RunHTTP` (stateless streamable HTTP, per-request client, `401`+`WWW-Authenticate` middleware, `/healthz`), sharing one `mcp.SchemaCache`.
- `mcp/cmd/freshbooks-mcp`: cobra root with `serve`/`version`/`tools`; `main.go` stays one statement.
- `docs/mcp.md` rewritten in full; `mcp/CHANGELOG.md`'s `[Unreleased]` entry; all three `internal/*/doc.go` files rewritten.

## Test counts (per package, `go test -v`, count of `--- PASS`)

| Package | Tests passing |
|---|---|
| `cmd/freshbooks-mcp` | 6 |
| `internal/config` | 26 |
| `internal/server` | 11 |
| `internal/tools` | 203 (168 of those are `TestRoundTrip`'s per-tool subtests) |

## Coverage

`mcp` module aggregate (via `mise run cover -- mcp`, which excludes `cmd/*/main.go` per `scripts/coverage-gate.sh`): **94.6%** (floor 90%). Per package: `tools` 96.2%, `config` 96.3%, `server` 94.4%, `cmd` 78.1% (dragged down only by `newServeCmd`'s success-path `RunE`, which blocks forever on a real listener/stdin loop and is not safely unit-testable without either a signal-based external cancel or breaking `run`'s signature further; the failure paths -- bogus transport, no token -- are covered).

## `mise run check` tail (all three modules, from repo root)

```
== cover: cli ==
coverage-gate: /home/.../freshbooks-tools/cli/coverage.out total = 100.0% (floor 90%)
coverage-gate: PASS
== vuln: cli ==
No vulnerabilities found.
== inventory-check: cli (skipped -- only freshbooks has an inventory) ==
== actionlint ==
== build ==
build: mcp ./cmd/freshbooks-mcp -> dist/freshbooks-mcp_{linux,darwin}_{amd64,arm64}, windows_{amd64,arm64}.exe
build: cli ./cmd/freshbooks -> dist/freshbooks_{linux,darwin}_{amd64,arm64}, windows_{amd64,arm64}.exe
build: done, artifacts in dist
check.sh: all OK
```

`freshbooks`'s own gate is unchanged and still green: `inventory-check: implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0`. Neither `freshbooks/` nor `cli/` was touched (verified via `git status --porcelain` and the diff in each commit).

## `git log --oneline main..phase-3/mcp`

```
db0dced fix(mcp): declare cobra as a direct go.mod dependency
b106db0 docs(mcp): rewrite docs/mcp.md and the mcp CHANGELOG for Phase 3
9d97451 test(mcp): add config and server tests; fix an empty-env-var precedence bug
573abbd test(mcp): add parity, round-trip, and redaction tests for the tool registry
70425ea feat(mcp): add the 168-tool data-driven registry, config, server, and cmd
77a86be docs(phase-3): add the MCP work order, definitive tool surface, and spec 6 callout
```

(`77a86be` is the pre-existing work-order commit this session started from, included for context; the session's own commits are the first five.)

## `git status --porcelain`

Empty.

## Parity: tool count and inventory-key count

`TestParityAgainstToolsMD`, `TestParityAgainstClient`, and `TestParityKeyCoverage` (`mcp/internal/tools/parity_test.go`) all pass:

- **168 tools** in the registry, matching `docs/phases/3/tools.md` row-for-row (name, wrapped `Service.Method`, inventory keys) and matching, bidirectionally, every exported non-`All` method reflected off `*freshbooks.Client`'s exported service fields.
- **212 inventory keys** carried across those 168 tools, each on exactly one tool; `identity_whoami` is the sole keyless tool; the 213th key (`Authorization/Revoke Refresh Token`) never appears on any tool.

## Dependencies added (all in `mcp/go.mod`)

| Module | Version | Direct/indirect |
|---|---|---|
| `github.com/modelcontextprotocol/go-sdk` | v1.7.0 | direct |
| `github.com/google/jsonschema-go` | v0.4.3 | direct (go-sdk's own dependency; imported directly per plan decision D3 for the `jsonschema.ForType`/`ForOptions`/`TypeSchemas` override machinery) |
| `github.com/spf13/cobra` | v1.10.2 | direct |
| `github.com/spf13/pflag` | v1.0.9 | indirect (cobra) |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | indirect (cobra, Windows double-click guard) |
| `github.com/segmentio/asm`, `github.com/segmentio/encoding` | v1.1.3, v0.5.4 | indirect (go-sdk's internal JSON layer) |
| `github.com/yosida95/uritemplate/v3` | v3.0.2 | indirect (go-sdk) |
| `golang.org/x/oauth2`, `golang.org/x/sync`, `golang.org/x/sys`, `golang.org/x/time` | v0.35.0, v0.20.0, v0.41.0, v0.15.0 | indirect (go-sdk) |

Nothing outside this set. `govulncheck` reports 0 reachable vulnerabilities; it flags one Windows-only, symbol-unreachable issue in the transitive `golang.org/x/sys@v0.41.0` (GO-2026-5024, fixed in v0.44.0) that the gate's own verdict already treats as non-blocking ("Your code is affected by 0 vulnerabilities... 1 vulnerability in modules you require, but your code doesn't appear to call" it) -- flagging it here for visibility, not as an open item.

## Spec discrepancies and ambiguities, and how each was resolved

1. **A real conflict between the inferred schema and go-sdk's decoder for `json.Number` fields (not one of D3's three overrides, not one of the plan's pre-declared "odd method" categories).** `freshbooks.RetainerCreateRequest`/`UpdateRequest`'s `Fee`/`ExcessRate` are `json.Number`, which reflects as JSON Schema type `"string"` (its Go `Kind()`) -- but go-sdk decodes tool input via `github.com/segmentio/encoding/json` (`internal/json/json.go`), which rejects a quoted string for a `json.Number` field outright and accepts only a bare numeric literal. No JSON value satisfies both the inferred schema and the decoder at once; a model dutifully sending `"fee": "100.00"` per the schema's own stated type would have hit a hard decode error on every call. Resolved by not embedding `RetainerCreateRequest`/`UpdateRequest` whole for these two tools: `retainers_create`/`retainers_update` expose `fee`/`excess_rate` as plain strings and convert with `json.Number(...)` in the Call closure (see `tools_retainers.go`'s doc comment). Found and fixed via the round-trip test, which is exactly what it is for.
2. **`Get` methods with a variadic `...RequestOption` beyond what their sibling `*ListOptions.Include` field would suggest.** Several resources' `Get` signatures (`Clients`, `Estimates`, `ExpenseCategories`, `Expenses`, `InvoiceProfiles`, `Invoices`, `Taxes`) accept `opts ...RequestOption` independent of whether their `List` counterpart's options struct has an `Include` field. Since the lib method genuinely accepts it, every such `*_get` tool exposes `include []string` -- exposing exactly what the lib allows, not inventing anything beyond it.
3. **Two Postman-derived inventory keys carry doubled internal whitespace** (`Clients/Delete Secondary  Contact ID`, `Invoices/Invoice Recurring Template/Delete  Invoice Profile`) and one carries a doubled dash-adjacent space (`Tokenization/1a. [STRIPE] -  Get Publishable Key`). Copied verbatim into the registry's `Keys` fields, matching `tools.md`/the Postman collection exactly rather than "fixing" whitespace the inventory tool's own normalization did not trim.
4. **Parity verification method.** The plan suggested parsing `mcp/internal/tools/*.go` with `go/parser`, mirroring `freshbooks/internal/inventory/check.go`, "or reuse that package's exported `Check`." Neither is directly available from the `mcp` module: `freshbooks/internal/inventory` is a Go-enforced `internal` package whose import path prefix (`.../freshbooks/internal/...`) makes it inaccessible outside the `freshbooks` module tree, and it operates on `// inventory:` doc comments on `func` declarations, a shape the data-driven registry (slice-literal `Spec` values, not one function per tool) does not produce. `TestParityKeyCoverage`/`TestParityAgainstToolsMD` instead read each `Spec.Keys` field directly (single source of truth backing both the schema and the tool's behavior, so there is no comment/data drift to catch) and cross-check it against a parsed `docs/phases/3/tools.md` -- the frozen, definitive spec the work order names as authoritative -- rather than re-deriving the same information from source comments. `// inventory:` comments were dropped in favour of `Spec.Keys` (correction, applied in the review-gate fix commit: an earlier draft of this item claimed a comment survived above every resource file's `Spec` slice for human grep-ability; in fact only one of 33 files (`tools_attachments.go`) carried one, and it has since been deleted -- `Spec.Keys` is the sole source of the inventory keys, with no comment anywhere duplicating it).
5. **The plan's "round trip... `//go:build integration` AND a plain build-tag-free variant" instruction.** `TestRoundTrip` has no build tag. It needs no network, no credentials, and no external service (a local `httptest.Server` fixture plus an in-memory MCP transport), so gating a duplicate copy behind `integration` would only run the identical assertions twice for zero additional signal; `scripts/check.sh`'s two test invocations (plain and `-tags integration`) both compile and run it as-is, since an untagged file is included regardless of which tags are passed. No separate integration-tagged file was added.

## Nothing to report as blocked

Everything in the work order was implemented and gates green. The one genuine surprise (item 1 above) was caught and fixed by the testing the plan itself asked for.
