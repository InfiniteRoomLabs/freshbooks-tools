# Phase 9 QA report -- release automation

Lane: QA / reality-check (the only lane that runs the gate). Subject: branch `phase-9/release-automation` @ `eb3a938`, 14 commits ahead of `main`, the last eight being the F1-F19 review-gate fix. Default verdict is NEEDS WORK; everything below is evidence collected first-hand.

## Verdict: PASS

Seven advisories, no blockers. Every mandatory probe passed. **Q1 is a one-line fix and should land before or with the merge**; the rest are follow-ups.

Nothing in the findings can cause an incorrect release: Q1 fails safe (a false FAIL, never a false OK), and Q2-Q7 are cosmetic, documentation, or latent-but-unreachable.

## Probe 1 -- the gate

```
mise run check  ->  GATE_EXIT=0
coverage-gate: <repo>/freshbooks/coverage.out total = 91.9% (floor 90%)
coverage-gate: <repo>/mcp/coverage.out       total = 92.1% (floor 90%)
coverage-gate: <repo>/cli/coverage.out       total = 91.6% (floor 90%)
```

`git status --porcelain` empty before and after the gate (this report is the only file I created, and `scripts/check.sh` excludes `docs/phases/*/reports/`).

**F19 -- repo-wide steps run once per gate, not once per module.** Header counts in `/tmp/qa-gate.log`:

| step | `== <step> ==` headers |
|---|---|
| actionlint | 1 |
| shellcheck | 1 |
| redaction-selftest | 1 |
| release-selftest | 1 |
| readme-drift-check | 1 |

**F8 -- `mise run docs` refreshes the README Status column and is idempotent.** Tested live rather than by reading: corrupted a Status cell to `freshbooks/v0.0.1`, ran `mise run docs`, the cell returned to `freshbooks/v0.3.0`; a second run left `README.md` and `docs/cli.md` byte-identical (md5 compare). `README.md` ends at its baseline md5 `c801cab5eac9f1900d7e1fed1b53dc80`.

`usage lint` clean on all 9 scripts. `mise exec -- shellcheck -S warning -e SC1008 -e SC2154 scripts/*.sh` exit 0.

## Probe 2 -- the self-test and its non-vacuity

24 probes, all PASS, matching `fix.md` line for line (full list in `/tmp/qa-gate.log`). Non-vacuity was re-established independently by reverting one fix at a time in a scratch copy of `release.sh` under `/tmp/qa-nv/scripts/`, driven by a scratch copy of the self-test (which locates the script as `$(dirname $BASH_SOURCE)/../scripts/release.sh` -- there is no `RELEASE_SH` override).

Control: pristine copy in the scratch harness -> `OK (24 cases)`, exit 0.

| revert | probes that FAILed | line |
|---|---|---|
| **F1** (restore the `git log \| grep -q` pipeline, drop the explicit `main`) | 1 | `FAIL re-running all prints SKIP for every mutating step and re-runs verify -- exit 1` |
| **F3** (remove `watch_head_ci "cut-ci-watch"` from `cut_binary`) | 1 | `FAIL cut mcp on a red CI FAILs cut-ci-watch with zero tags local or remote -- exit 0, origin tags=[a15f1617... refs/tags/mcp/v0.9.0` |
| **F5** (neuter `require_semver`) | 3 | `FAIL cut refuses the non-semver version '1.0' -- exit 0` / `'v1.0.0' -- exit 0` / `'.*' -- exit 1` |

F3's failure line is the important one: with the fix reverted, the fake origin really does end up carrying `mcp/v0.9.0` published off a red CI -- security finding A1 reproduced, then closed.

**F7** lives in `scripts/check.sh`, not the self-test, so it was proven on the real tree instead:

```
$ sed -i 's|`freshbooks/v0.3.0`|`freshbooks/v0.0.1`|' README.md
$ scripts/check.sh readme-drift-check
== readme-drift-check ==
release: OK docs
readme-drift-check: README.md Status column is stale -- run 'mise run release -- docs'
--- README.md
+++ /tmp/tmp.DeWYMMoYqb
-| Library | ... | `freshbooks/v0.0.1` |
+| Library | ... | `freshbooks/v0.3.0` |
EXIT=1
$ grep freshbooks/v0.0.1 README.md   # the operator's edit survived -- the gate did not mutate the tree
```

A methodology note for anyone repeating this: the first pass of these reverts was run with cwd `/tmp` and produced a spurious 6-probe cascade in every case, including the pristine control. That cascade is Q1, not the reverts. The numbers above were re-taken with cwd = the repo.

## Probe 3 -- real-repo dry run and verify

`scripts/release.sh all 0.4.0 --dry-run` from the branch: **exit 0**, full plan printed, `SKIP preflight-branch` and `SKIP preflight-ci-green` under the dry-run/off-main exemption as designed.

Zero writes proven by before/after capture:

| | before | after |
|---|---|---|
| HEAD | `eb3a938...` | `eb3a938...` |
| `git status --porcelain` | empty | empty |
| local tags | 9 | 9 |
| `git ls-remote --tags origin \| wc -l` | 18 | 18 |
| md5 of README + 4 changelogs + 2 go.mod | -- | **byte-identical** |

`verify` against the three published tags, all exit 0:

```
verify freshbooks/v0.3.0 -> OK verify-release, OK verify-proxy-pickup
verify mcp/v0.1.2        -> OK verify-release, verify-download, verify-checksum, verify-run,
                               verify-go-install, verify-go-install-run, verify-dogfood
verify cli/v0.1.2        -> the same plus OK verify-cli-no-md2man
```

Temp-dir cleanup across all four real runs (F14 on the live path, which allocates a `workdir` and a `gobin_dir` per binary verify): `/tmp` dir count 69 before, **69 after**; zero leftover `/tmp/release-cmd.*` logs. Dogfood install is current: `~/.local/bin/freshbooks` -> `0.1.2`, `~/.local/bin/freshbooks-mcp` -> `freshbooks-mcp 0.1.2`.

## Probe 4 -- blockers verified by reading

| F | claim | verified |
|---|---|---|
| F1 | no pipe into `grep -q`; `main` named | `release.sh:212-213` -- `git log main --format=%s --fixed-strings --grep=` captured, matched via `grep -qxF -- "$subject" <<<"$found"` (herestring) |
| F2 | discovery inside the poll loop, within the timeout | `release.sh:408-423` -- `gh run list` retried every `DISCOVERY_INTERVAL`; both loops share the one `waited`/`TIMEOUT` budget |
| F3 | `cut_binary` has the CI watch + the clean-tree check | `release.sh:941` `watch_head_ci "cut-ci-watch" "bump"` before `tag_push_and_watch`; `require_clean_tree` at `:839` (cut) and `:951` (bump) |
| F4 | `headSha == HEAD` and `status == completed` | `release.sh:505-525` -- one `--jq` reads `headSha status conclusion`; requires `completed`+`success`, then pins to HEAD, naming both shas on failure |
| F5 | regex exactly `^[0-9]+\.[0-9]+\.[0-9]+$`; literal changelog match | `release.sh:190` exact; `changelog_has_section` at `:177-182` is awk `index($0, "## [<v>]") == 1`, no regex arm |
| F6 | one blank after the heading | see below |
| F7 | the gate never mutates README | `check.sh:137-149` renders into `$(mktemp)` via `RELEASE_README_OUT`, diffs, `rm -f`; proven live above |
| F9 | stale local tag FAILs | `release.sh:804-810` -- compares `$tag^{commit}` to HEAD, `step_fail "cut-tag"` naming both shas; never deletes |
| F12 | no external `jq` | zero `jq` invocations; the only hits are 3 comments and `--jq` inside `gh_jq`/`gh run list` |
| F14 | one EXIT trap covers all `mktemp` | one `trap cleanup_tmp EXIT` at `:62`; `TMP_PATHS` seeded with `LOG_FILE`, and all five `mktemp` sites (`:282,312,565,646,682`) call `track_tmp` on the next line |

**F6 measured, not just read.** I extracted the actual awk program out of `changelog_add_bullet` and ran it three times against a scratch copy of the real root `CHANGELOG.md`:

```
run 0 (original): blanks_after_heading=1  added_bullets=27
run 1:            blanks_after_heading=1  added_bullets=28
run 2:            blanks_after_heading=1  added_bullets=29
run 3:            blanks_after_heading=1  added_bullets=30
```

Exactly one blank after `## [Unreleased]` on every call. R3's "four after one, seven after two" is gone. (See Q4 for a separate cosmetic artifact this surfaced.)

## Probe 5 -- F1-F19 landed

All nineteen present in the tree and attributable to `28a49bd..eb3a938` (592 lines changed in `release.sh`, 336 in `release-selftest.sh`, 66 in `check.sh`, plus `ci.yml`, `mise.toml`, `docs.sh`, `branch-protection.sh`, `building.md`, `CHANGELOG.md`). Spot-checks: F10 `bump-version-propose`, F11 the `all-ship` NOTE, F15 `ensure_go_bin`, F16 all three helpers (`commit_and_push`, `watch_head_ci`, `tag_push_and_watch`), F17 the dead `readme=".README.md"` gone, F18 `shellcheck` pinned in `mise.toml`, F19 `run_repo_wide` + the CI `repo-wide` job + the matching required context in `branch-protection.sh`.

`docs/building.md` matches reality: all six subcommands documented, in the same set the dispatch `case` handles (`preflight docs verify cut bump all`), and the flag descriptions for `--dry-run`, `--yes`, `--version auto`, `--binary-version` and `$RELEASE_LOCAL_BIN` are accurate. One omission -- see Q5.

## Probe 6 -- precedent check

Compared the dry-run plan against the three hand-driven releases in `git log main` and `docs/progress.md`:

| element | precedent | script | |
|---|---|---|---|
| lib release commit | `release(freshbooks): v0.3.0` | `release(freshbooks): v0.4.0` | match |
| bump commit | `release(mcp,cli): require freshbooks v0.3.0 and cut 0.1.2` | `release(mcp,cli): require freshbooks v0.4.0 and cut 0.1.3` | match |
| tag names | `freshbooks/v0.3.0`, `mcp/v0.1.2`, `cli/v0.1.2` | `freshbooks/v0.4.0`, `mcp/v0.1.3`, `cli/v0.1.3` | match |
| ship commit | `docs: ship Phase 8 and retarget GOAL.md to Phase 9 (...)` | `docs: ship v0.4.0` | **diverges -- Q3** |
| changelog heading date | operator-local date, equal to the commit date | UTC date | **diverges -- Q2** |

## Findings

### Q1 -- ADVISORY (fix before merge) -- `preflight` resolves the toolchain in the caller's cwd
`scripts/release.sh:527`

Expected: `preflight-mise-install` checks the repo's pinned toolchain, like its sibling `ensure_go_bin` at `:84`, which correctly runs `(cd "$repo_root" && mise where go)`.

Observed: `if read_cmd mise which go >/dev/null; then` runs in whatever directory the caller happened to be in. mise resolves tools per-directory, so the check reports on the caller's cwd rather than on the repo.

Failure scenario, reproduced: run the **in-repo** self-test with cwd `/tmp` and 6 of 24 probes fail, every one of them on

```
release: FAIL preflight-mise-install -- expected mise.toml toolchain resolvable,
  observed mise which go failed -- run mise install
mise ERROR go is a mise bin however it is not currently active.
```

| invocation | result |
|---|---|
| `scripts/release-selftest.sh`, cwd = repo root | `OK (24 cases)` |
| the same script, cwd = `mcp/` (repo subdir) | `OK (24 cases)` |
| the same script, cwd = `/tmp` | **6 assertion(s) failed** |

Two consequences. (a) `scripts/release.sh preflight` or `all`, invoked by absolute path from outside the repo -- a wrapper, a cron job, `$HOME` -- FAILs on a perfectly installed toolchain. (b) The self-test is not hermetic: D6 says "every probe runs against a throwaway scratch repo", `fix.md` says "still entirely against scratch repos and fake `gh`/`go`", and `docs/building.md` repeats it, but six probes reach out to the ambient `mise` and depend on the caller's cwd. The gate is green only because mise tasks and CI both happen to invoke it from the repo root.

This fails safe -- it can only produce a spurious FAIL, never a spurious OK, so it cannot ship a bad release. Fix is one line: `(cd "$repo_root" && mise which go)`, matching `:84`.

Note this is *not* security's A14 (which asked whether `mise which go` belongs in preflight at all, and the lead consciously kept it). The cwd dependency is a separate property no lane raised.

### Q2 -- ADVISORY -- changelog heading dates are UTC, precedent is local
`scripts/release.sh:911` and `:1003`

Expected (precedent): every hand-cut section is dated with the operator's local date, equal to the commit's local date -- `[0.1.0] - 2026-09-02`, `[0.2.0] - 2026-09-03`, `[0.3.0] - 2026-09-03`, `mcp [0.1.2] - 2026-09-03`, `cli [0.1.2] - 2026-09-03`, five for five.

Observed: `today=$(date -u +%F)`. During this run at 20:47 EDT (= 00:47 UTC the next day) the dry run emitted `changelog_cut_section ... 0.4.0 2026-09-04`, while the commit it would create is dated 2026-09-03 locally.

Failure scenario: any release cut between 20:00 and 24:00 local (a 4-hour window daily, 5 under EST) stamps a heading one day ahead of its own commit and of all five precedents.

Cosmetic only: `scripts/changelog-section.sh` and `changelog_has_section` both match `^## \[X.Y.Z\]` as a prefix, so the release workflow's guard and notes extraction are date-agnostic. The lead should either switch to `date +%F` for precedent consistency or record UTC as a deliberate change.

### Q3 -- ADVISORY -- the ship commit subject diverges from precedent
`scripts/release.sh` (`cmd_all`'s ship step)

Expected: all nine historical ship commits read `docs: ship <thing> and retarget GOAL.md to <next>`.

Observed: `docs: ship v0.4.0`, staging `README.md` only.

This is honest -- F11's `release: NOTE all-ship stages README.md only -- write the docs/progress.md ledger row and retarget GOAL.md by hand` says exactly what is missing, and it printed in the dry run. But the operator following that NOTE will amend the commit to add the ledger row and the GOAL retarget, at which point the subject no longer describes it. Worth either extending the subject or having the NOTE tell the operator to reword it.

### Q4 -- ADVISORY (low) -- `changelog_add_bullet` leaves a stray blank inside `### Added`
`scripts/release.sh:305-357`

F6's stated property holds exactly (one blank after `## [Unreleased]`, measured over three runs). Separately, new bullets are inserted immediately after the `### Added` heading, *before* the section's own leading blank, so a section that starts as `### Added / <blank> / - old` becomes:

```
### Added
- QA probe bullet 3
- QA probe bullet 2
- QA probe bullet 1

- `scripts/release.sh` (`mise run release`) automates ...
```

The orphan blank makes it a loose list in CommonMark (every item wrapped in `<p>`) from the first automated release onward. The self-test probe asserts blanks-after-heading and bullet count, so it will not catch this. Cosmetic; affects the rendered root changelog only.

### Q5 -- ADVISORY (low) -- `--timeout` is undocumented
`docs/building.md` "Release flow"

`release.sh` declares `#USAGE flag "--timeout <seconds>"` (default 1200) and D4 specifies it, but building.md documents only `--dry-run`, `--yes`, `--version auto` and `--binary-version`. An operator whose Release run legitimately exceeds 20 minutes has no documented escape.

### Q6 -- ADVISORY (informational) -- the SIGPIPE class F1 removed survives at 7 sibling sites
`scripts/release.sh:228,229,233,237,491,630,703,795`

These keep the `printf ... | grep -q` / `git ls-remote ... | grep -q` shape that, under `set -o pipefail`, turns an early `grep -q` exit into a 141 pipeline status -- the exact mechanism behind R1/F1.

Measured, and **not reachable today**: the pipe buffer is 64KB, and every input is far below it. `derive_bump_kind` (the `:228-237` sites) reads *module* changelogs only (`:864`, `:987`), whose `[Unreleased]` sections are currently 1 byte each; the 13,439-byte root `CHANGELOG.md` is never fed to it. `:491` is `gh auth status` output, `:630` a `go list -m` line, `:703` `go version -m`, `:795` a single filtered ref.

Recorded rather than raised as a defect, because `fix.md` itself notes F1's probe was only non-vacuous once the fixture grew to 250 commits with 400-char subjects -- the class is size-dependent and silent, so it is worth knowing where else it lives.

### Q7 -- ADVISORY (nit) -- the drift check's temp file leaks on failure
`scripts/check.sh:140-143`

`rendered=$(mktemp)` is cleaned by `rm -f` on the success path only. If `scripts/release.sh docs` fails, `set -e` exits `check.sh` first and the temp survives. Same class as F14, one directory over.

## Commands run

```
mise run check                                            # exit 0
scripts/check.sh readme-drift-check                       # F7, live, with a corrupted Status cell
mise run docs  x2                                         # F8 + idempotency
usage lint scripts/*.sh                                   # 9/9 OK
mise exec -- shellcheck -S warning -e SC1008 -e SC2154 scripts/*.sh
scripts/release-selftest.sh                               # in-repo, cwd repo / cwd mcp / cwd /tmp
/tmp/qa-nv/scripts/release-selftest.sh                    # control + F1/F3/F5 reverts
scripts/release.sh all 0.4.0 --dry-run                    # exit 0, zero writes
scripts/release.sh verify freshbooks/v0.3.0 | mcp/v0.1.2 | cli/v0.1.2
git ls-remote --tags origin | wc -l                       # 18 before, 18 after
```

No `git push`, no tag push, no mutating `gh`, no commit. Scratch dirs under `/tmp` removed; `README.md` and `CHANGELOG.md` restored to their baseline md5s; working tree carries only this report.
