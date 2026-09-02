# Phase 4 review gate -- simplification lane

Branch `phase-4/cli` (`git diff main...phase-4/cli`, f557d2d..523f51e). Scope: `cli/` (`internal/cmd`, `internal/auth`, `internal/config`, `internal/output`, `cmd/freshbooks`), `scripts/docs.sh`, the hand-written header of `docs/cli.md`. **Propose only** -- nothing here was applied, nothing was committed, no gate/test/build was run.

Every item is behaviour-preserving by construction: no command path, flag name or shorthand, env var, output byte, exit code, error string, file path or permission, generated `docs/cli.md` byte, or exported signature changes. Where the claim rests on a library detail I verified it in the module cache rather than assuming it (see "Evidence").

**Headline:** the registry is the right shape and it is already doing its job. 168 commands at ~7 lines of data plus a 1-3 line closure each is close to the floor for this design, and most of what scans as duplication across the 33 `commands_*.go` files is per-resource *data* (Short text, inventory keys, request types) that must stay distinct. There is one genuinely large cut (#1, the reports file), one that removes a real inconsistency with the sibling `mcp` module (#2), one real prologue duplication (#3), and a short tail of micros. This is a shorter report than Phase 3's and that is the honest outcome -- the implementer clearly read `docs/phases/3/reports/simplify.md` before starting, and it shows.

## A cheap way to verify any of these

Two commands, no credentials needed, both byte-exact:

```bash
cd cli
mise exec -- go run ./cmd/freshbooks docs /tmp/cli-before.md
# apply changes
mise exec -- go run ./cmd/freshbooks docs /tmp/cli-after.md
diff /tmp/cli-before.md /tmp/cli-after.md            # must be empty
```

`docs/cli.md` is the whole cobra tree -- every Use line, every flag, every Short, every SEE ALSO -- so this diff covers #2, #4, #7 and every flag-shape claim in one shot. It is also what `TestDocsUpToDate` already asserts against the committed file.

The report closures in #1 are not visible in the docs (they are runtime behaviour), so pair it with a dry-run sweep, which exercises exactly the nil-vs-`&opts` path #1 touches and needs no stored credentials (dry-run installs a static token source):

```bash
cd cli
for v in accounts-aging balance-sheet profit-loss trial-balance sales-tax-summary; do
  mise exec -- go run ./cmd/freshbooks reports "$v" --account ACM000TEST --dry-run
done > /tmp/reports-before.txt
# apply changes; re-run into /tmp/reports-after.txt
diff /tmp/reports-before.txt /tmp/reports-after.txt   # must be empty
```

---

## APPLY-RECOMMENDED

### 1. `cli/internal/cmd/commands_reports.go` -- twelve identical 12-line closures collapse to one generic helper

Twelve of the fourteen report commands have a byte-identical closure body apart from the options type and the method name: declare a zero options struct, `DecodeBodyOptional` it, propagate the error, then branch on `has` to call the lib method with either `nil` or `&opts`. Lines 28-38, 46-56, 64-74, 82-92, 110-120, 128-138, 146-156, 164-174, 182-192, 200-210, 218-228, 236-246.

Before (x12):
```go
Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
	var opts freshbooks.AccountsAgingOptions
	has, err := inv.DecodeBodyOptional(&opts)
	if err != nil {
		return nil, err
	}
	if !has {
		return c.Reports.AccountsAging(ctx, inv.Scope.AccountID, nil)
	}
	return c.Reports.AccountsAging(ctx, inv.Scope.AccountID, &opts)
},
```

After -- one helper (in `commands_reports.go` or `invocation.go`, either is fine):
```go
// reportOptions decodes a report command's optional -f/--file filter body
// into a fresh O, returning a nil *O when no body was supplied -- exactly
// the nil the hand-written `if !has { ... nil }` branch passed, so the lib
// method cannot tell the two forms apart.
func reportOptions[O any](inv *Invocation) (*O, error) {
	var opts O
	has, err := inv.DecodeBodyOptional(&opts)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return &opts, nil
}
```
and each closure becomes:
```go
Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
	opts, err := reportOptions[freshbooks.AccountsAgingOptions](inv)
	if err != nil {
		return nil, err
	}
	return c.Reports.AccountsAging(ctx, inv.Scope.AccountID, opts)
},
```

`download-invoice-details-csv` (line 100) and `time-entry-details` (line 254) are untouched -- neither takes an options body.

**Why behaviour-preserving.** The helper's return type is `*O`, so `opts` at the call site is a typed `*freshbooks.AccountsAgingOptions` that is either nil or points at the decoded struct. Passing a nil `*T` to a `*T` parameter is indistinguishable from passing the literal `nil` -- so this preserves the nil/non-nil distinction *without* relying on any claim about what the lib does with it. (For the record the lib's `(*Options).values()` methods are nil-safe and treat nil and a zero struct identically, but the refactor does not depend on that; see #15 for why I am not proposing to exploit it.) Error propagation is the same error value, unchanged. The parity test checks `Service`/`Method`/`Keys`/`Class`, none of which move.

`revive`'s only enabled rule is `exported`, and `reportOptions` is unexported, so no doc-comment requirement; `nilnil` is not enabled, so `return nil, nil` is not a lint finding (`.golangci.yml` enables errcheck, govet, staticcheck, revive, gosec, misspell only).

Net: 12 x 12 lines -> 12 x 6 lines plus a 12-line helper, about **-70 lines in one file**, no cross-file churn. **Risk: low.**

### 2. `cli/internal/cmd/registry.go:335-341` -- `joinAll` is `slices.Concat`, and the sibling module already knows that

```go
func joinAll(groups ...[]Command) []Command {
	var out []Command
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
```

`mcp/internal/tools/registry.go:224` already calls `slices.Concat(...)` for the identical 33-slice join -- this exact hand-rolled helper was Phase 3 simplification item #5, and it was applied there. The CLI reintroduced it. Delete `joinAll` and call:

```go
func buildRegistry() []Command {
	return sortedCommands(slices.Concat(
		attachmentsCommands, billPaymentsCommands, ...   // same 33 names, same order
	))
}
```

`slices.Concat` preserves order exactly, and `sortedCommands` sorts afterwards with `SliceStable` anyway, so `All`'s contents and index order are unchanged -- which matters because `TestRoundTrip`, `TestParityKeyCoverage` and `BuildTree` all iterate `All`. Both modules are on `go 1.26`. **-7 lines, and one fewer place where the two modules disagree with each other. Risk: low.**

### 3. `cli/internal/cmd/auth_cmd.go` (x4) + `state.go:219-227` -- five copies of the same credentials prologue

The same six lines open every `auth` subcommand and `buildClient`'s non-dry-run path: `auth_cmd.go:70-78` (login), `:114-122` (status), `:140-148` (logout), `:170-178` (token), `state.go:219-227` (buildClient).

Before (x5):
```go
ctxName, err := state.contextName(cmd)
if err != nil {
	return err
}
credPath, err := cliauth.CredentialsPath(ctxName)
if err != nil {
	return &runtimeError{err: err}
}
store := libauth.NewFileStore(credPath)
```

After -- one method on `runtimeState` (`state.go`):
```go
// credentialStore resolves the current context and opens its credentials
// FileStore: the prologue auth login|status|logout|token and buildClient
// all share (D5 -- one lib FileStore per context). The two error paths keep
// their existing shapes: contextName's error is already typed, and a
// CredentialsPath failure is a runtimeError (exit 1).
func (s *runtimeState) credentialStore(cmd *cobra.Command) (string, string, libauth.TokenStore, error) {
	ctxName, err := s.contextName(cmd)
	if err != nil {
		return "", "", nil, err
	}
	credPath, err := cliauth.CredentialsPath(ctxName)
	if err != nil {
		return "", "", nil, &runtimeError{err: err}
	}
	return ctxName, credPath, libauth.NewFileStore(credPath), nil
}
```
and each call site becomes:
```go
ctxName, credPath, store, err := state.credentialStore(cmd)
if err != nil {
	return err
}
```
(with `_` for the two of the three each site does not use -- `token` needs only `store`, `logout` needs `credPath` and `store`, `login` needs `credPath` and `store`, `status` and `buildClient` need all three).

**Why behaviour-preserving.** Identical call order, identical error values, identical wrapping (so exit codes are unchanged: a `contextName` failure keeps whatever type `loadConfig` gave it, a `CredentialsPath` failure stays exit 1). Returning `libauth.TokenStore` rather than the concrete `*libauth.FileStore` is safe because every one of the five consumers already takes the interface: `LoginOptions.Store`, `cliauth.Status`, `cliauth.Logout`, `cliauth.Token`, and `libauth.NewTokenSource(cfg, store)`. The interface is only ever returned on the success path, so there is no typed-nil hazard.

**-15 lines, and the D5 "one FileStore per context" rule stops being restated in five places. Risk: low.**

### 4. `cli/internal/cmd/docsgen.go:141-142` -- two calls cobra already makes

```go
c.DisableAutoGenTag = true
c.InitDefaultHelpCmd()      // <- redundant
c.InitDefaultHelpFlag()     // <- redundant
if err := doc.GenMarkdownCustom(c, buf, linkHandler); err != nil {
```

`GenMarkdownCustom` calls both itself, as its first two statements, before it renders anything (cobra v1.10.2, `doc/md_docs.go:57-59`). Both are idempotent, and nothing happens between them and the `GenMarkdownCustom` call, so `c`'s state when rendering begins -- and therefore the SEE ALSO child list and every subsequent `c.Commands()` walk -- is identical either way. Delete both lines; keep `DisableAutoGenTag` (that one is load-bearing and is *not* set by cobra).

**-2 lines. Byte-identical `docs/cli.md`. Risk: low** -- and it is the direct answer to the brief's "is there a simpler way to produce the same bytes?": the generation path is otherwise already minimal (`GenMarkdownTree` writes one file per command, so the single-file requirement genuinely needs the hand-rolled sorted walk).

### 5. `cli/internal/cmd/misc_test.go:192-207` -- `TestSortedKeys` is a duplicate of `output_test.go:242-258`

Two tests with the same name assert the same property of the same function, one from each side of the package boundary. The `cmd` copy's own comment concedes the point ("output.SortedKeys is exercised indirectly through `config contexts`; this drives it directly"), but `output_test.go:242` already drives it directly, in the package that owns it.

Delete `cli/internal/cmd/misc_test.go:192-207` and the now-unused `output` import if nothing else in that file needs it (it does not -- `output` appears only there in `misc_test.go`).

**No coverage effect:** `scripts/check.sh:41` runs `go test -race -coverprofile=coverage.out -covermode=atomic ./...` with no `-coverpkg`, so each package's profile counts only its own statements. `output.SortedKeys`'s coverage comes entirely from `output_test.go`; the `cmd` copy contributes nothing to either package's number. **-16 lines. Risk: low.**

### 6. `cli/internal/cmd/roundtrip_test.go:190-202` -- `setupCredentials` open-codes `writeCredentials`

`writeCredentials(t, dir, context, json)` (`auth_cmd_test.go:60-71`, same package) already does the MkdirAll-0700 / WriteFile-0600 pair that `setupCredentials` repeats inline, and `coverage_gap_test.go:64` already uses it.

Before:
```go
func setupCredentials(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	credDir := filepath.Join(dir, "freshbooks", "credentials")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tok := `{"access_token":"test-fixture-token","token_type":"Bearer"}`
	if err := os.WriteFile(filepath.Join(credDir, "default.json"), []byte(tok), 0o600); err != nil {
		t.Fatal(err)
	}
}
```
After:
```go
func setupCredentials(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	writeCredentials(t, dir, "default", `{"access_token":"test-fixture-token","token_type":"Bearer"}`)
}
```
Same directory, same file name, same modes, same content, same `t.Fatal` failure behaviour. Test-only. **-8 lines. Risk: low.**

### 7. `cli/internal/cmd/commands_journal.go:42` -- `NoPaging: false` is the zero value

```go
Class: ClassRO, Scope: ScopeAccount, List: true, NoPaging: false,
```
It is the only explicit zero-value field assignment in all 33 `commands_*.go` files (the real setting, `NoPaging: true`, is at `commands_retainers.go:20` and carries a three-line comment explaining why). Reading it as intentional emphasis is a trap: the next person may wonder what the other 167 commands are silently defaulting to. Drop `, NoPaging: false`. **Risk: low.** Micro.

---

## OPTIONAL

### 8. A `RequiredString` accessor for the five required-flag sites

`commands_callbacks.go:60-63`, `commands_projects.go:104-107` and `:121-124`, `commands_payment_options.go:58-61`, `commands_team_members.go:73-76` all do:
```go
verifier, _ := inv.Flags.GetString("verifier")
if verifier == "" {
	return nil, newUsageError("--verifier is required")
}
```
An `Invocation` accessor produces byte-identical messages:
```go
// RequiredString reads an ExtraFlags-registered string flag, rejecting an
// empty value as a usage error (exit 2).
func (inv *Invocation) RequiredString(name string) (string, error) {
	v, _ := inv.Flags.GetString(name)
	if v == "" {
		return "", newUsageErrorf("--%s is required", name)
	}
	return v, nil
}
```
(`commands_service_rates.go:51-58` has an int64 sibling for `--project-id` plus a string `--rate`; the string half can use this, the int64 half stays inline, so that site only half-collapses.)

**Why only OPTIONAL:** it saves roughly one line per site. The real value is that the `--<name> is required` message stops being retyped six times and cannot drift. Worth it if the lead likes the invariant; skip if the priority is a small diff. **Risk: low.**

### 9. `cli/internal/auth/login.go:151-237` -- `Login` and `LoginNoBrowser` share a prologue and an epilogue

Both open with the identical four statements (`o.config(fmt.Sprintf("https://localhost:%d/callback", o.port()))`, `NewState()`, `cfg.AuthCodeURL(state)`, two error returns -- lines 152-161 and 207-216) and close with the identical seven (`o.Store != nil` save, the wrapped save error, `"Login succeeded."` -- lines 193-199 and 230-236). Two small unexported helpers (`begin() (fbauth.Config, state, authURL, verifier string, err error)` and `persist(ctx, tok) error`) remove ~15 duplicated lines and, more usefully, make it structurally impossible for the two entry points to drift on the redirect URL or on whether they persist.

**Why only OPTIONAL:** this is security-relevant code that the security lane is reading in its current shape this same gate. Two reviewers reading two different shapes of the login flow is a worse trade than 15 lines. If it lands, land it after the gate, not inside it. **Risk: low mechanically, medium in review timing.**

### 10. `sort` -> `slices` in three files

`registry.go:347` (`sort.SliceStable` -> `slices.SortStableFunc`), `docsgen.go:154` (`sort.Slice` -> `slices.SortFunc`), `parity_test.go:197,222` (`sort.Strings` -> `slices.Sort`) and `:103,230` (`reflect.DeepEqual` on `[]string` -> `slices.Equal`; the `reflect` import stays, `TestParityAgainstClient` needs `reflect.TypeFor`). Exact-equivalent modern stdlib forms; in `docsgen.go` the sorted keys are sibling command names, which are unique, so the stable/unstable difference is unobservable. Phase 3 raised the same item (its #8) and it was not taken -- listing it for symmetry, not pressing it. **Risk: low.** Cosmetic.

### 11. Three test-local micros

- `parity_test.go:117` -- `func annotClass(c Class) string { return string(c) }` has one caller (line 106). Inline it as `string(c.Class)`.
- `roundtrip_test.go:257` -- `func isReadOnly(c Command) bool` has one caller (line 291). Inline it as `c.Class == ClassRO`.
- `auth_cmd_test.go:73` -- `fileExistsCLI` carries a `CLI` suffix to avoid a collision that does not exist: the other `fileExists` is in `cli/internal/auth` (`paths_test.go:13`), a different package. Rename to `fileExists` (2 call sites, lines 96 and 199).

All test-only, all trivially behaviour-preserving. **Risk: low.**

### 12. An upload prologue helper for the three `Upload` commands

`commands_attachments.go:18-22`, `commands_images.go:23-27` and `:38-42` share the identical five-line open/defer-close/error prologue. A `withUpload(inv, func(name string, r io.Reader) (any, error))` wrapper collapses each to two lines and puts the `defer f.Close()` in one place instead of three. Three sites is thin for a new abstraction, which is why it is optional -- but the "must not forget to close" property is worth centralizing if the surface ever grows. **Risk: low.**

---

## DO-NOT-APPLY

These are ideas I worked through and am recommending **against**, with the reasoning, so nobody has to re-derive them next phase.

### 13. A generic `decodeBody[T]` across all 58 required-body sites

The single biggest duplication by raw count -- 58 occurrences across 27 files of:
```go
var body freshbooks.ClientWriteRequest
if err := inv.DecodeBody(&body); err != nil {
	return nil, err
}
return c.Clients.Create(ctx, inv.Scope.AccountID, &body)
```
A `decodeBody[B any](inv) (*B, error)` helper turns four lines into three. **That is the whole win: one line per site, 58 lines, across 27 files, immediately before a merge gate.** It is not a clarity win -- arguably the opposite, since `var body freshbooks.ClientWriteRequest` on its own line names the request type exactly where a reader looks for it, and `decodeBody[freshbooks.ClientWriteRequest](inv)` buries it inside a call. This is structurally the same trade as Phase 3's item #7 (45 structs, ~190 lines, 20 files), which was rated OPTIONAL and, correctly, not taken. At a third the payoff and the same blast radius, it does not clear the bar. Leave it.

### 14. Merging `usageError` and `authError` into one code-carrying type

`errors.go:21-44` defines two structs that differ only in their `ExitCode()` return (2 vs 3). One `cliError{msg string; code int}` would remove ~15 lines. Against it: the two names are the D6 vocabulary, they read at every construction site (`newUsageErrorf` vs `newAuthErrorf` says which exit code you are choosing without looking anything up), and `misc_test.go:242,250-257` asserts on `*usageError` identity. Trading a self-documenting type name for 15 lines is a net loss. Leave it. (I also checked whether `classifyRunError`'s `*freshbooks.Error` branch, lines 70-73, is dead -- it is not: `freshbooks.Error` has `Error`, `Unwrap` and `RetryAfter` but no `ExitCode`, so it does not satisfy `exitCoder` and needs its own branch.)

### 15. Dropping the nil-vs-`&opts` distinction in the report closures

`(*AccountsAgingOptions).values()` and its eleven siblings are nil-safe and emit nothing for zero fields (`freshbooks/reports.go:97-105` and equivalents), so `AccountsAging(ctx, acct, &AccountsAgingOptions{})` and `AccountsAging(ctx, acct, nil)` produce the same query today. Collapsing the branch would shave another line per report on top of #1.

Do not. It converts a CLI-local refactor into a bet on twelve separate lib implementations staying nil-equivalent forever, across a module boundary, with nothing asserting it. #1 gets the same line saving while keeping the distinction airtight. If someone later wants this, it belongs in the lib as a documented contract, not in the CLI as an assumption.

### 16. Collapsing the seventeen `List`/`--all` closures

```go
opts := &freshbooks.ClientListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage(), Include: inv.Include()}
if inv.All() {
	return collectAll(c.Clients.All(ctx, inv.Scope.AccountID, opts))
}
return c.Clients.List(ctx, inv.Scope.AccountID, opts)
```
Seventeen near-copies. But the options *type* differs per resource, the field set differs (some have `Include`, retainers has neither `Page` nor `PerPage`), the scope argument differs by family, and `All` returns an iterator whose element type differs. A generic that covered all of that would need three or four type parameters and a pair of function values per call site -- longer and harder to read than what is there. `collectAll` (`registry.go:374`) already factors out the one part that genuinely is shared. Correctly left alone by the implementer; leaving it alone.

### 17. The `term.go` / `isTerminalIO` / `stdoutIsTerminal` three-layer stack

`state.go:352-363` -> `term.go:10-12` looks like two hops too many for one `term.IsTerminal` call. Each layer earns its place: the package vars are the documented test seam (swap them without a pty), `isTerminalIO` owns the `any` -> `*os.File` type assertion, and `term.go` isolates the `golang.org/x/term` import to one file (which is where a `//go:build` constraint would go if a platform ever needed one). `misc_test.go:16-33` tests both layers deliberately. Leave it.

### 18. The `authEndpoints()` one-line wrapper

`auth_cmd.go:52` is `func authEndpoints() libauth.Endpoints { return testAuthEndpoints }` with three call sites that could read the var directly. The wrapper is what makes "this is a test-only seam, do not set it from a flag" visible at each use, next to a nine-line comment saying so. Deleting it saves one line and costs that signal. Leave it.

### 19. Rewriting the `docs/cli.md` generation path

Answering the brief's question directly: no simpler path produces the same bytes. `doc.GenMarkdownTree` emits one file per command, and the requirement is one file; that forces the manual depth-first walk in `renderDocsTree`. The sorted-children filter, `DisableAutoGenTag`, and the anchor `linkHandler` are each load-bearing for idempotency or for in-page links. `scripts/docs.sh` is seven substantive lines and does nothing a shorter script could. The only cut available is #4's two redundant lines.

---

## Deliberately left alone (and why)

- **The 33 `commands_*.go` files' overall shape.** The per-command `Group`/`Verb`/`Short`/`Service`/`Method`/`Keys`/`Class`/`Scope` block is data, not duplication -- every field differs per row and the parity test reads all of them.
- **All `// inventory:` keys and the `Keys` slices.** Out of scope per brief and per the parity contract.
- **Every comment carrying a decision reference or a lib-signature discrepancy** -- e.g. `commands_services.go:9-12`, `commands_gateways.go:11-16`, `commands_ledger_accounts.go:9-14`, `commands_systems.go:9-19` (including its `STATE AS OF` callout), `commands_reports.go:11-20`, `state.go:257-263` (`testTransport`), `state.go:305-311` (why dry-run needs `NoRetry`). These are the opposite of noise: each one is the answer to "why does this not match `commands.md`?" and deleting them would cost a re-investigation. I found no comment in the diff that merely restates its code.
- **`root.go:167-181` `scanStringFlag`.** It looks like it is re-implementing pflag, and it is -- deliberately, for the one path (top-level error printing) that runs after `Execute()` failed and cannot trust cobra's parse state. The comment says exactly that and `misc_test.go:152-172` table-tests it. Correct as written.
- **`output.go` `rows`/`decodeFields`/`orderedKeys`/`cellValue`.** Each has a distinct job and `cellValue`'s "render from the raw literal, never through float64" is a deliberate precision decision (`output.go:319-322`). No cut here that is not a regression.
- **`config.go` `writeAtomic`.** Temp file + chmod + write + sync + rename with a named-return close is the correct shape; the `defer os.Remove` plus rename is not redundancy.
- **The round-trip and parity suites vs their MCP counterparts.** Same shape by design, separate modules, no shared code possible (`cli` does not import `mcp`). The assertions differ where the surfaces differ (cobra argv vs MCP JSON input). Nothing to merge.
- **`registry.go:172-202` `registerFlags` and `:221-313` `execute`.** These are the generic shapes the brief asked whether the `commands_*.go` files should be pushing more into -- and they already absorb everything that is genuinely uniform (paging, search, include, all, body, upload, binary, the `--yes` gate, id parsing, error classification). What is left in the closures is the one lib call each command exists to make. That is the correct floor.

## Outside my lane, flagged so it does not fall through the cracks

Two things I noticed while reading that are **not** simplifications and are **not** my proposals -- code-review / QA calls:

- `docs/phases/4/plan.md`'s Deliverables list names a `--sort field[:asc|desc]` flag "where the lib `List` takes `...RequestOption`" and a **Redaction** test section. Neither exists in the branch (`grep '"sort"'` in `cli/internal/cmd` finds only the `sort` package import; no test in `cli/` asserts that a token never appears in output). The implementer report may account for both -- worth a check against it.
- `api_cmd.go:32` and `:82` say `-q/--query` in their error text, but the flag is registered at `:65` as `--query` with no shorthand, deliberately (the comment at `:63-64` explains that global `-q/--quiet` owns it). The error text contradicts the flag it names.

## Evidence

- `cobra v1.10.2 doc/md_docs.go:57-59` (module cache) -- `GenMarkdownCustom` calls `InitDefaultHelpCmd()` and `InitDefaultHelpFlag()` as its first two statements, before rendering. Basis for #4.
- `mcp/internal/tools/registry.go:223-224` -- `slices.Concat` in the sibling module. Basis for #2.
- `scripts/check.sh:41` -- `go test -race -coverprofile=coverage.out -covermode=atomic ./...`, no `-coverpkg`. Basis for #5's no-coverage-change claim.
- `.golangci.yml` -- enabled linters are errcheck, govet, staticcheck, revive (rule `exported` only), gosec, misspell. Basis for #1's lint claims.
- `freshbooks/errors.go:55,85,104` -- `*Error` has `Error`, `Unwrap`, `RetryAfter`, no `ExitCode`. Basis for #14's dead-branch check.
- `freshbooks/reports.go:97-105` -- `(*AccountsAgingOptions).values()` is nil-safe. Context for #15 (and the reason #1 does not depend on it).
- `cli/go.mod:3`, `mcp/go.mod:3`, `freshbooks/go.mod:3` -- all `go 1.26`. Basis for #2 and #10.

## Summary

| # | Item | Files | Approx lines | Risk |
|---|---|---|---|---|
| 1 | reports: 12 closures -> one generic helper | 1 | -70 | low |
| 2 | `joinAll` -> `slices.Concat` | 1 | -7 | low |
| 3 | credentials prologue x5 -> one method | 2 | -15 | low |
| 4 | drop two cobra calls cobra makes | 1 | -2 | low |
| 5 | delete the duplicated `TestSortedKeys` | 1 | -16 | low |
| 6 | `setupCredentials` uses `writeCredentials` | 1 | -8 | low |
| 7 | drop `NoPaging: false` | 1 | -1 | low |

APPLY-RECOMMENDED total: **about -119 lines across 6 files**, all inside `cli/`, none touching a command path, flag, message, exit code, or generated doc byte. Verified end to end by the two commands at the top of this report.
