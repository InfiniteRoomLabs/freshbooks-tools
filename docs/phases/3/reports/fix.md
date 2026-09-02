# Phase 3 fix report (MCP server review-gate)

Branch `phase-3/mcp`, fix commit `0280faa` on top of `df6d17b` (the triage). Applies `docs/phases/3/triage.md`'s F1-F15 in full, exactly as numbered; nothing from the "Considered and NOT applied" list was touched. `mise run check` (all three modules) is green on a clean tree.

## Per-item disposition

| Item | Done | How |
|---|---|---|
| F1 (blocking) | Yes | Added `newSensitiveSpec` (`registry.go`) alongside `newSpec`: a hand-written `(*mcp.Server).AddTool` `ToolHandler` that resolves the tool's schema once at init, validates and decodes arguments itself, and on any failure returns one generic, name-only `errResult` -- nothing from the input interpolated. Applied to the four `payment_options_*` tokenization tools and `identity_update_application` (its `client_secret` input is required, same class of exposure per security.md). `docs/mcp.md`'s "Errors and security" section now says non-sensitive tools may have a malformed value quoted back by the generic SDK validator, and names the five sensitive tools that never do. |
| F2 (blocking) | Yes | `fakeUpstream` now returns a `*requestRecorder` (method/path/rawQuery/body, reset per subtest). `TestRoundTrip` asserts, per tool: the right scope id landed in the URL (`assertScopeInPath`), the HTTP method matches `ReadOnlyHint` (`assertMethodMatchesAnnotation`), a non-read-only tool's body carries a value from its synthesized input (`assertBodyCarriesInput`), and `page`/`per_page`/`search`/`include` probes reach the query string when the schema has them (`synthWithProbes` + `assertProbesInQuery`). Added a targeted check for the one pair no generic rule could distinguish -- `bills_archive` vs `bills_delete`, identical shape and path, differing only in the closure-hardcoded `vis_state` literal. Replaced the vacuous `json.Marshal(StructuredContent)` check with a real `json.Unmarshal` into the actual lib type for one tool per family (`identity_whoami`, `invoices_get`, `projects_get`, `time_entries_create`). |
| F3 | Yes | `Config.Validate` rejects `--transport http` with any of `AccountID`/`BusinessID`/`BusinessUUID` set (`hasDefaultScope`). `docs/mcp.md`'s env table marks the three "Stdio only" with the confused-scope reasoning. |
| F4 | Yes | `CrossOriginProtection: &http.CrossOriginProtection{}` set explicitly in the `StreamableHTTPOptions` literal (`server.go`). The field is deprecated in go-sdk v1.7.0 in favour of middleware wrapping; kept the field per the fix instruction and security.md's own recommendation, with a `//nolint:staticcheck` and a comment explaining why. |
| F5 | Yes | `Validate` requires `Path` to start with `/` and `Addr` to pass `net.SplitHostPort`. Tests cover `--path mcp`, `--path ""`, `--path relative/path`, and four bad `--addr` values. |
| F6 | Yes | `TestSensitiveToolsNeverEchoInput` (replaces the old single-tool test) tables all five `newSensitiveSpec` tools, each with a well-typed subtest (plants a real sensitive value) and a schema-invalid subtest (also corrupts one field's type). Both assert the sensitive value is absent from `Content`, `StructuredContent`, and a captured debug-level log. `TestApplicationSecretRedacted` now checks `Content` as well as `StructuredContent`. Added `server.TestLoggingNeverLeaksBearer` (new, in the `server` package): drives one real `tools/call` through `HTTPHandler` and one through the stdio path's client-construction building blocks (`TokenSource` + `clientOptions`), both at debug level with a known bearer, asserting neither logging path emits it. |
| F7 | Yes | `Load` now records a `FRESHBOOKS_BUSINESS_ID` parse failure on `Config` (`businessIDErr`); `Validate` returns it, naming the bad value. Test flipped from `[edge] silently ignored` to `[sad] rejected by Validate`. |
| F8 | Yes | `TestErrorShapeEndToEnd` (`errorshape_test.go`, new): a real 422-with-field fixture (`clients_get`) and a real 401 fixture (`identity_whoami`), each driven through a live MCP session, with the `IsError` content parsed as the documented `{status, code, message, field, family}` JSON and every field asserted. |
| F9 | Yes | Deleted the one stray `// inventory:` comment (`tools_attachments.go`). Corrected `implementer.md` item 4, which had claimed such comments survived across the module. |
| F10 | Yes | `tableRowPattern` now captures the annotation column; `mdRow` carries it; `TestParityAgainstToolsMD` compares it against each `Spec.Annotations`' resolved class via a new `annotClass` helper (mirrors `hintRO`/`hintD`/`hintI`/`hintW`'s exact field combinations). |
| F11 | Yes | Added `Server.Serve(ctx, net.Listener)`; `RunHTTP` is now a thin `net.Listen` + `Serve` wrapper. `TestRunHTTPShutsDownGracefully` binds its own listener, calls `Serve` directly, and polls `/healthz` (`waitHealthy`, 5ms interval, 5s deadline) until it answers before cancelling -- no sleep. |
| F12 | Yes | `getServer` returns `nil` (not a tool-less `*mcp.Server`) when `freshbooks.NewClient` fails, and logs at error level first. Added `TestGetServerReturnsNilOnClientConstructionFailure`, which forces the failure with an invalid `BaseURL` and asserts a real HTTP `400` (`no server available`, go-sdk's own message for a nil `getServer` return). |
| F13 | Yes | `docs/mcp.md`'s paging sentence names `retainers_list`, `ledger_accounts_list`, `staff_list`, and `service_rates_list` as the four `*_list` tools with no `page`/`per_page` fields, and why. |
| F14 | Yes | `Server` now has a `logger *slog.Logger` field, built once in `New` (`cfg.Logger()`); `clientOptions` and `HTTPHandler` both read `s.logger` instead of calling `cfg.Logger()` per request/per handler-build. |
| F15 | Yes (APPLY-RECOMMENDED #1-6, OPTIONAL #9) | `reportIn[O any]` (`tools_reports.go`) replaces twelve near-identical wrapper structs; the twelve report tools now instantiate it per options type. `flagDefs` (`config.go`) is one table carrying `usage` too; `stringFlag(cmd, name)` resolves the env twin from a `envForFlag` map built off the same table. `searchOf` (`shapes.go`) is shared by `listIn.search()` and `tools_retainers.go`'s list closure. `identityDeleteBusinessIn`/`identityDeleteBusinessSubscriptionIn` deleted; their two Call closures now take bare `BizScope`/`AcctScope`, matching six other single-embed tools already in the package. `buildRegistry` is one `slices.Concat` call. `newTestSession(t, upstream, defaults, logger) *mcp.ClientSession` (moved to `roundtrip_test.go`, `t.Cleanup`-based) replaces three separate open-coded wirings across `roundtrip_test.go`, `redaction_test.go`, and `unit_test.go`. `ok()` inlined into `void()`. |

Everything in F1-F15 was applied as specified; nothing was silently substituted. The one item where I made a judgment call within the instruction's own bounds is F4: the fix order names the exact field/literal to set, which golangci-lint's `staticcheck` flags as deprecated (`SA1019`) in go-sdk v1.7.0. I set the field as instructed and suppressed the lint line with a comment explaining the deprecation and why the field (not the alternative middleware-wrapping form) was kept, rather than substituting the middleware form on my own judgment.

## `mise run check` tail (mcp module, from repo root)

```
== fmt-check: mcp ==
== vet: mcp ==
== lint: mcp ==
level=warning msg="[runner/exclusion_rules] Skipped 0 issues by rules: [Text: \"G(101|304)\", Path: \"_test\\\\.go\", Linters: \"gosec\"]"
0 issues.
== test: mcp ==
ok  	github.com/InfiniteRoomLabs/freshbooks-tools/mcp/cmd/freshbooks-mcp	(cached)	coverage: 78.1% of statements
ok  	github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/config	(cached)	coverage: 97.1% of statements
ok  	github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/server	(cached)	coverage: 91.5% of statements
ok  	github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/tools	(cached)	coverage: 94.6% of statements
== cover: mcp ==
coverage-gate: .../mcp/coverage.out total = 91.9% (floor 90%)
coverage-gate: PASS
== vuln: mcp ==
Your code is affected by 0 vulnerabilities. (1 unreachable transitive advisory, GO-2026-5024, Windows-only x/sys/windows -- security.md finding 6, not warranted, unchanged)
== inventory-check: mcp (skipped -- only freshbooks has an inventory) ==
== build ==
build: mcp -> dist/freshbooks-mcp_{linux,darwin}_{amd64,arm64}, windows_{amd64,arm64}.exe
check.sh: all OK
```

Re-ran the same gate across all three modules from repo root: also `check.sh: all OK`, `freshbooks` inventory-check still `implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0`, `cli` coverage 100%. Neither `freshbooks/` nor `cli/` appears in this fix commit's diff.

## `git log --oneline main..phase-3/mcp`

```
0280faa fix(mcp): apply the review-gate findings
df6d17b docs(phase-3): add the review-gate triage and fix order
2290344 docs(phase-3): add the code-review, simplification, and security reports
0cc509b docs(phase-3): add the implementer report for the MCP server phase
db0dced fix(mcp): declare cobra as a direct go.mod dependency
b106db0 docs(mcp): rewrite docs/mcp.md and the mcp CHANGELOG for Phase 3
9d97451 test(mcp): add config and server tests; fix an empty-env-var precedence bug
573abbd test(mcp): add parity, round-trip, and redaction tests for the tool registry
70425ea feat(mcp): add the 168-tool data-driven registry, config, server, and cmd
77a86be docs(phase-3): add the MCP work order, definitive tool surface, and spec 6 callout
```

## `git status --porcelain`

Empty (before this report's own commit).

## Manifest diff

Captured before touching anything (`docs/phases/3/reports/implementer.md`+reports state, i.e. `0cc509b`+`df6d17b`) and again after the fix commit:

```
diff manifest-impl-before.json manifest-impl-after.json
(no output -- exit 0)
```

Byte-identical. No tool name, description, annotation, or input schema changed.

## Items I could not apply as specified

None. All 15 items applied; the one deviation-adjacent decision (F4's lint suppression) is disclosed above, not silent.
