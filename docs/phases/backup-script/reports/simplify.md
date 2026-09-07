# Simplification lane -- `feat/account-backup` @ `52e7ebc`

Scope reviewed: `scripts/freshbooks-account-backup.sh`, `scripts/backup-selftest.sh`, `docs/backup.md`, the wiring in `scripts/check.sh` / `mise.toml` / `scripts/site-sync.sh` / `site/sidebars.js`, and the `docs/cli.md` preamble added in `cli/internal/docsgen/docsgen.go`. Propose-only: nothing was modified, nothing was run against the real API, no gate/selftest/build was executed.

**Verdict: the branch is tight.** 132 lines of script and 76 of selftest for 27 resources plus a PDF pass, in the house style of `scripts/release-selftest.sh` and `scripts/redaction-selftest.sh`. Four small cuts are worth taking; two are code, two are drift the branch introduced in prose that describes the gate. The rest of what the work order asked me to hunt for (the manifest pipeline, the `--all` probe, the two loops, the comment volume) I looked at hard and am recommending against touching, with reasons below so the lead does not re-derive them.

---

## APPLY-RECOMMENDED

### 1. `scripts/freshbooks-account-backup.sh:81-82` -- run `jq length` once per collection, not twice

Before:

```bash
  if fetch_collection "$res" "$out"; then
    record "$res" "$(jq 'length' "$out")" ok
    say "$res: $(jq 'length' "$out") items"
```

After:

```bash
  if fetch_collection "$res" "$out"; then
    n="$(jq 'length' "$out")"
    record "$res" "$n" ok
    say "$res: $n items"
```

Behaviour-preserving: the same expression over the same file, evaluated once instead of twice. `$out` is not rewritten between the two calls, so the two current invocations cannot disagree. Same manifest bytes, same log line, same exit status. Saves 24 `jq` processes per run and removes the reader's obligation to check that the two expressions match.

Note `n` needs adding to the `local`-free top-level scope the loop already uses for `out` and `res` -- no declaration needed, and shellcheck at `-S warning` has nothing to say about it.

Risk: **very low.**

### 2. `scripts/freshbooks-account-backup.sh:125` -- stop building a JSON object by string interpolation

Before:

```bash
  --argjson pdfs "{\"skipped\": $([ "$no_pdfs" = true ] && echo true || echo false), \"written\": $pdf_count, \"failed\": $pdf_failed}" \
...
  '{... invoice_pdfs: $pdfs, ...}'
```

After:

```bash
  --argjson skipped "$([ "$no_pdfs" = true ] && echo true || echo false)" \
  --argjson written "$pdf_count" \
  --argjson pdf_failed "$pdf_failed" \
...
  '{... invoice_pdfs: {skipped: $skipped, written: $written, failed: $pdf_failed}, ...}'
```

Behaviour-preserving: the same three values land under the same three keys in the same object. The only difference is that jq assembles the object instead of the shell assembling a JSON literal out of escaped quotes. Key order inside `invoice_pdfs` is preserved as written. The selftest asserts `.invoice_pdfs.skipped` and `.invoice_pdfs.written`, both unchanged.

Why it is worth doing: the escaped-quote literal is the one place in the script where a shell quoting mistake would produce a syntactically invalid manifest, and it is the least readable line in the file. Structure belongs in the filter, values in the args -- which is exactly what every other argument on this call already does.

Risk: **low.**

### 3. `scripts/check.sh:10` -- the comment now undercounts the selftests

The branch adds a third selftest to `run_repo_wide` but leaves the header comment saying:

```
# (actionlint, shellcheck, the two selftests, readme-drift-check) then do
```

After: `# (actionlint, shellcheck, the three selftests, readme-drift-check) then do`.

This is drift the branch introduced. `mise.toml`'s two descriptions were both updated in the same commit; this one was missed. Same class of fix, one word.

Risk: **none** (comment only).

### 4. `docs/building.md:18` -- the gate's own guide does not list `backup-selftest`

The sentence enumerates the module-independent steps: "`actionlint` ..., `shellcheck` ..., `redaction-selftest`, `release-selftest`, and `readme-drift-check`". The branch adds a step to `run_repo_wide` and to both `mise.toml` descriptions but not to the prose that tells a contributor what the repo-wide job runs.

After: insert `backup-selftest` after `release-selftest`.

Note for the lead: that same sentence also omits `site-build`, which Phase 10 added to `run_repo_wide`. That omission is **pre-existing**, not this branch's, so fixing it is the lead's call -- but if the sentence is being edited anyway, fixing both costs nothing and stops the next reader trusting a list that is wrong twice.

Risk: **none** (docs only). Flagging that this overlaps the code-review lane's remit; take it once, not twice.

---

## OPTIONAL

### 5. `scripts/freshbooks-account-backup.sh:125-131` -- one helper for the three shell-to-JSON-boolean idioms

`$([ cond ] && echo true || echo false)` appears three times (the `skipped` value, the `ok` value, and -- in `yes`/`no` form -- the final `say`). A two-line helper

```bash
jbool() { if "$@"; then echo true; else echo false; fi; }
```

turns them into `jbool [ "$no_pdfs" = true ]` and `jbool [ "$failed" -eq 0 ]`. Behaviour-preserving (same two strings out of the same two conditions), and it removes the `&& ... || ...` construct, which is the shell idiom most likely to be misread.

I am not pushing this: three occurrences is right at the line where a helper starts costing more than it saves, and the `say` line's `yes`/`no` variant would not use it. Take it only if proposal 2 is taken and the file is being touched there anyway.

Risk: **low.** Interacts with proposal 2 -- apply in that order.

### 6. `scripts/freshbooks-account-backup.sh:122-129` -- feed the manifest records as jq positional args instead of a tab-joined blob

The work order asked specifically whether the `printf | jq -R -s` construction has a plainer behaviour-preserving form. It does:

```bash
jq -n --arg created "$stamp" --arg version "..." ... \
  '{created_at: $created, ...,
    resources: ($ARGS.positional | map(split("\t") | {(.[0]): {count: (.[1] | tonumber), status: .[2]}}) | add)}' \
  --args "${entries[@]}" >"$dir/manifest.json"
```

This drops the `printf` pipe, the `-R -s` raw-slurp, the `split("\n")`, and the `select(length > 0)` guard that only exists to eat the trailing newline. The records stay tab-joined (the `record()` helper is untouched); only the transport into jq changes.

Behaviour-preserving: I checked it, not just reasoned about it. Running both forms over the same two synthetic records (`clients<TAB>3<TAB>ok`, `callbacks<TAB>0<TAB>failed`) produces byte-identical `-c` output, key order included. No resource name in the collection or singleton lists contains a tab or a newline, so the encoding is unambiguous either way.

**Why this is OPTIONAL and not recommended:** `$ARGS.positional` requires jq >= 1.6. Nothing else in `scripts/` uses `--args`, `$ARGS`, `--rawfile` or `--slurpfile`, and neither `README.md` nor `docs/building.md` states a jq version anywhere -- `docs/backup.md:15` names `jq` as a prerequisite with no floor. So this trades a genuinely simpler five lines for a new, undocumented minimum-version constraint on a tool the repo does not manage through `mise`. Worth it only alongside a one-clause note in `docs/backup.md` ("needs jq 1.6+"), which is arguably a fair trade but is a decision, not a cleanup.

Risk: **medium** -- not to correctness, to portability. Re-run `backup-selftest` after; it asserts four manifest fields and would catch a regression.

### 7. `scripts/backup-selftest.sh:50,68` -- one helper for the two timestamped-directory lookups

```bash
latest_dir() { find "$1" -mindepth 1 -maxdepth 1 -type d | head -1; }
```

Two call sites, identical expression. Behaviour-preserving. Marginal at two uses; take it only if a third run is ever added.

Risk: **very low.**

---

## DO-NOT-APPLY

### 8. Replacing the `list --help | grep -q -- '--all'` probe with a hardcoded resource list

`registry.go:203` registers `--all` only when the command's `HasAll` is set, so the set of `--all`-capable resources is knowable at authoring time and could be baked into a bash array, removing 24 subprocesses.

**Rejected.** The probe asks the binary that is actually about to run. A hardcoded list is correct on the day it is written and silently wrong afterwards: a resource that gains `--all` in the CLI would keep being page-walked, and one that loses it would take the `--all` branch and fail. That is a behaviour change deferred into the future rather than avoided, which is worse than a behaviour change today. The probe also costs nothing that matters -- `--help` is local, no network, and the run it precedes is 27 authenticated API sweeps plus one request per invoice. There is no `inventory-check`-style gate that would catch the list drifting, so nothing would fail loudly.

### 9. Attempting `--all` unconditionally and falling back to the page walk on failure

**Rejected -- changes observable behaviour.** A genuine API failure (auth, 5xx, rate limit) on a resource that does have `--all` would currently be recorded as `failed`; under a fallback it would trigger a second, differently shaped set of requests, and only then fail -- doubling the load against an account that is already erroring, and writing a different sequence of lines into `backup.log`. The manifest could also flip from `failed` to `ok` for a resource whose `--all` sweep died halfway. Not a simplification.

### 10. Folding the collection loop (`:78-89`) and the singleton loop (`:91-104`) into one dispatch function

The two loops share the record-or-cleanup tail: `rm -f "$out"`, `record ... 0 failed`, `failed=1`, `say "... FAILED (see backup.log)"`.

**Rejected.** The heads are genuinely different -- one calls `fetch_collection` and derives a count from the file, the other splits a `name|command` pair, word-splits the command on purpose (with its own shellcheck disable), and records a fixed count of 1. A unifying helper would need a command callback plus a count callback, which is more machinery than the four duplicated lines it removes, and it would push the `SC2086` disable into a shared function where its "on purpose" justification no longer sits next to the code it excuses. The current shape reads top-to-bottom in one pass; the merged shape would not.

### 11. Trimming the script and selftest header comments

`freshbooks-account-backup.sh` opens with 15 comment lines, `backup-selftest.sh` with 8.

**Rejected.** This matches the exemplars exactly -- `redaction-selftest.sh` opens with 14 comment lines and `release-selftest.sh` with 11, both explaining *why* the fakes exist and what is deliberately not faked. The backup script's header carries the same kind of load-bearing content: the flags-are-not-forwarded constraint, the exit-1-for-cron contract, the 0700 rationale. None of it restates the code. Cutting it would move the file away from house style, not toward it.

### 12. `docs/backup.md:35` -- dropping the spelled-out 24-resource list because `collections=()` already has it

**Rejected.** `docs/backup.md` is published to the Docusaurus site (`site-sync.sh`, `sidebars.js`), where the reader has no repo checkout in front of them. A guide that says "see the array in the script" is worse for its actual audience. The duplication is real but it is the correct kind. Same reasoning for the `--all`-vs-page-walk sentence in the same paragraph, and for the `docs/cli.md` preamble paragraph overlapping `docs/backup.md`'s opening -- the preamble is a pointer from a different document, sized like one.

### 13. `scripts/backup-selftest.sh:51` vs `:60` -- dropping one of the "did the run fail" assertions

`:51` asserts exit status 1; `:60` asserts `.ok == false` in the manifest. Both trace back to `failed=1`.

**Rejected -- these are two different contracts on two different surfaces.** The exit status is what a cron job reads; `ok` is what a human or a later script reads out of the manifest. A change that set one without the other is exactly the regression a selftest exists to catch. I found no assertion in the file that is genuinely made twice.

---

## Deliberately left alone

- **`fetch_collection` (`:58-76`)** -- three genuinely distinct response shapes (`--all`, page-walked object, bare array) in 19 lines with an early return for each. No cheaper structure without losing a case.
- **`umask 077` + `mkdir -p` (`:46-47`)** and the 0700 assertion in the selftest -- security-relevant, and the security lane's call, not mine.
- **The fake CLI's `case` ordering (`selftest:24-39`)** -- specific patterns before `*" list ..."` catch-alls is the only ordering that works, and it reads correctly as written.
- **`mise.toml` / `check.sh` / `site-sync.sh` / `sidebars.js` wiring** -- each addition is one line in an existing list, in the established position. Nothing to cut.
- **The `docs/cli.md` preamble** -- generated from `docsgen.go`; the two files agree, and the generated copy must not be hand-edited.
- **`record()` / `say()` (`:52-53`)** -- two one-liners, used 27 and 30 times respectively. Correctly factored already.
