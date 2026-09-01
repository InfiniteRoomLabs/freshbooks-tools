# Phase 3 code-review lane report (MCP server)

Branch `phase-3/mcp`, `git diff main...phase-3/mcp` (77a86be..0cc509b). Read-only pass: no files modified, no gate run.

## Verdict

**REQUEST CHANGES** -- one BLOCKING finding, eleven ADVISORY.

The implementation itself is in good shape. I mechanically cross-checked all 168 registrations three ways and found no defects: every `Spec`'s declared `Service.Method` is the method its `Call` closure actually invokes (168/168), every tool's annotation class matches `docs/phases/3/tools.md` (168/168), and every scope argument matches the lib signature's ID family (`AccountID` vs `BusinessID` vs `BusinessUUID`), including the deliberately mixed services (`Staff`, `Services`, `Systems`, `Reports`). `include []string` appears on exactly the methods whose lib signature accepts a variadic `RequestOption` or whose `*ListOptions` has `Include`; `page`/`per_page` appear on exactly the list methods whose options carry them (`retainers_list` correctly omits them). The go-sdk usage checks out against v1.7.0's source: `AddTool` with `Out = any` yields `StructuredContent` plus a JSON `TextContent` fallback (`go-sdk/mcp/server.go:412-441`); `SchemaCache` is documented and implemented as goroutine-safe (`sync.Map`, `schema_cache.go:20,33`), and `jsonschema.Schema.Resolve` does not mutate its receiver, so sharing precomputed schemas across per-request servers is sound.

The blocking issue is not a defect in the code -- it is that the test suite as written would not catch one. See finding 1.

## BLOCKING

### 1. The 168-row round-trip asserts nothing about the request the lib sent, and parity is over hand-written strings -- so no test would fail if a closure called the wrong lib method

`mcp/internal/tools/roundtrip_test.go:232-247`

Each row does exactly three things: call the tool, assert `!result.IsError`, and assert `json.Marshal(result.StructuredContent)` returns no error. `fakeUpstream` (`roundtrip_test.go:121-165`) branches only on path suffix/prefix to pick a decodable envelope and records nothing.

The work order's deliverable was explicit: *"Assert: the request the lib sent (method, path, scope id, body for writes), no `IsError`, and that `StructuredContent` round-trips through the lib type."* Only the middle clause is implemented, and the implementer report does not disclose the omission (its item 5 discusses only the build-tag question).

Why it matters: `TestParityAgainstClient` and `TestParityAgainstToolsMD` reconcile `Spec.Service`/`Spec.Method` -- **hand-written string literals** (`registry.go:28-37`) -- against reflection and against `tools.md`. Nothing connects those strings to the closure body. So for all 168 tools, the following mutations pass the entire suite green:

- swap the bodies of `bills_delete` and `bills_archive` (both hit `/bills/{id}`, both decode the same generic fixture) -- a destructive-vs-archive mixup, silently shipped;
- change `staff_get`'s `scope.AccountID` to `scope.BusinessID` -- ID families crossed, exactly the failure `CLAUDE.md` calls out as the project's headline gotcha;
- drop `&in.Body` from any write tool and send an empty body;
- drop `in.opts()...` / `Search`/`Page`/`PerPage` from any list tool, so every filter and pagination field silently becomes a no-op.

The second assertion is additionally vacuous: `result.StructuredContent` on the client side is a `json.RawMessage` that was just parsed off the wire, so `json.Marshal` on it cannot fail. It proves nothing and reads as coverage padding.

I verified by static analysis (parsing every `newSpec(` block and extracting the single `c.X.Y(ctx` call inside) that all 168 closures are correct **today**, so this is a missing regression net rather than a live defect -- but it is the net the work order specified for the one part of this phase that is 168 near-identical hand-written closures, which is precisely where copy-paste drift lands on the next edit.

Fix (small, mechanical):

1. Give `fakeUpstream` a recorder: append `{method, path, rawQuery, body}` per request (mutex-guarded), and reset it per subtest.
2. Add an expectation column to the table -- for each tool, the HTTP method and a path fragment that pins the resource and the scope id (e.g. `invoices_delete` -> `PUT`, path contains `/accounting/account/ACM000TEST/invoices/invoices/1`). Deriving the path fragment from `testScope` is what proves the right scope field reached the URL.
3. For write tools, assert the recorded body is non-empty and contains a field from the synthesized input.
4. Replace the vacuous marshal with a real round-trip: `json.Unmarshal(result.StructuredContent, &<the lib type>)` for at least one representative tool per family, or drop the assertion and rely on 1-3.

Optionally add `page`/`per_page`/`search`/`include` to `synth` for the handful of tools that expose them and assert they land in `rawQuery` -- that closes the "options silently dropped" hole in one stroke.

## ADVISORY

### 2. `--path` is never validated; a bad value panics `serve --transport http`

`mcp/internal/config/config.go:137-155` (`Validate`), `mcp/internal/server/server.go:151`

`Validate` checks `Transport`, `LogFormat`, `LogLevel`, and the stdio token config, but never `Path` (or `Addr`). `HTTPHandler` then does `mux.Handle(s.cfg.Path, ...)`, and `http.ServeMux.Handle` **panics** on a syntactically invalid pattern (Go 1.26 `net/http/pattern.go:86` "empty pattern", `:113` "host/path missing /"; `server.go:2564` documents the panic). So:

- `--path mcp` or `FRESHBOOKS_MCP_PATH=mcp` (no leading slash) -> `panic: http: invalid pattern: host/path missing /`
- `--path=""` -> `panic: http: invalid pattern: empty pattern`

Both are plausible typos on a documented flag (`docs/mcp.md:82`), and both produce a stack trace instead of the clean, named config error every other invalid flag value gets.

Fix: in `Validate`, `if c.Transport == "http" && !strings.HasPrefix(c.Path, "/") { return fmt.Errorf("invalid --path %q: want a path beginning with /", c.Path) }`. A `net.SplitHostPort` sanity check on `Addr` in the same place is cheap and covers the sibling case.

### 3. A malformed `FRESHBOOKS_BUSINESS_ID` is silently discarded

`mcp/internal/config/config.go:124-128`; pinned as intended by `mcp/internal/config/config_test.go:69-75`

`strconv.ParseInt`'s error is dropped on the floor, so `FRESHBOOKS_BUSINESS_ID=1_234` or `="9000001 "` leaves `BusinessID` at 0. The operator does not find out at startup; they find out later, per call, as `missing required scope: business_id (set it on the request, or configure a server default for it)` -- a message that actively misdirects, since the variable *is* set.

This is the same class of "an empty env var should not silently defeat a default" reasoning the implementer already applied in `stringFlag` (`config.go:94-100`); it just was not applied here.

Fix: record the parse failure on `Config` in `Load` and return it from `Validate` (`invalid FRESHBOOKS_BUSINESS_ID %q: ...`). Flip the test from "[edge] silently ignored" to "[sad] rejected".

### 4. The tokenization "never echo the input" constraint has an uncovered echo path through go-sdk's schema validation

`mcp/internal/tools/tools_payment_options.go:17-19`; `docs/mcp.md:115`; `mcp/CHANGELOG.md`

`paymentOptionsFBPayTokenizeIn` embeds `freshbooks.FBPayTokenizeRequest`, whose `CardNumber` is `json:"card_number"` with no `omitempty` -- so the inferred schema types it `"string"` and marks it required (`jsonschema-go/jsonschema/infer.go:342`). go-sdk validates arguments against that schema **before** the handler runs (`go-sdk/mcp/server.go:359-366`), and jsonschema-go's type failure formats the offending value: `type: %v has type %q, want %q` (`jsonschema/validate.go:126`). A model that sends `"card_number": 4242424242424242` as a bare JSON number therefore gets the PAN echoed back inside the `IsError` content, on a path the `tools` package cannot intercept.

The written constraint and the docs state this absolutely -- *"never echo their input into a result, an error, or a log"* -- and the CHANGELOG repeats it. Real disclosure risk is low (the value goes back only to the caller that sent it, is not logged, and is not persisted), but the claim as written is not true.

Fix, cheapest first: (a) scope the wording in `docs/mcp.md:115` and the CHANGELOG to "this module never formats a tool's input; the SDK's own argument-validation error may quote a malformed value back to the caller that sent it", and add a test row proving that is the only path; or (b) register the four tokenization tools through the low-level `(*mcp.Server).AddTool` with hand-rolled decoding so no SDK validation error ever quotes the payload. (a) is proportionate.

### 5. The tokenization redaction test covers one of the four constrained tools, on a branch-conditional assertion

`mcp/internal/tools/redaction_test.go:124-167`

The plan names four tools; only `payment_options_fb_pay_tokenize` is exercised. `payment_options_stripe_tokenize` is the one that carries both a card number **and** a Stripe API key (`tools_payment_options.go:26`, `freshbooks.StripeTokenizeRequest.APIKey`), and it has bespoke logic -- the `body.APIKey = in.APIKey` copy at `tools_payment_options.go:54-56` -- that no other tool has and that this test does not touch.

Separately, `redaction_test.go:152-162` asserts *either* the success shape *or* the error shape depending on which happened. A regression that flips the call to `IsError` silently swaps to the weaker check.

Fix: table the four tools with per-tool sensitive inputs; assert unconditionally against both `errorContentText(result)` **and** `result.StructuredContent` (whichever is populated) plus the captured log, for every sensitive field the input carried -- card number, CVV, and the Stripe key.

### 6. No end-to-end test of the documented error shape

`mcp/internal/tools/unit_test.go:83-108`; contract at `docs/mcp.md:110`

The plan asked for "401/404/422-with-field/429 fixtures -> `IsError` JSON shape". What exists calls `errorText` directly with a hand-built `&freshbooks.Error{}`. Nothing proves an error produced by a real non-2xx response survives the lib -> `call` -> `newSpec` dispatcher -> `errResult` path as a `*freshbooks.Error` that `errors.As` can match. (It does: `freshbooks/transport.go:226` returns `apiErr` unwrapped -- but that is an untested cross-module coupling, and the lib is free to start wrapping it, at which point every API failure silently degrades to the `err.Error()` fallback and the documented `{"status",...,"field"}` shape disappears without a red test.)

Fix: one fixture server returning 422 with a field error, driven through the MCP session, asserting the content parses as the documented JSON with `status`, `code`, and `field` populated. A 401 row proves the same path for the family with the other error shape.

### 7. The implementer report's `// inventory:` claim is inaccurate

`docs/phases/3/reports/implementer.md`, discrepancy item 4

The report states: *"`// inventory:` comments are still present above each resource file's `Spec` slice for human grep-ability, matching the lib's convention."* They are not. Exactly **1 of 33** `tools_*.go` files carries one -- `tools_attachments.go:16`, a single key -- and the other 32 files have none. All 212 keys live only in `Spec.Keys`.

The `Spec.Keys` decision itself is sound and better argued than the plan's original suggestion (it is the same data the schema and the parity test read, so there is no comment-vs-data drift). But the lead is triaging against a report that describes a convention the tree does not follow, and the single surviving comment is now the one place in the module where a key is duplicated outside `Keys` -- the exact drift the report says the design avoids.

Fix: delete the stray comment in `tools_attachments.go` (or add them to all 33 files) and correct item 4 in the report to say the comments were dropped in favour of `Spec.Keys`.

### 8. The parity test parses `tools.md`'s annotation column and throws it away

`mcp/internal/tools/parity_test.go:29,53`

`tableRowPattern` matches the annot column as a non-capturing `[A-Z]+`, so RO/D/I/W is never reconciled between the registry and `tools.md`. `TestParityAgainstToolsMD` therefore would not notice a `destructiveHint` silently becoming a `readOnlyHint`, which is the annotation a client uses to decide whether to ask the human first.

I checked all 168 by hand and they match today. Making that a test is a two-line change: capture the column, add it to `mdRow`, and compare against a `map[*mcp.ToolAnnotations]string` (or compare the resolved hint fields directly).

### 9. `TestRunHTTPShutsDownGracefully` synchronizes on a sleep

`mcp/internal/server/server_test.go:273-274`

`time.Sleep(50 * time.Millisecond)` "to give `ListenAndServe` a moment to bind". Under `-race` on a loaded runner the cancel can land before the bind, in which case `Shutdown` returns nil on a server that never served -- the test passes without exercising graceful shutdown at all. It can also flake the other way if scheduling is worse than 50ms.

Fix: `net.Listen` in the test and add a `Serve(l)` seam on `Server` (or return the bound address over a channel from `RunHTTP`), then poll `/healthz` until it answers before cancelling. That makes both the bind and the shutdown deterministic.

### 10. `getServer` swallows a client-construction failure into a tool-less server

`mcp/internal/server/server.go:118-133`

When `freshbooks.NewClient` fails, the request gets a server with zero tools. The comment argues it is unreachable, which is fair, but the failure mode it chooses -- "this MCP server has no tools" -- is the most confusing one available to a model, and it is indistinguishable from a real empty registry. `RunStdio` (`server.go:75-78`) correctly returns the error.

Fix: register one always-failing handler that returns the construction error as an `IsError` result, or hoist the construction check to startup (build one throwaway client in `RunHTTP` before listening) and let the per-request path assume success.

### 11. `docs/mcp.md:106` overstates paging

The sentence directs readers to "the paginated `*_list` tool's `page`/`per_page` fields" as the replacement for the dropped `All` iterators. Four list tools have no such fields, because their lib options do not: `retainers_list` (`RetainerListOptions` has only `Search` -- correctly handled in `tools_retainers.go:14-17`), `ledger_accounts_list`, `staff_list`, and `service_rates_list`. Worth a clause noting the exceptions so a reader does not go looking for a field that is not in the schema.

### 12. A new `slog.Handler` is constructed on every HTTP request

`mcp/internal/server/server.go:58` (`clientOptions` calls `s.cfg.Logger()`), reached from `getServer` (`:120`) per request

`Config.Logger()` builds a fresh `slog.New(slog.NewTextHandler(...))` each call, and `HTTPHandler` also calls it once for the SDK logger (`:144`). Harmless but pointless per-request allocation on a hot path that already rebuilds 168 tool registrations. Build the logger once in `New` and store it on `Server`.

## Checked and clean (no action)

- **ID families.** Every `scope.AccountID` / `scope.BusinessID` / `scope.BusinessUUID` argument matches its lib signature, including the mixed services -- `Staff` (List business-scoped, Get/Update/Delete account-scoped), `Services` (Create/GetBillableItem account-scoped, Get/List business-scoped), `Systems` (both), `Reports.TimeEntryDetails` (business). No cross-family conversion anywhere.
- **`json.Number` handling (implementer report item 1).** The diagnosis is right and the fix is right. `RetainerCreateRequest.Fee` has no `omitempty`, so the tool's `fee` is schema-required and cannot arrive empty; `RetainerUpdateRequest`'s `Fee`/`ExcessRate` do have `omitempty`, and `json.Number("")` has `reflect.String` kind so `omitempty` elides it -- a partial update therefore cannot accidentally zero a fee. The one path that would have been dangerous (encoding/json rewrites an empty `json.Number` to `0` when the field is not `omitempty`) is closed by schema-required on create.
- **Variadic `RequestOption` exposure (item 2).** `include []string` is on exactly the seven `*_get` tools whose lib signature takes `opts ...RequestOption`, plus `invoices_create/update` and `invoice_profiles_create`. `items_get`, `payments_get`, `tasks_get` correctly omit it.
- **Error mapping design.** `newSpec` (`registry.go:70-80`) routes every error, including input-decode failures from `uploadIn.reader()`, to `errResult` with a nil Go error, so no API failure can become a JSON-RPC protocol error. `errorText` never formats an input struct. `errResult`/`fbErrorContent` shapes match `docs/mcp.md:110`.
- **`client_secret` redaction.** `redactApplication`/`redactApplications` (`tools_identity.go:45-57`) cover all three `Application`-returning tools and are tested to assert the key is *absent*, not merely empty.
- **Stateless HTTP.** `Stateless: true`, `JSONResponse: true`, no session id, 401 + `WWW-Authenticate` ahead of any JSON-RPC parsing (`requireBearer` wraps the streamable handler at `server.go:151`), unauthenticated `GET /healthz`, sane `http.Server` timeouts, `Shutdown` on ctx cancel with a buffered error channel (no goroutine leak). `TestStatelessProperty` genuinely proves per-request bearer isolation against a recording upstream -- the strongest test in the module.
- **Schema sharing.** `schemaFor` runs once per tool at package init; the shared `mcp.SchemaCache` is documented and implemented goroutine-safe (`sync.Map`); `jsonschema.Schema.Resolve` writes only to its own `Resolved` side state (`resolve.go:350`), so concurrent cold-cache resolution of a shared `*Schema` is not a race.
- **Go canon.** Doc comments on every exported identifier in all three internal packages; `%w` wrapping throughout; no package-level mutable state beyond the immutable `All` registry; no `t.Skip`; no committed `-run` filters; the one `panic` (`schema.go:40`) is init-time and unreachable from tool input; `main.go` is one statement.
- **Naming and conventions.** All 168 names match `tools.md` and the `{service_field_snake}_{method_snake}` rule; all 168 annotation classes match `tools.md`; `go.mod`'s lib pseudo-version `v0.0.0-20260901220418-d795b3fedd2b` resolves to a real commit (`d795b3f`), so the documented `go install ...@latest` path is not broken outside the workspace; docs are ASCII-only, unwrapped, `--`/`->`; CHANGELOG follows the existing style.
- **Tree hygiene.** `git status --porcelain` empty; `mcp/coverage.out`, `dist/`, and `.worktrees/` are all matched by `.gitignore` and untracked; no scratch files, no committed artifacts, no operator-specific strings in the diff.
