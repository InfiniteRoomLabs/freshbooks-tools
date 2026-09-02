# Phase 4 (CLI) -- QA / reality-check lane

Branch `phase-4/cli` @ `46a5bc4`, 26 commits ahead of `main` (`f557d2d`). Re-gate after the fix pass `2a14c3e..4412775` plus the lead's `46a5bc4`.

**Verdict: NEEDS WORK** -- 2 BLOCKING, 20 ADVISORY.

The branch is in good shape. The gate is genuinely green on a clean tree, all four mandatory probes pass, every attack input from the gate is covered by a named passing test, and the parity contract holds exactly. The two blockers are the same defect in two places: **`docs/cli.md`'s exit-code table promises exit 2 on two paths where the binary returns 0, 1, or 3.** The exit-code table is the CLI's automation contract, and F13/F14 were applied to only one of the two code paths each. Both are small, surgical fixes.

---

## 1. Gate, on the current tree

`mise run check` -> **exit 0**, clean tree before and after. Log: scratchpad `qa-check.log`.

```
== cover: freshbooks == coverage-gate: total = 91.8% (floor 90%) PASS
== cover: mcp ==        coverage-gate: total = 91.9% (floor 90%) PASS
== cover: cli ==        coverage-gate: total = 91.3% (floor 90%) PASS
   cli per-package: auth 88.7%  cmd 87.4%  config 85.2%  output 93.3%
== vuln == No vulnerabilities found (all three modules)
== actionlint == clean     == build == 12/12 cross-compile targets
check.sh: all OK
```

`git status --porcelain` empty before the gate and after it; **`go.work.sum` was not modified** (`git diff --stat go.work.sum` empty).

**The gate ran mostly from Go's test cache**, so I forced an uncached race run -- this is the evidence that matters:

```
mise exec -- go test -C <mod> -race -count=1 ./...   # all three modules
freshbooks: ok (3.1s / 1.1s / 2.6s)   EXIT=0
mcp:        ok (1.4s / 1.0s / 3.8s / 11.2s)  EXIT=0
cli:        ok (1.3s / 3.7s / 1.1s / 1.0s)   EXIT=0
```

`mise run inventory-check` -> `implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0`.

`mise run docs` is **idempotent**: `md5sum docs/cli.md` = `2344e58fdb1c985d3e2326e594a1e1c1` before and after; tree still clean.

`scripts/redaction-check.sh` -> `redaction-check: clean`.

## 2. GOAL.md stage-2 deliverables

| Deliverable | Verdict | Evidence |
|---|---|---|
| Registry + cobra tree | MET | 168 registry entries; `TestParityAgainstCobraTree` both directions |
| `auth login/status/logout/token` | MET | 4 subcommands; 24 passing `TestLogin*`/`TestStatus`/`TestLogout`/`TestToken` subtests |
| Config contexts | MET | `config view/contexts/use-context/set-context`; precedence probe below |
| Output formats `json\|yaml\|table\|name` | MET | all four demonstrated, TTY seams swapped both ways (probe 4) |
| `api` escape hatch | MET | `api <METHOD> <path> -f --query`; `--query` has no `-q` shorthand (spec callout 2) |
| `completion` | MET | `completion bash` emits "bash completion V2 for freshbooks" |
| `version` | MET | prints `0.0.0-dev`, `-ldflags -X main.version` seam present |
| `main.go` one statement | MET | `cli/cmd/freshbooks/main.go:18` -- `os.Exit(cmd.Run(...))`, single statement in `main()` |
| Parity test | MET | 4 parity tests, both directions, incl. reflection over `freshbooks.Client` |
| Round-trip w/ request recording | MET | 168/168 subtests; asserts path, method class, body, query (see 3) |
| Login-flow tests | MET | fake OAuth server; every gate attack input named and passing |
| Redaction test | MET | `TestRedactionSweep`; my independent sweep found 0 leaks (see 5) |
| `docs/cli.md` generated + drift test | MET | `TestDocsUpToDate` green; regeneration byte-identical |
| Changelogs incl. D8/D9 folds | MET (root CHANGELOG pending, F23) | `freshbooks/CHANGELOG.md:126` under `### Changed`; Added-before-Fixed in all three modules |
| Coverage >= 90% | MET | 91.3% cli |
| Dependencies exactly cobra/pflag/yaml.v3/x/term/lib | MET | `cli/go.mod` direct block is exactly those five |

Indirect `go-md2man`/`blackfriday` enter via `cobra/doc` -- the known cost recorded in the triage's "considered and NOT applied" (Security A6), deferred to Phase 5.

## 3. Probe 1 -- every registry command through cobra

`go test -race -count=1 -run TestRoundTrip -v ./internal/cmd/` -> **168/168 subtests PASS**, 0 failures, plus `TestRoundTripAllWalksTwoPages` and `TestRoundTripAllRejectsPage`.

What it actually asserts (read, not taken on trust): exact path per command against a `wantPath` template with `{account}/{business}/{uuid}/{id}` placeholders (`roundtrip_test.go:532`); `origHost == paid.freshbooks.com` for the two tokenization commands through the redirecting transport; GET for `ClassRO` and non-GET otherwise; `page=7`, `per_page=13`, `probe_filter=probe_value`, `sort=probe_sort_field_desc`, `include` in the query where registered; multipart filename + content for uploads; exact fixture bytes for binary output.

**My own black-box probe** -- real built binary, recording HTTP server, hand-checked against the FreshBooks docs:

```
POST /accounting/account/ACM000TEST/invoices/invoices
     body={"invoice":{"customerid":42,"create_date":"2026-09-01"}}
GET  /projects/business/9000001/projects/987654
GET  /timetracking/business/9000001/time_entries?page=3&per_page=25&sort=started_at_desc
```

All three correct. The invoice envelope is right: the CLI takes the **bare** invoice object from `-f` and the lib wraps it in `{"invoice": ...}` -- passing a pre-wrapped `{"invoice":{...}}` is correctly rejected (`unknown field "invoice"`). `--include lines` reaches the wire as `include%5B%5D=lines` (`include[]=lines`), matching the documented form.

## 4. Probe 2 -- login flow against a fake OAuth server

Every attack input the gate named has a dedicated, passing test (`go test -v ./internal/auth/`):

| Gate requirement | Test | Result |
|---|---|---|
| callback with `code`, NO `state` -> rejected, nothing saved | `[sad] a callback with a code but no state at all is rejected (F1/B1: CSRF)` | PASS |
| wrong `state` rejected | `[sad] a state mismatch is rejected and nothing is saved` | PASS |
| second callback rejected (single-use) | `[sad] a second callback request is rejected with 410 and never delivered again` | PASS |
| `--no-browser` pasted URL | `[happy] a pasted full redirect URL validates state and exchanges` | PASS |
| `--no-browser` bare code | `[happy] a bare pasted code skips state validation` | PASS |
| `auth token --refresh` persists before printing | `[happy] --refresh rotates and persists before returning` | PASS |
| `auth logout` posts BOTH tokens to `/revoke` then deletes | `[happy] revokes both the access and refresh token and removes the file` | PASS |

Source confirms the mechanism, not just the test names: `login.go:251` rejects an empty state when `requireState`; `login.go:272` uses `subtle.ConstantTimeCompare`; `login.go:344-350` makes the listener single-use via `sync.Once` + `410 Gone`; `login.go:325-326` sets `Cache-Control: no-store` and `Referrer-Policy: no-referrer` (F26).

**`auth logout --dry-run` (F3) and context validation (F2), black-box against the real binary:**

```
auth login|status|logout|token --dry-run   -> exit 2, "does not support --dry-run"
config view|contexts --dry-run             -> exit 2
credentials file after logout --dry-run    -> STILL PRESENT
--context '../evil' | 'a/b' | '.' | '..'   -> rejected, all three input channels
   (--context flag, FRESHBOOKS_CONTEXT, current-context in a --config file)
```

No file escaped the credentials directory. `--context ""` is treated as unset (falls through to `default`) rather than rejected -- correct and safe, though it differs from the triage's literal wording.

## 5. Probe 3 -- secrets never print

Eight scenarios captured with `--log-level debug` (`config view`, `auth status`, two `--dry-run`s, API 500, API 401, a success, `config contexts`), 4014 bytes of stdout+stderr, grepped for three fixture secrets:

```
qa-fixture-access-token-AAAA   -> 0 occurrences
qa-fixture-refresh-token-BBBB  -> 0 occurrences
qa-fixture-client-secret-CCCC  -> 0 occurrences
```

`freshbooks auth token` is the only command that prints the access token, exactly as documented.

**F25/F30 -- `identity applications`** against a fixture carrying a real `client_secret`:

| output | default | `--show-secrets` |
|---|---|---|
| json | 0 occurrences | 1 occurrence |
| table / name / yaml | 0 occurrences | -- |

**F24 -- control characters**, fixture value containing ESC, TAB, BEL, NUL:

- `table` / `name`: control bytes **stripped** -- `QA[31mRED[0mTABBED` (the ESC byte is gone, so no terminal colour injection).
- `json`: escaped as `\u001b` / `\u0007` / `\u0000` -- untouched-but-safe, as F24 intends.
- `yaml`: `"QA\e[31mRED\e[0m\tTABBED"` -- escaped, safe.

## 6. Probe 4 -- parity, exit codes, output formats

**Command surface, computed independently of the tests:**

```
docs/phases/4/commands.md numbered rows : 168
docs/cli.md '## freshbooks ...' headings: 219
commands.md rows missing from cli.md    : 0
cli.md entries not in commands.md       : 51  (34 group parents + auth/config/completion
                                               parents+subs (15) + api + version)
```

168 + 51 = 219. Exact both ways. Inventory keys cited in `commands.md`: **212**, `identity whoami` the only keyless command, `Authorization/Revoke Refresh Token` correctly auth-owned and uncited.

**`--all` covers the iterators:** `grep -c 'HasAll: *true'` = **17**; `func (s *XService) All(` in the lib = **17**. Match.

**Destructive markers:** 23 registry entries with `Class: ClassD`, 23 rows annotated `D` in `commands.md`, 23 distinct commands whose `Short` carries `(destructive: requires --yes on a TTY)` in `docs/cli.md`. Match.

**Exit codes, each demonstrated against the real binary:**

| Code | Case | Observed |
|---|---|---|
| 0 | successful GET | 0 |
| 1 | API 500 | 1 |
| 2 | unknown `--output` / `--all` with `--page` / `--sort name:sideways` / bad JSON body with **no** credentials (F13) / `--dry-run` on auth+config / destructive on a TTY without `--yes` / binary `-o` overwrite without `--force` / binary `-o -` on a TTY | 2 |
| 3 | `auth token` with no credentials (F8) / registry command with no credentials / API 401 | 3 |
| 4 | API 404 | 4 |

**Output formats with the TTY seams swapped both ways (F11):**

| | non-TTY (pipe) | TTY (pty via `script`) |
|---|---|---|
| default `-o` | `json` | `table` |
| error shape | `{"error":{"status":500,...,"exit":1}}` | `freshbooks: 500 business: qa forced (errno 1012)` |
| destructive, no `--yes` | exit 0 | exit 2 |
| binary `-o -` | PDF bytes, exit 0 | exit 2, "binary output would corrupt your terminal" |

**F19 precedence, live:** file -> `FROM-FILE`; `FRESHBOOKS_ACCOUNT_ID` -> `FROM-ENV`; `--account` -> `FROM-FLAG`; empty env -> falls back to `FROM-FILE`. Exactly as documented.

---

## Findings

### BLOCKING

**Q1 -- `docs/cli.md:90` promises exit 2 for an unrecognized `--log-level`; two reachable paths return 0 and 3.** (`cli/internal/cmd/state.go:245-278`)

`buildClient` calls `buildDryRunClient` (`state.go:254`) and, on the live path, `credentialStore` + `store.Load` (`state.go:259-270`) **before** `buildLogger` (`state.go:275`). Expectation per docs and per triage F14 ("unknown `--log-level` is a usage error"): exit 2 always.

```
clients list --account ACM1 --log-level bogus --dry-run        -> exit 0  (silently ignored, request dumped)
clients list --account ACM1 --log-level bogus  (no credentials) -> exit 3
clients list --account ACM1 --log-level bogus  (credentials)    -> exit 2  (only this one is right)
```

F14 was applied but is unreachable on the two paths that skip it. Fix: validate the log level before the dry-run/credential branch, the way F13 hoisted body validation.

**Q2 -- `docs/cli.md:90` promises exit 2 for "a malformed `--file` body"; `freshbooks api` returns 1 and echoes body content.** (`cli/internal/cmd/api_cmd.go:36-43`)

F13 hoisted `json.Valid` before `buildClient` for registry commands (`registry.go:342`) but not for `api`, which wraps the bytes in `json.RawMessage` and lets the marshal fail inside the lib.

```
clients create --account ACM1 -f bad.json -> {"error":{"message":"--file does not contain valid JSON","exit":2}}
api POST /accounting/... -f bad.json      -> {"error":{"message":"freshbooks: encoding the request body:
                                             json: error calling MarshalJSON for type json.RawMessage:
                                             invalid character 'o' in literal null ...","exit":1}}
```

Two defects in one: the exit code contradicts the documented table, and the `api` path leaks a fragment of the body into the error message where the registry path deliberately says only "does not contain valid JSON". Fix: `json.Valid` in `api_cmd.go` before `buildClient`, same message.

### ADVISORY

**Q3** -- `cli/internal/cmd/config_cmd_test.go:107-114`: `[edge] contexts on an empty config prints nothing` captures `stdout` and never reads it; only exit 0 is asserted. This is precisely the F17 bug class, surviving in a sibling subtest of one that *was* fixed (`:73-87` correctly asserts `"{}"`). `config contexts` could print `null`, `[]`, or a bare header and still pass.

**Q4** -- `cli/internal/auth/paths_test.go:106-125`: `TestDefaultScopes` builds `want` by iterating `scopeObjects`, the same package var `buildDefaultScopes` iterates (`scopes.go:38-44`). Emptying `scopeObjects` leaves the test green (`0 != 0*2` is false, both loops no-op). It verifies the read/write pairing invariant but cannot detect a dropped scope object. Pin `len(DefaultScopes) == 44` or write the strings down.

**Q5** -- `cli/internal/cmd/coverage_gap_test.go:69-80`: `TestBuildClient_CorruptCredentials` asserts only `code == 1` against an unroutable `--base-url`, never `stderr`. Empirically proven mutation-blind:

```
valid credentials   + http://127.0.0.1:1 -> exit 1   <- indistinguishable
corrupt credentials + http://127.0.0.1:1 -> exit 1   <- what the test asserts
no credentials      + http://127.0.0.1:1 -> exit 3
```

If the non-`ErrNoToken` `store.Load` branch were deleted and a corrupt file silently treated as an empty token, the test would still pass. Assert the stderr message.

**Q6** -- `roundtrip_test.go:325-332`: the `wantPath` map was captured from the implementation's own `Run` closures, so a wiring error present at capture time is frozen as the expectation. Nothing else ties a `Command`'s declarative `Service`/`Method` strings to what its closure actually calls. **It is empirically correct today** -- an independent cross-check of all 168 entries against `freshbooks/internal/inventory/testdata/inventory.json` (the vendor Postman collection) matched 166/168, the two exceptions being placeholder-syntax artifacts, not defects. It remains a genuine regression guard. The cheap fix that closes the loop: resolve `c.Keys[0]` against `inventory.json` and assert the recorded path *and method* against the vendor `pathTemplate`/`method`.

**Q7** -- `roundtrip_test.go:611-621`: `assertMethodMatchesAnnotation` is one bit wide (GET vs not-GET). A POST/PUT/PATCH/DELETE swap on the same path is undetected across all 168 commands. The inventory carries the exact method per key; wiring it in fixes Q6 and Q7 together.

**Q8** -- `roundtrip_test.go:736-737`: the JSON-parse assertion is opt-in on non-empty output, so a regression where `--output json` writes nothing passes for 166 of 168 commands. Only the two Binary commands require bytes.

**Q9** -- `cli/internal/config/config_test.go:104-118`: permission assertions compare against `dirMode`/`fileMode`, the same constants `Save` uses (`config.go:22-23`). Changing `fileMode` to `0o644` keeps the test green. `auth_cmd_test.go:109` does it correctly with a hard-coded `0o600`.

**Q10** -- `cli/internal/auth/status.go:15-32`: `StatusInfo` has no `json` tags, so `auth status -o json` emits `{"Context":..., "LoggedIn":..., "CredentialsPath":...}`. This is the exact defect class the D8 fold just fixed for `Page[T]`/`User`, one struct away. Verified live.

**Q11** -- an invalid `--context` exits **1**, while every other invalid global flag value exits 2 (`--output`, `--log-level`, `--sort`, `--timeout`). `docs/cli.md:90` defines 2 as "a usage error: a bad flag". Verified live.

**Q12** -- `auth login`'s local `--timeout` (callback wait, default 5m) shadows the global `--timeout` (per-request, 30s) and the header never says so, though the analogous Binary `-o` shadow *is* documented (`docs/cli.md:118-120`).

**Q13** -- `FRESHBOOKS_OUTPUT` is stated unconditionally in the env table (`docs/cli.md:64`) but is never consulted by the two Binary commands, which read their local `-o` directly (`state.go:373`).

**Q14** -- the `~/.config` fallback when `$XDG_CONFIG_HOME` is unset (the common case on macOS and most Linux desktops) is implemented (`config.go:57-65`, `paths.go:30-38`) and documented in package comments, but omitted from `docs/cli.md:34`, `:62`, and the `--config` flag help.

**Q15** -- the exit-2 enumeration at `docs/cli.md:90` reads as a closed list but omits ~15 other `newUsageError` sites (wrong positional count, non-integer `<id>`, non-integer `--business`, invalid `FRESHBOOKS_TIMEOUT`, `--query` parse failure, `--file` open/read failure, unknown context on `use-context`, missing client credentials on `auth login`, ...). F23 set out to complete this row, so the gaps look unintentional.

**Q16** -- `cli/internal/cmd/errors.go:47-49` comment says `runtimeError` wraps "a filesystem failure reading `--file`". It does not: `registry.go:334`, `api_cmd.go:39`, and `invocation.go:202` all return `usageError` (exit 2).

**Q17** -- `docs/cli.md:24` teaches `auth login --client-id <id> --client-secret <secret>`, which `docs/cli.md:132-135` then warns puts the secret in `ps` and shell history. The quickstart should lead with the env vars.

**Q18** -- `--base-url` is `MarkHidden` (`root.go:70-73`) but appears in the env table (`docs/cli.md:65`) with no note that it is hidden.

**Q19** -- root `CHANGELOG.md` has no Phase 4 entry; Phases 0-3 each have one (`CHANGELOG.md:10-19`). This is the lead's GOAL stage-4 job, not a branch defect, but it currently has no owner in the triage and must not be missed at ship.

**Q20** -- `cli/internal/cmd/redaction_sweep_test.go`: every scenario asserts only *absence*. Nothing asserts the debug log was populated, so the test cannot distinguish "redacted" from "nothing was logged". Two of five scenarios issue no HTTP request, and `FRESHBOOKS_CLIENT_SECRET` is unused by `clients get`, making that secret's assertion vacuous in four of five. Add one positive marker per HTTP scenario.

**Q21** -- `cli/internal/cmd/docsgen.go:12-152`: the `docsHeader` constant is hard-wrapped at ~63 chars average, while every other `docs/*.md` and every phase-4 artifact is unwrapped per the CLAUDE.md house rule. Defensible as "Go source, wrapped like Go source", but the drift test freezes whichever choice is made -- decide it explicitly.

**Q22** -- no `t.Parallel()` exists anywhere in the repo, so the package-level mutable seams (`stdinIsTerminal`/`stdoutIsTerminal` at `state.go:399-406`, `testTransport`, `testAuthEndpoints`) are safe today. Nothing guards against someone adding one and getting a silent intermittent race. A one-line comment on each swap helper is the cheap fix.

### Declared non-applies -- confirmed

- **F16's write-body-marker half: correctly declared, genuinely not applied.** `specialBodyContent` (`roundtrip_test.go:166`) still carries exactly one entry (`estimates/send`). The multipart and binary halves of F16 *are* applied and assert real content. Q6/Q7 are the same gap seen from the other side; fixing them via the inventory subsumes this.
- **F22's `mcp/CHANGELOG.md`: now applied** by `46a5bc4`. Added-before-Fixed verified in all three module changelogs.
- **F7's spec callout: applied** by `46a5bc4` at spec section 7 line 226. It covers all seven required points plus an eighth (context-name restriction). Accurate against the code.

### Fix verification F1-F30

All 30 verified. F1, F2, F3, F4, F5, F6, F8, F9, F11, F12, F13, F15, F18, F19, F24, F25, F26, F27, F30 verified black-box against the built binary (evidence in sections 3-6). F10, F14, F17, F20, F21, F22, F23, F28, F29 verified by source reading plus the passing named tests. F7 and F22's mcp half landed in `46a5bc4`. F16 is partial exactly as declared. Two items are only *partially* effective and are the blockers above: **F13 (registry only, not `api`) and F14 (unreachable on the dry-run and no-credential paths)**.

Repo-wide `t.Skip` sweep: exactly one hit, `freshbooks/live_test.go:26`, the documented `FRESHBOOKS_LIVE=1` opt-in suite. **No `t.Skip` in `cli/`** -- F21 fully landed, and on this machine (Linux, uid 1000) the converted assertions take the real branch, not the `t.Log` branch.

ASCII-only: `grep -rnP '[^\x00-\x7F]' docs/` returns nothing. F23's em-dash cleanup landed.

---

## Recommendation

Fix **Q1** and **Q2** -- both are a few lines, in `state.go` and `api_cmd.go`, plus no doc change needed once the code matches the table. Then re-run the gate and merge.

Of the advisories, **Q3, Q5, Q6+Q7** are the ones worth folding into the same commit: Q3 is the F17 class recurring, and Q6+Q7 together convert the round trip's central assertion from "the CLI still does what it did" into "the CLI does what FreshBooks documents", using an oracle already sitting in the tree. The rest are documentation polish and are reasonable Phase 5 backlog.

---

# Re-verification -- round 2 (`45a1d7c`, fix commit `de81644`)

**Verdict: NEEDS WORK** -- one new BLOCKING finding (R1). Everything the round-2 fix order set out to do (G1-G6) landed and is verified below; R1 is the half of G1's own stated scope that did not.

## 1. Gate on the current clean tree

`git status --porcelain` empty before the run. `mise run check` -> **exit 0**.

```
coverage-gate: freshbooks/coverage.out total = 91.8% (floor 90%)  PASS
coverage-gate: mcp/coverage.out        total = 91.9% (floor 90%)  PASS
coverage-gate: cli/coverage.out        total = 91.6% (floor 90%)  PASS
   cli per-package: auth 88.7%  cmd 87.7%  config 85.2%  output 93.3%
No vulnerabilities found (all three modules)
inventory-check: implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0
check.sh: all OK
```

`git status --porcelain` empty after; **`git diff --stat go.work.sum` empty** -- `go.work.sum` unchanged.

The gate again served cached test results, so I forced an uncached race run, which is the evidence that counts:

```
mise exec -- go test -C <mod> -race -count=1 ./...
freshbooks EXIT=0   mcp EXIT=0   cli EXIT=0
```

`TestRoundTrip` re-run verbosely: **168/168 subtests PASS**.

## 2. Q1 and Q2 re-probed black-box against the rebuilt binary

**Q2 -- FIXED, completely.** Exit 2 with the registry path's identical message on every path, and no body fragment anywhere. I seeded the malformed body with the marker `SECRETFRAGMENT` and grepped every captured stream for it:

```
clients create -f bad.json                      -> exit 2  "--file does not contain valid JSON"
api POST ... -f bad.json  (no credentials)      -> exit 2  same message
api POST ... -f bad.json  (credentials present) -> exit 2  same message
api POST ... -f -         (stdin)               -> exit 2  same message
grep -c SECRETFRAGMENT <all api error output>   -> 0
```

**Q1 -- FIXED for the `--log-level` flag on all three paths.**

```
clients list --log-level bogus  (credentials) -> exit 2
clients list --log-level bogus  --dry-run     -> exit 2  "invalid --log-level \"bogus\": want debug, info, warn, or error"
clients list --log-level bogus  (no creds)    -> exit 2  same message
```

The G1 mechanism is real: `state.go:260` hoists `s.buildLogger(cmd)` above both the dry-run branch and `credentialStore`/`store.Load`.

## 3. R1 (NEW, BLOCKING) -- `FRESHBOOKS_LOG_LEVEL` is inert; three places on this branch say otherwise

G1's own text, and the code comment at `cli/internal/cmd/state.go:214`, both claim an unrecognized `--log-level`**/`FRESHBOOKS_LOG_LEVEL`** is a usage error. The env half has never worked -- not the validation, the variable itself.

```
clients list --log-level bogus --dry-run     -> exit 2   (flag: validated)
FRESHBOOKS_LOG_LEVEL=bogus clients list --dry-run       -> exit 0   (env: silently ignored)
FRESHBOOKS_LOG_LEVEL=bogus clients list  (credentials)  -> exit 0
FRESHBOOKS_LOG_LEVEL=bogus clients list  (no creds)     -> exit 3
```

Not merely unvalidated -- entirely ignored. Decisive proof:

```
--log-level debug          -> 2 'level=DEBUG' lines on stderr
FRESHBOOKS_LOG_LEVEL=debug -> 0 'level=DEBUG' lines on stderr
```

**Root cause** (`cli/internal/cmd/root.go:70`): `flags.String("log-level", "warn", ...)` registers a **non-empty default**. `resolveLogLevel` (`state.go:207-210`) then calls `config.Resolve(flagVal, os.Getenv("FRESHBOOKS_LOG_LEVEL"), "", "warn")`, and `Resolve` (`config/config.go:140-142`) returns `flag` whenever it is non-empty -- which it always is. The env branch is unreachable.

This breaks the file's own stated convention. The comment directly above `registerGlobalFlags` (`root.go:55-57`) says defaults are deliberately omitted "so a flag left unset is distinguishable from one set to its zero value", and `--log-level` is the **only** string flag in that function with a non-empty default.

**Scope: exactly one of the nine env vars.** I audited all nine. The other seven `config.Resolve` sites (`state.go:41,65,89,97,109,142,201`) back flags registered with `""`, and `resolveTimeout` (`state.go:182-195`) correctly uses `cmd.Flags().Changed("timeout")` -- the right pattern for a flag that does have a non-empty default. `FRESHBOOKS_TIMEOUT` therefore works; `FRESHBOOKS_LOG_LEVEL` alone does not.

**Why blocking, and how small the fix is.** `docs/cli.md:69` ships an env-var table row promising this variable resolves `--log-level`; it resolves nothing. It is a one-line fix -- either register `--log-level` with a `""` default (the `def` slot in `Resolve` is already `"warn"`, so behaviour is preserved) or adopt `resolveTimeout`'s `Changed()` pattern -- in a file `de81644` already touched, plus a one-line test.

If the lead prefers to ship rather than re-gate, downgrading R1 to Phase 5 backlog is defensible on impact alone (log level is diagnostic-only) **provided this branch still corrects the three statements that are currently false**: the `docs/cli.md:69` table row, the `state.go:214` comment, and G1's claim in `fix.md`. Shipping the code as-is with the docs as-is is the one option I would not sign off.

Note this is a defect my round-1 docs audit missed: it verified each documented env var is *read* by the code, not that the read has any effect. Worth carrying into Phase 5's docs pass as a check shape.

## 4. G3-G6 confirmed

| Item | Evidence | Verdict |
|---|---|---|
| **G3** (Q3) | `config_cmd_test.go:118` -- `if got := strings.TrimSpace(stdout.String()); got != "[]"`. Live: `config contexts -o json` on an empty config prints exactly `[]` | DONE |
| **G3** (Q5) | `coverage_gap_test.go:84` -- asserts `stderr` contains both `"parsing"` and `"default.json"`, closing the gap I demonstrated (corrupt file and connection-refused both exit 1; only the message separates them) | DONE |
| **G4** (Q6, Q7) | `cli/internal/cmd/inventory_match_test.go`, called at `roundtrip_test.go:708` for all 168 commands -- see section 5 | DONE |
| **G5** (Q10) | `status.go:22-36` -- six snake_case tags. Live: `auth status -o json` emits `"context"`, `"credentials_path"`, `"logged_in"` | DONE |
| **G5** (Q11) | Live: `auth status --context '../evil'` -> exit **2** (was 1); `clients list --context 'a/b'` -> exit **2** on the registry path too | DONE |
| **G6** (Q13) | `docs/cli.md:67` -- `FRESHBOOKS_OUTPUT` row now states it does not apply to the two Binary commands | DONE |
| **G6** (Q18) | `docs/cli.md:68` -- `FRESHBOOKS_BASE_URL` row now states `--base-url` is hidden from `--help` | DONE |
| **G6** (Q14) | `~/.config/freshbooks` fallback now in `docs/cli.md:28` (First login), `:66` (`FRESHBOOKS_CONFIG` row), and `root.go:64`'s own `--config` help text | DONE |
| **G6** (Q16) | `errors.go:46-56` -- the false "a filesystem failure reading `--file`" line is gone, replaced with real examples and an explicit note that all three `--file` read sites return `usageError` | DONE |

## 5. `inventory_match_test.go` -- does it really check method AND path?

Yes, for every keyed command, against the vendor collection rather than the implementation.

`assertInventoryMatch` (`inventory_match_test.go:129-152`) loads `freshbooks/internal/inventory/testdata/inventory.json`, resolves `c.Keys[0]` to the vendor record, then asserts **both**:

- `req.method != entry.Method` -> `t.Errorf` (line 145)
- `req.path != resolveInventoryPath(c, entry.PathTemplate)` -> `t.Errorf` (line 149)

It is called at `roundtrip_test.go:708`, inside the per-command subtest, alongside the existing assertions. Coverage: 168 commands, minus `identity whoami` (no key, returns early at line 133), minus one allowlisted -> **166 commands cross-checked against the vendor's own method and path**.

Placeholder substitution (`resolveInventoryPath`, lines 76-100) handles `{accountId}`/`{businessId}`/`{businessUuid}` from the test scope and maps every other placeholder to the positional id, with a named special case for `service-rates update-project-rate` (two ids). An unrecognised placeholder is left **unsubstituted**, so it fails loudly rather than passing silently. It matches both `{curly}` and the collection's one `<angle_bracket>` holdout via `inventoryPlaceholderRE` (line 68).

**Non-vacuity is proven by the allowlist's existence**: the one genuine divergence in the tree had to be explicitly allowlisted, which is only necessary if the assertion actually fires.

**Three spot-checks, computed by me from `inventory.json` against live-recorded requests:**

| command | `Keys[0]` | vendor | observed |
|---|---|---|---|
| `clients get` | `Clients/Single Client` | `GET /accounting/account/{accountId}/users/clients/{customerId}` | `GET /accounting/account/ACM000TEST/users/clients/123` |
| `invoices create` | `Invoices/Create Invoice with Expense` | `POST /accounting/account/{accountId}/invoices/invoices` | `POST /accounting/account/ACM000TEST/invoices/invoices` |
| `time-entries list` | `Time Tracking/List Entries` | `GET /timetracking/business/{businessId}/time_entries` | `GET /timetracking/business/9000001/time_entries` |

All three match on method and on path after substitution.

**`projects/delete` allowlist -- reasoning checked against `freshbooks/projects.go`, and it holds.** The vendor record is `DELETE /comments/business/{businessId}/project/{projectId}`, `family: "internal"`, while every sibling `Projects/*` entry is `"business"`. The implementation sends `DELETE /projects/business/9000001/project/123` (live-confirmed). `ProjectsService.Delete`'s doc comment (`freshbooks/projects.go:195-211`, written in Phase 2, long before this fix) already records the decision, and gives a stronger reason than the allowlist text does: the Postman request is sourced from `my.freshbooks.com`, FreshBooks' internal host, **and its own captured response is a 404**, so it is not usable evidence for what a delete does. "The docs win" per CLAUDE.md; unconfirmed live either way.

The allowlist is the right call, logged via `t.Logf` at `roundtrip_test.go:708` rather than hidden -- I saw the line in the verbose run. Two small notes, neither blocking: it suppresses the **method** check as well as the path check, though only the path diverges; and the entry is keyed by command, so a future re-wiring of `projects delete` would go unchecked by this oracle (the `wantPath` assertion still covers it).

## 6. Final state

`git status --porcelain` -> only `?? docs/phases/4/reports/qa.md` (this updated report; the committed version is at `a0c320d`). All probe binaries, fixture servers, scratch credentials, and recorder logs deleted; no probe process left running; nothing written inside the repo.

## Final verdict

**NEEDS WORK** -- on R1 alone. G1-G6 are otherwise complete and correct, Q2 is fully resolved, and G4 delivered exactly the independent oracle Q6/Q7 asked for. R1 is a one-line code change plus a one-line test; alternatively, correct the three false statements and carry the behaviour to Phase 5.
