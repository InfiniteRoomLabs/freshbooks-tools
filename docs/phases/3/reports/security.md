# Phase 3 security review -- MCP server (`phase-3/mcp`, 77a86be..0cc509b)

**Verdict: BLOCK.** One blocking finding: a written, non-negotiable acceptance criterion (plan constraint (b), "the tokenization tools never echo their input into results, errors, or logs -- error paths included") is violated on the go-sdk's own argument-validation path, which runs before this module's handlers and therefore before its clean error mapping. Everything else is advisory. The rest of the surface -- statelessness, token handling, path validation, supply chain, public-repo hygiene -- audits clean, with evidence below.

Method: read-only. No `mise run check`, no tests, no builds. Findings come from reading the diff, the `mcp/` and `freshbooks/` sources, and the `go-sdk@v1.7.0` / `jsonschema-go@v0.4.3` sources in the module cache; plus `git`, `grep`, `go list -m all` (from `mcp/`), and a manual replay of `scripts/redaction-check.sh`'s term list against the branch's files.

---

## Findings

### 1. BLOCKING -- tool input is echoed verbatim on the SDK's schema-validation error path

`mcp/internal/tools/registry.go:70` (the `mcp.AddTool` registration inside `newSpec`).

**Evidence.** go-sdk validates and unmarshals `arguments` *before* it calls the typed handler:

- `go-sdk@v1.7.0 mcp/server.go:360-364` -- `applySchema(input, inputResolved, false)`; on failure it returns `IsError: true` with `errRes.SetError(fmt.Errorf("validating \"arguments\": %v", err))`.
- `go-sdk@v1.7.0 mcp/protocol.go:347-353` -- `SetError` puts `err.Error()` straight into a `TextContent` block.
- `jsonschema-go@v0.4.3 jsonschema/validate.go:126` -- `return fmt.Errorf("type: %v has type %q, want %q", instance, gotType, schema.Type)`, where `instance` is the offending input value itself. Same pattern at `:120` (`type: %v ... is not a valid JSON value`), `:145` (`enum: %v ...`), `:152` (`const: %v ...`), `:193`/`:198` (`minLength`/`maxLength: %q ...`), `:203` (`pattern: %q ...`).
- `go-sdk@v1.7.0 mcp/server.go:370-374` -- an `internaljson.Unmarshal` failure is likewise returned as `errRes.SetError(err)`.

None of this passes through `mcp/internal/tools/errors.go`. That file's `errorText` is careful and correct, but it only ever sees errors the handler returns -- and the handler never runs here.

**Failure scenario (concrete, reachable).** A model calls `payment_options_fb_pay_tokenize` with the card as a string where the schema wants an object:

```json
{"name":"payment_options_fb_pay_tokenize","arguments":{"body":"4242424242424242 exp 12/30 cvv 123"}}
```

The tool result comes back `IsError: true` with content:

```
validating "arguments": type: 4242424242424242 exp 12/30 cvv 123 has type "string", want "object"
```

-- full PAN and CVV returned into the model's context and into whatever transcript the MCP client persists. The nested variant is at least as likely: `{"body":{"card_number": 4242424242424242, "cvv": 123, ...}}` (a bare number where `FBPayTokenizeRequest.CardNumber` is a Go `string`, an extremely common LLM output shape) produces `type: 4.242424242424242e+15 has type "number", want "string"` -- every digit of the PAN recoverable. `name`, `email`, and `postal_code` leak by the same mechanism.

The same exposure applies to `identity_update_application`, whose `ApplicationUpdateRequest.ClientSecret` is a required input string (`freshbooks/settings.go:212-221`): a wrong-typed `client_secret` is echoed back into the result. That one is not covered by a written constraint, but it is the same class and the same fix.

`docs/mcp.md`'s "Errors and security" section currently states these four tools "never echo their input into a result, an error, or a log" -- as written, that claim is not true.

**Fix.** Take the four tokenization tools (and `identity_update_application`) off the generic `mcp.AddTool` path and register them via the untyped `(*mcp.Server).AddTool` with a hand-written `ToolHandler` that does its own `jsonschema` validate + unmarshal and, on any decode or validation failure, returns `errResult("invalid arguments for <tool>: the input did not match the tool's input schema")` with nothing from the input interpolated. A `Spec` field (e.g. `SanitizeInputErrors bool`) keeps this data-driven rather than a fork of the registry. `(*mcp.Server).AddReceivingMiddleware` (`go-sdk mcp/server.go:1770`) is a viable alternative: wrap `tools/call` and, for those tool names, replace the content of any `IsError` result the handler did not itself produce.

Then extend `TestTokenizationNeverEchoesCardData` to feed schema-invalid, card-shaped input to all four tools and assert the PAN is absent from `result.Content` as well as `StructuredContent` -- the current test only exercises the well-typed happy path, which is exactly why this got through.

---

### 2. ADVISORY -- default scope is applied in HTTP (multi-tenant) mode

`mcp/internal/server/server.go:131` with `mcp/internal/config/config.go:137-155`.

`getServer` passes `s.defaultScope()` -- built from `FRESHBOOKS_ACCOUNT_ID` / `FRESHBOOKS_BUSINESS_ID` / `FRESHBOOKS_BUSINESS_UUID` -- into every per-request tool registration. `Validate` guards only the *token* environment for stdio; the scope environment is never checked against transport. `docs/mcp.md`'s env table marks the token variables "Stdio only" but leaves the three scope variables unqualified.

**Failure scenario.** An operator runs `serve --transport http` in a shell that already exports `FRESHBOOKS_ACCOUNT_ID` (the docs' own stdio example exports it). Caller B, authenticating with their own bearer, invokes `invoices_list` with no `account_id`; the call silently addresses the operator's account instead of B's. Where B has no access the API rejects it; where B does (shared account, accounting-firm membership) it succeeds against the wrong scope -- including for destructive tools such as `identity_delete_business` and `identity_delete_business_subscription`, which take *only* a scope field.

FreshBooks' own authorization still gates every call, so this is a confused-scope hazard, not privilege escalation. It is silent, though, which is what makes it worth closing.

**Fix.** In `Validate`, return an error when `Transport == "http"` and any of the three scope fields is non-empty. Failing that, mark those three rows "Stdio only" in `docs/mcp.md`'s environment table.

---

### 3. ADVISORY -- cross-origin protection is not enabled

`mcp/internal/server/server.go:141-145`.

In go-sdk v1.7.0, `NewStreamableHTTPHandler` installs `http.CrossOriginProtection` only when the `MCPGODEBUG` parameter `enableoriginverification=1` is set (`go-sdk mcp/streamable.go:242-245`), so by default the handler performs no `Origin` validation. `HTTPHandler` leaves `CrossOriginProtection` nil.

Practical exploitability today is low: a browser cannot attach `Authorization` without a CORS preflight the server never answers, and `serveStateless` rejects anything whose base media type is not `application/json` (`streamable.go:388-391`), which a simple form POST cannot set. But that safety is incidental to two other checks rather than deliberate, and it would silently regress if either loosened.

**Fix.** Set `CrossOriginProtection: &http.CrossOriginProtection{}` explicitly in the `StreamableHTTPOptions` literal.

---

### 4. ADVISORY -- `--path` is never validated; a bad value panics at startup

`mcp/internal/config/config.go:137-155` (`Validate`) and `mcp/internal/server/server.go:151` (`mux.Handle(s.cfg.Path, ...)`).

`Validate` checks `Transport`, `LogFormat`, and `LogLevel` but not `Path`. `serve --transport http --path ""` (or any pattern `http.ServeMux` rejects, e.g. `--path mcp` with no leading slash) panics inside `mux.Handle`, so the process dies with a raw Go panic instead of the clean flag error every other invalid flag produces. Note the empty-env guard at `config.go:98` does not help: an explicitly passed `--path ""` sets `Changed`, so the empty string wins.

Operator-triggered, not remote. **Fix:** in `Validate`, require `strings.HasPrefix(c.Path, "/")` and a non-empty value.

---

### 5. ADVISORY -- the written constraints' tests are thinner than the constraints

`mcp/internal/tools/redaction_test.go`.

- `TestTokenizationNeverEchoesCardData:124-167` exercises 1 of the 4 tokenization tools (`payment_options_fb_pay_tokenize`). The plan names four.
- Its result assertion is an either/or (`if !result.IsError { check StructuredContent } else { check error text }`), and the success branch never inspects `result.Content`.
- Its log capture is wired only into `freshbooks.WithLogger` (`redaction_test.go:34-36`). `mcp.NewServer(..., nil)` at `:42` leaves `ServerOptions.Logger` at `DiscardHandler`, and no test drives `HTTPHandler`'s `StreamableHTTPOptions.Logger` -- so neither production logging path is under test.
- The plan's "log-capture test that a full stdio + HTTP request cycle at debug level never logs the bearer" does not exist anywhere in the module (grep for a log-capturing handler in `mcp/**/*_test.go` returns only the card-number test above). I verified by inspection that no bearer reaches a log today (see "Verified clean" below), so this is a missing regression guard rather than a live leak -- but it is the guard that would catch a future `WithLogger` or SDK-default change.
- `TestApplicationSecretRedacted:107-116` asserts only on `StructuredContent`. The work order asked for absence "on the wire, not just on the struct", which includes the `TextContent` block. (No live leak: `go-sdk mcp/server.go:398-440` derives that block from the same already-redacted value.)

**Fix.** Table the four tokenization tools; assert on both `Content` and `StructuredContent` unconditionally; add a debug-level log-capture test that drives one `tools/call` through `server.HTTPHandler()` with a known bearer and asserts the bearer is absent from the captured stderr.

---

### 6. ADVISORY -- `GO-2026-5024` / `golang.org/x/sys@v0.41.0`: the reachability claim checks out

Verified without running `govulncheck` (not installed; `scripts/check.sh:58` runs it via `go run ...@v1.7.0`, which would be a build).

`go list -m all` from `mcp/` confirms `golang.org/x/sys v0.41.0` as an indirect dependency. Grepping the whole dependency graph in the module cache, the only package of it that anything imports is `golang.org/x/sys/cpu`, pulled by `github.com/segmentio/asm/cpu/x86/x86.go:5` (go-sdk's JSON layer). Nothing in the graph -- go-sdk, jsonschema-go, cobra, mousetrap, pflag, uritemplate, oauth2, sync, time, segmentio/encoding -- imports `golang.org/x/sys/windows`. A Windows-only advisory in that module is therefore unreachable on *every* GOOS, including the `dist/freshbooks-mcp_windows_{amd64,arm64}.exe` artifacts the build matrix ships.

**Bumping is not warranted for security.** It is cheap hygiene and would quiet the gate's informational line; either choice is defensible.

---

### 7. ADVISORY -- `getServer`'s unreachable error path degrades confusingly

`mcp/internal/server/server.go:123-130`. When `freshbooks.NewClient` fails, `getServer` returns an `*mcp.Server` with no tools registered, so the client sees "unknown tool" for every call rather than a diagnosable failure. Currently unreachable (every option is a constant or already-validated config, as the comment says). **Fix:** return `nil` so the SDK answers `400 no server available` (`streamable.go:401-404`), and/or log at error level.

---

## Verified clean (with evidence)

**1. Secrets never leak into logs, errors, `String()`, `--help`, the manifest, fixtures, or docs.**
The lib logs exactly `method`, `redactPath(url)`, `status`, `attempt` and nothing else (`freshbooks/transport.go:323,345`); `redactPath` strips query, fragment, and userinfo (`transport.go:486-493`). `*freshbooks.Error.Error()` renders status/family/message/errno/field and never the raw body (`freshbooks/errors.go:30-82`). `Config.String()` and `Config.LogValue()` reduce `AccessToken`/`ClientSecret`/`RefreshToken` to `redacted` / presence booleans (`config.go:213-239`), and `Config` is the only type in `mcp/` with a `String()` method. A grep for `%v`, `%+v`, `Sprintf`, `Sprint`, and `Print` across non-test `mcp/` sources returns only that redacting `Sprintf` and three doc comments -- no tool input is formatted anywhere. `ServerOptions.Logger` is left nil, so the SDK's own `s.opts.Logger.Warn(...)` sites resolve to `DiscardHandler` (`go-sdk mcp/logging.go:105-109`). `StreamableHTTPOptions.Logger` is set, but its only call sites are connect and stream-close failures -- it never logs headers or message payloads. `--help` output is flag usage only (`config.go:75-81`); the `tools` manifest is name/description/annotations/schema (`root.go:53-63`). Logs go to stderr in both formats (`config.go:173-186`), so stdio's wire protocol is never corrupted.

**2. Written constraint (a) -- `client_secret` stripped from every `Application` path.**
`redactApplication` / `redactApplications` (`tools_identity.go:39-57`) zero `ClientSecret` on all three tools that can return one: `identity_create_application:117-122`, `identity_applications:127-133`, `identity_update_application:138-144`. Those three are the only paths in the lib that produce an `Application` (`freshbooks/settings.go:180,197,232` -- confirmed by grep). The field is `omitempty` (`settings.go:151`), so zeroing removes the key rather than emptying it, and the test asserts both the value and the key are gone. `Content` and `StructuredContent` are derived from the same redacted value by the SDK, so they cannot disagree.

**3. Stateless HTTP.**
`getServer` (`server.go:118-133`) builds a fresh `*mcp.Server` and a fresh `freshbooks.Client` per request, with `auth.StaticTokenSource(token)` closing over that request's bearer only. Nothing is keyed by token or client; the sole shared object is `*mcp.SchemaCache` (immutable precomputed schemas). `Stateless: true` makes the SDK neither read nor set `Mcp-Session-Id` and answer GET/DELETE with 405 (`go-sdk mcp/streamable.go:129-145,373-386`); the legacy session behavior is behind an `MCPGODEBUG` opt-in this code does not set. `requireBearer` (`server.go:102-111`) wraps the streamable handler at `mux.Handle`, so the 401 + `WWW-Authenticate: Bearer` is written before any body is read or parsed. `GET /healthz` writes a bare 200 with no body (`server.go:148-150`). `http.Server` sets `ReadHeaderTimeout` 10s, `ReadTimeout` 30s, `WriteTimeout` 60s, `IdleTimeout` 120s, with a 10s graceful-shutdown budget (`server.go:157-184`). HTTP mode structurally cannot fall back to an env token: `getServer` has exactly one token path, and it reads the header.

**4. Stdio token handling.**
The rotating path goes through `auth.NewFileStore` -- 0600 file inside a 0700 directory, written temp + chmod + fsync + rename (`freshbooks/auth/store.go:42-132`). The seed path saves only when `Load` returns `ErrNoToken` (`config.go:200-206`), so an existing token file is never clobbered; a corrupt or unreadable file surfaces as an error instead of being overwritten. Nothing else in `mcp/` touches the filesystem (grep for `os.`/`filepath`/`ioutil` returns only env reads, `os.Stderr`, and `os.Exit`).

**5. Trust boundaries.**
Tool inputs decode into typed Go structs with SDK schema validation ahead of the handler. Every caller-supplied string that becomes a path segment -- `AccountID`, `BusinessUUID`, ledger account UUID, ledger sub-type id, team-member UUID, report download token, OAuth client id, payment id -- passes `pathSegment` in the lib (51 call sites; `transport.go:304-315` rejects empty, `/`, `?`, `#`, `.`, `..`), with `noTraversal` in `resolve` as a second guard (`transport.go:281-293`). The MCP builds no request paths of its own -- it only calls lib methods -- so it cannot bypass that. `BusinessID` is `int64` throughout. Uploads: `upload.go:20-26` decodes base64, bounded upstream at 10 MiB by the lib (`freshbooks/transport.go:27,147-152`, which also reduces the filename to `filepath.Base`) and, in HTTP mode, at 4 MiB of request body by the SDK's `DefaultMaxRequestBodyBytes` (`streamable.go:223-249,344-346`). Binary results are bounded by `maxResponseBytes` on the read (`transport.go:337`). No shell-out, no `unsafe`, no filesystem path accepted from any tool input.

**6. Supply chain.**
`mcp/go.mod` declares 4 direct (go-sdk v1.7.0, jsonschema-go v0.4.3, cobra v1.10.2, the lib) and 9 indirect -- exactly the work order's allowlist plus each one's transitive closure, each traceable to cobra (pflag, mousetrap) or go-sdk (segmentio/asm, segmentio/encoding, uritemplate, oauth2, sync, sys, time). `mcp/go.sum` carries both the `h1:` and `/go.mod` hash for the lib pseudo-version `v0.0.0-20260901220418-d795b3fedd2b`, which `git cat-file` resolves to the real commit `d795b3f`. `.github/workflows/`, `scripts/`, `mise.toml`, `.golangci.yml`, `go.work`, `freshbooks/`, and `cli/` are all untouched by the diff (`git diff --stat main...phase-3/mcp` over those paths is empty).

**7. Public-repo hygiene.**
I replayed `scripts/redaction-check.sh`'s logic against every file the branch touches, using the 18 terms the agent-ops resolver returns: **clean**. Separately grepped the raw diff for the operator's home path, `deathnerd`, `100.x` tailnet addresses, `*.lab.*` / `*.internal.*` domains, `infiniteroomlabs.cloud`, `IRL/` vault paths, gitea, and Bitwarden references: no hits. Fixture identifiers are synthetic and obviously so -- `AccountID: "ACM000TEST"`, `BusinessID: 9000001`, `BusinessUUID: "00000000-0000-4000-8000-000000000099"` (`roundtrip_test.go:179-184`); the card number in the redaction test is `4242424242424242`, the universal Stripe test PAN. The two long digit runs my scan flagged (`2620494`, `6181061`) exist nowhere in the tree as standalone tokens -- they are substrings of `go.sum` base64 hashes. The implementer report elides its one absolute path as `/home/.../freshbooks-tools/...`.

**8. Scratch and stray files.**
`git status --porcelain` is empty. Including ignored files, the only untracked artifacts are `dist/` (the build matrix's output), `mcp/coverage.out`, and an empty leftover directory skeleton `.worktrees/docs/phases/2/reports/` from Phase 2 -- all three covered by `.gitignore`, and `git worktree list` shows no registered worktree, so the `.worktrees` remnant is an empty husk safe to delete at any time. No `_scratch`, no editor backups, no stray fixtures.

---

## Summary

| # | Severity | Finding |
|---|---|---|
| 1 | **BLOCKING** | Tool input echoed verbatim in the SDK's schema-validation error result; breaks written constraint (b) for the four tokenization tools and exposes `identity_update_application`'s `client_secret` the same way |
| 2 | ADVISORY | Default scope applied in HTTP mode; silently redirects scope-omitting calls (including destructive ones) to the operator's account |
| 3 | ADVISORY | `CrossOriginProtection` left nil; off by default in go-sdk v1.7.0 |
| 4 | ADVISORY | `--path` unvalidated; a bad value panics at startup |
| 5 | ADVISORY | Written-constraint tests cover 1 of 4 tokenization tools, skip `result.Content`, and the required bearer-in-logs test is missing entirely |
| 6 | ADVISORY | `GO-2026-5024` unreachability verified by import graph; bump optional |
| 7 | ADVISORY | `getServer`'s unreachable error path returns a tool-less server |

Fix #1 and re-gate. #2 and #5 are worth folding into the same fix commit -- #2 is a two-line `Validate` addition and #5 is the test that would have caught #1.
