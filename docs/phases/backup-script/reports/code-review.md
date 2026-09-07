# Code-review lane -- feat/account-backup

Branch: `feat/account-backup` @ `52e7ebc` (one commit, `git diff main...feat/account-backup`).
Scope reviewed: `scripts/freshbooks-account-backup.sh`, `scripts/backup-selftest.sh`, `scripts/check.sh`, `mise.toml`, `docs/backup.md`, `scripts/site-sync.sh`, `site/sidebars.js`, `README.md`, `CHANGELOG.md`, `cli/internal/docsgen/docsgen.go` + the regenerated `docs/cli.md`.
Method: read-only. `usage lint` (clean on both scripts), `shellcheck -S warning -e SC1008,SC2154` (clean, matching `scripts/check.sh`'s own invocation), plus four reproductions of the script driven by throwaway fake `freshbooks` binaries via `FRESHBOOKS_BIN` in the scratchpad. The real API was never contacted; no gate step, build, or selftest was run.

## Verdict: REQUEST CHANGES

Three blocking findings, all reproduced. Every one of them is the same shape: the script reports a green, `"status": "ok"`, exit-0 backup that is missing data, or hard-fails a resource forever. For a backup tool that is the worst failure mode there is, and the selftest's fixtures are built so that none of the three can be observed.

The rest of the change is good. The `usage` header is correct (`usage_out`, `usage_no_pdfs`, both verified against the real `usage` 6.0 runtime), the SC2086 disable on line 94 is justified and correctly scoped, the 24 collection names match the CLI's command groups exactly (checked against `Group:`/`Verb:` in `cli/internal/cmd/commands_*.go`), `identity me` / `gateways get` / `systems get` are the right three singleton verbs, `umask 077` + `mkdir -p` gives the documented 0700, the check.sh/mise wiring matches the `release-selftest` exemplar step for step, and `docs/backup.md` is ASCII-only and not hard-wrapped.

---

## BLOCKING

### 1. A mid-walk page failure silently truncates the backup and records it as `ok`

`scripts/freshbooks-account-backup.sh:70-76`

The page-walk loop pipes each page into `jq '.items // []'` inside a `{ ... } | jq -s 'add // []'` group. When a `--page N` call fails, the CLI writes nothing to stdout, `jq` gets empty input, produces no output, and exits 0. The failing page just vanishes from the concatenation. Because the group's exit status is the status of its *last* command, only a failure on the *final* page propagates out of `fetch_collection`; a failure on any earlier page is invisible.

Reproduced (fake CLI, `services` with `pages: 3`, page 2 exits 1):

```
exit=0
services.json: [ {"id": 1}, {"id": 3} ]
manifest services: {"count":2,"status":"ok"}  ok=true
```

A cron job sees exit 0, `ok: true`, `"status": "ok"`, and a file missing a third of the business's services. The error text is in `backup.log`, but nothing in the contract tells anyone to read it on a green run.

Fix: make each page's failure fatal to the collection. Collect into a temp file and check status per page, e.g.

```bash
local tmp; tmp="$(mktemp)"
jq '.items // []' <<<"$first" >"$tmp"
for ((p = 2; p <= pages; p++)); do
  page_json="$("$fb" "$res" list --page "$p" -o json 2>>"$log")" || { rm -f "$tmp"; return 1; }
  jq '.items // []' <<<"$page_json" >>"$tmp"
done
jq -s 'add // []' <"$tmp" >"$out"; rm -f "$tmp"
```

Also worth a cheap belt-and-braces guard while you are in there: compare the final `jq length "$out"` against the `.total` from page 1 and record `failed` (or a `partial` status) on a mismatch. That single check catches this whole class, including a page that returns 200 with fewer items than promised.

### 2. `retainers` can never be backed up past page 1, and the run can never exit 0

`scripts/freshbooks-account-backup.sh:33` (the `retainers` entry) and `:69-75`; documented as working at `docs/backup.md:35`.

`retainers list` is the one list command registered with `NoPaging: true` (`cli/internal/cmd/commands_retainers.go:20`), which per `registry.go:196-204` registers **neither** `--page`/`--per-page` **nor** `--all`. But `RetainersService.List` returns `*Page[Retainer]` (`freshbooks/retainers.go:76-82`), so `retainers list -o json` still prints the full `{items,page,pages,per_page,total}` object.

So the script takes the fallback branch, sees `pages > 1`, and runs `freshbooks retainers list --page 2 -o json`, which cobra rejects with `unknown flag: --page` (exit 2). Every page after the first fails; the last one fails too, so `fetch_collection` does return non-zero, and the result is:

```
exit=1
(no retainers.json)
manifest: {"count":0,"status":"failed"}  ok=false
```

Any account with more retainers than the server's default page size loses the retainers collection entirely and the script exits 1 on every single run, forever, with no flag to fix it. `docs/backup.md:35` states that where a command has no `--all` "the script fetches page 1 and walks `--page 2..N` itself" -- that is untrue for `retainers`, the only resource in the 24 where it matters.

Fix: probe for `--page` the same way you probe for `--all`, and treat "page-shaped output but no `--page` flag" as its own case -- keep page 1's items and record a truthful non-`ok` status (`"partial"`, with `total` recorded) rather than pretending it succeeded or losing the file:

```bash
help="$("$fb" "$res" list --help 2>/dev/null || true)"
case "$help" in *--all*) ... ;; esac
case "$help" in *--page*) walk pages ;; *) keep page 1, mark partial when .pages > 1 ;; esac
```

The three genuinely unpaginated lists (`staff`, `ledger-accounts`, `service-rates` -- not `List: true` at all, bare arrays) already fall out of the `type != object` branch correctly and are unaffected.

### 3. Empty collections are written as `null`, not `[]`, breaking the documented file contract

`scripts/freshbooks-account-backup.sh:61` and `:66`; contract stated at `docs/backup.md:35`.

`collectAll` (`cli/internal/cmd/registry.go:446-455`) declares `var items []T` and returns it unchanged, so an empty collection is a nil slice; `output.marshal` hands that to `json.Marshal`, which renders `null`. Every `--all` command therefore prints a bare `null` -- not `[]` -- for an empty collection, and the same is true of the bare-array lists whose lib method returns a nil slice.

The script writes that straight to disk. Reproduced end to end:

```
clients.json = null
manifest: ok=true, resources.clients = {"count":0,"status":"ok"}
stderr: jq: error (at <stdin>:1): Cannot iterate over null (null)
```

`docs/backup.md:35` promises "Every collection file is a flat JSON array either way, so `jq length` is the item count." `jq length` happens to survive (`null | length` is 0), but the file is not an array, and anything that does `jq '.[]'` -- including this script's own invoice-PDF loop at line 118, as the stderr above shows -- errors out. Any consumer or future restore script written against the documented contract breaks on the first empty collection, and on a real account several of these 24 (callbacks, bill-vendors, credit-notes, other-income) are routinely empty.

Fix: normalize on write in both branches, so the doc's claim becomes true.

```bash
"$fb" "$res" list --all -o json 2>>"$log" | jq 'if type == "array" then . else [] end' >"$out"
```
(mind `pipefail` and the exit status of the CLI, not of `jq` -- capture to a variable first, as in finding 1) and likewise for the `printf '%s\n' "$first" >"$out"` path at line 66.

---

## ADVISORY

### 4. The selftest's fixtures contradict the real CLI, which is why 1-3 got through

`scripts/backup-selftest.sh:28-32`

- `retainers` is the fixture chosen to prove page-walking (lines 29-30), and it is precisely the one list command that cannot be page-walked (finding 2). The resources that really do need the fallback -- `services`, `time-entries`, `journal-entry-accounts` -- are not modelled at all.
- The fake's `*" list --help"` arm answers `--page int` for everything except `invoices`, so 23 of 24 collections take the fallback path. In reality 17 of 24 take `--all`. The proportions are inverted, so the `--all` path gets one probe and the hand-walk gets 23.
- No fixture ever returns `null`, so the real empty-collection shape (finding 3) is never exercised.
- No fixture fails on a non-final page, so finding 1's silent-truncation path is never exercised.

The test is not vacuous -- it does assert real content and would catch a gross regression -- but its fixtures mirror what the script does rather than what the CLI does, which is the specific smell the work order asks about. Please add: a three-page collection whose middle page fails (asserting `status: failed` and no file), a collection returning `null` (asserting the file is `[]`), and a `NoPaging`-shaped fixture (page object, `--help` without `--page`).

### 5. `stat -c %a` is GNU-only

`scripts/backup-selftest.sh:64`

The gate runs on `ubuntu-latest` only, so CI is fine, but `mise run check` is the documented local gate and this is the only `stat -c` in `scripts/`. On macOS it fails spuriously (`stat -f %A`). A portable substitute avoids the question entirely, e.g. `find "$dir" -maxdepth 0 -perm 700` or comparing `ls -ld` output.

### 6. `--help | grep -q` is fragile under `pipefail`

`scripts/freshbooks-account-backup.sh:60`

`set -o pipefail` is on, and `grep -q` exits at the first match; if the CLI is still writing it takes SIGPIPE and exits 141, making the pipeline non-zero and silently pushing an `--all`-capable resource onto the slow hand-walk path. Verified as a real mechanism (a large writer reproduces 141 every time), and verified as *not* currently triggerable: real `list --help` output is around 2 KB, well inside the pipe buffer, so the CLI always finishes and exits 0 first. Still worth removing the dependence on output size, and it composes with finding 2's `--page` probe anyway:

```bash
help="$("$fb" "$res" list --help 2>/dev/null || true)"
case "$help" in *--all*) ... ;; esac
```

### 7. Two small docs-versus-behaviour mismatches

- `docs/backup.md:26` describes `backup.log` as "every stderr line the CLI produced, in run order". It also contains the script's own `backup: ...` progress lines, because `say()` tees into it (line 53). Either say so, or send progress to stderr only.
- `docs/backup.md:41` states the exit status is 1 on failure and 0 otherwise. That holds on the paths I exercised, but the manifest step at lines 122-129 runs with `errexit` live and outside any `if`; if `jq` fails there the script exits with jq's status (I saw exit 5 in a synthetic run) and leaves a truncated `manifest.json`. Wrapping the manifest write so a failure still yields exit 1 keeps the documented contract true.

### 8. The invoice-PDF loop's `jq` stderr escapes `backup.log`

`scripts/freshbooks-account-backup.sh:118`

`done < <(jq -r '.[].id' "$dir/invoices.json")` is the only command in the script whose stderr is not appended to `$log`; it goes to the terminal instead. That is exactly where finding 3's `Cannot iterate over null` surfaces, so the one diagnostic that would explain a zero-PDF run is the one that is not recorded. Add `2>>"$log"`.
