# Phase 9 code review -- release automation

Lane: code review (read-only). Branch `phase-9/release-automation`, `7738689..12b0e88` (5 commits) vs `main`.
Scope: `scripts/release.sh`, `scripts/release-selftest.sh`, `scripts/check.sh` + `mise.toml` wiring, `docs/building.md`, `docs/progress.md`, root `CHANGELOG.md`, against `docs/phases/9/plan.md` D1-D8 and the hand-run inventory.

## Verdict

**REQUEST CHANGES** -- 4 blocking, 13 advisory.

The design is right and D1 (dry-run) is genuinely airtight: I read every write path and then exercised `cut`/`bump`/`docs`/`all` under `--dry-run` against the real repo with a before/after hash of the working tree, HEAD, the tag list, and all five changelogs plus README.md -- zero writes escaped. D8's guards, the output contract, `usage lint`, ASCII cleanliness, and the absence of any `FRESHBOOKS_*`/`GITHUB_TOKEN` read all check out.

What does not hold up is the half the self-test cannot see. **D3 resume is broken on the real repository** -- provably, deterministically, with the script's own function -- and it is broken in exactly the situation resume exists for. The self-test's resume probe passes because its scratch repo has three commits. Separately, run discovery races GitHub, and the two defects compound: the most likely real failure (R2) lands the operator in a state the resume path (R1) cannot recover from.

## BLOCKING

### R1 -- `commit_with_subject_exists` always returns false on the real repo; D3 resume never fires

`scripts/release.sh:150-152`

```bash
commit_with_subject_exists() {
  git -C "$repo_root" log --format=%s 2>/dev/null | grep -qxF "$1"
}
```

`set -o pipefail` is in force (`:23`). `grep -q` exits the moment it matches; `git log` is still streaming and dies with SIGPIPE; the pipeline's status becomes 141, and the function reports "not found".

Evidence -- the function copied verbatim out of `release.sh`, run against this repo (183 commits on `main`):

```
MISSED (rc=1)     : docs: ship v0.3.0                                          (genuinely absent)
MISSED (rc=141)   : release(freshbooks): v0.3.0                                (present, 1 match)
MISSED (rc=141)   : release(mcp,cli): require freshbooks v0.3.0 and cut 0.1.2  (present, 1 match)
```

It only returns true when the match is the *last* line of `git log` output (the repo's oldest commit) -- i.e. never in a real resume, where the release commit is at or near HEAD.

Failure scenario: `cut freshbooks 0.4.0` commits and pushes, then `cut-ci-watch` fails. Operator re-runs. `cut-changelog` correctly SKIPs (`changelog_has_section` is grep-only, no pipe -- fine). `cut-commit` does *not* SKIP, so the script runs `git add` (nothing changed) then `git commit` -> "nothing to commit" -> exit 1 -> `release: FAIL cut-commit`. Same for `bump-commit` and `all-ship-commit`. The resume story in `docs/building.md` ("re-running `all`/`cut`/`bump` after a partial failure is safe") is false as shipped.

Fix: drop the pipe, or neutralise it.

```bash
commit_with_subject_exists() {
  [ -n "$(git -C "$repo_root" log --format=%s --fixed-strings --grep="$1" -1 2>/dev/null)" ]
}
```
(`--grep` is a substring match, so also compare the returned subject to `$1` for exactness, or keep the pipeline and wrap it: `if git log --format=%s | { grep -qxF "$1" || false; }` still trips pipefail -- prefer letting git do the filtering.) Note the comment at `:142-149` says "search all of main's history" but the command reads HEAD's history; pass `main` explicitly.

### R2 -- no retry on CI/Release run discovery: `watch_run` races GitHub

`scripts/release.sh:301-314`, callers at `:698`, `:719`, `:747`, `:862`, `:924`

`watch_run` does one `gh run list --limit 1` immediately after the push that triggers the run, and treats anything unexpected as terminal:

- CI (`expect_sha` given): GitHub takes seconds to register a workflow run. The one-shot list returns the *previous* run for the *previous* sha -> `step_fail "$step" "a CI run for $expect_sha" "latest run is for $sha"`.
- Release (`--branch <tag>`, no `expect_sha`): the tag's first run does not exist yet -> `run_json` empty -> `step_fail ... "no run found"`.

There is no poll around discovery -- only around the *conclusion* of an already-found run. Every real `cut`/`bump`/`all` invocation pushes and then immediately lists, so this races on every run and will hit often.

The self-test cannot catch it: the fake `gh run list` (`scripts/release-selftest.sh:91-101`) always returns `"status":"completed"` with `headSha` = `git rev-parse "$branch"`, so the expected run is always already there.

Compounds with R1: a discovery race fails `cut-ci-watch`, and the resumed run then fails at `cut-commit` instead of skipping it -- the operator has to finish by hand, which is what this phase set out to eliminate.

Fix: wrap discovery in the same poll loop as the conclusion check -- retry the `gh run list` for up to (say) 120s until a run whose `headSha` matches `expect_sha` appears (or, for Release, until any run for the tag appears), and only then fall through to the conclusion poll. Fold the discovery wait into `TIMEOUT`.

### R3 -- `changelog_add_bullet` injects a blank line into `[Unreleased]` on every call; the root changelog degrades on every release

`scripts/release.sh:258-262`

```bash
{
  printf '%s\n\n' "$before"     # $before already ends at "## [Unreleased]"
  printf '%s\n\n' "$section"    # $section still begins with the file's own blank line
  printf '%s\n' "$after"
} >"$tmp"
```

`before` ends with the `## [Unreleased]` line and gets a hard `\n\n`; `section` (from `changelog_unreleased_section`) still carries the original blank line that followed the heading. One extra blank per call, and the root `[Unreleased]` section is never cut, so it accumulates without bound.

Evidence -- three calls (exactly what one `all` run makes: one in `cut_lib:670`, two in `do_bump:820`) against the real root `CHANGELOG.md`:

```
## [Unreleased]<EOL>
<EOL>
<EOL>
<EOL>
<EOL>
### Added<EOL>
- bullet C<EOL>
```

Four blank lines after one release; seven after two. Nothing in the gate checks changelog formatting, so this ships silently into a public file.

Fix: strip leading blanks from `section` before emitting (the `else` branch at `:250` already does `sed '/./,$!d'` -- apply it unconditionally, before the `grep -qxF "### $heading"` branch), and emit `printf '%s\n' "$before"` + one explicit blank.

### R4 -- `preflight-ci-green` does not check that the green run is HEAD's

`scripts/release.sh:376-382`

```bash
sha=$(git -C "$repo_root" rev-parse HEAD)          # assigned, never read
ci_conclusion=$(gh_jq '.[0].conclusion // "none"' -- run list --workflow CI --branch main --limit 1 --json headSha,conclusion) || ci_conclusion="none"
```

`headSha` is requested in `--json` and then discarded; `sha` is dead (shellcheck would flag it, but shellcheck is not in the gate). The plan's precondition is explicit: "`gh run list --workflow CI --branch main --limit 1` conclusion `success` **for HEAD**". As written, preflight passes whenever the most recent CI run on `main` was green -- including when local `main` is ahead of what CI has seen, or when the newest run for HEAD is still in progress and the previous green run is what `--limit 1` returns after a filter shift. That is the one precondition standing between an unbuilt commit and a tag push.

Fix: pull `.[0].headSha` alongside the conclusion and `step_fail` unless it equals `$sha`; treat `status != completed` as not-green.

## ADVISORY

### R5 -- `changelog_cut_section` eats the blank line under the new version heading

`scripts/release.sh:211-226`. The `skip_blank` machinery drops the blank line following `## [Unreleased]`, so the cut section reads `## [0.1.3] - 2026-09-03` / `### Changed` with no separator, unlike every existing section in all four changelogs (`## [0.1.2] - 2026-09-03` / blank / `### Changed`). Verified against real `freshbooks/CHANGELOG.md` and `mcp/CHANGELOG.md`. `scripts/changelog-section.sh` still extracts it, so the Release guard is unaffected -- purely a style regression, but on every future release. Fix: drop `skip_blank` and print the blank after the new heading instead of before it.

### R6 -- self-test probes that cannot fail

`scripts/release-selftest.sh`

- **The resume probe (`:459-471`) passes vacuously on R1.** Its scratch repo has 3 commits, so `git log` finishes writing before `grep -q` exits and no SIGPIPE occurs. Demonstrated: identical code returns FOUND in a 6-commit repo and `rc=141` in a 200-commit one. The implementer's claim that the probe "failed against the literal-`git log -1` version and passes now" is true only at scratch scale. Fix: seed the scratch repo with ~200 commits (a `for` loop of empty commits costs ~1s), or assert on a repo built from `git log` output large enough to fill a pipe buffer.
- The same probe asserts only `SKIP cut-commit` and `SKIP cut-tag-push` despite the message "SKIP for every mutating step" -- `bump-commit`, `bump-go-get-*`, `all-ship-commit` are unasserted.
- **R2 is unreachable** -- the fake `gh run list` (`:91-101`) always answers with a completed run at the branch tip. Add a probe where the fake returns a stale `headSha` (or an empty array) on the first call and the real one on the second, asserting the script waits rather than failing.
- **No probe covers the `verify-release` assertions themselves** (`isDraft`, `name`, 0-vs-13 asset count). Deleting the draft check or the asset-count check from `do_verify` would keep the suite green. Add a `FAKE_GH_RELEASE_DRAFT=true` / `FAKE_GH_ASSET_COUNT` knob and two probes.
- **No probe checks what `docs` writes.** The happy path runs `cmd_docs` for real but never asserts `README.md` now contains `freshbooks/v0.9.0`. D5's correctness rests entirely on the gate's idempotence check, which only proves it does not change a *correct* README.
- `scripts/check.sh` is stubbed to `exit 0` in the scratch repo (`:291`), so `bump-check-*` is structurally untested. Acceptable, but worth a comment saying so.

### R7 -- a stale local tag gets pushed, and `OK cut-tag` is printed for a tag that was not created

`scripts/release.sh:705-712` and `:733-740`

```bash
if ! git -C "$repo_root" rev-parse "$tag" >/dev/null 2>&1; then
  run_cmd git ... tag -a "$tag" ...
fi
step_ok "cut-tag"
run_cmd git ... push origin "$tag"
```

If an aborted earlier run left `freshbooks/v0.4.0` pointing at a commit that has since been amended or superseded, the local tag is silently accepted and pushed at the *stale* commit -- and `cut-tag` reports OK as if it had just been created. Fix: when the local tag exists, compare `git rev-parse "$tag^{commit}"` to `git rev-parse HEAD` and `step_fail` on mismatch; `step_skip "cut-tag"` when it already matches.

On the remote-side check (`:701`, `:729`): `git ls-remote --tags origin "refs/tags/$tag" | grep -q "$tag"` does match both the tag object and its `^{}` peel, so a hand-pushed *lightweight* tag false-SKIPs the annotated-tag creation. Since the Release workflow has already run on that tag, skipping is the right outcome -- flagging only so the behaviour is deliberate rather than accidental.

### R8 -- the gate mutates `README.md`

`scripts/check.sh:111-127`. `run_readme_drift_check` runs `scripts/release.sh docs` for real, which rewrites `README.md` in place, then diffs. Two consequences: (a) when tags have genuinely moved, a failing gate leaves the working tree modified (and the error message tells you to run the command it just ran); (b) an unrelated pre-existing edit to `README.md` is picked up by `git diff -- README.md` and misreported as Status-column drift. Fix: have `release.sh docs` accept an output path (or copy `README.md` to a temp file, run the rewrite there, and `diff` the two) so the check is read-only.

### R9 -- D5 half-wired: `mise run docs` does not refresh the Status column

D5 asks for both a `scripts/docs.sh` call *and* a drift check. `scripts/docs.sh` is unchanged on this branch (confirmed: empty diff), so the only ways to fix Status drift are `mise run release -- docs` or tripping the gate. Add the call to `scripts/docs.sh` after `docs/cli.md` generation, as specified.

### R10 -- `all`'s ship commit drops half of plan step 18

`scripts/release.sh:895-916`. Step 18 is "README Status rows; `docs/progress.md` ledger row; commit `docs: ship vX.Y.Z`". The script stages `README.md` only and leaves progress.md to the operator via a source comment -- nothing in the *output* tells them. Every precedent ship commit also retargeted `GOAL.md` (`docs: ship Phase 8 and retarget GOAL.md to Phase 9`, `docs: ship v0.1.0 and retarget GOAL.md ...`). Fix: print a step line, e.g. `release: OK all-ship-reminder -- write the docs/progress.md ledger row and retarget GOAL.md, then amend or follow up`, and/or stage `docs/progress.md` and `GOAL.md` when they are already modified.

### R11 -- `bump`'s patch default should say something when the module changelog is additive

`scripts/release.sh:767-773`. `propose_binary_version` is a blind patch bump of the shared mcp/cli tag and never looks at the module's `[Unreleased]`. The implementer's reasoning is sound and matches precedent: 0.1.1 (dependency-only) and 0.1.2 (which shipped a new 169th tool under `### Added`) were both patches, and pre-1.0 that is defensible. But `bump` is where a genuinely additive mcp/cli release silently gets under-bumped, and the operator gets no signal. Fix: run `derive_bump_kind` on each module changelog for information only and fold it into the step line -- `release: OK bump-version-propose -- mcp/cli 0.1.2 -> 0.1.3 (patch; note: mcp [Unreleased] has ### Added -- pass --binary-version to bump minor)`. Do not change the default.

### R12 -- bare `jq` is a new dependency; the plan said prefer `gh --jq`

`scripts/release.sh:435-440` shells out to `jq` three times; `scripts/release-selftest.sh:80` does too, inside the fake `gh`. The plan's Rules allow "`jq` if already on the machine -- prefer `gh --jq`", and the script already has a `gh_jq` helper (`:117-123`) that would do this without the dependency. On a machine without `jq` the failure message is misleading ("valid JSON from gh release view", "unparsable" -- when the real cause is `jq: command not found`). Fix: `read_cmd gh release view "$tag" --json isDraft,name,assets --jq '[.isDraft, .name, (.assets|length)] | @tsv'` and read the three fields with one `read`.

### R13 -- temp dirs leak on failure

Only `LOG_FILE` has a trap (`:50`). `do_verify_binary`'s `workdir` and `gobin_dir` (`:487`, `:522`) are removed at `:560` -- unreachable on any of the eight `step_fail` paths between them. `changelog_cut_section`/`changelog_add_bullet` `mktemp` files (`:210`, `:235`) leak if `awk` fails. Fix: extend the EXIT trap to a cleanup array, or `trap 'rm -rf "$workdir" "$gobin_dir"' RETURN` inside `do_verify_binary`.

### R14 -- some failures exit silently, bypassing the output contract

A failing command substitution in a plain assignment trips `set -e` with no `release: FAIL` line printed:

- `:317-318` `status=$(gh_jq '.status' -- run view "$id" --json status)` -- a transient `gh` failure mid-poll kills the run with no diagnosis.
- `:310-311` `id=`/`sha=` -- if the JSON lacks `databaseId`, `grep -oE` returns 1 and the script exits silently.
- `:402`/`:411` in `cmd_docs` -- if `README.md` is missing, `awk` fails and the script exits with nothing printed.

Fix: `|| step_fail ...` on each, per D2.

### R15 -- cosmetic / contract nits

- `:392` uses `dry_echo` (prefix `dry-run:`) for the tag-ruleset line, which is printed on *every* preflight, dry-run or not. D8 wants it printed; the prefix is misleading outside `--dry-run`. Use a plain informational form, or fold it into the `preflight-tag-ruleset-warn` step line.
- `:667` the dry-run echo of the root bullet drops the backticks the real call at `:670` adds -- the dry-run transcript does not match what would actually be written.
- `:273` `module_kind` writes `release: unknown module: bogus` to stderr *in addition to* the `release: FAIL` line. Confirmed live. Minor D2 violation.
- `:399` `local module tag readme=".README.md" tmp` -- `readme` is immediately overwritten on the next line. Dead assignment.
- `:633` `accept_proposed_version` prints `release: OK version-propose -- <detail>`, extending the `release: OK <step>` shape with a `--` suffix that D2 reserves for SKIP/FAIL. Harmless and useful; worth one line in D2 or the docs so it is deliberate.
- `shellcheck` is clean at `-S warning` (only SC1008 for the `usage` shebang and SC2154 for `usage_subcommand`, both expected) but is **not wired into the gate**. Given two 500-900 line bash scripts now carry push/tag power, consider adding `shellcheck -S warning -e SC1008,SC2154` as a gate step -- it would have caught R4's dead `sha` and R15's dead `readme`.

### R16 -- `docs/progress.md` "Next action" is stale

`docs/progress.md`, the section after the rewritten "Release flow" block, still reads "Next action: Phase 9 (Docusaurus docs site, unattended)" while `GOAL.md` retargeted Phase 9 to release automation in `3cec68d`. The implementer correctly flagged this as the lead's ship-time job -- noting it so it does not merge stale.

## What checked out

- **D1 dry-run: no write escapes.** Audited path by path (changelog edits `:665-671`/`:813-821`, git add/commit/push/tag via `run_cmd` `:78-85`, `go get`/`mod tidy` `:792-794`, `scripts/check.sh` `:829-830`, README rewrite `:404-406`, dogfood copy `:551-556`, `verify_after_cut` `:576-584`), then confirmed empirically: `cut freshbooks 0.4.0 --dry-run`, `bump 0.4.0 --dry-run`, `bump 0.4.0 --binary-version 0.2.0 --dry-run`, `docs --dry-run` and `all 0.4.0 --dry-run` left the working tree, HEAD, the tag list, `README.md` and all four changelogs byte-identical. The only real network touches under `--dry-run` are reads (`gh run list`, `gh auth status`, `git ls-remote`), which is what D1 specifies.
- **D2**: `release: OK/SKIP/FAIL` shape holds; `dry-run:` echoes go to stdout, FAIL to stderr with a 40-line log tail; no `FRESHBOOKS_*`/`GITHUB_TOKEN` read anywhere (only a comment mentions them).
- **D8**: `require_main_unless_dry_run` (`:337-343`) is called from `cut`, `bump`, `all` and not from `verify`/`docs`, as specified. No `--force`, no tag deletion, no ruleset write anywhere in the script.
- **`verify`'s assertions match the inventory**: `isDraft=false`, `name == tag`, 0 assets for the lib and 13 for the binaries (which matches `release.yml`: 6 archives + 6 SBOMs + `checksums.txt`); `sha256sum -c checksums.txt --ignore-missing`; extracted binary prints `freshbooks-mcp X.Y.Z` / `X.Y.Z`; `go install` prints `freshbooks-mcp vX.Y.Z` / `vX.Y.Z` -- the version-vs-v-version distinction between the goreleaser build and the `go install` build is correctly modelled; `GOFLAGS=-mod=mod`, `cd /tmp`, and `GO_BIN` resolved by absolute path via `mise where go` all match plan steps 8 and 16; the `md2man`/`blackfriday` check is cli-only.
- **Workflow names** `CI` and `Release` match `.github/workflows/*.yml`. `--limit 1` + 30s poll + 1200s cap match D4.
- **`local` scoping**: audited all 14 multi-assignment `local` lines. No remaining same-statement self-references; the one at `:786` reads the enclosing `for` loop's `module`, which is already assigned. The `cut_lib` fix at `:658-659` is correct and the comment explaining it is worth keeping.
- **`set -e` AND-OR guards** (`:565`, `:590`, `:870`) do not trip `set -e` on the happy path -- verified empirically.
- **`usage lint`** clean on both scripts. Both files are ASCII-only and unwrapped, as are `docs/building.md`, `docs/progress.md`, the root `CHANGELOG.md` line, and the phase docs.
- **`docs/building.md`** accurately describes the six subcommands and all four flags against the implementation; the root `CHANGELOG.md` bullet is in the right file (process/CI/docs) and no module changelog was touched, per D7.
- **`mise.toml`**: `[tasks.release]`, `[tasks.release-selftest]` added and the `check` description updated to match the new step list.

## Triage suggestion

R1, R2 and R4 are the ones that decide whether this can be trusted for the v0.4.0 cut; R1 and R2 are each a handful of lines. R3 and R5 are one rewrite of the two changelog helpers plus a formatting assertion in the self-test. R6 (seed the scratch repo deep enough, plus a stale-run probe) is what stops R1/R2 from coming back.
