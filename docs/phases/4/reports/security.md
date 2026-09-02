# Phase 4 security review -- `phase-4/cli`

Lane: security (read-only). Range: `git diff main...phase-4/cli` (f557d2d..523f51e).
Reviewer scope: spec sections 3, 7, 9.2, 10; `docs/phases/4/plan.md` D4/D5/D6; `docs/phases/3/reports/security.md` bug classes.

**Verdict: BLOCK** -- 3 blocking findings, 9 advisory. None are remote-exploit class; all three blockers are cheap, local fixes in the credential/consent path, which is exactly where this phase's trust boundary lives.

What was checked and passed is listed under "Clean" at the end -- most of the checklist is genuinely clean, and the token-rotation and dry-run cores are good.

---

## BLOCKING

### B1. The loopback callback accepts a callback with no `state` at all, skipping CSRF validation

`cli/internal/auth/login.go:246` (called from `:189`)

```go
if gotState != "" && gotState != wantState {
    return nil, errors.New("auth: state mismatch on the callback; not proceeding")
}
```

The `gotState != ""` guard exists for `LoginNoBrowser`'s bare-code paste path (`parsePastedInput` returns `state == ""` for a bare code, `login.go:262-268`), where there is genuinely nothing to compare. But `Login` -- the loopback listener path -- shares the same `finishExchange`, so a request to `https://127.0.0.1:8765/callback?code=ANYTHING` with **no** `state` parameter is accepted without any validation and goes straight to `cfg.Exchange`.

**Failure scenario.** During the 5-minute login window (`DefaultLoginTimeout`), any page the user's browser is on can issue a cross-origin navigation/image request to `https://localhost:8765/callback?code=x`. `runCallbackServer`'s `once.Do` (`login.go:319`) fires on that first request, so the real callback that arrives moments later is dropped, and `Login` proceeds to exchange the attacker's code. PKCE S256 is mandatory here (`freshbooks/auth/oauth.go:141-163`, always sends `code_challenge`), so an attacker-supplied code fails the exchange rather than binding the user's CLI to an attacker account -- PKCE is doing the work that `state` is supposed to be doing. The realized impact today is therefore a login denial-of-service plus the loss of the intended CSRF control; the code-injection impact returns the moment anyone switches `Endpoints` to a set that does not enforce PKCE (the spec still treats the two endpoint sets as recently-confirmed rather than settled).

The test suite confirms the gap is unintended: `login_test.go:243` covers a *wrong* state on the listener, `:363` covers a bare pasted code, but nothing covers a *missing* state on the listener.

**Fix.** Make the requirement explicit per path instead of inferring it from emptiness. Add a `requireState bool` to `finishExchange` (true from `Login`, false from `LoginNoBrowser`), and:

```go
if requireState && gotState == "" {
    return nil, errors.New("auth: the callback carried no state; not proceeding")
}
if gotState != wantState && (requireState || gotState != "") {
    return nil, errors.New("auth: state mismatch on the callback; not proceeding")
}
```

Add a `[sad]` test that GETs `/callback?code=x` with no `state` and asserts the error plus that nothing was saved.

*(Minor, same line: the comparison is a plain `!=`, not `subtle.ConstantTimeCompare`. State is a single-shot 256-bit random value compared once against a value the attacker is trying to guess, not a repeated oracle, so the timing channel is not practically exploitable. Switching to `subtle.ConstantTimeCompare` costs nothing if you are touching the line anyway; I am not blocking on it.)*

### B2. The context name is interpolated into a filesystem path with no validation

`cli/internal/auth/paths.go:24-30`, reached from `cli/internal/cmd/state.go:223` and `cli/internal/cmd/auth_cmd.go:74, 118, 144, 174`.

```go
func CredentialsPath(context string) (string, error) {
    dir, err := CredentialsDir()
    ...
    return filepath.Join(dir, context+".json"), nil
}
```

`filepath.Join` cleans the result, so a context of `../../../../home/<user>/.ssh/authorized_keys` resolves *outside* the credentials directory and `auth login` will `FileStore.Save` a token JSON over that path (0600, atomic rename -- so it succeeds and clobbers). An empty-ish name like `.` yields `..json`; a name with a `/` silently nests. `grep -rn "filepath.Clean\|validateContext" cli/` returns nothing: there is no validation anywhere.

**Why this is not purely self-inflicted.** The context name is not only a flag. `runtimeState.contextName` (`state.go:59-66`) resolves `--context` > `$FRESHBOOKS_CONTEXT` > **`config.yaml`'s `current-context`** > `"default"`, and `--config`/`$FRESHBOOKS_CONFIG` can point at any file (`state.go:40-48`). A `config.yaml` the operator did not author -- checked into a repo, shipped in a container image, dropped in a shared home -- can therefore choose the path that the next `auth login` writes to, and `auth logout` deletes (`status.go:97`, `os.Remove(credentialsPath)`). Attacker-chosen path, non-attacker-chosen content, but arbitrary-file-clobber and arbitrary-file-delete driven by a config file is a real primitive.

**Fix.** Validate once in `CredentialsPath` (and reject at `config set-context` for a good error message):

```go
var contextNameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func CredentialsPath(context string) (string, error) {
    if context == "" || context == "." || context == ".." || !contextNameRE.MatchString(context) {
        return "", fmt.Errorf("auth: invalid context name %q: use letters, digits, '.', '_', '-'", context)
    }
    ...
}
```

Tests: `[sad]` cases for `../evil`, `a/b`, `.`, `..`, `""`.

### B3. `--dry-run` is a persistent flag that the auth and config commands silently ignore, including a destructive one

`cli/internal/cmd/root.go:67` declares `--dry-run` on `root.PersistentFlags()`, so cobra accepts it on every subcommand. It is read in exactly one place -- `runtimeState.buildClient` (`state.go:208`), which only registry commands and `api` call (`grep -rn '"dry-run"' cli/` outside tests returns those two lines).

Consequences, all silent (no warning, no error, exit 0):

- `freshbooks auth logout --dry-run` **really revokes the refresh token and deletes the credentials file** (`auth_cmd.go:150` -> `status.go:91-101`). This is irreversible: the refresh token is dead server-side and the user must re-run the browser flow.
- `freshbooks auth token --dry-run` prints the live access token to stdout.
- `freshbooks auth token --refresh --dry-run` burns and rotates the one-time-use refresh token.
- `freshbooks config use-context --dry-run` / `set-context --dry-run` writes `config.yaml`.
- `freshbooks auth login --dry-run` opens a browser and completes a real authorization.

`docs/cli.md:72` says "Every registry command accepts `--dry-run`", which is true and also reads as permissive, not as "the other commands accept and ignore it". A flag whose contract is "send nothing" performing a destructive network call is a safety defect regardless of intent, and `--dry-run` is precisely the flag a cautious operator reaches for before `logout`.

**Fix (pick one, cheaply).** Either honour it on the non-registry commands (print what would happen, change nothing, exit 0), or reject it explicitly. The smallest correct change is a `PersistentPreRunE` on the `auth` and `config` parents:

```go
PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
    if dry, _ := cmd.Flags().GetBool("dry-run"); dry {
        return newUsageErrorf("%s does not support --dry-run", cmd.CommandPath())
    }
    return nil
},
```

Then say so in `docs/cli.md`'s "Automation notes". Add an exit-code test asserting `auth logout --dry-run` exits 2 and leaves the credentials file in place.

---

## ADVISORY

### A1. Table and `name` output write API strings to the terminal verbatim, escape sequences included

`cli/internal/output/output.go:328-332` (`cellValue`), used by `writeTable:236` and `writeName:264`.

`json.Unmarshal(raw, &str)` decodes a JSON \u001b escape back into a real ESC byte, and the result is written straight to stdout. `json`/`yaml` output is safe (both escape or quote control characters); **`table` is the default whenever stdout is a terminal** (`output.go:50-55`). Any field whose value a third party can influence -- a client name, an invoice note, a vendor's address, anything a customer types into a FreshBooks-hosted form -- can therefore carry OSC/CSI sequences that rewrite the terminal title, move the cursor, or hide text, and a tab or newline silently forges a column boundary or a row (a name of `Ada<TAB>ACTIVE` looks like two cells). For `-o name` a newline also breaks the "one identifier per line, pipe it to xargs" contract.

Fix: sanitize in `cellValue`'s string branch before returning -- drop or visibly escape C0 control characters and DEL, e.g. `strings.Map(func(r rune) rune { if r < 0x20 || r == 0x7f { return -1 }; return r }, str)`. Keep `json`/`yaml` untouched. One `[corner]` test with an ESC and a tab in a fixture value.

### A2. `auth logout` revokes only the refresh token; the access token stays live

`cli/internal/auth/status.go:91-95`. `Logout` calls `cfg.Revoke(ctx, tok.RefreshToken)` and then removes the file. The access token in the same file is never revoked, so a copy of it (a shell scrollback from `auth token`, a backup, a synced dotfile) keeps working until its natural expiry.

The revoke-then-delete *order* is correct -- do not flip it. Deleting first would mean a revoke failure leaves a live credential with nothing on disk to retry against. Making the delete unconditional on the revoke result is also right, for the same reason: a network-down logout must still clear local state.

Fix: revoke both, best-effort, before removing:

```go
if err == nil {
    if tok.RefreshToken != "" { _ = cfg.Revoke(ctx, tok.RefreshToken) }
    if tok.AccessToken  != "" { _ = cfg.Revoke(ctx, tok.AccessToken)  }
}
```

### A3. `--client-secret` on the command line lands in `ps` and shell history

`cli/internal/cmd/auth_cmd.go:29-33`. The flag is a deliberate D5 affordance and `resolveClientCredentials` prefers it over the env var, which is fine. But while the process runs, the secret is in `/proc/<pid>/cmdline`, readable by any process of the same user (and by root), and it is in `~/.bash_history` afterwards. The repo's own convention (`CLAUDE.md`: "never export them in a shell, never write them to a file") is stricter than what this flag encourages.

Fix: no code change required, but add a line to `docs/cli.md`'s "Security notes": prefer `FRESHBOOKS_CLIENT_ID`/`FRESHBOOKS_CLIENT_SECRET` (or a secret injector); the flags exist for one-off use and expose the secret to `ps` and shell history. Optionally mention it in the flag's own help string.

### A4. `identity applications` prints every application's `client_secret`

`freshbooks/settings.go:149-164` deliberately redacts `ClientSecret` in `Application.String()`, but the CLI's output path marshals via `json.Marshal` (`output.go:88`), which honours the `json:"client_secret,omitempty"` tag and prints the secret in full -- in `json`, `yaml`, and `table` alike, for `identity applications` (read-only, plural), `create-application`, and `update-application` (`cli/internal/cmd/commands_identity.go:98-134`).

For `create-application` this is correct and necessary: it is the only time the secret is shown. For the plural `applications` listing it is a leak-amplifier -- a routine read-only command dumps every registered app's secret into CI logs, terminal scrollback, and `-o json | tee`.

`docs/cli.md:88` currently claims "The access token is printed by exactly one command", which is true and also incomplete: a *client secret* is printed by three.

Fix: at minimum extend the "Security notes" section to name these three commands. Better: redact `client_secret` in the `applications` listing unless an explicit `--show-secrets` is passed, leaving `create-application` alone.

### A5. Binary output clobbers `-o <path>` silently and dumps raw bytes to a TTY

`cli/internal/cmd/state.go:330-345` (`writeBinaryResult`), for the two `Binary` commands (`commands_invoices.go:92`, `commands_reports.go:99`).

`os.WriteFile(path, b, 0o600)` truncates an existing file with no prompt and no `--force`, and the `-o` shorthand here shadows the global output-format flag on exactly these two commands -- so `freshbooks invoices share-link 5 -o json`, muscle-memory from every other command, silently creates a file named `json`. With the default `-` and a terminal on stdout, a PDF's raw bytes go to the tty.

Both are the operator's own filesystem, so this is usability-shaped rather than a privilege boundary -- but it is the kind of surprise worth closing. Fix: refuse to overwrite an existing file without `--force`, and refuse `-` when `stdoutIsTerminal(cmd.OutOrStdout())` with "binary output would corrupt your terminal; pass -o <file> or pipe" (`stdoutIsTerminal` is already available at `state.go:353`).

### A6. `cobra/doc` (go-md2man + blackfriday) is linked into the shipped release binary

`cli/internal/cmd/docsgen.go:8` imports `github.com/spf13/cobra/doc` from an ordinary (non-test, non-build-tagged) file in package `cmd`, which `cli/cmd/freshbooks` imports. So `github.com/cpuguy83/go-md2man/v2` and `github.com/russross/blackfriday/v2` (`cli/go.mod:14-16`, and the three new `go.work.sum` lines) ship in every released binary, reachable only from the hidden, developer-only `docs` command.

No known vulnerability in either at these versions, and the only input they ever see is the CLI's own command tree -- so this is size and future-CVE surface, not a live risk. Note it or shrink it: move `docsgen.go` + `docs_cmd.go` behind a `//go:build docs` tag, or into a `cli/cmd/docsgen` tool binary that `scripts/docs.sh` runs instead. Whichever way it lands, record the decision so a future dependency audit does not re-discover it.

The rest of the dependency delta is clean and within the CLAUDE.md allowance: cobra, pflag, `x/term`, `yaml.v3` direct; `mousetrap`, `go.yaml.in/yaml/v3`, `x/sys` indirect. The `golang.org/x/sys` skew (cli v0.47.0, mcp v0.41.0) is not an inconsistency -- `go list -m all` under the workspace resolves a single v0.47.0 by MVS, whose hash is in `cli/go.sum`. No `.github/workflows` changes in the diff.

### A7. The listener binds 127.0.0.1 only while the redirect URI says `localhost`

`cli/internal/auth/login.go:299` listens on `127.0.0.1:<port>` -- correct, and notably *not* `0.0.0.0` -- but the redirect URI is `https://localhost:%d/callback` (`:152`, `:207`). On a dual-stack host whose resolver returns `::1` first, the browser connects to `[::1]:8765`, gets ECONNREFUSED, and the CLI sits until the 5-minute timeout with no explanation.

This is availability, not exposure (binding `::1` is still loopback-only). Fix: listen on both and serve the same handler, or at least detect the case and say so in the timeout message.

### A8. The callback page sends no `Cache-Control` / `Referrer-Policy`

`cli/internal/auth/login.go:335-343`. The page itself is clean -- the only reflected value is `error_description`, correctly run through `html.EscapeString` (`:339`), and the success page is fully static, so there is no XSS -- but the URL that renders it carries the authorization code. Adding `Cache-Control: no-store` and `Referrer-Policy: no-referrer` alongside the existing `Content-Type` is two lines of belt-and-braces against the code being cached or referred onward.

### A9. `openBrowser` leaves a zombie, and puts the authorize URL in the process table

`cli/internal/auth/login.go:385-396`. The program name is a fixed literal per GOOS and the URL is a single argv element with no shell -- exactly right, and the `#nosec G204` annotations are justified. Two small things: `cmd.Start()` with no matching `Wait`/`Release` leaves a zombie child for the CLI's lifetime; and the authorize URL (carrying `state` and `code_challenge`, though never the verifier) is visible in `/proc/<pid>/cmdline` to other processes of the same user for as long as the helper runs. Neither is a credential leak -- the verifier never leaves the process, confirmed by reading every write path -- but a `go func() { _ = cmd.Wait() }()` closes the zombie.

---

## Clean

Verified with evidence, no finding:

- **PKCE end to end.** `AuthCodeURL` always sends `code_challenge` + `S256` (`freshbooks/auth/oauth.go:156-157`); the verifier is returned to the caller and never printed, never in the success page, never in an error. Confirmed by reading every `Fprintf` in `login.go` and by `login_test.go:424`.
- **The ephemeral certificate.** ECDSA P-256, generated in-process, never written to disk, SANs `localhost` + `127.0.0.1` split correctly into `DNSNames`/`IPAddresses`, 1h validity, random 128-bit serial, `MinVersion: TLS12` (`login.go:293-303, 348-380`). The private key never leaves the `tls.Certificate`.
- **One-shot callback.** `once.Do` (`:319`) delivers exactly one result; `stop()` shuts the listener down via `defer` on every exit path including timeout and context cancellation (`:167, 181-187`); `ReadHeaderTimeout` is set (`:321`).
- **The paste path.** Nothing pasted is echoed; no error message interpolates the code (`finishExchange:249-251`); a wrong state on a pasted URL is rejected (`login_test.go:389`).
- **Credential storage.** One lib `FileStore` per context, `0600` file inside a `0700` directory, temp + chmod + write + fsync + atomic rename (`freshbooks/auth/store.go:100-152`). `config.yaml` carries only account/business/business_uuid (`config/config.go:26-42`) and is itself `0600`/`0700`; `config view` marshals that struct, so it cannot print a secret. `auth status` returns a `StatusInfo` with no token field (`status.go:43-62`). `auth token` is the only token-printing path and its `Short` says so.
- **Refresh rotation.** Every non-dry-run client is built with `libauth.NewTokenSource(cfg, store)` over the context's store (`state.go:240`), and `refreshingSource.Token` persists the rotated pair *before* returning it, with a `pendingSave` retry that never re-spends a live refresh token (`freshbooks/auth/token.go:157-208`). `auth token --refresh` saves before printing (`status.go:117-124`).
- **Dry run (D4).** `dryRunTransport.RoundTrip` returns `errDryRun` without touching the network, prints only `METHOD URL` and the body -- never a header, so the bearer cannot reach it (`state.go:286-303`); the token source is `StaticTokenSource("dry-run-placeholder")` and retries are `NoRetry` (`:312-326`); `buildClient` branches to dry-run *before* resolving the context or loading any credential file (`:215-217`), so a dry run works with no credentials at all. `dryrun_test.go:37` asserts nothing Authorization-shaped is printed.
  **On a body containing a secret** (`identity update-application -f` with a `client_secret`): printing it is correct. The whole point of a dry run is to show the exact bytes that would go on the wire, the operator authored that file, and suppressing it would make the feature lie. The one thing dry-run must never print is the credential the *operator did not type* -- the bearer -- and it does not. Worth one sentence in `docs/cli.md` noting that dry-run echoes your request body, so nobody pipes it into a shared CI log.
- **Path handling for `api`.** `client.Do` -> `resolve` rejects absolute URLs and runs `noTraversal` (`freshbooks/transport.go:258-293`); a protocol-relative `//evil.com/x` cannot redirect the host because `u := *c.baseURL` keeps the base host and only `u.Path` is replaced. `appendQuery` (`api_cmd.go:253-271`) parses and re-encodes through `url.Values`, so a `--query` value cannot smuggle a fragment or an extra parameter. Positional ids are `strconv.ParseInt` before reaching the lib (`registry.go:236`), and the one string id (`update-application`) goes through the lib's `pathSegment` (`freshbooks/settings.go:236`).
- **Logs and errors.** The lib logs only method, `redactPath`ped URL (query, fragment, and userinfo all stripped -- `transport.go:486-493`), and status, at Debug (`:323, :345`). The CLI adds no request/response dump: `buildLogger` (`state.go:180-191`) is the only logger construction. The JSON error object (`root.go:113-144`) carries status/code/message/field/family/exit and no header. `DecodeBody`'s error surfaces `encoding/json`'s message, which names a field but never echoes a body value.
- **`-f file` / `--file` upload.** Operator-supplied paths against the operator's own filesystem; `OpenUpload` strips directory components from the transmitted filename via `filepath.Base` (`invocation.go:145`).
- **D8/D9 folds.** The `json` tags added to `Membership`, `User`, and `Page` (`freshbooks/identity.go`, `freshbooks/page.go`) change nothing on the wire: all three are built from separate wire structs (`identityResponse.user()`, `newPage`) and are never decoded from a response, so the tags affect encoding only -- which is what the CLI's output path and `rows()`' `"items"` probe need. The MCP bearer change (`mcp/internal/server/server.go:99-104`) is correct: `bearerPrefix` includes the trailing space, so `"Bearerx..."` fails `EqualFold`, a short header is length-guarded, and an empty token still returns `ok == false`.
- **Public-repo hygiene (section 10).** Grepped the full diff for operator paths, `100.x`/`192.168.x` addresses, internal `*.lab.*`/`*.internal.*` domains, Gitea and vault references, vault item names, and personal identifiers: zero hits. Every identifier in fixtures and docs is synthetic (`ACM000TEST`, `ACM123`, sequential integers). `scripts/redaction-check.sh` would pass. No secret-shaped additions beyond the `client_secret` *field name* discussed in A4.
- **Destructive `--yes` gate.** `registry.go:249-254` requires `--yes` only when stdin is a terminal, which is the inverse of most tools -- but it is a deliberate, documented decision (`docs/cli.md:75-76`) so that pipelines are not forced to pass `--yes`. Recorded here for completeness, not as a finding.

## Not run

`govulncheck` is not installed on this machine (`command -v govulncheck` returns nothing; `~/go/bin` has no copy) and installing it is a mutation this read-only lane will not make. **Someone else must run `govulncheck ./...` in each module before merge** -- this phase adds a new module's worth of dependency graph (cobra/doc's go-md2man and blackfriday among them) that no earlier phase's scan covered.
