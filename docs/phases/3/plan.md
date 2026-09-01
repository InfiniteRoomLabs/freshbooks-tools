# Work order: Phase 3 implementer (MCP server)

Dispatch: `Agent(subagent_type: "general-purpose", model: "sonnet", name: "phase-3-impl")`.

---

You are implementing **Phase 3 (MCP server, module `mcp/`)** of `freshbooks-tools`, a public MIT Go monorepo. Work ONLY inside `<repo root>` on branch `phase-3/mcp` (already checked out, clean; this work order and `docs/phases/3/tools.md` are its first commit). Do not touch other branches. All `git`, `go` and `mise` commands run from `<repo root>` or with `-C mcp`.

## Read first (pointers, not pasted)

1. The oracle: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections 6 (MCP design, including the `STATE AS OF 2026-09-01` callout added with this work order), 8.1 (testing), 2 (locked, do not redesign). Skim 5.1 for the lib's method vocabulary.
2. Conventions: `CLAUDE.md` (toolchain, commits, green rule, parity contract, public-repo hygiene, model rules), `GOAL.md` stage 2 for this phase.
3. **The definitive tool surface:** `docs/phases/3/tools.md` -- 168 tools, one per exported lib service method minus the 17 `All` iterators, with the inventory keys each tool carries and its annotation class. Implement exactly this list; if you believe it is wrong, report it, do not silently deviate.
4. go-sdk v1.7.0 (confirmed the current release on 2026-09-01; spec pin holds). Read in the module cache after `go get`: `examples/server/hello/main.go` (stdio + generic `AddTool`), `examples/http/main.go` (streamable HTTP handler shape), `examples/server/toolschemas/main.go` (explicit schemas), and the doc comments on `mcp.StreamableHTTPOptions` (`Stateless`, `JSONResponse`), `mcp.ServerOptions.SchemaCache`, `mcp.AddTool`, `mcp.ToolAnnotations`, `mcp.CallToolResult`. Also https://modelcontextprotocol.io/specification/2025-06-18/basic/transports (stateless streamable HTTP, `Mcp-Session-Id` absence, 401 handling).
5. Lib surfaces you consume: `freshbooks/client.go` (`NewClient`, service fields), `freshbooks/options.go` (`WithTokenSource`, `WithBaseURL`, `WithLogger`, `WithUserAgent`), `freshbooks/auth/token.go` (`StaticTokenSource`, `NewTokenSource`, `TokenStore`), `freshbooks/auth/store.go` (`FileStore`), `freshbooks/errors.go` (`*Error` fields), `freshbooks/page.go` (`Page[T]`, `PageMeta`), `freshbooks/types.go` (`AccountID`, `BusinessID`, `BusinessUUID`, `Search`, `Date`, `DateTime`, `Money`, `Include`, `Sort`), `freshbooks/settings.go:140-230` (`Application` and the written `client_secret` constraint), `freshbooks/payment_options.go` (the tokenization card structs and their redacting `String()`).
6. Exemplar for shape, tests, and doc style: `freshbooks/identity.go` + `identity_test.go` (stacked `// inventory:` comments, fixture usage), `freshbooks/testdata/<resource>/*.json` (fixtures you may REUSE from the mcp tests via a relative path; do not copy them).
7. Existing scaffold in `mcp/`: `cmd/freshbooks-mcp/{main.go,run.go,run_test.go}` (keep `main.go` at exactly one statement; replace `run.go`'s body with the cobra root), `internal/{config,server,tools}/doc.go` (replace the placeholder package docs), `CHANGELOG.md`, `.goreleaser.yaml` (leave alone).

## Stage-1 decisions you inherit (do not re-open; report if reality disagrees)

- **D1 -- no `_all` tools.** The 17 `All` iterators are lib conveniences over `List`; an unbounded page walk is the wrong shape for a model context. `List` tools take `page`/`per_page`. The parity test excludes methods named `All` explicitly and nothing else.
- **D2 -- `Authorization/Revoke Refresh Token` is not a tool.** It lives on `auth.Config.Revoke`; the MCP is a token consumer. So tools carry 212 of the 213 inventory keys; the parity test asserts exactly that (the union of keys on tool registrations == inventory keys minus that one).
- **D3 -- schema inference needs three overrides.** `mcp.AddTool`'s reflection (`github.com/google/jsonschema-go` v0.4.3, go-sdk's own dependency) FAILS for 54 of the 169 lib request/option/result types: `freshbooks.Date` and `freshbooks.DateTime` embed `time.Time` ("custom schema for embedded struct must have type object, got string"), and `freshbooks.ProfitLossLine` is recursive ("cycle detected"). Verified fix (169/169 pass): build every input schema yourself with `jsonschema.ForType(reflect.TypeFor[In](), &jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{reflect.TypeFor[freshbooks.Date](): {Type: "string", Format: "date"}, reflect.TypeFor[freshbooks.DateTime](): {Type: "string"}, reflect.TypeFor[freshbooks.ProfitLossLine](): {Type: "object", AdditionalProperties: &jsonschema.Schema{}}}})` and set `Tool.InputSchema` before calling `AddTool`; use `Out = any` (go-sdk then omits the output schema) and return the lib value as `StructuredContent` plus its JSON as one `TextContent`. Compute schemas ONCE at package init (a registry), never per request; also pass a shared `mcp.SchemaCache` in `ServerOptions`. Importing `jsonschema-go` directly is acceptable because it is already in the dependency graph via go-sdk; say so in `go.mod` comments and your report.
- **D4 -- tool naming** is `{service_field_snake}_{method_snake}` exactly as in `tools.md` (`invoices_pdf`, `reports_download_invoice_details_csv`, `payment_options_fb_pay_tokenize`).

## Deliverables

### `mcp/internal/tools` -- the registry (the bulk of the work; data-driven, not 168 hand-rolled handlers)

- A registry type `[]Spec` (or equivalent) where each entry carries: `Name`, `Description` (one sentence + the FreshBooks docs URL for the resource, e.g. `https://www.freshbooks.com/api/invoices`), `Annotations` per the `annot` column in `tools.md` (RO -> `ReadOnlyHint: true`; D -> `DestructiveHint: true`; I -> `IdempotentHint: true`; W -> nothing set; `OpenWorldHint` true for all), the input Go type, the precomputed `*jsonschema.Schema`, and a `Call func(ctx, *freshbooks.Client, In) (any, error)` closure. Every registration carries the lib method's `// inventory:` key comments, stacked exactly as the lib does (see `identity.go:86-87`), so `grep` and the parity test can read them.
- **Input shapes.** snake_case `json` tags, `jsonschema` tag descriptions on every scope field. Scope fields: `account_id` (string), `business_id` (int64), `business_uuid` (string) -- optional when the server has an env default (see config), and the tool returns an `IsError` result naming the missing field when neither is present. Prefer a handful of generic shapes (`scoped`, `scopedID[T]`, `scopedBody[T]`, `scopedIDBody[T]`, `scopedList` with `search map[string]string`, `page`, `per_page`, plus `include []string` where the lib option has `Include`) and bespoke structs only for the odd methods (reports, uploads, thread comments, rates, verify, checkout links, tokenization). Mirror the lib request structs by EMBEDDING or referencing them as a `body` field -- do not retype their fields.
- **Uploads** (`attachments_upload_expense_receipt`, `images_upload`, `images_upload_without_account`): input `filename` + `content_base64`; decode, wrap in a reader, call the lib. **Binary results** (`invoices_pdf`, `reports_download_invoice_details_csv`): output `{content_type, size, content_base64}`.
- **Written security constraints (non-negotiable):** (a) every tool returning `freshbooks.Application` (`identity_applications`, `identity_create_application`, `identity_update_application`) zeroes `ClientSecret` before the value reaches `StructuredContent`/text -- the field is `omitempty`, so zeroing removes it from the wire; test it. (b) The tokenization tools (`payment_options_fb_pay_tokenize`, `payment_options_stripe_tokenize`, `payment_options_stripe_create_setup_intent`, `payment_options_save_credit_card`) never echo their input into results, error text, or logs -- return only what the lib returns, format errors from the lib error only, and never `%v` an input struct anywhere in this module; test that a card number given as input is absent from the result and from a captured log.
- **Error mapping:** `*freshbooks.Error` -> `CallToolResult{IsError: true}` whose content is the JSON `{"status": <StatusCode>, "code": <Code>, "message": ..., "field": ..., "family": ...}`; any other error -> `IsError: true` with `err.Error()` (the lib already redacts tokens from its errors; never add the bearer or request body). Tool handlers never panic and never return a Go error for API failures (that would become a JSON-RPC error and hide the failure from the model); reserve Go errors for input decoding failures the SDK surfaces itself.
- `Register(server *mcp.Server, client *freshbooks.Client, defaults Scope)` (or equivalent) adds every registry entry to a server, closing over the per-request client. `Manifest() []mcp.Tool` (name, description, annotations, input schema) for the `tools` command.

### `mcp/internal/config`

- Cobra flags with `FRESHBOOKS_MCP_*` env twins, flag > env, no config file: `--transport stdio|http` (default stdio), `--addr` (default `127.0.0.1:8080`), `--path` (default `/mcp`), `--log-level`, `--log-format json|text` (`log/slog`, to stderr -- stdout is the stdio transport).
- Token/scope env (NOT `FRESHBOOKS_MCP_*`; these are the lib-wide names from spec 6): `FRESHBOOKS_ACCESS_TOKEN` (static bearer), or `FRESHBOOKS_CLIENT_ID` + `FRESHBOOKS_CLIENT_SECRET` + `FRESHBOOKS_TOKEN_FILE` (lib `FileStore` + `auth.NewTokenSource` for rotation; `FRESHBOOKS_REFRESH_TOKEN` seeds the store only when the file does not exist yet). `FRESHBOOKS_ACCOUNT_ID`, `FRESHBOOKS_BUSINESS_ID`, `FRESHBOOKS_BUSINESS_UUID` as default scope; `FRESHBOOKS_BASE_URL` override (tests and future sandboxes). Stdio mode requires one of the two token configurations and fails fast with a clear message otherwise; HTTP mode ignores the token env entirely (bearer comes from each request).
- Secrets never appear in logs, errors, `--help`, or `String()` output; a `Config.String()`/`LogValue()` that redacts is the pattern.

### `mcp/internal/server`

- `New(cfg, registry)` wiring for BOTH transports. **Stdio:** one `*mcp.Server`, one lib client built from the env token source, `server.Run(ctx, &mcp.StdioTransport{})`. **HTTP:** `mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})` where `getServer(req)` reads `Authorization: Bearer <token>`, builds a per-request lib client with `auth.StaticTokenSource(token)`, builds a fresh `*mcp.Server` (shared `SchemaCache`, shared precomputed registry) and registers the tools; a middleware in front returns `401` + `WWW-Authenticate: Bearer` when the header is missing or malformed (before any JSON-RPC parsing); `GET /healthz` returns `200`. No `Mcp-Session-Id` is ever emitted, nothing is cached by client or token, the bearer lives only in the request's closure and is never logged (the lib's `WithLogger` gets a logger; confirm the lib never logs the token). `http.Server` with sane timeouts; graceful shutdown on context cancel.
- Implementation `mcp.Implementation{Name: "freshbooks-mcp", Version: <ldflags version>}`.

### `mcp/cmd/freshbooks-mcp`

- Cobra root with `serve`, `version`, `tools` (prints the manifest as JSON: name, description, annotations, inputSchema, sorted by name). `main.go` stays one statement (`os.Exit(run(...))`); `run` builds the root command and executes it; all logic in the internal packages so the coverage gate sees it (`scripts/coverage-gate.sh` fails on an empty filtered profile).

### Tests (module coverage >= 90%, `-race`, no `t.Skip`)

- **Parity test** (`[parity]`): reflect over `freshbooks.Client`'s exported pointer-to-service fields, collect exported methods, drop `All`, and assert the set equals the registry's `(service, method)` pairs both ways; assert registry names match `tools.md`'s naming rule; assert the union of `// inventory:` keys read from the registry source (parse `mcp/internal/tools/*.go` with `go/parser` the way `freshbooks/internal/inventory/check.go` does, or reuse that package's exported `Check`) equals the inventory's keys minus `Authorization/Revoke Refresh Token` (212), with no key on two tools.
- **Round trip per tool** (integration, `//go:build integration` AND a plain build-tag-free variant that covers the same path so coverage counts it): go-sdk client over `mcp.NewInMemoryTransports()` -> server -> tool -> lib -> `httptest.Server` serving a fixture; a table with one row per registry entry (168 rows; rows may share generic fixtures per family; reuse `freshbooks/testdata` where a fixture exists). Assert: the request the lib sent (method, path, scope id, body for writes), no `IsError`, and that `StructuredContent` round-trips through the lib type.
- **Stateless property:** an `httptest.Server` around the HTTP handler; two sequential `tools/call` POSTs with different bearers; assert the upstream fixture server saw each bearer exactly once in order, no `Mcp-Session-Id` response header, `GET /mcp` -> 405, missing header -> 401, `/healthz` -> 200.
- **Redaction:** the `client_secret` and card-field tests from the constraints above; a log-capture test that a full stdio + HTTP request cycle at debug level never logs the bearer.
- **Config precedence:** flag > env > default, the two token modes, the fail-fast paths, `String()` redaction.
- **Error mapping:** 401/404/422-with-field/429 fixtures -> `IsError` JSON shape; malformed input -> the SDK's own error path.
- `cmd` tests for `version`, `tools` (valid JSON, 168 entries), `serve --transport bogus`.

### Docs and changelog

- Rewrite `docs/mcp.md` (ASCII, no hard wraps): install (`go install .../mcp/cmd/freshbooks-mcp@latest`), stdio setup for Claude Desktop (`claude_desktop_config.json` snippet with the env vars) and Claude Code (`claude mcp add freshbooks -- freshbooks-mcp serve`), HTTP setup (run behind TLS, the bearer requirement, the claude.ai custom-connector note that it needs a reachable HTTPS URL and a bearer header, a `curl` `initialize` + `tools/call` example with `Accept: application/json, text/event-stream`), the env/flag table, tool naming and scope defaults, the error shape, the two security constraints and a PCI note on the tokenization tools, the `tools` manifest command.
- `mcp/CHANGELOG.md` `[Unreleased]` entry in the existing style (replace the scaffold line's future tense).
- Package `doc.go` files rewritten to describe what exists.

### Gate

- `go get github.com/modelcontextprotocol/go-sdk@v1.7.0` and `github.com/spf13/cobra@v1.10.2` from inside `mcp/` (workspace mode: no `-mod=mod`; plain `go get` + `go mod tidy` in `mcp/` works). Dependencies allowed in `mcp/`: go-sdk (+ its transitive jsonschema-go), cobra, the lib, testify in tests. Anything else is a design decision: flag it, do not add it.
- `mise run check` green on a clean tree (all three modules; the lib and cli must stay untouched -- if you need a lib change, STOP and report it rather than making it). `mise run inventory-check` still `implemented 213, todo 0` (the lib is the only inventory scan; your parity test is the mcp-side check).
- Conventional commits `feat(mcp): ...` (docs `docs(mcp): ...`), TDD-sized, each green. **Stage and commit in separate Bash calls.** Run `scripts/redaction-check.sh` before each commit. Do NOT push, do NOT merge.

## Gotchas

- `go.work` is in force: `go` commands run inside `mcp/` resolve the lib from the workspace; `GOFLAGS=-mod=mod` is rejected in workspace mode.
- `mcp.AddTool` PANICS on schema failure -- which is exactly why D3 sets `InputSchema` explicitly. A test that instantiates the full registry into a server catches any type you missed.
- Stdout is the stdio transport: every log line goes to stderr; `version`/`tools` write to stdout only when not serving.
- `freshbooks.Search` is `map[string]string`; `Date` marshals as `YYYY-MM-DD`; `Money` is `{amount, code}` strings; ids are `int64` except `AccountID`/`BusinessUUID` (strings) and the ledger/team UUID paths. Never "convert" account and business ids.
- `RetainerListOptions` has only `Search`; `ClientListOptions`/`EstimateListOptions` have `Include`. Do not invent `page`/`per_page` for a method whose lib options lack them.
- `Invoices.Create/Update` and `InvoiceProfiles.Create` take variadic `RequestOption`s (`Include`); expose `include []string` on those inputs.
- `Whoami` has no inventory key (documented in `tools.md`); the parity test must tolerate exactly that one keyless tool.
- Fixture IDs are synthetic. Docs ASCII-only, no hard wraps, `--` and `->` not dashes/arrows.
- If the spec (section 6) is wrong about something the SDK does, implement what the SDK does, add a `> **STATE AS OF 2026-09-01**` callout in section 6 in the same commit, and list it in your report.

## Reporting (both channels)

When done (check green, committed, `git status --porcelain` empty): write the report to `docs/phases/3/reports/implementer.md` (commit it), send the same report with `SendMessage` to `team-lead` (full report in `message`, not `summary`), AND return it as your final text. Report: files created/changed, test counts per package, mcp module coverage, the exact `mise run check` tail, `git log --oneline main..phase-3/mcp`, `git status --porcelain` output, tool count and inventory-key count from the parity test, every dependency added with its version, and every spec discrepancy or ambiguity you hit and how you resolved it. If genuinely blocked, report the blocker the same way instead of guessing. If you receive this prompt twice, treat the second as a no-op.
