# Phase 4 (CLI) -- fix report

Branch `phase-4/cli`, applying `docs/phases/4/triage.md`'s F1-F30 fix list against the previous implementer's uncommitted partial work (which had reached F9). This report covers the whole pass across seven checkpoint commits, ending with a green `mise run check` on a clean tree.

## Per-item status

- **F1** (callback state validation) -- done (found already applied in the uncommitted WIP; verified `finishExchange`'s `requireState`, `subtle.ConstantTimeCompare`, and the single-use listener's 410 response). `cli/internal/auth/login.go`.
- **F2** (context-name path traversal) -- done (found already applied). `ValidContextName`/`CredentialsPath` in `cli/internal/auth/paths.go`. Added the missing test coverage this pass: `TestValidContextName` and `TestCredentialsPathRejectsInvalidNames` against the exact five inputs the triage names (`../evil`, `a/b`, `.`, `..`, `""`) -- `cli/internal/auth/paths_test.go`.
- **F3** (--dry-run rejected on auth/config) -- done (found already applied). `rejectDryRun` in `cli/internal/cmd/auth_cmd.go:41`, mirrored in `config_cmd.go`. Added the missing tests this pass: `auth login|status|logout|token --dry-run` and `config view|set-context --dry-run` all exit 2; `logout --dry-run` additionally asserts the credentials file survives -- `cli/internal/cmd/auth_cmd_test.go`, `config_cmd_test.go`.
- **F4** (-q/--quiet routing) -- done (found already applied). `state.go:159`'s quiet-aware writer, used for login/logout/context confirmations.
- **F5** (destructive Short suffix) -- done (found already applied). `registry.go:233` `destructiveSuffix`.
- **F6** (--sort) -- done (found already applied). `HasSort` on all 21 qualifying List commands, `Invocation.SortOpt()`. Added the round-trip `--sort` assertion this pass (`roundtrip_test.go`'s `assertProbesInQuery`).
- **F7** (spec section 7 callout) -- not applicable to this repo's docs tree as scoped; the callout target is `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` section 7, which is the lead's stage-4 job per the triage's own text ("`docs/progress.md` is the lead's stage-4 job; leave it"). No spec edit made.
- **F8** (auth subcommand exit codes) -- done (found already applied). `classifyRunError`/`exitCodeFor` in `errors.go`; `auth token|status|logout` route through it. Fixed one stale test expectation (`TestAuthStatusLogoutToken`'s "no stored credentials" case still asserted exit 1) and added the missing `--refresh` against a 401 token endpoint case.
- **F9** (--include on more commands) -- **applied this pass**. Decoupled `HasInclude` from `List` in `registry.go`'s `registerFlags`/`execute`; added `Invocation.IncludeOpt()`; wired `HasInclude` + `inv.IncludeOpt()` onto `clients get`, `estimates get`, `expense-categories get`, `expenses get`, `invoice-profiles get/create`, `invoices get/create/update`, `taxes get` (every single-resource get/create/update whose lib method takes `opts ...RequestOption`). Round-trip assertion added.
- **F10** (TestOpenBrowserAttempts forking a browser) -- done (found already applied). `browserCommand(goos, url)` pure function, table-tested; `openBrowser` reaps the child via `go func(){ cmd.Wait() }()`.
- **F11** (--yes/TTY gate untested) -- **applied this pass**. New `cli/internal/cmd/tty_test.go`: `TestYesGateTTY` (D-class + TTY + no --yes -> 2; + --yes -> 0; non-TTY + no --yes -> 0), `TestOutputDefaultTTY` (table on TTY, json otherwise), `TestErrorIsJSONTTY` (JSON error object on non-TTY, plain line on TTY).
- **F12** (round trip never asserts the endpoint path) -- **applied this pass**. Captured the real path (with `{account}`/`{business}`/`{uuid}`/`{id}` placeholders) for all 168 commands via a one-off capture harness, then hand-built a `wantPath` map + `assertWantPath` in `roundtrip_test.go`; added `origHost` capture (`redirectTransport.setPendingHost`) and a `wantHost` map asserting `payment-options fb-pay-tokenize`/`stripe-tokenize` hit `paid.freshbooks.com`; `assertScopeInPath` now has a `default:` that fails on an unhandled `ScopeFamily` (previously silently passed).
- **F13** (validate body/required flags before buildClient) -- done (found already applied: `json.Valid` check, `RequiredFlags`/`RequiredInt64Flags` loop in `registry.go`, both before `buildClient`). Added the two missing tests: malformed `--file` and a missing required extra flag, both with **zero** stored credentials, both exit 2 not 3.
- **F14** (log-level validation) -- **applied this pass**. `buildLogger` now returns `(*slog.Logger, error)`, rejecting an unrecognized `--log-level`/`FRESHBOOKS_LOG_LEVEL` as a usage error instead of silently defaulting to warn. `TestBuildLogger` now asserts the resolved `slog.Level` via `Enabled()`, not just non-nil; added the invalid-level case.
- **F15** (logout revokes both tokens) -- done (found already applied: `status.go`'s `Logout` revokes `RefreshToken` then `AccessToken`). Added the missing assertion: the fake OAuth server's `/revoke` handler now records the `token` form value, and the test asserts both fixture tokens were posted.
- **F16** (write-body/upload/binary assertions, method-check exemption) -- **partially applied**. Removed the `!c.Binary` exemption from `assertMethodMatchesAnnotation` (verified both binary commands issue GET, so the exemption was a no-op hiding a gap). Added `assertUploadBody` (parses the multipart body, asserts filename + content) and `assertBinaryOutput` (exact fixture bytes reach `-o -`) to the round-trip suite. **Not applied**: "extend `specialBodyContent` with one distinguishing body per resource family" -- this needed a valid, non-empty JSON body for each of ~27 distinct Write*Request Go structs (`DisallowUnknownFields` means any wrong field fails decode), which was a large, error-prone lookup task against the remaining budget for the other 20 items. Flagging rather than silently skipping, per the no-silent-substitution rule.
- **F17** (five tests passing with their target deleted) -- **applied this pass**. `config view` on a missing file now asserts stdout is exactly `"{}"`; `--all` mid-walk error now asserts stdout stayed empty; the dry-run "never retries" test now counts occurrences of the printed request line (must be 1, not 3); two file-existence checks (`TestSetContextExplicitConfigFlag`, `config.Save`'s mode test) now read the file back and assert its content.
- **F18** (binary -o overwrite/TTY guard) -- **applied this pass**. `writeBinaryResult` refuses an existing file without `--force` and refuses `-o -` on a TTY stdout; `--force` flag added to Binary commands; local `-o` help string documents the shadowing. Tests added/updated in `misc_test.go`.
- **F19** (env-var table + precedence tests) -- **applied this pass**. Rewrote the incorrect blanket precedence sentence in `docsgen.go`'s header, added the nine-`FRESHBOOKS_*`-var table plus the client-id/secret note; new `cli/internal/cmd/env_precedence_test.go` covers `FRESHBOOKS_CONTEXT`/`ACCOUNT_ID`/`BUSINESS_ID`/`OUTPUT`/`CONFIG` end to end (flag > env > file, empty env unset).
- **F20** (six untested branches) -- **applied this pass**. `Store.Save` failure on both `Login` and `LoginNoBrowser`; `net.Listen` port-in-use; an empty pasted line (`code == ""`); missing `--file` on both a Body and an Upload command; invalid `--output` value; credentials file 0600 mode after `auth login`.
- **F21** (six t.Skip guards) -- **applied this pass**. All six converted to conditional assertions (`t.Logf` on windows/root, real assertion otherwise) in `cli/internal/config/config_test.go` (5) and `cli/internal/cmd/coverage_gap_test.go` (1); verified running as uid 1000 (non-root) so the real assertions executed, not just the log branch.
- **F22** (changelog edits) -- **applied this pass**. `freshbooks/CHANGELOG.md`'s json-tag entry moved from Fixed to a new Changed section, restoring Added-before-Fixed order. `cli/CHANGELOG.md`'s Added section now names the list flags, `--sort`, `--include`, `-f/--file`, `--force`, and the destructive suffix. **Not applied**: `mcp/CHANGELOG.md` also has Fixed-before-Added, matching the triage's "in every module" wording, but the delegation prompt explicitly says not to touch `mcp/**` -- left alone, flagging the conflict rather than picking one instruction silently.
- **F23** (stale strings) -- **applied this pass**. `api_cmd.go`'s error text and `api_cmd_test.go`'s test names drop the never-real `-q/` prefix on `--query`; `mise.toml`'s docs task description no longer says "stub until Phase 4"; `writeTable` derives columns from the union of every row's keys (order of first appearance), not just row 0's, with a new corner-case test; replaced the 22 em dashes in `docs/phases/4/reports/implementer.md`. The `login.go:38` `--callback-port` comment and `errors.go`'s `FlagErrorFunc` comment were already correct in the uncommitted WIP -- no change needed.
- **F24** (control-character stripping) -- **applied this pass**. `stripControlChars` in `output.go`, applied to the string branch of `cellValue` (table/name only; json/yaml untouched). `[corner]` tests with ESC and TAB in a fixture value, for both table and name output.
- **F25** (redact client_secret) -- **applied this pass**. `identity applications` zeroes `ClientSecret` unless `--show-secrets`; `create-application`/`update-application` keep printing it (their `Short` says so); docs security notes name all three and warn about `ps`/shell-history exposure of `--client-secret`.
- **F26** (Cache-Control/Referrer-Policy) -- done (found already applied in `runCallbackServer`). Added the missing test asserting both headers on the actual callback response (caught and fixed a data race in this new test during the gate re-run -- see below).
- **F27** (login-timeout message) -- done (found already applied: the message already names the ::1/127.0.0.1 gap and suggests `--no-browser`). Strengthened the existing timeout test to assert those specifics, not just "timed out".
- **F28** (simplification items #1-#8) -- **applied this pass** for #1 (`reportOptions[O]` generic helper, -70 lines in `commands_reports.go`, verified byte-identical via the dry-run sweep below), #2 (`joinAll` -> `slices.Concat`), #4 (dropped the two redundant cobra init calls in `docsgen.go`), #5 (deleted the duplicate `TestSortedKeys`), #6 (`setupCredentials` now calls `writeCredentials`). #3 (`credentialStore`), #7 (`NoPaging: false`), and OPTIONAL #8 (`Invocation.RequiredString`) were already applied in the uncommitted WIP -- verified present, left alone.
- **F29** (redaction sweep) -- **applied this pass**. New `TestRedactionSweep`: five scenarios (`config view`, `auth status`, a `--dry-run`, an API 500, a 401), each run with `--log-level debug`, asserting the fixture access token, refresh token, and client secret never appear in stdout or stderr.
- **F30** (identity applications redaction test) -- **applied this pass** (folded into F25's work): `TestIdentityApplicationsRedaction` covers json and table output redacting by default and `--show-secrets` including the secret.

## What was NOT applied, and why (no silent substitutions)

1. **F16's "one distinguishing body per resource family."** Scoped down to what was tractable in the remaining budget: the multipart-upload and binary-output halves of F16 are fully applied and tested; the write-body distinguishing-marker half is not. Doing it properly means picking a real, already-valid field on each of ~27 distinct `Write*Request`/`CreateRequest` Go structs (since `DecodeBody` uses `DisallowUnknownFields`, an arbitrary marker field would break decode for structs that don't have it) -- a per-struct lookup task, not a mechanical one.
2. **F22's "every module."** `mcp/CHANGELOG.md` has the same Fixed-before-Added ordering issue named in F22, but the delegation prompt's explicit constraint ("do not touch ... `mcp/**`") overrides the triage's "every module" wording. `freshbooks/CHANGELOG.md` and `cli/CHANGELOG.md` are fixed; `mcp/CHANGELOG.md` is untouched.
3. **F7's spec callout** was not added to `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` -- the triage itself says the equivalent `docs/progress.md` update is the lead's stage-4 job, and by extension the section-7 spec callout (outside `cli/`, outside this delegation's file set) was left for the same owner.

## A bug the re-gate caught: a data race in the F26 test

The first full `mise run check` run failed `-race` on the new F26 test (`cli/internal/auth/login_test.go`): the browser-goroutine's write of `respHeader = resp.Header` had no happens-before relationship with the main goroutine's read after `Login()` returned, because `Login()` unblocks when the callback **server** handler (a different goroutine from the client-side `Get()` call) sends to `resultCh`. Fixed with a `done` channel the client goroutine closes, waited on before reading `respHeader`. Re-ran `-race -count=5` clean before re-running the full gate.

## mise run check (final, clean tree)

```
== fmt-check: freshbooks == / == vet: freshbooks == / == lint: freshbooks == 0 issues.
== test: freshbooks == coverage: 91.5% / 93.8% / 92.2%
== cover: freshbooks == coverage-gate: 91.8% (floor 90%) PASS
== vuln: freshbooks == No vulnerabilities found.
== inventory-check: freshbooks == implemented 213, ignored 0, todo 0, uncovered 0, double-covered 0, stale 0, unknown 0
== fmt-check: mcp == / == vet: mcp == / == lint: mcp == 0 issues.
== test: mcp == coverage: 78.1% / 97.1% / 91.5% / 94.6%
== cover: mcp == coverage-gate: 91.9% (floor 90%) PASS
== vuln: mcp == No vulnerabilities found.
== fmt-check: cli == / == vet: cli == / == lint: cli == 0 issues.
== test: cli == coverage: 88.7% (auth) / 87.4% (cmd) / 85.2% (config) / 93.3% (output)
== cover: cli == coverage-gate: 91.3% (floor 90%) PASS
== vuln: cli == No vulnerabilities found.
== actionlint == (clean)
== build == all 12 cross-compile targets (mcp x6, cli x6) built OK
check.sh: all OK
```

(One prior run failed only on `govulncheck`'s transient network fetch of `vuln.go.dev` -- retried clean, not a code issue.)

## git log --oneline main..phase-4/cli

```
43c46db test(cli): add the missing F1-F3 evidence tests the QA re-gate expects
4304a45 fix(cli): synchronize the F26 header-capture test against a data race
1f6853b docs(cli): regenerate docs/cli.md for the review-gate fixes
a3f8a65 fix(cli): apply the review-gate findings (F28)
e546801 fix(cli): apply the review-gate findings (F29)
5eb92b1 fix(cli): apply the review-gate findings (F24-F27, F30)
8aca6c5 fix(cli): apply the review-gate findings (F22-F23)
7a74646 fix(cli): apply the review-gate findings (F21)
e944ee6 fix(cli): apply the review-gate findings (F20)
fe31e6b fix(cli): apply the review-gate findings (F18-F19)
f1291d9 fix(cli): apply the review-gate findings (F13-F17)
2a14c3e fix(cli): apply the review-gate findings (F1-F12)
8375699 docs(phase-4): add the lane reports and the review-gate triage
... (12 earlier phase-4 commits, unchanged)
```

## git status --porcelain

Empty (clean tree). `go.work.sum` was not modified by the gate run.

## Simplification lane's dry-run sweep (before/after, must be empty)

Ran against `git worktree add /tmp/freshbooks-before 8375699` (the true starting point, before any uncommitted WIP) as "before" and the final tree as "after":

```
cd cli
for v in accounts-aging balance-sheet profit-loss trial-balance sales-tax-summary; do
  mise exec -- go run ./cmd/freshbooks reports "$v" --account ACM000TEST --dry-run
done
```

`diff /tmp/reports-before.txt /tmp/reports-after.txt` -- **empty**. The `reportOptions[O]` refactor (F28 #1) is behavior-preserving.

The docs-generation before/after diff (`freshbooks docs`) was **not** run as a byte-identity check, per the triage's own note: "The help tree and `docs/cli.md` WILL change in this fix ... so a byte-identical check is not the criterion this time." `TestDocsUpToDate` (the actual criterion) is green against the regenerated file instead.
