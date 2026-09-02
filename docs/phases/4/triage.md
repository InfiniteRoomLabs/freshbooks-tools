# Phase 4 triage (CLI) -- lead decisions, 2026-09-02

Inputs: `docs/phases/4/reports/{implementer,code-review,simplify,security}.md`. Verdicts: code review REQUEST CHANGES (10 blocking, 15 advisory), simplification 7 APPLY-RECOMMENDED / 5 OPTIONAL / 7 DO-NOT-APPLY, security BLOCK (3 blocking, 9 advisory). Two lanes converged on the same top blocker (a callback with no `state` skips CSRF validation) and on the unvalidated context name; no lane-vs-lane conflicts.

One fix commit: `fix(cli): apply the review-gate findings` (it may touch the spec, `docs/cli.md`, changelogs, and the implementer report; the D8/D9 folds are untouched). Then re-gate (QA lane), merge `--no-ff`.

## Verification handoff

The help tree and `docs/cli.md` WILL change in this fix (new `--sort`, `--include` on four more commands, destructive markers in `Short`, `--force`), so a byte-identical check is not the criterion this time. Instead: `mise run docs` regenerates `docs/cli.md` and `TestDocsUpToDate` must pass on the result; the simplification lane's dry-run sweep of five report commands (`docs/phases/4/reports/simplify.md`, top) must be byte-identical before and after, because nothing in this order changes report behaviour.

## Fix list (numbered; the implementer applies exactly these)

### Blocking

- **F1** (security B1, review B1, review A13) -- `finishExchange` accepts a listener callback with no `state`. Add `requireState bool` (true from `Login`, false only for the bare-code paste branch); reject an empty state when required; compare with `subtle.ConstantTimeCompare`; make the listener strictly single-use (any request after the first gets a 410 and is not exchanged). Tests: `[sad]` callback with `code` and no `state` -> error, nothing saved; second callback request rejected.
- **F2** (security B2, review A7) -- context name reaches `filepath.Join` unvalidated. `CredentialsPath` rejects empty, `.`, `..`, and anything not matching `^[A-Za-z0-9._-]+$`; `config set-context`/`use-context` and `--context`/`FRESHBOOKS_CONTEXT`/`current-context` all flow through it. Tests for `../evil`, `a/b`, `.`, `..`, `""`.
- **F3** (security B3) -- `--dry-run` is silently ignored by `auth` and `config` commands, including `auth logout` which really revokes. `PersistentPreRunE` on the `auth` and `config` parents returns a usage error (exit 2) when `--dry-run` is set; `docs/cli.md` automation notes say so; test that `auth logout --dry-run` exits 2 and leaves the credentials file.
- **F4** (review B2) -- `-q/--quiet` is a no-op. Route the confirmation lines (`Login succeeded.`, `Logged out.`, `use-context`/`set-context` confirmations) through a writer that is silent under `--quiet`. The login URL prompt and the dry-run request dump are results, not chatter: they stay. Test both states.
- **F5** (review B3) -- the docs point at a "D annotation" that is never rendered. In `buildCobra`, `ClassD` commands get `Short += " (destructive: requires --yes on a TTY)"`; regenerate `docs/cli.md`; the header sentence names that suffix.
- **F6** (review B4) -- `--sort field[:asc|desc]` is a spec-7 requirement, not a heuristic. `HasSort bool` on `Command`, set on every `List` whose lib method takes `extra ...RequestOption` (21 per the review), `Invocation.SortOpt()` built from `freshbooks.Sort`, appended to the variadic. Flag help carries the caveat: "direction encoding for business-scoped resources is unconfirmed against the live API (see docs/progress.md)". Round-trip assertion: `--sort name:desc` reaches the query.
- **F7** (review B5) -- no spec callout was added. One `> **STATE AS OF 2026-09-02**` callout in spec section 7 covering: `systems get` takes no positional id (both `--account` and `--business`); `api --query` has no `-q` shorthand (global `-q/--quiet` owns it); `--sort` direction caveat; report filters come from `-f file|-` JSON rather than per-report flags; boolean global flags (`--no-headers`, `--dry-run`, `--yes`) have no env twin and only the scope flags and `--context` consult `config.yaml` (review A9 -- document the real chain rather than widen it); `--yes` is required only when stdin is a TTY. `docs/progress.md` is the lead's stage-4 job; leave it.
- **F8** (review B6) -- `auth token|status|logout|login` wrap every error in `runtimeError` (exit 1). Use `classifyRunError`; map `auth.ErrNoToken` to an auth error (exit 3) in `cliauth.Token`/`Status`/`Logout` the way `state.go` does for registry commands. Tests: `auth token` with no credentials -> 3; `--refresh` against a 401 token endpoint -> 3.
- **F9** (review B7) -- `--include` is missing where the lib and the MCP have it: `invoices get|create|update`, `invoice-profiles create`, and the other `get` commands whose lib method takes `opts ...RequestOption` (mirror `mcp/internal/tools` -- clients, estimates, expense-categories, expenses, invoice-profiles, invoices, taxes). Decouple `HasInclude` from `List` in `registerFlags`. Round-trip assertion: `--include lines` reaches the query on `invoices get`.
- **F10** (review B8, security A9) -- `TestOpenBrowserAttempts` forks a real browser. Extract `browserCommand(goos, url) (name string, args []string)`, table-test it per GOOS, never call `Start` in tests; in `openBrowser`, reap the child with `go func() { _ = cmd.Wait() }()`.
- **F11** (review B9) -- the `--yes` gate and the TTY defaults have zero tests despite the documented seams. Swap `stdinIsTerminal`/`stdoutIsTerminal` in table tests: D-class + TTY + no `--yes` -> exit 2; D-class + TTY + `--yes` -> 0; non-TTY + no `--yes` -> 0; `-o` default is `table` on a TTY and `json` otherwise; `errorIsJSON` likewise.
- **F12** (review B10) -- the round trip never asserts the endpoint path. Add `WantPath` per command (a template with `{account}`, `{business}`, `{uuid}`, `{id}` placeholders) declared in the test table (not in the registry), assert it for all 168; `assertScopeInPath` gets a `default:` that fails on an unhandled family; the two tokenization commands assert the `paid.freshbooks.com` host through the redirecting transport.

### Advisory, apply in the same commit

- **F13** (review A1) -- validate the JSON body (`json.Valid`) and the required extra flags before `buildClient`, so a bad body on a machine with no credentials exits 2, not 3. Test.
- **F14** (review A2) -- `TestBuildLogger` asserts the resolved `slog.Level`; add the `"warn"` case; unknown `--log-level` is a usage error.
- **F15** (review A3, security A2) -- `auth logout` revokes BOTH the refresh token and the access token (best effort, before delete; delete stays unconditional); the fake revoke handler records what was posted and the test asserts it.
- **F16** (review A4, A5) -- write bodies: extend `specialBodyContent` with one distinguishing body per resource family and assert the body carries it; uploads: assert the multipart body carries `probe upload content` and the base filename; binary: assert the fixture bytes reach `-o -`; remove the `!c.Binary` exemption from the method check.
- **F17** (review A6) -- fix the five tests that pass with their target deleted: `config view` on a missing file asserts the printed empty config; `--all` mid-walk error asserts that page 1 was NOT printed; dry-run "never retries" asserts the attempt count; the two file-existence tests read the file.
- **F18** (review A8, security A5) -- binary `-o <file>`: refuse to overwrite an existing file without `--force`; refuse `-` when stdout is a TTY ("binary output would corrupt your terminal"); the local `-o` usage string says it shadows the global output format on this command; one sentence in the docs header.
- **F19** (review A9, A10) -- `docs/cli.md` header gets an env-var table (the nine `FRESHBOOKS_*` vars actually read) and a corrected precedence sentence; add precedence tests for `FRESHBOOKS_CONTEXT`, `FRESHBOOKS_ACCOUNT_ID`, `FRESHBOOKS_BUSINESS_ID`, `FRESHBOOKS_OUTPUT`, `FRESHBOOKS_CONFIG` (flag beats env, env beats file, empty env is unset).
- **F20** (review A11) -- cover: `Store.Save` failure on both login paths, `net.Listen` failure (port in use), `code == ""`, `--file is required`, `invalid --output "bogus"` -> 2; stat the credentials file mode after `auth login` (0600).
- **F21** (review A12) -- the six `t.Skip`s: convert to conditional assertions (`if runtime.GOOS == "windows" || os.Geteuid() == 0 { t.Log(...) } else { assert mode }`) so the rest of each test still runs.
- **F22** (review A14) -- `freshbooks/CHANGELOG.md`: the json-tag change moves under `### Changed` (it changes marshaled output); `### Added` before `### Fixed` in every module for consistency; `cli/CHANGELOG.md` names the list flags, `-f/--file`, `--sort`, `--include`, `--force`, and the destructive marker.
- **F23** (review A15) -- stale strings: `api_cmd.go` error text and test name drop `-q/`; `login.go:38` comment says `--callback-port`; `errors.go:93` comment drops `SetFlagErrorFunc`; `mise.toml` docs task description; `docs/cli.md` exit-code row 2 lists the two missing cases; `writeTable` derives columns from the union of all rows' keys (order of first appearance); replace the 22 em dashes in `docs/phases/4/reports/implementer.md` with `--`.
- **F24** (security A1) -- `cellValue` strips C0 control characters and DEL from strings in `table` and `name` output (json/yaml untouched); `[corner]` test with ESC and TAB in a fixture value.
- **F25** (security A3, A4) -- `identity applications` redacts `client_secret` unless `--show-secrets`; `create-application`/`update-application` keep printing it (that is the only time it is shown) and their `Short` says so; docs security notes name the three commands and warn that `--client-secret` on the command line is visible to `ps` and shell history (prefer the env vars).
- **F26** (security A8) -- callback responses send `Cache-Control: no-store` and `Referrer-Policy: no-referrer`.
- **F27** (security A7) -- on the login timeout, the error message mentions that `localhost` may resolve to `::1` while the listener is on `127.0.0.1` and suggests `--no-browser`. Dual-stack binding is backlog.
- **F28** (simplify APPLY-RECOMMENDED #1-#7 and OPTIONAL #8) -- generic `reportOptions[O]`; `slices.Concat` for `joinAll`; `credentialStore` prologue helper; drop the two redundant cobra init calls in `docsgen.go`; delete the duplicate `TestSortedKeys` in `cmd`; `setupCredentials` uses `writeCredentials`; drop `NoPaging: false`; `Invocation.RequiredString`.
- **F29** (simplify out-of-lane flag; plan Deliverables) -- the promised redaction test does not exist. Add one: capture stdout, stderr, and the debug log for `config view`, `auth status`, a `--dry-run`, an API 500, and a 401, and assert the fixture access token, refresh token, and client secret never appear.
- **F30** -- one test that `identity applications` output (json and table) does not contain the fixture `client_secret` without `--show-secrets` and does with it.

### Considered and NOT applied (do not re-derive)

- Security A6 (`cobra/doc` + go-md2man + blackfriday linked into the release binary via the hidden `docs` command): recorded as a known cost; moving it behind a build tag would take the drift test out of the default gate. Revisit in Phase 5 with the goreleaser pass (a `docsgen` tool binary is the likely shape).
- Security A7 dual-stack loopback binding: backlog (F27 covers the message).
- Review A9's implied widening (env twins for boolean flags, `config.yaml` for every global flag): documented honestly in F7/F19 instead; nobody asked for `FRESHBOOKS_YES`.
- Simplify OPTIONAL #9 (login prologue/epilogue helpers): the login flow is being edited under F1/F15/F20/F26 already; a structural refactor on top of security fixes in one commit is the wrong trade. Backlog.
- Simplify OPTIONAL #10-#12 and DO-NOT-APPLY #13-#19: agreed as written.
- Business-family `Sort()` direction: still deferred to a live-conformance pass (F6 documents the caveat on the flag).

## Re-gate

QA lane (opus) after the fix commit: full gate on a clean tree (`mise run vuln` covers govulncheck for all three modules -- the security lane could not run it), the four mandatory acceptance probes from `GOAL.md` stage 3, `TestDocsUpToDate` green after regeneration, the report dry-run sweep byte-identical, and a re-run of the F1/F2/F3 evidence with the exact attack inputs the lanes described.

## Round 2 (after QA, 2026-09-02)

QA verdict NEEDS WORK: 2 blocking (Q1, Q2), 20 advisory. Second fix commit `fix(cli): apply the QA findings`, then QA re-verifies the gate and the two probes.

- **G1** (QA Q1) -- validate `--log-level` before the dry-run and credential branches in `buildClient`, so `--log-level bogus` exits 2 on every path (dry-run, no credentials, credentials). Tests for all three.
- **G2** (QA Q2) -- `api` validates the `-f` body with `json.Valid` before `buildClient`, same message and exit 2 as the registry path; never echo body content in the error. Test.
- **G3** (QA Q3, Q5) -- `contexts on an empty config` asserts the printed output; `TestBuildClient_CorruptCredentials` asserts the stderr message names the credentials parse failure.
- **G4** (QA Q6, Q7) -- the round trip resolves each command's first inventory key against `freshbooks/internal/inventory/testdata/inventory.json` and asserts the recorded method equals the vendor `method` and the recorded path matches the vendor `pathTemplate` (placeholders substituted), for every command that carries a key; `identity whoami` keeps its hand-written expectation. This subsumes F16's write-body half.
- **G5** (QA Q10, Q11) -- `StatusInfo` gets snake_case `json` tags; an invalid `--context` is a usage error (exit 2) like every other bad flag value.
- **G6** (QA Q13, Q14, Q16, Q18) -- docs/help corrections: the env table notes `FRESHBOOKS_OUTPUT` does not apply to the two binary commands and that `--base-url` is hidden; mention the `~/.config` fallback in the docs and the `--config` help; fix the `runtimeError` comment.
- Not applied now (Phase 5 backlog): Q4, Q8, Q9, Q12, Q15, Q17, Q20, Q21, Q22, Q19 (root changelog is the lead's ship step).
