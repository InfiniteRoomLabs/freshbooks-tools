# Phase 9 implementer report -- release automation

Branch `phase-9/release-automation`, three checkpoint commits on top of `7738689`:

- `173e57e` feat(scripts): add release automation script -- D1-D4, D8
- `5ab6b1f` feat(scripts): add release.sh self-test and gate wiring -- D6, plus D5's README-drift wiring, plus three real bugs found and fixed
- `25239fd` docs: document the release automation flow -- D7

`mise run check` is green on a clean tree (see gate tail below); `git status --porcelain` is empty.

## Subcommand table

| Subcommand | Does |
|---|---|
| `preflight` | branch=main (unless `--dry-run`), clean tree, `gh auth status` scopes, `main`'s CI green, toolchain resolvable, prints (never applies) the tag-ruleset `gh api` call |
| `cut <module> <version>` | freshbooks: changelog cut -> commit -> push -> CI watch -> tag -> tag push -> Release watch -> verify. mcp/cli: tag -> tag push -> Release watch -> verify |
| `bump <lib-version> [--binary-version A.B.C]` | in `mcp/`+`cli/`: `go get`+`go mod tidy`, a `### Changed` line, changelog cut, `fmt-check/vet/lint/test/cover`, one shared commit, push, CI watch |
| `verify <tag>` | release view (not draft, named, asset count); mcp/cli also: download+checksum+extract+run, `go install`+run, cli-only md2man/blackfriday absence check, dogfood copy into `$RELEASE_LOCAL_BIN` |
| `docs` | rewrites the README Status column from `git tag --list '<module>/v*'` |
| `all <lib-version> [--binary-version A.B.C]` | preflight -> cut freshbooks -> bump -> cut mcp -> cut cli -> docs -> `docs: ship vX.Y.Z` commit |

Flags: `--dry-run` (zero writes anywhere -- git/gh/go mutations, changelog edits, the dogfood copy -- echoed as `dry-run: <command>`; read-only checks still run for real), `--yes` (skip the TTY confirmation before the first push), `--binary-version`, `--timeout` (CI/Release watch cap, default 1200s). `--version auto` on `cut`/`all` derives the bump kind from the module's own `[Unreleased]` section and requires `--yes` or a TTY `y`. Output is the `release: OK/SKIP/FAIL <step>` contract; the script never reads `FRESHBOOKS_*`/`GITHUB_TOKEN`.

## Self-test output (`mise run release-selftest`)

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
release-selftest: OK
```

Ten probes, all against scratch repos under `mktemp -d` with a bare local origin and fake `gh`/`go` (git, sha256sum, tar stay real). ~7-10s wall time, stable across repeated runs. `GH_HOST=localhost` set; the fake `gh` never shells out; every `git push` targets the bare repo under the scratch dir.

## `mise run release -- all 0.4.0 --dry-run` on the real repo (zero writes)

Ran on a clean `phase-9/release-automation` tree. `git status --porcelain` was empty before and after.

```
[release] $ scripts/release.sh all 0.4.0 --dry-run
release: SKIP preflight-branch -- not on main (phase-9/release-automation), continuing under --dry-run
release: OK preflight-clean-tree
release: OK preflight-gh-auth
release: OK preflight-ci-green
release: OK preflight-mise-install
dry-run: gh api repos/InfiniteRoomLabs/freshbooks-tools/rulesets (tag ruleset for refs/tags/{freshbooks,mcp,cli}/v*, warn-only, never applied by this script)
release: OK preflight-tag-ruleset-warn
dry-run: changelog_cut_section <repo root>/freshbooks/CHANGELOG.md 0.4.0 2026-09-03
dry-run: changelog_add_bullet <repo root>/CHANGELOG.md Added "freshbooks cut to 0.4.0, ahead of the freshbooks/v0.4.0 tag."
release: OK cut-changelog
dry-run: git -C <repo root> add freshbooks/CHANGELOG.md CHANGELOG.md
dry-run: git -C <repo root> commit -m release(freshbooks): v0.4.0
release: OK cut-commit
dry-run: git -C <repo root> push origin main
release: OK cut-push
dry-run: watch CI on main for the release commit
release: SKIP cut-ci-watch -- dry-run
dry-run: git -C <repo root> tag -a freshbooks/v0.4.0 -m freshbooks 0.4.0
release: OK cut-tag
dry-run: git -C <repo root> push origin freshbooks/v0.4.0
release: OK cut-tag-push
dry-run: watch the Release workflow for freshbooks/v0.4.0
release: SKIP cut-release-watch -- dry-run
dry-run: verify freshbooks/v0.4.0 (release view, checksum/extract/run, go install, dogfood)
release: SKIP verify -- dry-run -- freshbooks/v0.4.0 was not actually cut
dry-run: cd <repo root>/mcp && go get github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks@v0.4.0 && go mod tidy
release: SKIP bump-go-get-mcp -- dry-run
dry-run: changelog_add_bullet <repo root>/mcp/CHANGELOG.md Changed "Requires `freshbooks` v0.4.0"
dry-run: changelog_cut_section <repo root>/mcp/CHANGELOG.md 0.1.3 2026-09-03
dry-run: changelog_add_bullet <repo root>/CHANGELOG.md Added "mcp cut to 0.1.3, ahead of the mcp/v0.1.3 tag."
dry-run: cd <repo root>/cli && go get github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks@v0.4.0 && go mod tidy
release: SKIP bump-go-get-cli -- dry-run
dry-run: changelog_add_bullet <repo root>/cli/CHANGELOG.md Changed "Requires `freshbooks` v0.4.0"
dry-run: changelog_cut_section <repo root>/cli/CHANGELOG.md 0.1.3 2026-09-03
dry-run: changelog_add_bullet <repo root>/CHANGELOG.md Added "cli cut to 0.1.3, ahead of the cli/v0.1.3 tag."
dry-run: scripts/check.sh fmt-check mcp
dry-run: scripts/check.sh vet mcp
dry-run: scripts/check.sh lint mcp
dry-run: scripts/check.sh test mcp
dry-run: scripts/check.sh cover mcp
release: OK bump-check-mcp
dry-run: scripts/check.sh fmt-check cli
dry-run: scripts/check.sh vet cli
dry-run: scripts/check.sh lint cli
dry-run: scripts/check.sh test cli
dry-run: scripts/check.sh cover cli
release: OK bump-check-cli
dry-run: git -C <repo root> add mcp/go.mod mcp/go.sum mcp/CHANGELOG.md cli/go.mod cli/go.sum cli/CHANGELOG.md CHANGELOG.md
dry-run: git -C <repo root> commit -m release(mcp,cli): require freshbooks v0.4.0 and cut 0.1.3
release: OK bump-commit
dry-run: git -C <repo root> push origin main
release: OK bump-push
dry-run: watch CI on main for the bump commit
release: SKIP bump-ci-watch -- dry-run
dry-run: git -C <repo root> tag -a mcp/v0.1.3 -m mcp 0.1.3
release: OK cut-tag
dry-run: git -C <repo root> push origin mcp/v0.1.3
release: OK cut-tag-push
dry-run: watch the Release workflow for mcp/v0.1.3
release: SKIP cut-release-watch -- dry-run
dry-run: verify mcp/v0.1.3 (release view, checksum/extract/run, go install, dogfood)
release: SKIP verify -- dry-run -- mcp/v0.1.3 was not actually cut
dry-run: git -C <repo root> tag -a cli/v0.1.3 -m cli 0.1.3
release: OK cut-tag
dry-run: git -C <repo root> push origin cli/v0.1.3
release: OK cut-tag-push
dry-run: watch the Release workflow for cli/v0.1.3
release: SKIP cut-release-watch -- dry-run
dry-run: verify cli/v0.1.3 (release view, checksum/extract/run, go install, dogfood)
release: SKIP verify -- dry-run -- cli/v0.1.3 was not actually cut
dry-run: rewrite README.md Status cell for freshbooks -> freshbooks/v0.3.0
dry-run: rewrite README.md Status cell for mcp -> mcp/v0.1.2
dry-run: rewrite README.md Status cell for cli -> cli/v0.1.2
release: OK docs
dry-run: git -C <repo root> add README.md
dry-run: git -C <repo root> commit -m docs: ship v0.4.0
release: OK all-ship-commit
dry-run: git -C <repo root> push origin main
release: OK all-ship-push
dry-run: watch CI on main for the ship commit
release: SKIP all-ship-ci-watch -- dry-run
```

Exit 0. Note `--binary-version` auto-derived to `0.1.3` (patch bump over the current `mcp/v0.1.2`/`cli/v0.1.2` tags -- see the "reality disagreed" note below on why `bump`'s default is patch, not the D1-step-1 kind rule).

Also verified live against real, already-published releases (read-only, no writes): `verify freshbooks/v0.3.0`, `verify mcp/v0.1.2`, `verify cli/v0.1.2` all pass end-to-end (release view, proxy pickup / download+checksum+extract+run+go-install+dogfood).

## Gate tail (`mise run check`, full run, clean tree, exit 0)

```
coverage-gate: <repo root>/freshbooks/coverage.out total = 91.9% (floor 90%)
coverage-gate: PASS
...
coverage-gate: <repo root>/mcp/coverage.out total = 92.1% (floor 90%)
coverage-gate: PASS
...
coverage-gate: <repo root>/cli/coverage.out total = 91.6% (floor 90%)
coverage-gate: PASS
...
== actionlint ==
== redaction-selftest ==
redaction-selftest: OK
== release-selftest ==
release-selftest: OK
== build ==
build: done, artifacts in <repo root>/dist
== readme-drift-check ==
release: OK docs
check.sh: all OK
```

## `git log --oneline main..phase-9/release-automation`

```
25239fd docs: document the release automation flow (D7)
5ab6b1f feat(scripts): add release.sh self-test and gate wiring
173e57e feat(scripts): add release automation script
7738689 docs(phase-9): add the release-automation plan and work order
```

## `git status --porcelain`

(empty)

## Where reality disagreed with D1-D8

- **D3's literal `git log -1` resume check was wrong inside a single `all` run.** "release commit already on main (subject match in `git log -1`)" only sees HEAD. By the time `bump`/`all-ship` check whether an *earlier* step's commit already happened, later commits (the ship commit, etc.) have moved HEAD past it, so the check never matched on the second half of a resumed `all`. Fixed by searching all of `main`'s history for the exact subject (`commit_with_subject_exists`), which is safe here because `scripts/branch-protection.sh` enforces `required_linear_history` -- a commit's subject, once on `main`, never moves or gets rewritten. Confirmed by the self-test's resume probe, which failed against the literal-`git log -1` version and passes now.
- **`do_verify` cannot run for real under `--dry-run` when called from `cut`/`all`.** The tag/release it would check genuinely does not exist (zero writes), so a real verification attempt always failed uninformatively -- which broke the very deliverable this report needs (`all 0.4.0 --dry-run` on the real repo would otherwise end in `FAIL verify-release`, not exit 0). `verify_after_cut` now skips verification under `--dry-run` for cut/all's internal use; the standalone `verify <tag>` subcommand still always runs for real (it has nothing to check but already-published state).
- **A classic bash scoping bug, not a plan disagreement, but worth flagging**: `local a="$1" b="...$a..."` does not see the just-assigned `a` -- every RHS in one `local` statement is expanded before any of it takes effect, so a same-line reference to an earlier name in the same statement falls through to dynamic scope (the caller's variable of that name, or unbound). `cut_lib`'s `changelog=...$module...` hit this and worked by accident when called from `cmd_cut` (which happens to have its own `module` local of the same value) but was unbound when called from `cmd_all` (no `module` local there). Audited every other multi-assignment `local` line in the script for the same pattern; none found.
- **`bump`'s default `--binary-version` is a plain patch bump, not the D1-step-1 kind rule.** D1's `--version auto` derivation ("per step 1") is written for `freshbooks`'s own `[Unreleased]` section and is explicitly listed as available on `cut`/`all`, not `bump`. Since `bump` *always* adds a `### Changed` "Requires `freshbooks` vX.Y.Z" line to mcp/cli's changelog before cutting, applying the D1 rule there would make every dependency-only bump a minor bump -- but the historical v0.1.0->v0.1.1->v0.1.2 precedent (a dependency-only bump, then an Added-tool bump) was patch-only both times. `bump`'s default (no `--binary-version`) is therefore a plain patch bump of the current shared mcp/cli tag, matching precedent; `--binary-version` overrides it explicitly when a minor/major bump is actually wanted.
- **`docs/progress.md`'s "Next action: Phase 9 (Docusaurus docs site...)" section was left untouched.** It is now stale (GOAL.md already retargeted Phase 9 to release automation per `3cec68d`), but updating the phase ledger/next-action narrative is the lead's ship-time job in this repo's process, not something D1-D8 asked the implementer to touch.
- **The README-drift check (D5) and the `docs` subcommand's file rewrite use awk/sed, not python3**, despite an earlier draft using a `python3 -` heredoc -- caught before commit: the plan's Rules section bars new dependencies, and this machine's global CLAUDE.md requires all Python go through `uv`. Rewritten as a pure-awk row rewrite; verified idempotent (zero diff) against the real README.md.

## Notes

- `scripts/release.sh`'s `GO_BIN` resolves the pinned toolchain by absolute path via `mise where go` (never bare `go`, never `mise exec` outside the repo), overridable via `RELEASE_GO_BIN` -- used only by the self-test to point at the fake `go` shim.
- `RELEASE_LOCAL_BIN` (default `~/.local/bin`) is the dogfood-install target, overridable for the same reason.
- Coverage floors, lint, vet, fmt, inventory-check all pass unchanged (no application code touched -- this phase is scripts + docs only).
