# Phase 9 security review -- release automation

Branch `phase-9/release-automation` (`7738689..12b0e88`, 5 commits). Reviewed `scripts/release.sh` (941 lines, new), `scripts/release-selftest.sh` (536 lines, new), `scripts/check.sh`, `mise.toml`, `docs/building.md`, `docs/progress.md`, `CHANGELOG.md`, against `docs/phases/9/plan.md` D1-D8 and `.github/workflows/release.yml`.

## Verdict: BLOCK

Three blocking findings. Two of them (A1, A3) are direct misses against the plan's own guards; A2 is an unguarded write to a tracked file inside the gate. All three are small, local fixes.

The good news up front: no destructive git anywhere, no token handling anywhere, the self-test is provably hermetic, and `--dry-run` performs literally zero writes -- all four verified empirically, not by reading.

---

## BLOCKING

### A1 -- `cut mcp|cli <version>` pushes a public tag with zero CI verification

`scripts/release.sh:726-749` (`cut_binary`)

`cut_lib` gates its tag push behind `watch_run "cut-ci-watch" "CI" "main" "$sha"` (`:692-699`), which pins the exact pushed sha and `step_fail`s on any conclusion but `success`. `cut_binary` has no equivalent: it goes straight from `ls-remote` to `git tag -a` to `git push origin "$tag"` to the Release watch. `cmd_cut` (`:588-606`) never calls `cmd_preflight`, so the `preflight-ci-green` check (`:377-382`) is not in the path either.

**Evidence** -- ran the standalone subcommand against the self-test's fake `gh` answering `failure` for every CI run:

```
===== cut mcp 0.9.0 --yes with FAKE_GH_CI_CONCLUSION=failure =====
release: OK cut-tag
release: OK cut-tag-push
release: OK cut-release-watch
release: OK verify-release
... exit: 0
origin tags after:
b36e1824...  refs/tags/mcp/v0.9.0
354e43ad...  refs/tags/mcp/v0.9.0^{}
```

For contrast, the same probe against `cut freshbooks 0.9.0 --yes` behaves correctly:

```
release: OK cut-push
release: FAIL cut-ci-watch -- expected conclusion=success, observed conclusion=failure
... exit: 1
origin tags after: (none)
local tags after:  (none)
```

**Failure scenario.** `main` is red. Operator runs the documented `mise run release -- cut mcp 0.1.3 --yes` (`docs/building.md` lists `cut <module> <version>` as a first-class subcommand). A tag lands on the public GitHub repo off a broken tree. `release.yml`'s `ci` job then blocks the *release*, but the tag itself is permanent -- and D8 forbids the script from ever deleting a tag, so there is no automated recovery. This is exactly the property the work order names first.

**Fix.** Give `cut_binary` the same gate `cut_lib` has: before the `confirm_first_push`/tag block, `watch_run "cut-ci-watch" "CI" "main" "$(git -C "$repo_root" rev-parse HEAD)"` (skipped under `--dry-run`, as in `cut_lib:692-699`). Inside `cmd_all` this is a cheap no-op -- the run is already green from `do_bump`'s watch -- and it closes the standalone path. Add the matching self-test probe (see A6).

---

### A2 -- the gate silently overwrites tracked `README.md` in the working tree

`scripts/check.sh:111-127` (`run_readme_drift_check`), reachable from `mise run check` and from every CI job

`run_readme_drift_check` runs `"$repo_root/scripts/release.sh" docs` **for real** -- no `--dry-run` -- and `cmd_docs` (`scripts/release.sh:398-423`) rewrites `README.md` in place via `mv "$tmp" "$readme"`. A verification step must not mutate tracked source.

Two concrete consequences:

1. **Silent data loss.** An operator with an uncommitted edit to a Status cell loses it, and the gate reports OK.
2. **Dirty tree on failure.** When the README genuinely is stale, the rewrite happens, `git diff` is non-empty, `return 1` aborts under `set -e`, and the modified README is left behind for the operator to clean up by hand.

**Evidence** -- corrupted the committed Status cell, then ran the gate step:

```
$ sed -i 's|`freshbooks/v0.3.0`|`freshbooks/v0.0.1`|' README.md
$ git status --porcelain README.md
 M README.md
$ scripts/check.sh readme-drift-check
== readme-drift-check ==
release: OK docs
check.sh: readme-drift-check OK
EXIT=0
$ git status --porcelain README.md
        <- empty: my edit was silently reverted, and the gate said OK
```

**Fix.** Make the check non-mutating. Either render to a temp copy and diff it (`RELEASE_README=<tmp> scripts/release.sh docs` after parameterising `cmd_docs`'s target), or drive it off `release.sh docs --dry-run`, which already prints the exact proposed cell per module (`dry-run: rewrite README.md Status cell for freshbooks -> freshbooks/v0.3.0`) and can be compared against the committed table without touching the file. The in-repo precedent -- `docs_drift_test.go` -- generates into a temp dir, not over the tracked file.

---

### A3 -- no semver validation on `<version>` / `--binary-version`

`scripts/release.sh:589-590` (`cmd_cut`), `:754-755` (`cmd_bump`), `:869` (`cmd_all`), consumed at `:599`, `:706-711`, `:734-739`, `:139`, `:211`, `:799`

The module argument *is* validated -- `module_kind` (`:268-277`) rejects anything outside `freshbooks|mcp|cli`, confirmed: `cut evil 1.0.0` exits 1. The version argument is never validated at all. It flows unchecked into `tag="$module/v$version"`, `git tag -a`, `git push origin "$tag"`, `go get ...@v$version`, an `awk -v ver=` substitution (`:211`) and a `grep -E` pattern (`:139`).

**No command injection exists** -- every call site passes argv arrays, and the script contains no `eval`, no backticks, and no unquoted expansion into a command string. The exposure is different:

1. **A malformed tag reaches the public remote.** `cut freshbooks 1.0 --dry-run` walks all the way to the push:
   ```
   dry-run: git -C <repo> tag -a freshbooks/v1.0 -m freshbooks 1.0
   release: OK cut-tag
   dry-run: git -C <repo> push origin freshbooks/v1.0
   release: OK cut-tag-push
   ```
   `release.yml`'s `guard` job does enforce `^[0-9]+\.[0-9]+\.[0-9]+$`, so no bad *release* gets published -- a real compensating control. But the tag is already public, and D8 forbids the script from deleting it.

2. **`changelog_has_section` (`:139`) treats the version as a regex,** so a metacharacter turns the resume check into a false positive and silently skips the changelog cut:
   ```
   version [0.4.0] -> no section (cut would proceed)
   version [.*]    -> HAS SECTION (cut would SKIP)     <- matches "## [Unreleased]"
   version [0.3.0] -> HAS SECTION (cut would SKIP)
   ```

**Fix.** One guard at the top of `cmd_cut`/`cmd_bump`/`cmd_all`, mirroring the workflow's regex so the two cannot drift:

```bash
require_semver() {
  [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
    step_fail "version-guard" "strict semver X.Y.Z" "$1"
}
```

Apply it to `$version`, `$lib_version` and `$BINARY_VERSION` after the `auto` branch resolves. Separately, drop the regex fallback in `changelog_has_section` and keep only the `grep -qxF` literal match.

---

## ADVISORY

**A4 -- orphaned temp dirs on every `verify` failure.** `scripts/release.sh:488,522`: `workdir` and `gobin_dir` are `mktemp -d`'d with no `trap`, and the `rm -rf` at `:560` only runs on the success path. Every `step_fail` between `:493` and `:558` leaks them. Proven: one self-test run left two behind (`/tmp/tmp.Ri3etOeNl1`, `/tmp/tmp.SqFJHFqpsJ`) holding the downloaded tarball, `checksums.txt` and an extracted binary. Mode `0700`, so no confidentiality issue -- unbounded `/tmp` growth. Fix: extend the existing EXIT trap (`:50`) to cover them.

**A5 -- `cut` and `bump` never check the working tree is clean.** `scripts/release.sh:588-606`, `:753-761`. Only `preflight` checks (`:359-364`), and only `all` calls `preflight`. The commits stage explicit paths, so unrelated changes are not swept in, but combined with A1 this makes `cut` the unguarded entry point to a public tag push. Note the guard does work where it is wired: `all 0.9.9 --dry-run` refused on this branch with `release: FAIL preflight-clean-tree`.

**A6 -- the self-test never exercises a red CI.** The fake `gh` supports `FAKE_GH_CI_CONCLUSION` (`scripts/release-selftest.sh:97,108`), but no probe sets it -- only `FAKE_GH_RELEASE_CONCLUSION` (`:488`). The single most safety-critical property in the script is untested. I verified it by hand (see A1 evidence) and it holds for `cut_lib`; it should be a permanent probe, and a second one should assert A1's fix.

**A7 -- `watch_run` races GitHub's run-creation delay.** `scripts/release.sh:305-314`: `gh run list --limit 1` fires immediately after the push. If Actions has not registered the run yet, the latest run is the *previous* commit's, the sha check trips, and the step FAILs; if there is no run at all, `no run found` FAILs. Fail-closed, so not a safety hole -- but it will produce spurious failures on real runs and push operators toward `--yes` reruns. Fix: poll `run list` for a run matching `expect_sha` for ~2 minutes before giving up.

**A8 -- the Release watch has no sha binding.** `scripts/release.sh:719,747` call `watch_run` with no `expect_sha`, trusting the `--branch <tag>` filter alone. Sound today because D8 forbids force-pushing or deleting tags, so a tag maps to exactly one ref -- but it is the one watch without the belt-and-braces check the CI watch has.

**A9 -- external `jq` dependency contradicts the stated design.** `scripts/release.sh:435-440` shells out to `jq -r`, while the comment at `:115` says "gh has an embedded jq; no external jq dependency" and the plan says "prefer `gh --jq`". Fails closed if `jq` is absent (present here at `/usr/bin/jq`). Fix: use the existing `gh_jq` helper, or amend the comment.

**A10 -- `read -ra args` mangles arguments containing whitespace.** `scripts/release.sh:41` re-word-splits `$usage_args`, so a quoted multi-word argument is silently split rather than rejected. Proven: `cut freshbooks '0.0.0 --force'` yields version `'0.0.0`, not an error. Harmless once A3's semver guard lands (any mangled version fails the regex), but worth knowing.

**A11 -- `step_fail` dumps 40 lines of `$LOG_FILE` to stderr.** `scripts/release.sh:64-69`. `$LOG_FILE` captures raw `gh auth status` output (`:366`). `gh` masks the token itself unless `--show-token` is passed, and the script never passes it -- so this is currently safe. Keep it that way: never add `--show-token`, and treat the log tail as operator-pasteable output.

**A12 -- module-independent gate steps run three times in CI.** `.github/workflows/ci.yml:22,34,46` each run `mise run check -- <module>`, and `check.sh`'s `all` branch (`:163-168`) runs `actionlint`, `redaction-selftest`, `release-selftest`, `build` and `readme-drift-check` unconditionally. `release-selftest` takes ~10s, so that is ~30s of duplicated work per CI, plus three README rewrites.

**A13 -- stale comment in the self-test.** `scripts/release-selftest.sh:294`: "init -b main above already did this" refers to `:273`, which inits the *bare origin*, not `$work`. The `|| true` makes it harmless; the comment is wrong.

**A14 -- `preflight` reaches the real `mise`.** `scripts/release.sh:384` runs `mise which go` unqualified, so the self-test's `preflight` probes depend on host tooling. Local-only, no network (confirmed by the namespace run below), so this does not break D6's claim -- just noting the one shim that is not faked.

---

## What passed, with evidence

**No destructive git (D8).** `grep -nE '\-\-force|push +-f|--delete|tag -d|reset|checkout|clean |restore|filter-branch|update-ref'` over both scripts returns only `step_fail "preflight-clean-tree"` (a string), `rm -rf "$workdir" "$gobin_dir"` (`release.sh:560`, both `mktemp -d` results), `rm -rf "$scratch"` / `rm -rf "$stubdir"` (self-test, both `mktemp -d`), and `git checkout -q -b not-main` (self-test, a branch *create* on the scratch repo). No force, no tag deletion, no reset, no ruleset mutation -- `preflight:392` only `dry_echo`s the `gh api` call, unconditionally.

**Branch guard works.** `require_main_unless_dry_run` (`:337-343`) is called by `cut`, `bump` and `all`; `preflight` FAILs off main (`:356`). Both self-test probes pass, and `all 0.9.9 --dry-run` on this branch printed `SKIP preflight-branch` then correctly `FAIL preflight-clean-tree`.

**Token hygiene (D2).** `grep -nE 'FRESHBOOKS|GITHUB_TOKEN|GH_TOKEN|credential|netrc|set -x'` over `release.sh` matches only the comment at `:15`. No reference to the CLI credentials file. All `gh` JSON is parsed through `--jq` or `jq`, never echoed raw. The dogfood copy writes only to `$RELEASE_LOCAL_BIN` (`:47`, default `~/.local/bin`), and the self-test redirects it to a scratch dir (`release-selftest.sh:322`).

**Self-test isolation (D6).** Every `git` invocation targets `$w`/`$work`/`$origin`/`$FAKE_GH_REPO_DIR`, all under one `mktemp -d` (`:21`); `repo_root` is used only to *read* `scripts/release.sh` (`:19`, `:289`), never as a git target. `trap 'rm -rf "$scratch"' EXIT` (`:22`) fires on failure paths too. `GH_HOST=localhost`, fake `gh` via PATH prepend, fake `go` via `RELEASE_GO_BIN` absolute path (`:319-329`); the fake `gh` never shells out to a network tool. **Proven network-free** -- the full suite passes inside a namespace with no interfaces:

```
$ sudo unshare -n runuser -u <operator> -- env HOME=... bash -c 'scripts/release-selftest.sh'
release-selftest: PASS preflight FAILs on a dirty tree
... 10 PASS ...
release-selftest: OK
```

**`--dry-run` performs zero writes.** Every mutation in the script is guarded: changelog rewrites (`:665-671`, `:813-821`), `git add`/`commit`/`push`/`tag` (all via `run_cmd`, `:78-85`), `go get`/`go mod tidy` (`:792-811`), `scripts/check.sh` (`:829-834`), the README rewrite (`:404-407`), and the dogfood copy (`:551-557`). `verify_after_cut` (`:576-584`) short-circuits so `all --dry-run` never reaches the download path. Ran `cut freshbooks 0.9.9 --dry-run`, `bump 0.9.9 --dry-run` and `docs --dry-run` on the real repo (`all` was blocked by another lane's untracked report file, correctly):

```
git status --porcelain      before == after   (IDENTICAL)
md5sum README.md CHANGELOG.md {freshbooks,mcp,cli}/CHANGELOG.md {mcp,cli}/go.mod   all OK
HEAD                        12b0e88 == 12b0e88
git tag                     9 == 9 (IDENTICAL)
git ls-remote --tags origin 18 lines == 18 lines (IDENTICAL)
~/.local/bin/freshbooks*    does not exist, before and after
new /tmp entries            (none)
```

**Red CI blocks `cut freshbooks`.** Proven above (A1 evidence): `FAIL cut-ci-watch`, exit 1, zero tags local or remote. The sha binding is real -- `watch_run` compares `headSha` from `run list` against the pushed HEAD (`:312-314`) and then polls `run view "$id"` by that run's own id, so it cannot drift to a different run.

**Module allowlist enforced.** `cut evil 1.0.0 --dry-run` exits 1 with `release: FAIL cut -- expected module in {freshbooks,mcp,cli}, observed evil`. `verify evil/v1.0.0` exits 1. Same allowlist in `release.yml`'s guard.

**Leak sweep clean (public repo).** All added lines are ASCII-only (`LC_ALL=C grep -P '[^\x00-\x7F]'` over the `+` lines of the full diff: no hits). No operator paths, no internal hostnames, no vault item names, no real account/business ids -- the only `/home/`-shaped hit in the branch is `~/.local/bin` in the implementer report, which is the documented default. `scripts/redaction-check.sh` reports `clean`.

## Fix order

1. A1 -- CI watch in `cut_binary` (+ A6 probe).
2. A2 -- make `readme-drift-check` non-mutating.
3. A3 -- `require_semver` on all three entry points; drop the regex fallback in `changelog_has_section`.
4. A4 -- extend the EXIT trap over the verify temp dirs.
5. A5, A7 -- clean-tree check on `cut`/`bump`; retry `run list` until the sha appears.
6. The rest at the lead's discretion.
