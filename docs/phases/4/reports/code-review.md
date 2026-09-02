# Phase 4 (CLI) -- code-review lane

Branch `phase-4/cli`, `git diff main...phase-4/cli` (12 commits `f557d2d..523f51e`, 89 files, +17544/-77). Read-only pass: no file modified, no commit, no `mise run check`/test/build run (QA owns the gate).

## Verdict: REQUEST CHANGES

10 BLOCKING, 15 ADVISORY. None of the blocking items is architectural -- the registry design is sound, the scope-family assignments are correct against the lib, and the disagreement table in the implementer report checks out row for row. What blocks is a security hole in the login callback, a documented-but-inert flag, docs that point at annotations that do not exist, three spec deltas shipped without the callout the work order requires, and a test suite with a structural blind spot plus one test that forks a browser on every run.

---

## BLOCKING

### B1. The loopback callback accepts a code with **no** `state` parameter, skipping CSRF validation entirely

`cli/internal/auth/login.go:246`

```go
if gotState != "" && gotState != wantState {
    return nil, errors.New("auth: state mismatch on the callback; not proceeding")
}
```

`finishExchange` is shared by both login paths. The `gotState != ""` guard exists for the `--no-browser` **bare-code paste** path, where there is genuinely nothing to compare against. But the browser listener path (`login.go:189`) goes through the same function, so a callback request that carries `code` and **omits `state` altogether** passes validation and is exchanged.

Failure scenario: while `auth login` is waiting (up to 5 minutes, `DefaultLoginTimeout`), any page the user visits can top-level-navigate to `https://localhost:8765/callback?code=<attacker_code>`. The CLI exchanges the attacker's code and `Store.Save`s the resulting token pair. The user is now logged into the **attacker's** FreshBooks account and every subsequent `freshbooks ... create` writes their data there. This is the exact attack `state` exists to prevent; the work order's D-list says "validates `state` (mismatch -> error, exit 1, listener closes)" -- a missing `state` is not a mismatch under this code.

Fix: make state validation mandatory on the listener path. Add a `requireState bool` parameter to `finishExchange` (true from `Login`, false only for the bare-code branch of `LoginNoBrowser`), and reject `gotState == ""` when it is set. Better still, hand `state` to `runCallbackServer` and reject at the handler before `once.Do`.

### B2. `-q/--quiet` is registered and documented but is a complete no-op

`cli/internal/cmd/root.go:66` registers it. `grep -rn '"quiet"' cli/ --include=*.go` returns exactly one hit -- that registration. Nothing anywhere calls `GetBool("quiet")`.

Meanwhile real chatter exists and is unconditional: `auth login`'s "Open this URL..." and "Login succeeded." (`cli/internal/auth/login.go:169,198,218,235`), `auth logout`'s "Logged out." (`cli/internal/cmd/auth_cmd.go:153`), `config use-context`/`set-context`'s confirmations (`config_cmd.go:91,126`), and the dry-run request dump (`state.go:297`). `docs/cli.md:53-54` tells the user "`-q/--quiet` suppresses non-result chatter (never errors)"; D7 says the same. It suppresses nothing.

`root.go:94-95`'s comment ("`-q/--quiet` silences non-error chatter, never errors (D6), so it has no effect here") is true of the error path and obscures that it has no effect anywhere.

Fix: thread the resolved `quiet` value into the writers above (route their `Fprintln`s through a helper that no-ops when set), or drop the flag and the doc sentence. Add a test either way.

### B3. `docs/cli.md` tells readers to look for a "D annotation" that does not exist anywhere in the file

`docs/cli.md:75-76` (source `cli/internal/cmd/docsgen.go:88`): "Destructive commands (the D-annotated ones in the command reference below) refuse to run without `--yes`..."

`Command.Class` (`registry.go:15-24,76-77`) is registry-internal. It is read at exactly one place, `registry.go:249` (`if c.Class == ClassD`), and is never written into the cobra command's `Short`, `Long`, or `Annotations`. Verified: `freshbooks bill-vendors delete` (`docs/cli.md:634+`) is structurally identical to `bill-vendors list`. The 220 occurrences of "destructive" in `docs/cli.md` are all the inherited `--yes` flag help line, which appears identically under all 202 command sections.

Net: **a reader cannot determine which 23 of the 168 commands need `--yes`.** The documentation actively misdirects them to a marker that was never emitted.

Fix: append the class to `Short` (or set `cc.Annotations["class"]` and have `docsgen.go` render it) for `ClassD` at minimum, then regenerate. Cheapest correct version: in `Command.buildCobra`, `if c.Class == ClassD { cc.Short += " (destructive: requires --yes)" }`.

### B4. `--sort field[:asc|desc]` was dropped from all 21 eligible list commands -- this is a spec gap, not a defensible cut

Spec section 7's command surface lists it explicitly: `freshbooks <resource> list [--all] [--page N] [--per-page N] [--search k=v]... [--include x]... [--sort field[:asc|desc]]`. The work order's Deliverables repeat it without hedging: "`--sort field[:asc|desc]` **where the lib `List` takes `...RequestOption`**" -- that is a *scoping condition* naming which commands get it, not a hedge on whether to implement it.

The implementer's stated justification is that `docs/phases/4/commands.md`'s flags column never names it. But `commands.md`'s own preamble calls that column "a heuristic guide," and the plan says the same. The more specific instruction (the Deliverables line) was not followed.

The capability is present and one line away: `freshbooks.Sort(field, dir)` exists (`freshbooks/types.go:281`) and **21 of 24** `List` methods take `extra ...RequestOption` (`bills, expense_categories, callbacks, bill_vendors, clients, credit_notes, retainers, items, invoices, estimates, payments, projects, invoice_profiles, taxes, expenses, team_members, other_income, services_svc, tasks, time_entries` plus `journal_entries`). `Invocation.ReqOpts()` (`invocation.go:237`) already demonstrates the exact pattern.

The known lib caveat (business-family sort direction unconfirmed, `docs/progress.md` backlog item 2) is an argument for a documented caveat on the flag, not for omitting it from 21 commands.

Fix: add `HasSort bool` to `Command`, register `--sort` in `registerFlags`, add `Invocation.SortOpt() []RequestOption`, and append it to the `extra` variadic in the 21 closures. **Or**, if the team elects to defer: that decision must be recorded as a `> **STATE AS OF 2026-09-01**` callout in spec section 7 and a `docs/progress.md` backlog item -- see B5, which is currently missing both.

### B5. Three spec deltas shipped with no `STATE AS OF` callout in spec section 7

The work order is explicit: "If the spec (section 7) is wrong about something cobra or the lib does, implement what works, **add a `> **STATE AS OF 2026-09-01**` callout in section 7 in the same commit**, and list it in your report."

`git diff --name-only main...phase-4/cli -- docs/superpowers/` is **empty**. The spec was not touched. Every prior phase added its callout (spec lines 69, 75, 157, 161, 197); Phase 4 added none. At least three deltas qualify:

1. `systems get` takes no positional id -- `--account` and `--business` together address the resource (`SystemsService.Get(ctx, AccountID, BusinessID)`). The implementer wrote the callout as a **code comment** in `commands_systems.go:17-19` and the report says "Left as a callout for whoever next revises section 7." That is the instruction not followed.
2. `api --query` has no `-q` shorthand (collides with the global `-q/--quiet`); spec section 7 shows `[-q k=v]...`.
3. `--sort` not implemented (B4).

Also missing: `docs/progress.md` is untouched by this branch, against `CLAUDE.md`'s "living status. Always read before starting work; **update at every phase boundary**."

Fix: one `docs(spec)` commit adding the section 7 callout covering all three, plus the `docs/progress.md` phase-boundary update.

### B6. `auth token` / `auth logout` force-wrap every error in `runtimeError`, breaking D6's exit-code mapping

`cli/internal/cmd/auth_cmd.go:181-183` (and `:125-126`, `:150-152`, `:94-96`):

```go
tok, err := cliauth.Token(cmd.Context(), cfg, store, refresh)
if err != nil {
    return &runtimeError{err: err}   // ExitCode() == 1, unconditionally
}
```

`exitCodeFor` (`errors.go:99-101`) does `errors.As(err, &ec)` first, which matches the **outermost** `runtimeError` and returns 1 before it ever inspects the wrapped `*freshbooks.Error`. Two D6 violations follow:

- `freshbooks auth token` with no stored credentials -> `store.Load` returns `ErrNoToken` -> exit **1**. D6: "3 auth (**no credentials for the context**, or the API answered 401)". The registry path gets this right (`state.go:234`, `newAuthErrorf`); the auth commands do not.
- `freshbooks auth token --refresh` against a revoked/expired refresh token -> the token endpoint's 401 -> exit **1**, not 3.

This matters for the primary automation idiom `TOKEN=$(freshbooks auth token) || handle $?`.

Fix: replace `&runtimeError{err: err}` with `classifyRunError(err)` in the four auth subcommands, and map `ErrNoToken` to `newAuthErrorf` in `cliauth.Token`/`Status`/`Logout` the way `state.go:232-235` already does.

### B7. `--include` is unreachable on `Invoices.Get/Create/Update` and `InvoiceProfiles.Create` -- a capability the MCP has and the lib documents

`cli/internal/cmd/commands_invoices.go:31-33`:

```go
Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
    return c.Invoices.Get(ctx, inv.Scope.AccountID, inv.IntID())   // no opts
},
```

The lib's own doc comment at `freshbooks/invoices.go:247` reads "Get fetches one invoice by id. **Pass `Include("presentation")`** for the..." and `invoices.go:183-188` documents that `Lines`, `Presentation`, and `AllowedGateways` are populated *only* when the corresponding `Include` is passed. The MCP exposes exactly this via `includeIn` (`mcp/internal/tools/shapes.go:57-71`) on `Invoices.{Get,Create,Update}` and `InvoiceProfiles.Create` (`tools_invoices.go:64,71,78`, `tools_invoice_profiles.go:65`).

So `freshbooks invoices get 123` **cannot retrieve an invoice's line items at all**, on a CLI whose headline resource is invoices. The `api` escape hatch is the only workaround.

The implementer's rationale ("`commands.md`'s flags column never calls this out on those rows either... left out to keep the write commands' shape simple") does not survive the MCP comparison -- the same four methods were wired in Phase 3.

Fix: `HasInclude` already exists on `Command`; decouple it from `List` in `registerFlags` (`registry.go:173-185`) so it can be set on non-list commands, and pass `freshbooks.Include(inv.Include()...)` into the four closures.

### B8. `TestOpenBrowserAttempts` asserts nothing and forks a real browser process on every test run

`cli/internal/auth/login_test.go:189-194`:

```go
func TestOpenBrowserAttempts(t *testing.T) {
	// openBrowser shells out to a platform program; this only proves it
	// builds and starts the right command for this GOOS without
	// requiring the program to actually exist or succeed.
	_ = openBrowser("https://example.invalid/probe")
}
```

`openBrowser` (`login.go:385-396`) ends in `cmd.Start()`. On Linux this actually forks `xdg-open https://example.invalid/probe` -- `go test ./cli/...` opens a browser tab on the developer's desktop and leaks the child (no `Wait()`). The result is `_ =` discarded; nothing checks `cmd.Path` or the argv, so the comment's claim ("proves it builds and starts the right command for this GOOS") is not what it tests. It cannot fail short of a panic. This is coverage padding with a real side effect.

Fix: extract the argv construction into a pure `browserCommand(goos, rawURL) (string, []string)` and table-test it per GOOS; delete the `Start()` call from the test path.

### B9. The `--yes` destructive gate has **zero** test coverage, with the injection seam sitting documented and unused

`registry.go:249-253` is the only thing between a mistyped command and 23 destructive endpoints. `state.go:347-355` says outright:

> "`stdoutIsTerminal` and `stdinIsTerminal` are package vars, not plain functions, so tests can inject a fixed answer without needing a real pty: TestMain-less unit tests exercise the `--yes` gate and the output format's TTY-sensitive default by swapping these out for the duration of one test."

`grep -rn "stdinIsTerminal\s*=\|stdoutIsTerminal\s*=" --include=*_test.go cli/` returns **nothing**. Neither var is ever assigned in any test. `roundtrip_test.go:251-253` always passes `--yes`, and every test's stdin is a `strings.Reader`, so `stdinIsTerminal` is always false and the guard never fires. **Deleting the entire `if` block breaks no test.**

The same unused seam leaves the TTY branches of `resolveOutputFormat` (`state.go:124`) and `errorIsJSON` (`root.go:157`) untested -- i.e. the documented "table on a TTY, json otherwise" default is asserted in neither direction at the CLI level.

Fix: three table tests swapping the vars -- D-class + TTY + no `--yes` -> exit 2; D-class + TTY + `--yes` -> exit 0; non-TTY + no `--yes` -> exit 0. Plus one each for the two format defaults.

### B10. `roundtrip_test.go` never asserts the endpoint path, leaving all 168 commands mutation-blind to cross-resource mis-wiring

`assertScopeInPath` (`roundtrip_test.go:262-284`) is the loop's only URL check, and it only does `strings.Contains(path, <scope id>)`. Combined with `assertMethodMatchesAnnotation` (`:289-300`, GET vs not-GET only), **any two commands in the same scope family and class are interchangeable as far as this suite is concerned.** A Run closure rewired from `c.Items.Delete` to `c.Taxes.Delete` passes: same account id in the path, still non-GET, still exit 0.

`parity_test.go` does not close this. It compares the *declared* `Service`/`Method` **strings** against `commands.md` and against reflection over `*freshbooks.Client`. Nothing ties a `Command`'s declaration to the method its closure actually invokes.

Two aggravating specifics:

- **14 commands get no URL assertion whatsoever.** `assertScopeInPath`'s `switch` has no `ScopeNone` case, so it is a silent no-op for `identity {me,whoami,register,add-business,create-application,applications,update-application}`, `images upload-without-account`, `ledger-accounts {types,sub-types,sub-type}`, `payment-options {fb-pay-tokenize,stripe-tokenize}`, `team-members invite`. The two tokenize commands hard-code the `paid.freshbooks.com` host and neither host nor path is ever checked.
- The only full-path assertion in the whole package covers **one** command: `dryrun_test.go:235`. The mechanism exists; it is applied to 1 of 168.

Fix: add `WantPath string` (a path template with `{account}`/`{business}`/`{id}` placeholders) to `Command`, or a `map[string]string` keyed by `group/verb` in the test, and assert it in the loop. Give `assertScopeInPath` a `default:` that fails loudly on an unhandled family.

---

## ADVISORY

### A1. Usage errors are reported as auth errors when not logged in (ordering)

`registry.go:295-300` calls `state.buildClient(cmd)` **before** `c.Run`, but `DecodeBody`'s JSON validation (`invocation.go:184-194`) and every `ExtraFlags` required-value check (e.g. `commands_callbacks.go:59-62`, `commands_service_rates.go:51-58`, `commands_projects.go:102-106`) live inside `Run`. So `freshbooks clients create -f bad.json` on a machine with no credentials exits **3** ("no credentials"), not the 2 D6 mandates for a bad JSON body -- and `freshbooks callbacks verify 5` (missing `--verifier`) does the same. The exit-code tests pass only because they set up credentials first. Cheap fix: `json.Valid(raw)` check in `execute` right after `readBodySource`, before `buildClient`.

### A2. `TestBuildLogger` is tautological, and `"warn"` has no switch case

`misc_test.go:90-107` asserts only `logger == nil` on a `slog.New(...)` return, which can never be nil. All five branches run purely for coverage; if `"debug"` mapped to `LevelError` the test still passes. Separately, `state.go:182-189`'s switch has no `case "warn"` -- it falls to the `LevelWarn` default alongside every *invalid* value, so a typo'd `--log-level=wrn` is silently accepted as warn. Assert the resolved `slog.Level`, and either add the `"warn"` case or reject unknown levels with a `usageError`.

### A3. `auth logout` "revoke-then-delete" only ever asserts "delete"

`status_test.go:105-129` is named "[happy] revokes the refresh token and removes the file" and asserts exactly one thing: `if fileExists(path)`. The fake `/revoke` handler (`login_test.go:108-110`) records nothing. Removing the `cfg.Revoke` call from `Logout` entirely leaves both this test and the CLI-level `auth_cmd_test.go:187-202` green. Two-line fix: record a bool in the handler, assert it, and assert the posted form carries the stored refresh token.

### A4. The write-body assertion is weaker than the standard Phase 3 shipped for the same surface

`roundtrip_test.go:383-389` only checks `len(req.body) != 0`, and every write command is fed the same literal `{}` (`:342-345`). Phase 3's fix lane shipped `assertBodyCarriesInput` (`mcp/internal/tools/roundtrip_test.go:423-446`), which fails unless the body contains one of the synthesized input's own string values. That did not survive the port. Genuinely harder here (the closures decode into typed structs, so a marker key is dropped before the wire), so the realistic fix is to extend the existing `specialBodyContent` map (`roundtrip_test.go:139-141`) with one distinguishing body per resource family.

**Credit:** the `bills archive` vs `bills delete` pin you asked about **is present, correct, and load-bearing** -- `roundtrip_test.go:111-120` documents the swap risk and the map (`archive -> "vis_state":2`, `delete -> "vis_state":1`) matches `freshbooks/types.go:212-214`, enforced at `:391-395`.

### A5. Upload and binary command payloads are driven but never asserted

`roundtrip_test.go:292` exempts `Binary` commands from the method check (`method != http.MethodGet && !c.Binary`) -- both are `ClassRO` GETs, so the exemption buys nothing and only weakens the check. `:397-405` skips the stdout check for `Binary`, and nothing asserts the fixture PDF/CSV bytes reached `-o -`. The 3 `Upload: true` commands declare `Body: false`, so the body check is skipped; `uploadFile` is written with the distinctive content `"probe upload content"` (`:346-349`) and nothing ever asserts it appears in the multipart body or that the filename is carried.

### A6. Tests named for behavior their bodies never check

All pass with the asserted target deleted: `config_cmd_test.go:165-172` ("view on a missing config file prints an empty config" -- asserts only `code != 0`, never reads stdout); `:174-181` (same shape); `coverage_gap_test.go:36-57` ("proves `--all` propagates a mid-walk page error instead of silently truncating" -- asserts only exit 1, so a version that printed page 1 *and* exited 1, i.e. exactly the truncation it disclaims, passes); `dryrun_test.go:261-272` ("never retries" -- asserts only exit 0, no attempt counter or time bound, despite the comment explaining a missing `NoRetry` "would make this test slow"); `coverage_gap_test.go:99-112` (asserts the file exists but never reads it).

### A7. `--context` flows unvalidated into a filesystem path

`cli/internal/auth/paths.go:119-125` does `filepath.Join(dir, context+".json")` with no validation. `--context ../../../../home/u/.ssh/id_rsa` makes `auth logout` call `os.Remove` on `~/.ssh/id_rsa.json`, and `auth login` write credentials outside the 0700 dir. Self-inflicted rather than remote, and constrained by the `.json` suffix, but it is a two-line fix: reject a context name containing `os.PathSeparator`, `/`, or `..`, or that is empty.

### A8. `-o json` on the two binary commands silently writes the payload to a file named `json`

`registry.go:197` gives `Binary` commands a local `-o/--output` that shadows the global format flag (spec-mandated: `freshbooks invoices pdf <id> -o out.pdf`). So `freshbooks -o json invoices pdf 5` writes PDF bytes to `./json`. Given `-o json` is the most-typed flag in this CLI, warn in the flag's usage string and in the `docs/cli.md` header.

### A9. The documented flag > env > file > default chain is wrong on both ends, and no env var is ever named in the docs

`docs/cli.md:43-46` (source `docsgen.go:56-59`) and `root.go:53` both claim every global flag has the full chain. Neither end holds: `--no-headers`, `--dry-run`, `--yes` have **no env twin at all**; `--output`, `--timeout`, `--log-level`, `--base-url`, `--config` pass `""` for the file argument (`state.go:41,122,153-160,167,175`), so only the scope flags and `--context` consult `config.yaml`. Separately, `grep -c 'FRESHBOOKS_' docs/cli.md` = 6, all of them `FRESHBOOKS_CLIENT_ID`/`_SECRET` inside generated `--client-id` help. The nine vars actually read (`_CONFIG, _CONTEXT, _ACCOUNT_ID, _BUSINESS_ID, _BUSINESS_UUID, _OUTPUT, _TIMEOUT, _LOG_LEVEL, _BASE_URL`) are undiscoverable from the docs. Add an env-var table to `docsHeader` and correct the precedence sentence.

### A10. Untested env-var precedence

Only `FRESHBOOKS_TIMEOUT` has a real resolution test (`misc_test.go:54-65`). `FRESHBOOKS_ACCOUNT_ID`/`_BUSINESS_UUID`/`_CLIENT_ID`/`_CLIENT_SECRET` appear only set to `""` to force absence. Untouched: `_OUTPUT, _BASE_URL, _BUSINESS_ID, _CONFIG, _CONTEXT, _LOG_LEVEL`. `FRESHBOOKS_CONTEXT` selects which credentials file is read -- that one is security-relevant.

### A11. Uncovered error branches that matter

`net.Listen` failure (`login.go:299-302`) -- the realistic failure, since port 8765 is fixed; `Login`'s `ctx.Done()` branch (`:185-186`); `finishExchange`'s `code == ""` branch (`:249-251`); **`Store.Save` failure on both login paths** (`:193-197`, `:230-234`) -- `Token`'s equivalent is covered by `brokenSaveStore`, `Login`'s is not, so "exchanged successfully then failed to persist" is an untested user-visible state; the `--file is required` usage errors (`registry.go:277,289`); `invalid --output "bogus"` -> exit 2 (`state.go:127-128`) has no test at any level. No test stats the credentials file's mode after `auth login` (the 0600 check lives in the lib's `FileStore`).

### A12. `t.Skip` without issue links

`config_test.go:70,73,88,126,129` and `coverage_gap_test.go:78`. All six are legitimate environment guards (windows / running as root), but `CLAUDE.md` says "no `t.Skip` without an issue link." Either add links or convert to `t.Log` + conditional assertion.

### A13. The loopback listener is not strictly single-use

`login.go:308-320` registers a handler that serves **every** request to `/callback`; `once.Do` guards only the channel send. A second (or attacker-replayed) request still renders "Login succeeded." The plan says "serving `/callback` exactly once." Cosmetic given B1's fix would reject on state, but worth a guard.

### A14. Changelog nits

`freshbooks/CHANGELOG.md:10-19` files the `json`-tag change under **Fixed**, but it changes the marshaled wire shape of `Page[T]`/`User`/`Membership` (`Items` -> `items`) -- breaking for any consumer marshaling them. `Changed` is the Keep-a-Changelog home for that. `freshbooks/` and `mcp/` put `### Fixed` above `### Added` while `cli/` does the reverse. `cli/CHANGELOG.md` never mentions the user-facing list flags (`--all`, `--page`/`--per-page`, `--search`, `--include`) or `-f/--file`.

### A15. Stale strings and encoding

- `api_cmd.go:32,82` -- runtime errors say `-q/--query`; the `-q` shorthand does not exist (a user who follows the message gets `--quiet`). Test name `api_cmd_test.go:13` is stale the same way.
- `login.go:38` -- doc comment says "when `--port` is not given"; the flag is `--callback-port`.
- `errors.go:93` -- cites "(via `SetFlagErrorFunc`)"; never called in `cli/`. The exit-2 outcome is still correct via the untyped fallthrough at `:114`.
- `mise.toml:45` -- docs task description still reads "(stub until Phase 4 adds cobra/doc)".
- `docs/phases/4/reports/implementer.md` -- 22 em dashes (U+2014) across 20 lines, against `CLAUDE.md`'s ASCII-only rule. Every other doc on the branch is clean.
- `docs/cli.md:62`'s exit-code row 2 omits two cases `errors.go:17-19` lists (unknown `--output` value; destructive without `--yes`).
- `writeTable` (`output.go:209`) derives columns from `items[0]` only. Harmless for lib types (`freshbooks/types.go` has zero `omitempty`), but for `freshbooks api` raw output, where the API does use `omitempty`, later rows' extra keys silently vanish.

---

## What checked out correctly (verified, not assumed)

- **Registry vs lib signatures.** Every judgment call in the implementer's disagreement table is accurate: `Gateways.Get(ctx, acct)` takes no id; `LedgerAccounts.{Types,SubTypes,SubType}`, `Images.UploadWithoutAccount`, `PaymentOptions.{FBPayTokenize,StripeTokenize}`, `TeamMembers.Invite`, and the seven scopeless `Identity` methods genuinely take no scope; `LedgerAccounts.List`/`Staff.List`/`ServiceRates.List` take no options. All mixed-family services are right: `Staff` (List business, Get/Update/Delete account), `Services` (Get/List business, Create/GetBillableItem account), `Reports.TimeEntryDetails` (business), `Systems.Get` (both). `ScopeAccountAndBusiness` is the correct call over faking a positional.
- **`retainers list` `NoPaging`.** I initially flagged this as silent truncation; it is correct. `RetainerListOptions.opts()` (`freshbooks/retainers.go:63-69`) hard-codes `listOpts(o.Search, 0, 0)` and the lib documents that the endpoint returns no `meta` block. The CLI faithfully mirrors a deliberate lib decision.
- **`auth token --refresh` ordering.** `status.go:87-94` refreshes, `Save`s, *then* returns the token. Correct, and the failure path is pinned by `status_test.go:251-262`'s `brokenSaveStore`.
- **Dry run.** `dryRunTransport` (`state.go:292-303`) prints method/URL/body and never a header, so `Authorization` structurally cannot leak; the static placeholder source plus `WithRetry(NoRetry)` (`:312-317`) means nothing refreshes and nothing repeats; `errDryRun` unwrapping via `errors.Is` survives the lib's `*url.Error` re-wrap.
- **Config.** Atomic temp+chmod+write+sync+rename in a 0700 dir (`config.go:91-134`), `Resolve` precedence with empty-env-as-unset, `use-context` unknown name -> exit 2, `set-context` using `Changed()` so an unpassed flag never clears a field. `config.File` structurally cannot hold a secret. The failure branches are genuinely tested (`config_test.go:113-140`).
- **Docs pipeline.** The header is a Go constant (`docsgen.go:12-105`), so `scripts/docs.sh`'s full-file overwrite regenerates it rather than clobbering it; `DisableAutoGenTag` plus sorted children make it idempotent; `docs_drift_test.go:37` compares **byte-for-byte** through `Run(["docs", out])` (not a bare root), so it picks up cobra's lazily-added `completion` subcommand and would catch both a header edit and an undocumented new command. An independent static parity check of all 168 `Group:`/`Verb:` pairs against the 202 `## freshbooks ...` headings in `docs/cli.md` showed **zero difference in both directions**.
- **Parity tests.** Bidirectional and real: reflection over `*freshbooks.Client` both ways, `commands.md` both ways, key-coverage union == 212 with one owner each and `identity whoami` the only keyless row, plus a check against the built cobra tree.
- **Exit-code tests.** `exitcodes_test.go` covers 0/1/2/3/4 and decodes the stderr JSON object field-by-field (though it omits `code` and never checks `family` -- `:143-175`).
- **D8/D9 folds.** Both correct and minimal. The `json` tags match the documented wire shape with no decode-path change (both types are hand-built from separate wire structs). `bearerToken`'s `EqualFold` rewrite (`server.go:99-104`) is correct including the short-header guard. `docs/mcp.md`'s added envelope sentence is accurate. No tool definition file was touched.
- **Go canon.** `main.go` is one statement; every package has a doc comment; every exported identifier is documented; dependencies are exactly the five allowed (cobra, pflag, yaml.v3, x/term, lib) plus `cobra/doc`'s transitive indirects.
- **Security notes in `docs/cli.md:87-92`** are accurate on every point: `auth token` is the only token printer, `StatusInfo` has no token field, the cert is in-process ECDSA P-256 with the right SANs and validity and is never written to disk, credentials are 0600 in a 0700 dir.
- **Redaction.** Fixture ids are synthetic (`ACM000TEST`, business `9000001`); no vault names, internal hosts, or real account ids anywhere on the branch.

---

## Suggested fix order

1. B1 (state validation) -- security, smallest diff, highest severity.
2. B6, B2, B3 (exit codes, `-q`, D annotation) -- three small behavioral/doc corrections.
3. B7 (`--include` on the four invoice methods).
4. B4 + B5 (`--sort` and the spec/progress callouts) -- decide implement-vs-defer, then land the spec callout either way.
5. B8, B9, B10 (test fixes) -- delete/rewrite the browser test, add the `--yes` and TTY tests via the existing seam, add path assertions to the round trip.
6. ADVISORY sweep, at the team's discretion.

---

## Tree-state note for QA (not a finding against the implementer)

At the end of this lane `git status --porcelain` shows:

```
 M go.work.sum
?? docs/phases/4/reports/code-review.md
?? docs/phases/4/reports/security.md
?? docs/phases/4/reports/simplify.md
```

The three `??` entries are the gate lanes' own reports (expected, uncommitted by design).

`go.work.sum` is **modified in the working tree**, dropping the three hashes commit `271acd9` added for the new `cobra/doc` indirects and substituting an unrelated `cloud.google.com/go/compute/metadata` line:

```
-github.com/cpuguy83/go-md2man/v2 v2.0.6 h1:...
-github.com/russross/blackfriday/v2 v2.1.0 h1:...
-go.yaml.in/yaml/v3 v3.0.4 h1:...
+cloud.google.com/go/compute/metadata v0.3.0/go.mod h1:...
```

This lane made no edits, so it is most likely an artifact of a `go` invocation during the parallel gate lanes rather than anything the implementer left behind -- the committed version on the branch is the correct one. I did not revert it (read-only lane). Before QA runs `mise run check`, restore it:

```
git checkout -- go.work.sum
```

If it comes back dirty after `mise run check`, that is a real finding and `go.work.sum` needs re-committing.
