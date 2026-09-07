# Security lane -- `feat/account-backup` (HEAD `52e7ebc`, one commit vs `main`)

Verdict: **PASS** with 4 advisories. Nothing blocking. Read-only review: `git`, `grep`, and file reads only; the backup script and the selftest were never executed, and nothing was run against the real API.

Scope reviewed: `scripts/freshbooks-account-backup.sh`, `scripts/backup-selftest.sh`, `scripts/check.sh`, `mise.toml`, `docs/backup.md`, `docs/cli.md`, `cli/internal/docsgen/docsgen.go`, `CHANGELOG.md`, `README.md`, `scripts/site-sync.sh`, `site/sidebars.js`, plus the CLI code paths the script depends on (`cli/internal/cmd/{root,errors,registry,state}.go`, `freshbooks/transport.go`, `freshbooks/auth/oauth.go`).

---

## Findings

### 1. ADVISORY -- `mkdir -p` on a predictable directory name: a same-second rerun silently overwrites, and a pre-created `$root/<stamp>` redirects the whole backup

`scripts/freshbooks-account-backup.sh:44-47`

```
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
dir="$root/$stamp"
umask 077
mkdir -p "$dir"
```

`mkdir -p` succeeds when the target already exists, and it succeeds through a symlink. Two consequences:

- **Collision inside one second.** Two runs that start in the same UTC second (a cron overlap, a shell loop, a fast manual rerun) land in the same `$dir`. Every `<resource>.json` is written with `>` (truncating redirect) at lines 61/66/75 and every singleton at line 95, so run 2 silently overwrites run 1's collection data; `backup.log` interleaves via `tee -a`. The invoice PDFs are the only files protected, and only because `writeBinaryResult` (`cli/internal/cmd/state.go:393-397`) refuses to overwrite without `--force` -- so run 2 reports every PDF as failed while quietly having destroyed the JSON. This makes `docs/backup.md:41` ("Reruns never overwrite: each run gets its own timestamped directory") factually wrong.
- **Predictable-name pre-creation.** The directory name is a pure function of the clock, so anyone with write access to `$root` can create `$root/<stamp>` in advance -- as a symlink to a directory they want written into, or as a mode-0777 directory. `mkdir -p` accepts both, and (unlike a fresh `mkdir` under `umask 077`) it does not reset the mode of a directory that already exists, so the 0700 guarantee the script and the selftest advertise is lost in that case. File contents stay protected (shell redirects inherit `umask 077` -> 0600; the CLI writes PDFs `0o600` explicitly at `state.go:399`), so this is a write-redirection and integrity problem rather than a direct read of the books. It only bites when `$root` is a shared/world-writable directory; the default (`./freshbooks-backup`) and the documented example (`~/backups/freshbooks`) are user-private, which is why this is an advisory and not a block.

Fix (one line, and it makes the doc claim true):

```
mkdir -p "$root"
mkdir "$dir"          # no -p: fail loudly if this second's directory already exists
```

`mkdir` without `-p` fails on an existing directory and on an existing symlink, so both cases become a hard error under `set -e` instead of silent data loss. Worth adding a selftest probe that two runs forced to the same `$stamp` do not clobber, and correcting `docs/backup.md:41` to describe the new behaviour.

### 2. ADVISORY -- the invoice `id` from the API is passed to the filesystem path unvalidated; safety rests entirely on the CLI's argument parser

`scripts/freshbooks-account-backup.sh:110-118`

```
while read -r id; do
  if "$fb" invoices pdf "$id" -o "$dir/pdfs/invoices/$id.pdf" 2>>"$log"; then
done < <(jq -r '.[].id' "$dir/invoices.json")
```

`$id` is API-supplied and is interpolated straight into an output path. I traced whether that is exploitable and it currently is not:

- `Command.execute` parses the positional id with `strconv.ParseInt` at `cli/internal/cmd/registry.go:274-285` and returns a `usageError` (exit 2) on anything non-integer. That happens **before** `buildClient`, before `Run`, and before `writeBinaryResult` (`registry.go:368-384`), so an id of `../../../etc/passwd` or `x y` fails with no file created and no request issued. `invoices pdf` is `HasID: true` with no `IDKind: "string"` (`cli/internal/cmd/commands_invoices.go:88-93`), so the integer path is the one taken.
- An id beginning with `-` would be consumed by pflag as a flag rather than a positional, leaving zero positionals, which cobra rejects (exit 2). A crafted `-o<path>` is defeated by the script's own later `-o` winning last, and the run still fails on the positional count.
- `read -r` strips leading/trailing IFS whitespace, and an embedded newline splits into two lines that both fail the integer parse.

So the outcome today is "logged failure, `failed=1`, exit 1" rather than a traversal. The concern is that the script's filesystem safety is an emergent property of another program's flag parser, with nothing in the script or its tests pinning it. Cheap defense in depth:

```
while read -r id; do
  case "$id" in ''|*[!0-9]*) say "invoice id $id: skipped (not an integer)"; failed=1; continue ;; esac
  if "$fb" invoices pdf -- "$id" -o "$dir/pdfs/invoices/$id.pdf" 2>>"$log"; then
```

(the `--` also removes the leading-dash class outright). A selftest case with a hostile id in the canned `invoices list` output would lock it in.

### 3. ADVISORY -- the "this is sensitive data" warning is under-placed and incomplete

`docs/backup.md:43`

The warning ("A backup is the whole business's books, including client contact details and every invoice line: treat the directory like the credentials file") is accurate and correctly names the 0700 mode, but it is the last paragraph of a section headed **Failure handling**, which is not where a reader looks for handling guidance. It also stops short of the three things an operator actually needs to be told about a directory holding an entire business's accounting records:

- **Retention.** The script deliberately never deletes an old run, so an unattended cron accumulates full copies of the books indefinitely. Say so, and say the operator owns pruning.
- **Where it must not go.** No warning against a backup root inside a git working tree, a synced cloud folder, or a shared/temp directory -- the last of which is also the precondition for finding 1.
- **Encryption at rest and what `backup.log` holds.** `backup.log` is CLI stderr verbatim; under `-o json` that is the `apiErrorJSON` object (`cli/internal/cmd/root.go:117-125`), whose `message` is server-supplied text that can quote record contents. Worth one sentence, plus a nudge toward encrypting the directory if it leaves the machine.

Fix: promote it to its own `## Handling the output` section with those points, and mirror one sentence into the script header comment (which currently mentions only the 0700 mode, at lines 15-16).

### 4. ADVISORY (nit) -- the `SC2086` suppression is inert, and the unquoted `$cmd` would be better as an array

`scripts/freshbooks-account-backup.sh:38-42, 91-95`

The unquoted expansion itself is safe: `singletons` is a hardcoded literal array of `identity me`, `gateways get`, `systems get`, defined at lines 38-42 and never touched by user, environment, or API input. It contains no glob metacharacters, so neither word splitting nor pathname expansion can produce anything unintended.

The nit is that `# shellcheck disable=SC2086` at line 94 does nothing under this gate: `SC2086` is an info-level check and `run_shellcheck` runs `shellcheck -S warning` (`scripts/check.sh:93`), which filters info out. The directive reads as a load-bearing suppression when it is documentation. Either drop the directive and keep the comment, or remove the class entirely by making each entry a command array:

```
singletons=(identity gateways systems)
declare -A singleton_cmd=([identity]="identity me" [gateways]="gateways get" [systems]="systems get")
# then: read -r -a argv <<<"${singleton_cmd[$name]}"; "$fb" "${argv[@]}" -o json ...
```

Low priority; it is a readability and gate-honesty point, not a vulnerability.

---

## Checked and clean (evidence)

**Secrets cannot reach `backup.log`, `manifest.json`, or any output file.** Traced every write path:

- The script never reads, echoes, or interpolates `FRESHBOOKS_CLIENT_SECRET`, an access token, or a refresh token. Credentials are resolved entirely inside the CLI process; `docs/backup.md:10-11` only tells the operator to export the two client variables, using `<your client id>` placeholders.
- `backup.log` is fed exclusively by `2>>"$log"` (lines 61, 64, 73, 95, 111) and `say`'s `tee -a` (line 53) -- i.e. CLI stderr plus the script's own one-line status messages. On stderr the CLI emits either one line of `err.Error()` or the `apiErrorJSON` struct `{status, code, message, field, family, exit}` (`cli/internal/cmd/root.go:117-136`). That struct has no header, token, or credential field.
- No error type carries a secret in its string form: `usageError`/`authError`/`runtimeError` (`cli/internal/cmd/errors.go:22-61`) wrap plain messages; `freshbooks/auth.Error.Error()` (`freshbooks/auth/oauth.go:287-296`) prints only status, the OAuth `code`, and the server's `description`; the `auth` package's `fmt.Errorf` sites (`oauth.go:213,242`, `token.go:164-204`, `store.go:110-149`) name the operation, never the token value; and the two `String()` methods that touch a client secret render it literally as `redacted` (`freshbooks/settings.go:162,224`).
- Even with `--log-level debug` in the environment, the transport logs only `method`, `url` (through `redactPath`, which strips query, fragment, and userinfo -- `freshbooks/transport.go:485-492`), `status`, and `attempt` (lines 323, 345). The `Authorization` header is set at line 393 and never logged. `TestRedactionSweep` (`cli/internal/cmd/redaction_sweep_test.go`) already pins this across five scenarios at debug level, with positive marker assertions so it cannot pass vacuously.
- `freshbooks version` prints only the build-injected version string (`cli/internal/cmd/root.go:103-112`), and the script discards its stderr (line 124). `<resource> list --help` is static cobra help text, also stderr-discarded (line 60). Neither can carry a secret.
- `manifest.json` (lines 122-129) is built from `$stamp`, the version string, the tab-separated `entries` array (resource name, count, status), and the PDF tallies. No credential-derived value is in scope.

**File modes.** `umask 077` is set at line 46 before every `mkdir` and every redirect, so directories are 0700 and shell-created files 0600; the CLI writes PDFs `0o600` explicitly. The selftest asserts the 0700 directory mode (`scripts/backup-selftest.sh:64`). Subject to finding 1 in the pre-existing-directory case.

**`--out` handling.** `$root` is used only inside double quotes (lines 45, 47) and is never `eval`ed, never passed to a shell, and never used to build a command. A caller pointing `--out` at a directory they should not write to is their own privilege, not a boundary crossing.

**Temp files.** The selftest's only scratch space is `mktemp -d` (0700) with `trap 'rm -rf "$scratch"' EXIT` (`scripts/backup-selftest.sh:14-15`). The backup script creates no temp files at all -- `fetch_collection`'s page walk is a brace group piped into `jq -s` (lines 70-75), entirely in memory.

**`FRESHBOOKS_BIN` cannot be steered by a committed file.** `fb="${FRESHBOOKS_BIN:-freshbooks}"` (line 24) falls back to a bare `freshbooks`, which is a PATH lookup -- the same trust the operator already extends to their PATH. Setting the variable is strictly narrower than editing PATH, so it adds no privilege an attacker able to set the environment does not already have. In the gate, `run_backup_selftest` (`scripts/check.sh:111-115`) invokes the selftest with no environment injection, and the selftest sets `FRESHBOOKS_BIN` itself to an absolute path under its own `mktemp -d` (`backup-selftest.sh:44`), overriding any inherited value. No tracked file in the diff sets, exports, or defaults `FRESHBOOKS_BIN`; `mise.toml` gains only the `backup-selftest` task. The fake binary is generated at runtime from a quoted heredoc (`<<'FAKE'`, line 20), so nothing in it is expanded at write time.

**Public-repo hygiene: clean.** Swept the full diff for internal IPs (`100.x`, `192.168.x`), `*.lab.*` / `*.internal.*` domains, vault item paths (`IRL/`), the internal gitea host, operator home paths (`/home/...`, `<operator username>`), and personal names -- no matches (the sole hit was the substring "wes" inside "newest" in an unchanged `mise.toml` description). Identifiers in the diff are synthetic: `AAA111` in the selftest's fake `identity me` response (`backup-selftest.sh:33`) and integer ids 1, 2, 11, 12. `docs/backup.md` uses `<your client id>` / `<your client secret>` placeholders and the generic `~/backups/freshbooks`. The `ACM123` in the `docs/cli.md` / docsgen preamble is pre-existing and already the repo's synthetic example account. No real FreshBooks account, business, or business-UUID value appears anywhere in the diff. The new script and doc are both inside the paths `scripts/redaction-check.sh` scans, so they stay covered going forward.

**Read-only guarantee.** Every CLI invocation in the script is a `list`, a `get`/`me`, `invoices pdf`, or `version`. `invoices pdf` is registered `Class: ClassRO` (`commands_invoices.go:92`), so the destructive `--yes` gate is not in play and no write endpoint is reachable from this script.
