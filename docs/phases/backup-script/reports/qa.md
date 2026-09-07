# QA / reality-check lane -- `feat/account-backup`

Subject: `feat/account-backup` @ `1e94f94`, two commits ahead of `main` (`52e7ebc` feature, `1e94f94` review-gate fix commit). Lane default is NEEDS WORK; PASS requires evidence.

**Verdict: NEEDS WORK** -- 1 blocking, 5 advisory.

The branch is in good shape. The gate is green, every claim in the `1e94f94` commit message is met in the tree bar one wording overreach, and all nine independent reproductions the work order asked for behaved exactly as specified. The blocking finding is a failure mode the fix commit did not cover: a `list` that exits 0 but prints unparseable stdout is recorded `ok` with a zero-byte file, and it truncates `manifest.json` for the whole run to zero bytes.

Scratch paths below are written as `<scratch>`. No command in this report ran the real CLI or touched the network; every reproduction drove the script through a fake binary via `FRESHBOOKS_BIN`, and the fakes were deleted afterwards.

---

## 1. Gate and tree

`mise run check` from the repo root: **exit 0**. `git status --porcelain` before and after: **empty** (checked again after `mise run docs`, see section 4).

`backup-selftest` lines, all 16 probes plus the summary:

```
== backup-selftest ==
backup-selftest: PASS exit 1 when a resource fails
backup-selftest: PASS --all collection written
backup-selftest: PASS empty --all collection (null) written as []
backup-selftest: PASS three-page collection walked and merged
backup-selftest: PASS a failed middle page fails the collection
backup-selftest: PASS page-shaped list without --page keeps page 1 as partial
backup-selftest: PASS unpaginated array kept
backup-selftest: PASS failed collection leaves no file
backup-selftest: PASS stderr captured in backup.log
backup-selftest: PASS singletons written
backup-selftest: PASS invoice pdfs: one written, one failed, hostile id skipped
backup-selftest: PASS hostile invoice id logged
backup-selftest: PASS manifest records outcomes
backup-selftest: PASS backup dir is 0700
backup-selftest: PASS an existing directory at the run's name is refused
backup-selftest: PASS --no-pdfs skips pdfs
backup-selftest: all probes passed
```

Coverage-gate lines (all three modules over the 90% floor):

```
coverage-gate: <repo>/freshbooks/coverage.out total = 91.9% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo>/mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: PASS
coverage-gate: <repo>/cli/coverage.out total = 91.6% (floor 90%)
coverage-gate: PASS
```

Gate tail:

```
== readme-drift-check ==
release: OK docs
== site-build ==
site-sync: regenerated site/docs/ (9 pages)
[SUCCESS] Generated static files in "build".
site-build: OK
check.sh: all OK
```

No `DIRTY TREE:` banner. `mise run check` was the only gate run in this tree.

---

## 2. Every claim in the `1e94f94` commit message

| # | Claim | Verdict | Evidence |
|---|---|---|---|
| 1 | A failed page mid-walk fails the whole collection instead of recording `ok` | **met** | `scripts/freshbooks-account-backup.sh:91` (`\|\| { rm -f "$tmp"; return 1; }`); reproduction (a) below |
| 2 | A page-1 `total` the walk did not deliver is recorded `partial` | **met** | `scripts/freshbooks-account-backup.sh:106-109`; reproduction (d) |
| 3 | `retainers` probed for `--page` like `--all`; keeps page 1, records `partial`, not "fails forever" | **met** | `scripts/freshbooks-account-backup.sh:88-102`; reproduction (c) |
| 4 | An empty collection (CLI prints `null`) is written as `[]` | **met** | `scripts/freshbooks-account-backup.sh:59-61` (`as_array`), `:75`, `:81`; reproduction (b) |
| 5 | Run directory created without `-p`; a same-second rerun or pre-created name or symlink is refused | **met** | `scripts/freshbooks-account-backup.sh:50-52`; reproduction (f) |
| 6 | The doc's "never overwrites" claim is now true | **met** | `docs/backup.md:45`; reproduction (f) writes nothing into the pre-created directory |
| 7 | Invoice ids validated as plain integers before becoming file names | **met** | `scripts/freshbooks-account-backup.sh:160-167`; reproduction (e) |
| 8 | Invoice ids passed after `--` | **met** | `scripts/freshbooks-account-backup.sh:168` |
| 9 | `--help` probe reads into a variable instead of `\| grep -q` under `pipefail` | **met** | `scripts/freshbooks-account-backup.sh:71` |
| 10 | The pdf-loop `jq` stderr goes to `backup.log` | **met** | `scripts/freshbooks-account-backup.sh:175` (`2>>"$log"`) |
| 11 | A manifest write failure still exits 1 | **met** | `scripts/freshbooks-account-backup.sh:190-193`; observed exit 1 in finding 1 below |
| 12 | The singleton command is an array, no shellcheck directive | **met** | `scripts/freshbooks-account-backup.sh:140` (`read -r -a argv`); `grep -n shellcheck` over both new scripts returns nothing |
| 13 | `jq length` runs once per collection | **not met** (advisory 3) | `scripts/freshbooks-account-backup.sh:106` and `:119` both run it on the same file |
| 14 | The manifest object is built by jq, not shell string interpolation | **met** | `scripts/freshbooks-account-backup.sh:180-189` |
| 15 | Fake CLI mirrors the real one: `--all` bare array / `null`, page objects, a NoPaging `retainers`, a three-page walk with a failing middle page, a hostile invoice id, with probes for each | **met** | `scripts/backup-selftest.sh:32-53` (fake), `:66-97` (probes) |
| 16 | Portable 0700 check | **met** | `scripts/backup-selftest.sh:83` (`find "$dir" -maxdepth 0 -perm 700`) |
| 17 | `docs/backup.md` gains a "Handling the output" section (retention, where not to put it, `backup.log` sensitivity) | **met** | `docs/backup.md:47-52` |
| 18 | `docs/backup.md` documents `backup.log` contents | **met** | `docs/backup.md:26`, `docs/backup.md:52` |
| 19 | `docs/backup.md` documents the `partial` status | **met** | `docs/backup.md:37`, `docs/backup.md:43` |
| 20 | `scripts/check.sh` and `docs/building.md` count and list the three selftests and site-build | **met** | `scripts/check.sh:10` ("the three selftests"), `scripts/check.sh:172-178`, `docs/building.md:18` |
| 21 | Live read-only run after the fix: 27 resources ok, 3 PDFs, exit 0 | **not verified** | Out of this lane's scope by instruction (no live run). Structurally corroborated: reproduction (g) yields exactly 27 resource entries and exit 0 |

---

## 3. Independent reproductions

Fakes were written under `<scratch>/qa-backup/bin/` and are not the selftest's fake. Each run: `FRESHBOOKS_BIN=<scratch>/qa-backup/bin/<fake> ./scripts/freshbooks-account-backup.sh --out <scratch>/qa-backup/<outdir>`.

**(a) three-page collection whose page 2 fails -> no file, `status: failed`, exit 1.** Fake `fb-a`: `bills list --help` advertises `--page`; page 1 returns `{"items":[{"id":1}],"pages":3,"total":3}`; `--page 2` prints to stderr and exits 1; `--page 3` succeeds.

```
backup: bills: FAILED (see backup.log)
bills.json exists? NO
manifest .resources.bills -> {"count":0,"status":"failed"}
EXIT=1
```

Expected no file / `failed` / exit 1. Observed exactly that. Note the walk does not "recover" by continuing to page 3 -- correct, that is the point of the fix.

**(b) `--all` collection printing `null` -> file is `[]`, count 0, `status: ok`.** Fake `fb-m`, `clients list --all -o json` prints `null`.

```
clients.json -> []
manifest .resources.clients -> {"count":0,"status":"ok"}
```

**(c) page-shaped list, `--help` has neither `--all` nor `--page`, `pages: 2` -> page 1 kept, `partial`, exit 1.** Same fake, `retainers`.

```
retainers.json -> [ { "id": 1 } ]
manifest .resources.retainers -> {"count":1,"status":"partial"}
EXIT=1
```

**(d) page 1 promises `total: 5`, walk delivers 3 -> `partial`.** Same fake, `taxes list -o json` returns `{"items":[{id:1},{id:2},{id:3}],"pages":1,"total":5}`.

```
manifest .resources.taxes -> {"count":3,"status":"partial"}
```

**(e) invoice ids `../../x` and `12 34` -> skipped, logged, no file outside `pdfs/invoices/`.** Same fake, `invoices list --all` returns ids `7`, `"../../x"`, `"12 34"`, `8`.

```
backup: invoice id '../../x': skipped, not an integer
backup: invoice id '12 34': skipped, not an integer
find <dir>/pdfs -> pdfs/, pdfs/invoices/, pdfs/invoices/7.pdf, pdfs/invoices/8.pdf
manifest .invoice_pdfs -> {"skipped":false,"written":2,"failed":2}
```

No file was created anywhere outside `pdfs/invoices/`; the run directory holds only the expected `<resource>.json`, singletons, `backup.log`, `manifest.json`, `pdfs/`.

**(f) pre-created directory at the run's timestamp -> non-zero exit before writing anything.** Pre-created `+0`, `+1`, `+2`, `+3` seconds so the probe does not race the clock.

```
mkdir: cannot create directory '<scratch>/qa-backup/pref/20260907T180357Z': File exists
EXIT=1 ; files written inside: 0
```

**(g) all-green fake -> exit 0, `ok: true`, full manifest, exactly 27 resource names.** Fake `fb-g`: every list offers `--all`, `invoices` returns one id, the pdf succeeds, all three singletons succeed.

```
EXIT=0
{"created_at":"20260907T180354Z","cli_version":"qa-green-2.0","ok":true,
 "invoice_pdfs":{"skipped":false,"written":1,"failed":0}}
resource count: 27
every resource has both count and status: true
distinct statuses: ["ok"]
run directory mode: 700
```

24 collections + 3 singletons = 27, matching `docs/backup.md:35`'s list.

**(h) `--no-pdfs` -> no `pdfs/` dir, `invoice_pdfs.skipped == true`.**

```
EXIT=0 ; pdfs dir exists? NO ; {"skipped":true,"written":0,"failed":0}
```

**(i) linters clean on both scripts.**

```
usage lint scripts/freshbooks-account-backup.sh   -> No issues found. (exit 0)
usage lint scripts/backup-selftest.sh             -> No issues found. (exit 0)
shellcheck -S warning -e SC1008,SC2154 scripts/freshbooks-account-backup.sh scripts/backup-selftest.sh -> exit 0, no output
```

---

## 4. Docs versus behaviour

`docs/cli.md`: ran `mise run docs` (exit 0). `git status --porcelain` afterwards was **empty** -- neither `docs/cli.md` nor README.md's Status column changed, so the "Example: full account backup" section at `docs/cli.md:63-66` is byte-identical to what `cli/internal/docsgen/docsgen.go:82-85` regenerates. No `git checkout` was needed; the tree never went dirty.

`site/build/backup.html` exists after the gate's site-build (24829 bytes). `site/sidebars.js:14` lists `'backup'` between `'cli'` and `'building'`, matching `scripts/site-sync.sh:44`.

Sentence-by-sentence on `docs/backup.md`, every claim that states what the script does or writes:

| Line | Claim | Verdict |
|---|---|---|
| 3 | Reads everything the CLI can list for one context, writes JSON plus one PDF per invoice, never writes to FreshBooks | **confirmed** -- all 27 reads are `list` / `get` / `me` / `pdf`; no create/update/delete anywhere in the script |
| 15 | Needs `freshbooks`, `jq`, `usage`; `mise install` provides `usage`; `FRESHBOOKS_BIN` overrides the binary | **confirmed** -- `mise.toml:7` pins `usage = "6.0.0"`; `scripts/freshbooks-account-backup.sh:1` is the `usage bash` shebang; `:25` reads `FRESHBOOKS_BIN` |
| 19 | `-o/--out`, UTC-timestamped subdirectory, default `./freshbooks-backup` | **confirmed** -- `:20`, `:26`, `:46-47`; observed `20260907T180354Z` |
| 20 | `--no-pdfs` skips the PDF pass | **confirmed** -- reproduction (h) |
| 26 | `backup.log` holds the script's progress lines plus **every** stderr line the CLI produced, in run order | **partly refuted** -- see advisory 5; two CLI invocations send stderr to `/dev/null`, not the log |
| 27 | `manifest.json` holds cli version, ok flag, per-resource count and status, pdf tallies | **confirmed** -- reproduction (g). `created_at` is also present but undocumented (advisory 4) |
| 28 | One JSON array per collection, 24 files | **confirmed with a caveat** -- 24 in a clean run (g); a failed collection leaves no file (`:130`), so fewer, which `:43` states but the tree diagram does not (advisory 4) |
| 29-31 | `identity.json`, `gateways.json`, `systems.json` from `identity me` / `gateways get` / `systems get` | **confirmed** -- `:40-44`, `:138-151` |
| 32 | `pdfs/invoices/<id>.pdf` | **confirmed** -- reproduction (e) |
| 35 | The 24 named collections are every `<resource> list`; `--all` used where offered, otherwise page 1 plus a hand-walked `--page 2..N`, or the array as is | **confirmed** -- `:31-37` matches the list exactly; `:72-83` implements the three shapes; reproductions (a)-(d) |
| 35 | "Every collection file is a flat JSON array either way ... `.[]` always iterates"; empty is `[]` | **refuted** -- empty-as-`[]` holds (b), but see **finding 1**: a `list` that exits 0 with unparseable stdout leaves a zero-byte file that is not a JSON array and is still recorded `ok` |
| 37 | `retainers list` prints pages but registers no `--page`; the script keeps page 1, records `partial`, exits 1. A page-1 `total` the walk did not deliver is `partial` too | **confirmed** -- reproductions (c) and (d) |
| 39 | `time-entries list-with-totals` is the same wire request and is not repeated | **confirmed** -- `:29-30`, and `time-entries` appears once at `:36` |
| 43 | A failed resource is logged, leaves no file, is `"status": "failed"`; a failed page fails its whole collection; the run continues | **confirmed** -- reproduction (a); the run continued through all 27 resources |
| 43 | Exit status 1 if anything failed or was partial, 0 otherwise | **confirmed** -- exit 1 in (a), (c), (d), (e); exit 0 in (g), (h) |
| 43 | If the manifest itself cannot be written the exit status is still 1 | **confirmed** -- observed in finding 1, exit 1 with `manifest.json: FAILED to write` |
| 45 | Each run creates its own timestamped directory and refuses to start if it exists; nothing is ever overwritten | **confirmed** -- reproduction (f) |
| 49 | Output directory 0700 via `umask 077`; the CLI writes PDFs 0600 | **confirmed** -- `:48`, observed mode 700 in (g); `cli/internal/cmd/state.go:379-400` (`writeBinaryResult`) uses `os.WriteFile(path, b, 0o600)` |
| 50 | Nothing prunes old runs | **confirmed** -- no delete of any prior run anywhere in the script |
| 52 | `backup.log` holds the CLI's stderr verbatim and is as sensitive as the data files | **confirmed for the calls that log** -- `:74`, `:79`, `:91`, `:142`, `:168`, `:175` all use `2>>"$log"`; see advisory 5 for the two that do not |
| 56-58 | Does not restore, does not run reports, does not fetch per-record sub-resources | **confirmed** -- no `create`, `reports`, or `get <id>` call in the script |
| 62 | Description of the selftest's fake | **partly refuted** -- advisory 2 |

---

## 5. Public-repo hygiene

Clean.

- `git grep -n -i <author local-part> -- docs/phases/backup-script/` returns **one** hit, and it is the security lane's own prose at `docs/phases/backup-script/reports/security.md:111` describing the false positive (the three-letter local part occurs inside the word "newest"). No standalone occurrence of the name. The lead's scrub of the earlier mention holds.
- Same search over the full `git diff main...HEAD`: 4 hits, all the same "newest" substring inside `mise.toml` and `docs/building.md` descriptions.
- The nine-character home-directory username: **zero** hits under `docs/phases/backup-script/` and zero in the diff.
- `git diff main...HEAD | grep -nE "/home/[a-z]|/Users/"` -> no matches.
- No internal IPs (`100.x`, `192.168.x`), no `*.lab.*` / `*.internal.*` domain, no `gitea` host, no `IRL/` vault path in the diff.
- No token-shaped strings: a `[A-Za-z0-9]{20,}` sweep over the diff, after filtering known project nouns, returns nothing.
- Identifiers in the diff are synthetic: `AAA111` (`scripts/backup-selftest.sh:48`), small integer ids, `ACM123` (pre-existing in `docs/cli.md`). `docs/backup.md:10-12` uses `<your client id>` / `<your client secret>` and `~/backups/freshbooks`.
- `scripts/redaction-check.sh` -> `redaction-check: clean`.

---

## 6. Findings

### 1. BLOCKING -- a `list` that exits 0 with unparseable stdout is recorded `ok` with a zero-byte file, and destroys `manifest.json` for the whole run

`scripts/freshbooks-account-backup.sh:61` (`as_array`), consumed at `:75` and `:81`; downstream at `:119` and `:180-193`.

`as_array` is `jq 'if type == "array" then . else [] end'`. Its exit status is never checked, and `fetch_collection` runs with errexit suspended (it is always called as `fetch_collection ... || status=$?`, `:116`). So when `jq` fails to parse, the `>"$out"` redirect has already truncated the file, `as_array` returns non-zero, and `:76` / `:82` return **0** anyway. The collection is recorded `ok`.

It then cascades. At `:119` `n="$(jq length "$out")"` on a zero-byte file produces the empty string, not a number, so the manifest entry becomes `<name><TAB><TAB>ok`; the manifest jq program's `(.[1] | tonumber)` at `:189` then fails on the empty count, and because the `>"$dir/manifest.json"` redirect has already created the file, `manifest.json` is left at **0 bytes**. The outcomes of all 27 resources -- including the 26 that succeeded -- are gone.

Expectation: the collection is recorded `failed`, leaves no file (`docs/backup.md:43`), and the manifest still records the other 26 resources. Observed: `status: ok`, a zero-byte `bills.json`, and a zero-byte `manifest.json`.

Reproduction, fake `fb-bad` where `bills list --all -o json` prints `not json at all` and exits 0 (a second fake, `fb-empty`, printing nothing at all and exiting 0, behaves identically):

```
$ FRESHBOOKS_BIN=<scratch>/qa-backup/bin/fb-bad ./scripts/freshbooks-account-backup.sh --out <scratch>/qa-backup/outbad2
backup: invoice pdfs: 0 written, 0 failed
jq: error (at <stdin>:27): Expected JSON value (while parsing '')
backup: manifest.json: FAILED to write
backup: done: <dir> (ok=no)
EXIT=1

bills.json    size=0  content=[]
manifest.json size=0  content=[]
```

This is the same failure class as review finding B1 -- "silently dropping it and recording ok" -- which `1e94f94` fixed for a failed page but not for a page that parses to nothing. The trigger requires the CLI to break its own contract (exit 0 with non-JSON on stdout), which the Go CLI should not do on its own; it is reachable in practice through `FRESHBOOKS_BIN` pointing at a wrapper that prints a banner, or any future CLI path that writes a notice to stdout. Rated blocking not for its likelihood but for its blast radius: one bad resource silently converts a 26-of-27-good backup into a run with no machine-readable record at all, and `docs/backup.md:35`'s "`.[]` always iterates" contract is what a restore script would be built on.

Smallest fix: have `as_array` propagate jq's status (drop the wrapper, or `as_array ... >"$out" || return 1`) at `:75` and `:81`, and guard `:119` so a non-numeric count cannot reach the manifest program. The selftest already has the shape for a probe -- add a fake case that exits 0 with garbage on stdout.

### 2. ADVISORY -- `docs/backup.md:62` misdescribes the selftest's fake, omitting the two probes the fix commit was written for

`docs/backup.md:62` says the fake answers with "an `--all` collection, a two-page collection walked by hand, an unpaginated array, a collection that fails, a PDF that fails, and the three singletons".

Expectation: a description matching `scripts/backup-selftest.sh:32-53`. Observed two mismatches. First, there is no two-page collection: `services` is **three** pages (`:38-40`) and `tasks` is three pages with a failing middle page (`:41-43`). Second, the sentence omits both headline cases from `1e94f94` -- the failing middle page and the NoPaging `retainers` at `:44-45` -- even though the following sentence does list the empty-collection, hostile-id, and pre-existing-directory checks. A reader auditing test coverage from the doc would conclude the two most important regressions are untested.

Suggested: "a three-page collection walked by hand, one whose middle page fails, a page-shaped list with no `--page` flag, an unpaginated array, ...".

### 3. ADVISORY -- the commit message's "`jq length` runs once per collection" is not accurate

`scripts/freshbooks-account-backup.sh:106` and `:119`.

`1e94f94`'s message says "`jq length` runs once per collection and the manifest object is built by jq". The second half is true (`:180-189`). The first is not: for any paginated collection whose page 1 carries a `total`, `jq length "$out"` runs at `:106` (the total check) and again at `:119` (the count for the record), and on the partial-by-total path a third time at `:107` inside the `say`. Simplify finding 1 targeted the `record`/`say` double call, which **is** fixed -- `n` is captured once at `:119` and reused at `:120-121`. Only the commit message overstates the result. No code change needed; noting it so the phase record is accurate.

### 4. ADVISORY -- two small manifest/layout claims in `docs/backup.md` are incomplete

`docs/backup.md:27-28`.

- `:27` lists the manifest's keys as "cli version, ok flag, per-resource count and status ..., pdf tallies". `created_at` (`scripts/freshbooks-account-backup.sh:187`) is also present and is the only record of when the run started other than the directory name. Worth one word.
- `:28` states "one JSON array per collection (24 files)" unconditionally. A failed collection leaves no file (`:130`), which `:43` says, but the tree diagram reads as a guarantee. "(24 files in a clean run)" would reconcile them.
- Also at `:27`: a singleton's count is hard-coded to 1 at `:143` regardless of payload. In the selftest's own fake, `gateways get` returns `[]` yet the manifest records `{"count":1,"status":"ok"}`. Harmless, but "per-resource count" reads as an item count.

### 5. ADVISORY -- `docs/backup.md:26` overstates what reaches `backup.log`

`scripts/freshbooks-account-backup.sh:71` and `:182`, against `docs/backup.md:26`.

`:26` says `backup.log` holds "every stderr line the CLI produced, in run order". Two invocations discard stderr instead: the `--help` probe at `:71` (`2>/dev/null || true`) and the version read at `:182` (`2>/dev/null || echo unknown`). Both discards are deliberate and correct -- neither should be able to fail a run -- but the doc's "every" is false, and the `--help` probe is exactly where a misconfigured CLI would say something useful (an unknown-command or auth error would vanish, and the collection would then fall through to the unpaginated path). Suggested: "every stderr line from the data-fetching calls". Optionally send the `--help` probe's stderr to the log too, which costs nothing since its status is already ignored.

### 6. ADVISORY -- nothing promised is missing

Checked every pointer for a dangling promise; all resolve.

- README.md:57 -> `docs/backup.md` exists.
- CHANGELOG.md:11 claims 24 collections, three singletons, one PDF per invoice, `ok`/`partial`/`failed` per resource, a fresh 0700 directory that refuses reuse, `mise run backup-selftest` in the gate, `docs/backup.md` published on the site, README and `docs/cli.md` pointers -- every one verified above.
- `mise.toml:68-70` defines `backup-selftest`; `scripts/check.sh:208` dispatches it; `scripts/check.sh:175` puts it in `repo-wide`; the task descriptions at `mise.toml:49` and `mise.toml:89` both list it.
- `scripts/site-sync.sh:44` and `site/sidebars.js:14` both carry `backup`, and `site/build/backup.html` was produced by the gate.
- Conversely, nothing the script does is undocumented beyond the wording gaps in advisories 4 and 5. The flag surface (`--out`, `--no-pdfs`), the `FRESHBOOKS_BIN` override, the exit-status contract, the directory mode, and the "no flags are forwarded, use a context or `FRESHBOOKS_*`" scoping rule all appear in `docs/backup.md` and in the script's own header at `:2-17`.

---

## 7. What would flip this to PASS

Finding 1, with a selftest probe covering it. Advisories 2, 4, and 5 are one-line doc edits and can ride the same fix commit; advisory 3 needs no change.

Every fake used for this report has been deleted; the run outputs under `<scratch>/qa-backup/` are outside the repo. The working tree is clean apart from this file, which sits under `docs/phases/*/reports/` and is excluded from the dirty-tree banner.
