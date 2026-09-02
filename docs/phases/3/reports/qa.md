# Phase 3 QA / reality-check report (MCP server)

Subject: branch `phase-3/mcp` @ `494bc49` (11 commits ahead of `main`, `77a86be..494bc49`; fix commit `0280faa`).
Lane: QA -- the only lane permitted to run the gate. No source file was modified; nothing was committed. Every throwaway probe file created during this pass has been deleted.

## Verdict: PASS

Zero BLOCKING findings. Six ADVISORY findings, none of which should hold the merge; two of them (Q1, Q2) are worth carrying into the Phase 4 backlog rather than being fixed under a re-gate.

Basis: the gate is green on a clean tree, all 168 registered tools round-trip with real request assertions, all four `GOAL.md` stage-3 mandatory probes pass under my own independently written probes (not just the repo's tests), and all fifteen triage items F1-F15 verify against a command or a `file:line`. The one place I expected the implementation to be wrong -- `projects_get`'s path -- turned out to be my expectation that was wrong, and the captured Postman collection confirms the code.

---

## 1. Gate and clean tree

`mise run check` run twice: once before writing anything, once after deleting every probe file. Both green, both on a clean tree.

```
$ mise run check ; echo EXIT=$?
coverage-gate: .../freshbooks/coverage.out total = 91.8% (floor 90%)
coverage-gate: PASS
== inventory-check: freshbooks ==
implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0
coverage-gate: .../mcp/coverage.out total = 91.9% (floor 90%)
coverage-gate: PASS
coverage-gate: .../cli/coverage.out total = 100.0% (floor 90%)
coverage-gate: PASS
check.sh: all OK
EXIT=0

$ git status --porcelain
(empty)
```

Coverage floor met per module: freshbooks 91.8%, mcp 91.9%, cli 100.0%. `inventory-check` still reports `implemented 213, todo 0`.

Note on toolchain: bare `go` on this machine is currently version-skewed (`compile: version "go1.26.6" does not match go tool version "go1.26.7"`). Every probe below was run through `mise exec -- go ...`, which resolves go1.26.6 and matches what `scripts/check.sh` uses. This does not affect the branch; it is an operator-environment note.

## 2. `GOAL.md` stage 2 deliverables

| Deliverable | Met | Evidence |
|---|---|---|
| `mcp/internal/config` (env config: token, account/business ids, base URL override) | Yes | `mcp/internal/config/config.go:18-62` (Config), `:121-151` (Load reads `FRESHBOOKS_{ACCESS_TOKEN,CLIENT_ID,CLIENT_SECRET,TOKEN_FILE,REFRESH_TOKEN,ACCOUNT_ID,BUSINESS_ID,BUSINESS_UUID,BASE_URL}`), `:163-193` (Validate) |
| `mcp/internal/server`, BOTH transports, stateless HTTP via `StreamableHTTPOptions{Stateless: true}` | Yes | `mcp/internal/server/server.go:74-88` (RunStdio), `:150-178` (HTTPHandler, `Stateless: true` at `:152`), `:190-223` (Serve/RunHTTP) |
| `mcp/internal/tools` registry, one tool per lib method, typed input schemas | Yes | `mcp/internal/tools/registry.go:221` (`All = buildRegistry()`), 168 entries; `schema.go:37-43` derives schemas from the input structs; `TestParityAgainstClient` proves the mapping in both directions |
| `// inventory:` provenance carried onto registrations for the parity check | Yes (by design substitution) | `Spec.Keys` (`registry.go:40`) replaces the comments; `TestParityKeyCoverage` (`parity_test.go:184`) enforces it. F9 corrected `implementer.md` to say so; `grep -rn '// inventory:' mcp/` now returns only the doc comment at `registry.go:36` describing the field |
| `mcp/cmd/freshbooks-mcp` with `serve` / `version` / `tools` | Yes | `root.go:46-48`; verified against the built binary: `freshbooks-mcp version` -> `freshbooks-mcp 0.0.0-dev`, `tools` -> 168-entry JSON, `serve` exercised live in probe 2 |
| One-statement `main.go` | Yes | `mcp/cmd/freshbooks-mcp/main.go:12-14`: `os.Exit(run(os.Stdout, os.Stderr, os.Args[1:], version))`, one statement |
| Parity test failing on any lib method without a tool and vice versa | Yes | `parity_test.go:140-174` reflects over `freshbooks.Client`'s exported service fields, excludes `All`, and errors in both directions |
| Round-trip tests MCP tool -> lib -> httptest fixture for every tool | Yes | `TestRoundTrip` (`roundtrip_test.go:537`), 168/168 subtests pass; assertions listed in section 3 |
| `docs/mcp.md` with per-transport setup | Yes | `docs/mcp.md`: Claude Desktop (`:22-40`), Claude Code (`:42-48`), HTTP + curl (`:50-72`). Curl sequence verified live, section 9 |
| `mcp/CHANGELOG.md` entry | Yes | `mcp/CHANGELOG.md:8` `## [Unreleased]` with an `### Added` block covering the registry, both transports, config, and cmd |
| Coverage >= 90% for the mcp module | Yes | 91.9%, gate line above |
| No lib/cli changes | Yes | `git diff --name-only main..HEAD -- freshbooks/ cli/` -> empty |

## 3. Mandatory probe 1 -- every registered tool round-trips

`mise exec -- go test -race -run TestRoundTrip -v ./internal/tools/`: **168 PASS, 0 FAIL, 0 SKIP** (counted from the `--- PASS: TestRoundTrip/` lines).

F2 is real, not cosmetic. `TestRoundTrip` now asserts, per tool:

- **method vs annotation** -- `assertMethodMatchesAnnotation` (`roundtrip_test.go:367`): `ReadOnlyHint` must be GET, non-read-only must not be.
- **scope id in the path** -- `assertScopeInPath` (`:338`): whichever of `account_id`/`business_id`/`business_uuid` the schema declares must appear in the recorded URL with the value `testScope` supplied, so an account/business scope crossover fails.
- **body for writes** -- `assertBodyCarriesInput` (`:423`): a non-read-only tool with string-valued input must send a body containing one of those values.
- **query probes** -- `assertProbesInQuery` (`:452`): `page`/`per_page`/`search`/`include` must reach `rawQuery` wherever the schema declares them.
- **the one pair no generic rule separates** -- `visStateBodyExpectation` (`:491`) pins `bills_archive` to `"vis_state":2` and `bills_delete` to `"vis_state":1`.
- The vacuous `json.Marshal(StructuredContent)` check is gone; `representativeDecode` (`:502`) unmarshals into the real lib type for one tool per family.

**My own probe** (deleted): three tools driven through a session against a recording `httptest.Server`, hand-checked against the FreshBooks docs and the captured Postman collection.

```
OBSERVED POST /accounting/account/ACM000TEST/invoices/invoices  body={"invoice":{"customerid":3,"create_date":"2026-01-02"}}
OBSERVED GET  /projects/business/9000001/projects/424242
OBSERVED GET  /timetracking/business/9000001/time_entries?page=3&per_page=25
```

`invoices_create` and `time_entries_list` matched my computed expectation exactly. `projects_get` did not: I expected `/project/424242` (singular). **My expectation was wrong.** The Postman collection is explicit that FreshBooks is itself inconsistent here -- `Projects/Single Project` is `GET .../projects/{{projectId}}` (plural) while `Projects/Create Single Project` and `Projects/Update Project` are `.../project` (singular) -- and `freshbooks/projects.go:108` mirrors the capture correctly. No finding; recorded because a QA lane that only reports where it was right is not reporting.

## 4. Mandatory probe 2 -- stateless property

Repo tests read: `TestStatelessProperty` (`server_test.go:169`) drives two sequential sessions with different bearers through a real `StreamableClientTransport` and asserts bearer order plus absence of `Mcp-Session-Id`. `TestLoggingNeverLeaksBearer` (`:355`) drives one `tools/call` through `HTTPHandler` and one lib call through the stdio building blocks, both at debug level.

**My own probe** against `server.New(...).HTTPHandler()` (deleted). Every assertion passed:

```
upstream bearers = [Bearer qa-bearer-A  Bearer qa-bearer-B]   (each exactly once, in order)
Mcp-Session-Id headers seen = ["" ""]
GET /mcp     -> 405
GET /healthz -> 200 (unauthenticated)
missing          -> 401 WWW-Authenticate="Bearer realm=\"freshbooks-mcp\"" body="" sessionID=""
malformed-scheme -> 401 WWW-Authenticate="Bearer realm=\"freshbooks-mcp\"" body="" sessionID=""
empty-bearer     -> 401 WWW-Authenticate="Bearer realm=\"freshbooks-mcp\"" body="" sessionID=""
lowercase-scheme -> 401 WWW-Authenticate="Bearer realm=\"freshbooks-mcp\"" body="" sessionID=""
```

Each 401 was sent with a deliberately unparseable JSON-RPC body (`{{{not json`) and came back with an **empty response body** and **no new upstream request**, proving `requireBearer` (`server.go:107`) rejects before any JSON-RPC parsing and before any FreshBooks call.

**`FRESHBOOKS_ACCESS_TOKEN` is not used in HTTP mode:** with `FRESHBOOKS_ACCESS_TOKEN=ENV-TOKEN-MUST-NOT-BE-USED` exported for the whole probe, the upstream saw only `Bearer qa-bearer-A` and `Bearer qa-bearer-B` -- never the env value. This is the one stateless assertion the repo's own `TestStatelessProperty` does not make; it holds. Structurally it holds because `getServer` (`server.go:131-143`) builds the token source from `bearerToken(r)` only, and `Config.AccessToken` is read solely by `TokenSource`, which only `RunStdio` calls.

## 5. Mandatory probe 3 -- `client_secret` and card fields never surface

`TestSensitiveToolsNeverEchoInput`, `TestApplicationSecretRedacted`, `TestLoggingNeverLeaksBearer` all pass.

**My own probe** (deleted), driving the exact inputs the work order named:

| Input | Result content |
|---|---|
| `payment_options_fb_pay_tokenize` `{"body": "4242424242424242 exp 12/30 cvv 123"}` (string where object expected) | `invalid arguments for payment_options_fb_pay_tokenize: the input did not match the tool's input schema` |
| `payment_options_fb_pay_tokenize` `{"body": {"card_number": 4242424242424242, ...}}` (bare number) | same generic message |
| `identity_update_application` with an array-typed `client_secret` | `invalid arguments for identity_update_application: the input did not match the tool's input schema` |

In all three the digit string / secret was absent from `Content`, absent from `StructuredContent` (empty), and absent from a debug-level captured log. F1 holds against exactly the failure shapes `security.md` finding 1 described.

**F1 did not break the generic path.** A non-sensitive tool still gets the useful SDK validator error:

```
invoices_get {"id": "not-a-number"}
-> isError, content: validating "arguments": validating root: validating /properties/id: type: not-a-number has type "string", want "integer"
```

That is the documented behaviour (`docs/mcp.md:112`) and it is what a model needs to self-correct. The split between the five `newSensitiveSpec` tools and the other 163 is real and correctly placed.

## 6. Mandatory probe 4 -- parity

`TestParityAgainstToolsMD`, `TestParityAgainstClient`, `TestParityKeyCoverage` all pass. Their assertions are non-vacuous: 168 rows parsed from `tools.md` or `parseToolsMD` fails (`parity_test.go:66`); 212 keys or `TestParityKeyCoverage` fails (`:225`); `authOwnedKey` on any tool is an error (`:210`); only `identity_whoami` may be keyless (`:205`); and F10's annotation column is compared via `annotClass` (`:105`, `:121`).

**Independent manifest check:**

```
$ mise exec -- go run ./mcp/cmd/freshbooks-mcp tools | jq 'length'
168
$ jq -r '.[].name' | wc -l ; sort -u | wc -l
168 / 168            (unique)
$ diff <(LC_ALL=C sort names) names
(empty)              (byte-sorted, matching Manifest()'s sort.Slice)
```

Manifest names vs the `tools.md` tool column: `diff` empty, 168 on both sides. `identity_me` is present, satisfying spec section 6's "an `identity_me` tool is always present".

## 7. Fix verification, F3-F15

Every item verified with a command or a `file:line`. F1 and F2 are covered in sections 3 and 5.

| Item | Verified by |
|---|---|
| F3 | `FRESHBOOKS_ACCOUNT_ID=ACM1 freshbooks-mcp serve --transport http` -> exit 1, `http transport must not have a default scope: ...`; same for `FRESHBOOKS_BUSINESS_UUID`. `config.go:189-191`. `docs/mcp.md:95` marks all three "Stdio only" with the confused-scope reasoning |
| F4 | `server.go:169` `CrossOriginProtection: &http.CrossOriginProtection{}` with the `//nolint:staticcheck` and the deprecation rationale at `:155-168`. Lint is clean in the gate |
| F5 | `serve --transport http --path mcp` -> `invalid --path "mcp": want a path beginning with /`; `--path ""` -> same; `--addr bogus` -> `invalid --addr "bogus": address bogus: missing port in address`; `--addr 127.0.0.1` -> same. `config.go:177-182` |
| F6 | `redaction_test.go:123-200` tables all five `newSensitiveSpec` tools with a well-typed and a schema-invalid row each; `assertNoLeak` (`:255`) checks Content, StructuredContent, and log. `TestApplicationSecretRedacted` (`:78`) checks both content shapes and asserts the `client_secret` *key* is absent, not merely emptied. See Q2/Q3 for the two weak rows |
| F7 | `FRESHBOOKS_BUSINESS_ID=notanumber freshbooks-mcp serve --transport stdio` -> exit 1, `invalid FRESHBOOKS_BUSINESS_ID "notanumber": strconv.ParseInt: ...`. `config.go:58-61`, `:138-149`, `:183-185` |
| F8 | `errorshape_test.go`: 422 subtest asserts `status`, `code=1012`, `field="email"`, `family="accounting"`; 401 subtest asserts `status` and `family`. Real fixtures, real session, every documented field checked |
| F9 | `grep -rn '// inventory:' mcp/` -> only the field's own doc comment at `registry.go:36`. `implementer.md:89` now states the comments were dropped in favour of `Spec.Keys`, and says so as an explicit correction |
| F10 | `parity_test.go:105` compares `annotClass(spec.Annotations)` against `row.annot`; `annotClass` (`:121`) mirrors `hintRO`/`hintD`/`hintI`/`hintW` exactly |
| F11 | No sleep in `TestRunHTTPShutsDownGracefully` (`server_test.go:301`). It binds its own listener, calls `Serve` directly, and polls `/healthz` via `waitHealthy` (`:332`) before cancelling. The 5ms `time.Sleep` inside the poll loop is the polling interval the fix order asked for, not a timing assumption |
| F12 | `getServer` returns `nil` and logs at error level on client-construction failure (`server.go:135-139`). `TestGetServerReturnsNilOnClientConstructionFailure` (`server_test.go:217`) forces it with `BaseURL: "not-an-absolute-url"` and asserts HTTP **400** |
| F13 | `docs/mcp.md:106` names `retainers_list`, `ledger_accounts_list`, `staff_list`, `service_rates_list` and why |
| F14 | `server.go:23` `logger *slog.Logger` field, built once at `:38` in `New`; `clientOptions` (`:63`) and `HTTPHandler` (`:154`) both read `s.logger`. No `cfg.Logger()` call on any request path |
| F15 | Manifest on HEAD byte-identical to the lead's pre-fix copy: `diff .../scratchpad/manifest-before.json /tmp/qa-manifest.json` -> **exit 0, zero lines**. No name, description, annotation, or schema changed across the fix commit |

## 8. Findings

All ADVISORY. None blocks the merge.

### Q1 (ADVISORY) -- 23 tools emit Go field names in `structuredContent`, and `docs/mcp.md` documents no output shape at all

`freshbooks/page.go:12-23` (`Page[T]`) and `freshbooks/identity.go:36-46` (`User`) carry **no JSON tags**, while `Invoice` (`freshbooks/invoices.go:90`), `Project` (`freshbooks/projects.go:23`) and the rest do. Because tool results are `json.Marshal`ed straight out (`registry.go:66`), the untagged types reach the model as Go identifiers.

Expected: a model-facing result whose keys match the API's own snake_case, consistent with the input schema.
Observed, swept across all 168 tools with a probe (deleted):

```
tools emitting Go-cased structured keys: 23
  invoices_list -> {"Items":null,"Page":0,"Pages":0,"PerPage":0,"Total":0}
  identity_whoami -> {"Email":"","FirstName":"","ID":0,"LastName":"","Memberships":null}
  ... 21 more: every *_list tool plus identity_register, time_entries_search
```

Confirmed live through the documented curl path:

```
tools/call identity_whoami ->
"structuredContent":{"ID":1,"Email":"probe@example.com","FirstName":"","LastName":"","Memberships":null}
```

Why it matters rather than being cosmetic: (a) `invoices_list` takes `page`/`per_page` on input and returns `Page`/`PerPage` on output -- the same concept under two spellings in one call; (b) `docs/mcp.md:72` singles out `identity_whoami` as "the fastest way to confirm a token works and to **discover the `account_id`/`business_id`/`business_uuid`**", and the tool answers with `Memberships`, so the doc's own entry-point walkthrough crosses a naming gap it never mentions; (c) `docs/mcp.md` documents input paging (`:106`) but says nothing anywhere about the result envelope.

This is a **lib** defect (Phase 1's `Page[T]` and `User`), not something Phase 3 introduced, and adding tags is an API-visible change to a released-shaped package -- not a re-gate fix. All four lanes missed it because it is only observable in a marshaled result, which nothing before this pass inspected. Recommend: Phase 4/5 backlog (add `json` tags to `Page[T]` and `User`/`Membership`), and, cheaply and now-or-later, one sentence in `docs/mcp.md` describing the list envelope.

### Q2 (ADVISORY) -- the log-leak assertion is vacuous on the five schema-invalid redaction subtests

`redaction_test.go:230-248` captures a debug logger and passes it to `assertNoLeak`, which checks `strings.Contains(logs, s)`. For a schema-invalid call the handler rejects before any HTTP request, so **nothing is ever logged**. Measured with a probe (deleted):

```
well-typed    payment_options_fb_pay_tokenize -> 310 log bytes (real "freshbooks request"/"freshbooks response" lines)
schema-invalid payment_options_fb_pay_tokenize ->   0 log bytes
```

So five of the ten subtests assert non-containment against an empty string. The Content and StructuredContent assertions in those same rows are real, and the well-typed rows exercise the logger genuinely (310 bytes, correctly free of the card number), so the constraint is still properly covered -- this is a redundant third assertion that silently does nothing, not a coverage hole. Worth a comment saying so, or an `if logBuf.Len() == 0` guard on the well-typed rows only.

### Q3 (ADVISORY) -- one redaction row asserts nothing at all

`redaction_test.go:180-183`: `payment_options_save_credit_card`'s `mutateInvalid` sets `args["body"] = "not-an-object"` and returns `nil`. `assertNoLeak` then iterates an empty slice, so that subtest's only real assertion is `IsError`. It is the one row of the five whose schema-invalid case carries no sensitive value to look for. Give it a card-shaped invalid body (e.g. `args["body"] = "tok_super_secret_saved_card_token"`) and it becomes a real check for free.

### Q4 (ADVISORY) -- the `Bearer` scheme match is case-sensitive; RFC 7235 says it should not be

`server.go:91-102`: `bearerPrefix = "Bearer "` compared with `strings.HasPrefix`. RFC 7235 section 2.1 makes the auth-scheme token case-insensitive. Observed: `Authorization: bearer qa` -> **401**, where `Bearer qa` -> 200. Every mainstream MCP client sends canonical `Bearer`, so the practical blast radius is near zero, and rejecting is fail-closed rather than fail-open. Still a spec deviation and a plausible support ticket. One-line fix: `strings.EqualFold` on the first token.

### Q5 (ADVISORY) -- `assertProbesInQuery` matches bare digits, not `key=value`

`roundtrip_test.go:459` and `:464` assert `strings.Contains(rawQuery, "7")` and `Contains(rawQuery, "13")` for `page`/`per_page`. It happens to be sound today (I checked: no other query value in any of the 168 round trips contributes a stray `7` or `13`), but it would also pass if the value landed under the wrong key, or if `per_page=137` were sent. `Contains(rawQuery, "page=7")` costs nothing and closes it. Cheap hardening, not a defect.

### Q6 (ADVISORY) -- the well-typed sensitive subtests do not assert success

`redaction_test.go:213-228` never checks `result.IsError` on the well-typed rows -- a regression that made every valid tokenization call fail would still pass `TestSensitiveToolsNeverEchoInput` (an error result trivially contains no card number). `TestRoundTrip` covers all five tools for `IsError`, so nothing is actually uncovered; adding the two-line check keeps the failure attributable to the right test.

## 9. Docs

`docs/mcp.md` is ASCII-only (`grep -nP '[^\x00-\x7F]'` -> no matches) and not hard-wrapped (longest line 1101 chars).

**The curl sequence in the doc works exactly as written.** I built the binary, pointed `FRESHBOOKS_BASE_URL` at a local fixture server, ran `serve --transport http --addr 127.0.0.1:8798`, and executed the doc's two commands verbatim (only the host substituted):

```
GET /healthz -> 200

curl #1 (initialize) -> 200
{"jsonrpc":"2.0","id":1,"result":{"capabilities":{"logging":{},"tools":{"listChanged":true}},
 "protocolVersion":"2025-06-18","serverInfo":{"name":"freshbooks-mcp","version":"0.0.0-dev"}}}
   response headers carry no Mcp-Session-Id  <- the doc's "nothing to echo" claim is true

curl #2 (tools/call identity_whoami) -> 200
{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"..."}],"structuredContent":{...}}}
   response headers carry no Mcp-Session-Id
```

Both flag names and both env names the examples rely on match what the code reads (`config.go:68-76`). The Claude Desktop and Claude Code stanzas name `FRESHBOOKS_ACCESS_TOKEN`/`FRESHBOOKS_ACCOUNT_ID`/`FRESHBOOKS_BUSINESS_ID` under `serve` with no `--transport`, which is correct: stdio is the default and all three are stdio-only. No mismatch found. The only doc gap is Q1's missing output-shape description.

## 10. Test quality

- **Zero `t.Skip`** anywhere (`grep -rn 't\.Skip' --include='*.go'` -> no matches) and zero SKIP lines across a full verbose run of all 31 top-level tests.
- **No committed `-run` filters**; `-race` is on throughout.
- **No non-determinism**: the only remaining `time.Sleep` is `waitHealthy`'s 5ms poll interval (`server_test.go:343`), which is a bounded poll with a 5s deadline, not a timing assumption. F11 removed the real one.
- **No coverage padding found.** The `cmd/freshbooks-mcp` package sits at 78.1% against a 91.9% module total, but `main.go` holds exactly one statement (the Phase 0 lesson's requirement) and `run_test.go` exercises `version`, `tools`, an invalid transport, a missing token, and an unknown subcommand -- all with real assertions on exit code and output.
- **Fixtures mirror the docs, not the implementation.** `fakeUpstream` (`roundtrip_test.go:212`) is built from `freshbooks/client.go`'s family classification with per-endpoint comments citing the lib decode path that demands each shape; the parity fixture is `docs/phases/3/tools.md`, the frozen surface; the error-shape fixtures are real 422/401 bodies. Q2 and Q3 are the only vacuous assertions found, both narrow.

## 11. Anything undocumented or promised-and-missing

- Spec section 6 promises `identity_me` "is always present" -- present, confirmed in the manifest.
- Spec section 6's 2026-09-01 callout's three corrections (no `_all` tools; `TypeSchemas` overrides with `Out = any`; 168 tools / 212 keys) all match the implementation.
- Q1 is the one behaviour the docs do not describe.
- Nothing promised in `GOAL.md` stage 2 is missing.
- Root `CHANGELOG.md` and `docs/progress.md` are not updated on this branch. That is correct: `GOAL.md` assigns both to stage 4 (Ship), after the merge. Flagging only so the lead does not lose them.

## Commands run

```
mise run check                                              (x2: pre-report, post-probe-cleanup; EXIT=0 both)
git status --porcelain                                      (empty both times)
mise exec -- go test -race -run TestRoundTrip -v ./internal/tools/
mise exec -- go test -race -v ./...                          (mcp module; 31 tests, 0 fail, 0 skip)
mise exec -- go run ./mcp/cmd/freshbooks-mcp tools | jq ...
diff <lead pre-fix manifest> <HEAD manifest>                 (exit 0, empty)
mise exec -- go build -o /tmp/qa-fbmcp ./cmd/freshbooks-mcp
  + serve --transport http --path mcp | --path "" | --addr bogus | --addr 127.0.0.1
  + FRESHBOOKS_BUSINESS_ID=notanumber serve --transport stdio
  + FRESHBOOKS_ACCOUNT_ID=... / FRESHBOOKS_BUSINESS_UUID=... serve --transport http
  + live serve --transport http + the two curl commands from docs/mcp.md
4 throwaway probe files (2 in internal/tools, 1 in internal/server, 1 fixture server) -- all deleted
```

Final state: `git status --porcelain` shows only `?? docs/phases/3/reports/qa.md`.
