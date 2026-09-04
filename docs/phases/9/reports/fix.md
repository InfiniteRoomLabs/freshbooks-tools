# Phase 9 fix report -- release automation

Branch `phase-9/release-automation`, `28a49bd..6b9603d` (7 fix commits). Order: `docs/phases/9/triage.md` F1-F19, six checkpoints plus one correction. No network writes: no push, no tag, no mutating `gh`. The real repo was touched only by `all 0.4.0 --dry-run` (proven byte-identical before/after) and three read-only `verify` runs against already-published tags.

## Verdict

F1-F19 applied. Every blocker (F1-F7, F9) has a self-test probe, and each of F1, F2, F3, F4, F5, F6, F7, F9 and F14 was proven non-vacuous by reverting the fix and recording the probe's failure line. Self-test grew from 10 cases to 24. `mise run check` green at every checkpoint; `usage lint` and `shellcheck -S warning` clean.

One place the triage was wrong: **F4 as specified breaks `all --dry-run` off `main`** -- see "Where the triage was wrong" below.

## F1-F19

| F | Commit | State |
|---|---|---|
| F1 | `e1158ba` | Done. `commit_with_subject_exists` filters with `git log main --format=%s --fixed-strings --grep=<subject>` and matches the captured output via a herestring. No pipe, so no SIGPIPE, so no `pipefail` 141. |
| F2 | `e1158ba` | Done. `watch_run`'s discovery is a poll: CI retries until a run whose `headSha` is the pushed sha appears, Release until any run for the tag appears; both inside `--timeout`. `RELEASE_DISCOVERY_INTERVAL`/`RELEASE_POLL_INTERVAL` exist so the self-test can exercise it without real sleeps. |
| F3 | `e1158ba` | Done. `cut_binary` watches CI for HEAD before the tag block (SKIP under `--dry-run`); `cut` and `bump` both run the clean-tree check via `require_clean_tree` (`cut-clean-tree`, `bump-clean-tree`). |
| F4 | `e1158ba`, corrected in `6b9603d` | Done. `preflight-ci-green` reads `headSha status conclusion` from one `--jq`, requires `completed`+`success`, and pins the run to HEAD, naming both shas on failure. See the correction below. |
| F5 | `e1158ba` | Done. `require_semver` (`^[0-9]+\.[0-9]+\.[0-9]+$`, the release workflow's regex) on `$version`, `$lib_version` and `$binary_version` after `auto` resolves. `changelog_has_section` is an awk literal-prefix match (`index($0, "## [<v>]") == 1`), no regex arm. |
| F6 | `9fa594a` | Done. `changelog_cut_section` emits heading + blank + body. `changelog_add_bullet` is one awk pass that re-emits `[Unreleased]` with exactly one blank after the heading on every call. |
| F7 | `9fa594a` | Done. `cmd_docs` honours `RELEASE_README_OUT`; `run_readme_drift_check` renders into a temp file, diffs, and never writes the tree. |
| F8 | `9fa594a` | Done. `scripts/docs.sh` calls `release.sh docs` after `docs/cli.md`. |
| F9 | `0a53855` | Done. An existing local tag must be at HEAD: SKIP `cut-tag` if it is, FAIL naming both shas if it is not. Never deleted. |
| F10 | `0a53855` | Done. New `bump-version-propose` step; `derive_bump_kind` runs on each module's `[Unreleased]` for information and emits a `release: NOTE` when it argues for more than the patch default. Default unchanged. |
| F11 | `0a53855` | Done. `all` prints `release: NOTE all-ship stages README.md only -- write the docs/progress.md ledger row and retarget GOAL.md by hand ...`. |
| F12 | `0a53855` | Done. No external `jq` in `release.sh`; `do_verify` reads `isDraft`, `name` and the asset count from one `gh --jq` string. The fake `gh` keeps its `jq`. |
| F13 | `2b25d9e`, `6b9603d` | Done. 14 new probes (below). |
| F14 | `9fa594a` (trap), `cc5462e` (verify dirs) | Done. One EXIT trap owns `LOG_FILE`, both changelog/README scratch files and `do_verify_binary`'s `workdir`/`gobin_dir`. |
| F15 | `cc5462e` | Done. `ensure_go_bin` resolves `mise where go` lazily and memoises it; failure is a `go-toolchain` FAIL line. The silent `set -e` exits in `watch_run` (`gh run view`) and `cmd_docs` (`git tag --list`, the awk) are `step_fail` lines. Also `latest_tag_version` now returns 0 when a module has no tags -- its trailing AND-list made "no tags yet" the function's status, which killed direct callers with no `release:` line. |
| F16 | `cc5462e` | Done. `commit_and_push <prefix> <subject> <path>...`, `watch_head_ci <step> <what>`, `tag_push_and_watch <module> <version> <tag>`; `watch_run` reads `id sha` from one `--jq` and `status conclusion` from one `run view`. Applied after F1-F3, so the helpers carry the fixed logic. `cut_binary` is now two calls. |
| F17 | `9fa594a`, `2b25d9e`, `cc5462e` | Done. Dead `readme=".README.md"` gone; preflight's `sha` kept (F4 uses it); `case_n` in the summary; `changelog_has_section` single arm; fake-`gh` `conclusion_for_workflow` helper and `jq -n` assets; `seed_added_changelog`; the corrected `git init` comment (and its `|| true` swallow dropped); the `cut` guard is an `if`. |
| F18 | `d2ce70b` | Done. The tag-ruleset line is a `release: NOTE`; the dry-run root-bullet echo carries the backticks the real call writes; `module_kind` returns 1 silently; `version-propose` keeps the bare `OK` shape with its detail on a NOTE line. `shellcheck` 0.11.0 pinned via `aqua:koalaman/shellcheck`, wired as a gate step at `-S warning` (excluding SC1008 for the `usage` shebang and SC2154 for `$usage_*`). `scripts/*.sh` is clean; proven non-vacuous by dropping a file with an unused variable into `scripts/` and watching the step go red on SC2034. |
| F19 | `d2ce70b` | Done. `run_repo_wide` groups `actionlint`, `shellcheck`, `redaction-selftest`, `release-selftest`, `readme-drift-check`. A bare `scripts/check.sh all` runs them; `check.sh all <module>` prints `== repo-wide steps skipped (module filter: ...) ==`. CI gets a `repo-wide` job (`mise run repo-wide`, `fetch-depth: 0` for the tags), added to `scripts/branch-protection.sh`'s required contexts so those steps keep blocking merges. |

Nothing under "Not applied" (A8, A11, A14, S10, R16) was touched.

## Non-vacuity evidence

Each fix was reverted in the working tree, the suite re-run, and the failure line recorded; then restored.

```
===== non-vacuity: F1 reverted =====
release-selftest: FAIL re-running all prints SKIP for every mutating step and re-runs verify -- exit 1: ...
===== non-vacuity: F2 reverted =====
release-selftest: FAIL watch_run retries discovery through a stale CI headSha and an empty Release list -- exit 1, CI run-list calls=1: ...
===== non-vacuity: F3 reverted =====
release-selftest: FAIL cut mcp on a red CI FAILs cut-ci-watch with zero tags local or remote -- exit 0, origin tags=[1721d06d... refs/tags/mcp/v0.9.0 ...]
===== non-vacuity: F4 reverted =====
release-selftest: FAIL preflight FAILs ci-green when the newest CI run is for a different sha than HEAD -- exit 0: ...
release-selftest: FAIL preflight FAILs ci-green when HEAD's CI run has not completed -- exit 0: ...
===== non-vacuity: F5 reverted =====
release-selftest: FAIL cut refuses the non-semver version '1.0' -- exit 0: ...
release-selftest: FAIL cut refuses the non-semver version 'v1.0.0' -- exit 0: ...
release-selftest: FAIL cut refuses the non-semver version '.*' -- exit 1: ...
===== non-vacuity: F6 reverted =====
release-selftest: FAIL the cut section is heading + one blank + body, and the root [Unreleased] keeps one blank after three bullets -- exit 0, root blanks=0 (want 1), root bullets=3 (want 3), cut body=[### Added ...]
===== non-vacuity: F9 reverted =====
release-selftest: FAIL cut FAILs cut-tag rather than pushing a local tag that is not at HEAD -- exit 0, origin tags=[cd7aa79e... refs/tags/freshbooks/v0.9.0 ...]
```

F1's probe is only non-vacuous because the happy-path scratch repo is now deep-filled with 250 empty commits carrying 400-char subjects (`deepen_history`). Short subjects never fill a pipe buffer, so `git log` finishes writing before `grep -q` exits and no SIGPIPE occurs -- which is exactly why the original three-commit probe passed on broken code.

F7 lives in `scripts/check.sh`, not the self-test. Reverted to the mutating form, with an operator edit in the tree:

```
$ sed -i 's|`freshbooks/v0.3.0`|`freshbooks/v0.0.1`|' README.md
$ git status --porcelain README.md
 M README.md
$ scripts/check.sh readme-drift-check
== readme-drift-check ==
release: OK docs
check.sh: readme-drift-check OK
EXIT=0
-- tree after the gate:
        <- empty: the edit was silently reverted and the gate said OK
```

With F7 applied, same scenario:

```
EXIT=1
== readme-drift-check ==
release: OK docs
readme-drift-check: README.md Status column is stale -- run 'mise run release -- docs'
-- tree after the gate:
 M README.md          <- the operator's edit survives
```

F14 has no step line to assert, so it was measured instead -- `/tmp` directory count around one self-test run:

```
WITH F14:    /tmp dirs before=68 after=68 (delta 0)
WITHOUT F14: /tmp dirs before=68 after=70 (delta 2)
```

F18's shellcheck step, proven to actually lint:

```
$ printf '#!/usr/bin/env bash\nfoo=1\necho $bar\n' > scripts/zzz-tmp.sh && mise run shellcheck
== shellcheck ==
In .../scripts/zzz-tmp.sh line 2:
foo=1
^-^ SC2034 (warning): foo appears unused. Verify use (or export if used externally).
[shellcheck] ERROR task failed
EXIT=1
```

## Self-test

24 cases, ~24s (the deep-fill costs ~8s), still entirely against scratch repos and fake `gh`/`go`.

```
release-selftest: PASS preflight FAILs on a dirty tree
release-selftest: PASS preflight FAILs on a non-main branch
release-selftest: PASS --version auto proposes patch for a Fixed-only changelog
release-selftest: PASS --version auto proposes minor for an Added changelog
release-selftest: PASS all 0.9.0 --dry-run prints the full plan with zero pushes
release-selftest: PASS all 0.9.0 --yes completes with every step OK and tags the fake origin
release-selftest: PASS re-running all prints SKIP for every mutating step and re-runs verify
release-selftest: PASS a failing Release run FAILs cut before the next module's tag push
release-selftest: PASS verify FAILs when the stub binary prints the wrong version string
release-selftest: PASS verify FAILs when the checksum file is altered
release-selftest: PASS preflight FAILs ci-green when the newest CI run is for a different sha than HEAD
release-selftest: PASS preflight FAILs ci-green when HEAD's CI run has not completed
release-selftest: PASS preflight SKIPs the ci-green HEAD pin off main under --dry-run, keeping the plan previewable
release-selftest: PASS cut freshbooks on a red CI FAILs cut-ci-watch with zero tags local or remote
release-selftest: PASS cut mcp on a red CI FAILs cut-ci-watch with zero tags local or remote
release-selftest: PASS watch_run retries discovery through a stale CI headSha and an empty Release list
release-selftest: PASS verify FAILs verify-release on a draft release
release-selftest: PASS verify FAILs verify-release on a release named for the wrong tag
release-selftest: PASS verify FAILs verify-release on a release with 12 of the expected 13 assets
release-selftest: PASS docs rewrites all three README Status cells to the newest tag and is idempotent
release-selftest: PASS the cut section is heading + one blank + body, and the root [Unreleased] keeps one blank after three bullets
release-selftest: PASS cut refuses the non-semver version '1.0'
release-selftest: PASS cut refuses the non-semver version 'v1.0.0'
release-selftest: PASS cut refuses the non-semver version '.*'
release-selftest: PASS cut FAILs cut-tag rather than pushing a local tag that is not at HEAD
release-selftest: OK (24 cases)
```

New fake-`gh` knobs: `FAKE_GH_CI_STALE_COUNT`, `FAKE_GH_RELEASE_EMPTY_COUNT`, `FAKE_GH_CI_LIST_STATUS`, `FAKE_GH_RELEASE_DRAFT`, `FAKE_GH_RELEASE_NAME`, `FAKE_GH_ASSET_COUNT`.

## Dry-run on the real repo

`scripts/release.sh all 0.4.0 --dry-run`, exit 0. State captured before and after (HEAD, `git status --porcelain`, the local tag list, `git ls-remote --tags origin`, and md5sums of README.md, all four changelogs and both go.mod files): **byte-identical**. The full transcript is in the final message to the lead; the shape:

```
release: SKIP preflight-branch -- not on main (phase-9/release-automation), continuing under --dry-run
release: OK preflight-clean-tree
release: OK preflight-gh-auth
release: SKIP preflight-ci-green -- HEAD is phase-9/release-automation, not main -- main's newest run (3cec68d...) is green
release: OK preflight-mise-install
release: NOTE gh api repos/InfiniteRoomLabs/freshbooks-tools/rulesets (tag ruleset for refs/tags/{freshbooks,mcp,cli}/v*, warn-only, never applied by this script)
release: OK preflight-tag-ruleset-warn
...
release: OK bump-version-propose
release: NOTE mcp/cli 0.1.2 -> 0.1.3 (patch, the default)
...
dry-run: watch CI on main for the bump commit
release: SKIP cut-ci-watch -- dry-run          <- new, once per binary module (F3/A1)
...
release: NOTE all-ship stages README.md only -- write the docs/progress.md ledger row and retarget GOAL.md by hand, then amend or follow up
release: SKIP all-ship-ci-watch -- dry-run
```

Note the `NOTE mcp/cli 0.1.2 -> 0.1.3 (patch, the default)` line carries no follow-up note: neither `mcp/CHANGELOG.md` nor `cli/CHANGELOG.md` has a non-empty `[Unreleased]` right now, so `derive_bump_kind` says `none` and F10 stays quiet, correctly.

## Verify against the published tags (read-only)

```
===== scripts/release.sh verify freshbooks/v0.3.0 =====
release: OK verify-release
release: OK verify-proxy-pickup
EXIT=0
===== scripts/release.sh verify mcp/v0.1.2 =====
release: OK verify-release
release: OK verify-download
release: OK verify-checksum
release: OK verify-run
release: OK verify-go-install
release: OK verify-go-install-run
release: OK verify-dogfood
EXIT=0
===== scripts/release.sh verify cli/v0.1.2 =====
release: OK verify-release
release: OK verify-download
release: OK verify-checksum
release: OK verify-run
release: OK verify-go-install
release: OK verify-go-install-run
release: OK verify-cli-no-md2man
release: OK verify-dogfood
EXIT=0
```

Tree clean afterwards. This is the live proof for F12: the `gh --jq` rewrite reads real `gh release view` output, and `verify-release`'s 0-asset and 13-asset assertions hold against the real releases. The dogfood copies into `~/.local/bin` are the documented behaviour and were the only local writes.

## Where the triage was wrong

**F4 as specified makes `all --dry-run` unusable from a feature branch.** `preflight-branch` already SKIPs off `main` under `--dry-run` ("continuing under --dry-run"), but pinning `preflight-ci-green` to HEAD without the same exemption means the branch tip is compared against `main`'s newest CI run and always mismatches:

```
release: FAIL preflight-ci-green -- expected the newest CI run to be for HEAD d2ce70b..., observed newest run is for 3cec68d...
```

That is the very command this order asks for as the acceptance transcript, and previewing the plan from a branch is what `--dry-run` is for. `6b9603d` gives ci-green the same exemption preflight-branch takes: `main`'s newest run must still be `completed` and `success`, but the HEAD comparison SKIPs when off main under `--dry-run`. On `main`, and on every non-dry-run path, the pin is exactly what F4 asked for. A probe asserts the SKIP.

Two smaller judgement calls worth the lead's eye:

- **F3's clean-tree check is strict, not dry-run-exempt** -- it matches `preflight-clean-tree`, which fails under `--dry-run` too. That changed two existing self-test probes (`--version auto`), which now commit their seeded changelog before running `cut`. The alternative (SKIP under `--dry-run`) would let you preview from a dirty tree; I took consistency with preflight, since `all` reaches the strict form anyway.
- **F19 needed a CI job and a required-check line.** `check.sh all` already ran the repo-wide steps once per invocation; the waste was CI invoking the gate three times. Skipping them under a module filter without adding a job would have dropped `release-selftest` and `readme-drift-check` out of CI entirely, so `.github/workflows/ci.yml` gains a `repo-wide` job and `scripts/branch-protection.sh` gains the matching required context (a local edit; the script itself is still the operator's to run).

`docs/building.md` and the root `CHANGELOG.md` were updated to keep D7 true: the `release: NOTE` line class, `cut mcp|cli`'s new CI watch, the strict `main`/clean-tree/semver preconditions, the `shellcheck` pin, and the repo-wide grouping.

## Gate

`mise run check` exit 0 at every checkpoint, on a clean tree.

```
release-selftest: OK (24 cases)
== readme-drift-check ==
release: OK docs
check.sh: all OK
coverage-gate: <repo>/freshbooks/coverage.out total = 91.9% (floor 90%)
coverage-gate: <repo>/mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: <repo>/cli/coverage.out total = 91.6% (floor 90%)
```

`git status --porcelain`: empty. `usage lint` clean on `release.sh`, `release-selftest.sh`, `check.sh`; `shellcheck -S warning` clean on `scripts/*.sh`; `scripts/redaction-check.sh` clean before every commit. No push, no tag, no merge.
