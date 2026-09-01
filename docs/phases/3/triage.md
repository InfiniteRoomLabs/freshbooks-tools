# Phase 3 triage (MCP server) -- lead decisions, 2026-09-01

Inputs: `docs/phases/3/reports/{implementer,code-review,simplify,security}.md`. Verdicts: code review REQUEST CHANGES (1 blocking, 11 advisory), simplification 6 APPLY-RECOMMENDED / 6 OPTIONAL / 12 DO-NOT-APPLY, security BLOCK (1 blocking, 6 advisory). No lane-vs-lane conflict; the two blockers are independent and both are real.

One fix commit: `fix(mcp): apply the review-gate findings`. Then re-gate (QA lane), merge `--no-ff`.

## Verification handoff (run before and after the fix)

```
go run ./mcp/cmd/freshbooks-mcp tools > /tmp/manifest-before.json   # on 0cc509b+reports
go run ./mcp/cmd/freshbooks-mcp tools > /tmp/manifest-after.json    # on the fix commit
diff /tmp/manifest-before.json /tmp/manifest-after.json               # must be empty
```

The fix commit must not change the manifest (names, descriptions, annotations, schemas). Simplify's recipe; QA re-runs it.

## Fix list (numbered; the implementer applies exactly these)

### Blocking

- **F1** (security #1, review #4) -- SDK schema-validation errors echo the raw input before our handler runs (`go-sdk mcp/server.go:360-374`, `jsonschema-go validate.go:126`), which breaks written constraint (b) for the four `payment_options_*` tools and exposes `identity_update_application`'s `client_secret`. Fix: add a `Spec` flag (e.g. `sanitizeInputErrors bool`) set on those FIVE tools; for flagged specs `newSpec`'s registration uses the untyped `(*mcp.Server).AddTool` with a handler that validates against the precomputed resolved schema and unmarshals itself, and on ANY validation/decode failure returns `errResult("invalid arguments for <tool>: input did not match the tool's input schema")` with nothing from the input interpolated. Unflagged tools keep the generic path (the SDK's quoted-value errors are useful to a model on non-sensitive tools). `docs/mcp.md`'s constraint sentence stays true as a result; add one clause saying non-sensitive tools may have a malformed value quoted back by the SDK's validator.
- **F2** (review #1) -- `TestRoundTrip` asserts nothing about the request the lib sent, and its `json.Marshal(StructuredContent)` assertion is vacuous. Fix per the review's four steps: recorder on `fakeUpstream` ({method, path, rawQuery, body}, mutex, reset per subtest); an expectation column per tool (HTTP method + path fragment derived from `testScope` so the right scope field is proven in the URL); for write tools assert the recorded body contains a field from the synthesized input; for list tools synthesize `page`/`per_page`/`search`/`include` where the schema has them and assert they land in `rawQuery`; replace the vacuous marshal with `json.Unmarshal(StructuredContent, &<lib type>)` for one representative tool per family (or drop it).

### Advisory, apply in the same commit

- **F3** (security #2) -- `Validate` returns an error when `Transport == "http"` and any of `FRESHBOOKS_ACCOUNT_ID` / `FRESHBOOKS_BUSINESS_ID` / `FRESHBOOKS_BUSINESS_UUID` is set (loud, like the token-env rule). Mark the three rows "Stdio only" in `docs/mcp.md`'s env table and say why (multi-tenant confused scope).
- **F4** (security #3) -- set `CrossOriginProtection: &http.CrossOriginProtection{}` explicitly in the `StreamableHTTPOptions` literal.
- **F5** (security #4, review #2) -- `Validate` requires `Path` to begin with `/` and `Addr` to pass `net.SplitHostPort`; tests for `--path mcp`, `--path ""`, bad addr.
- **F6** (security #5, review #5) -- redaction tests: table all four tokenization tools with per-tool sensitive inputs (card number, CVV, Stripe API key); assert unconditionally against BOTH `Content` text and `StructuredContent` and the captured log; add rows feeding schema-INVALID card-shaped input (bare number for `card_number`, string for `body`) to prove F1; `TestApplicationSecretRedacted` also asserts on the `TextContent` block; add a debug-level log-capture test that drives one `tools/call` through `server.HTTPHandler()` (SDK logger AND lib logger captured) and one through the stdio server construction with a known bearer, asserting the bearer never appears.
- **F7** (review #3) -- a malformed `FRESHBOOKS_BUSINESS_ID` is a `Validate` error, not a silent zero; flip the `[edge] silently ignored` test to `[sad] rejected`.
- **F8** (review #6) -- end-to-end error-shape test through the MCP session: a 422-with-field fixture and a 401 fixture, asserting the `IsError` content parses as the documented `{status, code, message, field, family}` JSON.
- **F9** (review #7, simplify #12) -- delete the stray `// inventory:` comment in `tools_attachments.go`; `Spec.Keys` is the single source of truth. Correct item 4 of `docs/phases/3/reports/implementer.md` to say the comments were dropped in favour of `Spec.Keys` (edit the report in place; it is committed).
- **F10** (review #8) -- `TestParityAgainstToolsMD` captures and compares the annotation column (RO/D/I/W) against the registry's resolved hints.
- **F11** (review #9) -- replace the 50ms sleep in `TestRunHTTPShutsDownGracefully` with a deterministic bind (a `Serve(net.Listener)` seam or the bound address over a channel), poll `/healthz` until it answers, then cancel.
- **F12** (security #7, review #10) -- `getServer` returns `nil` when client construction fails (SDK answers 400) and logs at error level.
- **F13** (review #11) -- `docs/mcp.md` paging sentence notes the four `*_list` tools without `page`/`per_page` (`retainers_list`, `ledger_accounts_list`, `staff_list`, `service_rates_list`).
- **F14** (review #12) -- build the `slog.Logger` once in `server.New` and store it; stop calling `cfg.Logger()` per request.
- **F15** (simplify APPLY-RECOMMENDED #1-#6 and OPTIONAL #9) -- generic `reportIn[O]` for the twelve report inputs; one `flagDefs` table carrying `usage` with `stringFlag(cmd, name)` resolving the env twin from it; `searchOf` helper shared by `listIn.search()` and retainers; drop the two single-embed identity wrapper types; `slices.Concat` in `buildRegistry`; `newTestSession(t, upstream, defaults, logger)` shared by the three test files; inline `ok()` into `void()`. The manifest diff must stay empty.

### Considered and NOT applied (do not re-derive)

- Simplify OPTIONAL #7 (collapse 45 per-resource input structs into 7 shared shapes): ~20 files of churn for a line-count win right before a merge gate; several of those structs are already diverging on descriptions. Revisit in Phase 5 if at all.
- Simplify OPTIONAL #8 (`sort` -> `slices`), #10 (upstream helper cleanup), #11 (spec lookup helper): cosmetic micros; skip.
- Simplify DO-NOT-APPLY 1-12: agreed as written.
- Security #6 (bump `golang.org/x/sys` for GO-2026-5024): reachability verified negative on every GOOS; not warranted. Phase 5 dependency refresh will pick it up.
- Review #4 option (a) (reword the constraint): superseded by F1, which makes the constraint true rather than narrower.

## Re-gate

QA lane (opus) after the fix commit: full gate on a clean tree, the four mandatory acceptance probes from `GOAL.md` stage 3, the manifest diff, and a re-run of the F1/F6 evidence with schema-invalid card input.
