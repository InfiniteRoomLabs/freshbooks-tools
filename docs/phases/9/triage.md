# Phase 9 gate triage (lead, 2026-09-03)

Inputs: `docs/phases/9/reports/{implementer,code-review,simplify,security}.md`. Verdicts: code review **REQUEST CHANGES** (R1-R4 blocking, R5-R16 advisory), security **BLOCK** (A1-A3 blocking, A4-A14 advisory), simplification S1-S9 + S11 apply, S10/S12/S13 optional. The lanes converged: R2 = A7 (discovery race), R12 = A9 = S5 (bare `jq`), R13 = A4 (temp-dir leak), R15 dead `readme` = S6, S11 = A13. One opus fix agent, six checkpoint commits `fix(release): apply the review-gate findings (Fa-Fb)`; the self-test must be extended so every blocker has a probe that fails on the old code.

## Fix order

| F | Source | Action |
|---|---|---|
| F1 | R1 | `commit_with_subject_exists`: no pipe into `grep -q`; use `git -C "$repo_root" log main --format=%s --fixed-strings --grep="$1"` and compare a line exactly (`grep -qxF` on the captured output, not a streaming pipe), pass `main` explicitly. |
| F2 | R2, A7 | `watch_run`: discovery is inside the poll loop -- retry `gh run list` every 15s until a run whose `headSha` equals `expect_sha` (CI) or any run for `--branch <tag>` (Release) appears, within the same `--timeout`; only then poll its conclusion. |
| F3 | A1, A5 | `cut_binary` gets `watch_run "cut-ci-watch" "CI" "main" "$(git rev-parse HEAD)"` before the tag block (skipped under `--dry-run`); `cut` and `bump` run the clean-tree check (not only `all`). |
| F4 | R4 | `preflight-ci-green` compares the newest run's `headSha` to HEAD and requires `status == completed && conclusion == success`; otherwise FAIL naming both shas. |
| F5 | A3, A10 | `require_semver` (`^[0-9]+\.[0-9]+\.[0-9]+$`, the workflow's regex) on every version argument after `auto` resolves; `changelog_has_section` uses a literal prefix match only (`grep -qF "## [$2]"` anchored by building the exact prefix, no regex). |
| F6 | R3, R5 | Rewrite the two changelog helpers so a cut section is `## [X.Y.Z] - date` + blank + body and `[Unreleased]` keeps exactly one blank after the heading regardless of how many bullets are added; single awk pass each is fine now because F13 adds content assertions. |
| F7 | A2, R8 | `readme-drift-check` renders to a temp file and diffs; the tree is never modified by the gate; a stale README is reported with the diff. |
| F8 | R9 | `scripts/docs.sh` calls `release.sh docs` after `docs/cli.md` generation so `mise run docs` refreshes the Status column. |
| F9 | R7 | Before pushing an existing local tag, assert `git rev-parse "$tag^{commit}"` equals HEAD; otherwise FAIL `cut-tag` naming both shas (never delete it). |
| F10 | R11 | `bump-version-propose` runs `derive_bump_kind` on each module's `[Unreleased]` for information and appends a note when it says minor while the default is patch. |
| F11 | R10 | `all`'s ship step prints a `release: NOTE` line listing what it did not do (progress ledger row, GOAL retarget) so the lead's follow-up is explicit. |
| F12 | R12, A9, S5 | No external `jq` in `release.sh`: `gh --jq` with `"\(.a) \(.b)"` strings and `read -r`. The fake `gh` in the self-test may keep `jq`. |
| F13 | R6, A6 | Self-test: scratch repo seeded with 200+ commits before the resume probe (so R1's class fails on the old code); a fake `gh` mode that answers a stale `headSha` for the first N `run list` calls and an empty list for the first N Release lookups (R2); `FAKE_GH_CI_CONCLUSION=failure` probes for `cut freshbooks` AND `cut mcp` (must FAIL before any tag push, zero tags on the fake origin); probes that a draft release, a wrong name, and a wrong asset count each FAIL `verify-release`; a probe that `docs` writes the expected three cells and is idempotent; a probe asserting the post-`cut` changelog body (heading, one blank, bullets) and the root `[Unreleased]` blank count after three bullets; a probe that `1.0`, `v1.0.0`, and `.*` are rejected by `require_semver`; a stale-local-tag probe for F9. Each probe must name what it asserts. |
| F14 | R13, A4 | One EXIT trap owns every `mktemp -d` in `release.sh`; `step_fail` paths leak nothing. |
| F15 | R14, S12 | Lazy `go_bin()` resolved on first use; `set -e` exits inside `watch_run` and `cmd_docs` become `step_fail` lines. |
| F16 | S1, S2, S3, S4 | `tag_push_and_watch`, `commit_and_push <prefix>`, `watch_head_ci`; `watch_run` reads `id sha` from one `--jq` and `status conclusion` from one `run view`. Apply after F1-F3 so the helpers carry the fixed logic. |
| F17 | S6, S7, S8, S9, S11, S13 | Dead vars removed (`readme=".README.md"`, preflight `sha` now used by F4 so keep it, `case_n` printed in the summary), `changelog_has_section` single arm (per F5), fake-`gh` conclusion helper + `jq -n` assets, `seed_added_changelog`, the wrong `git init` comment, the `[ -z a ] || [ -z b ] &&` guard as an `if`. |
| F18 | R15 | `dry_echo` prefix off the always-printed ruleset line (print it as `release: NOTE`), dry-run root-bullet echo matches the real call, `module_kind` reports through `step_fail` only, the `version-propose` step keeps the `OK` shape (note in a following `release: NOTE` line). Add `shellcheck` to `mise.toml` (aqua backend, pinned) and a `shellcheck -S warning scripts/*.sh` step to `scripts/check.sh` beside actionlint; fix whatever it reports. |
| F19 | A12 | `scripts/check.sh all` runs `release-selftest`, `redaction-selftest`, `readme-drift-check`, and `shellcheck` once per gate (like actionlint), not once per module, so CI pays for them once. |

Checkpoints: F1-F5 (blockers in the script), F6-F8 (changelog helpers, drift check, docs wiring), F9-F12, F13 (self-test), F14-F17, F18-F19. Full gate after each.

## Not applied

- **A8** (Release watch has no sha pin): sound under D8; the tag cannot move. Recorded.
- **A11** (`step_fail` tails the log): keep; `gh` masks tokens and the script never asks for one.
- **A14** (`mise which go` in preflight): local-only, no network; keep.
- **S10** (probe boilerplate wrapper): churn across every probe for little gain; F13 adds probes in the existing style.
- **R16** (`docs/progress.md` stale next action): the lead's ship step.

## Lane-vs-lane

R2/A7, R12/A9/S5, R13/A4, R15/S6, S11/A13 are the same findings from different lanes. R11's patch default was questioned by the lead and answered by code review: keep the default, add the note (F10). Security's A1 is the one finding no other lane saw, and it is the most important: `cut mcp` could publish a tag off a red `main`.

## QA round (2026-09-03): PASS, seven advisories

- **Q1** (toolchain probe ran in the caller's cwd, so the self-test was not hermetic from outside the repo): fixed by the lead, `(cd "$repo_root" && mise which go)`.
- **Q2** (UTC dates vs the local-date precedent): `date +%F`, matching the five hand-cut sections.
- **Q3** (ship-commit subject diverges from the precedent once amended): the `release: NOTE` now says to reword the subject when amending.
- **Q4** (a new bullet landed before the section's own blank, splitting the list): the awk skips the blank after inserting under an existing heading; hand-verified three insertions contiguous.
- **Q5** (`--timeout` undocumented): one sentence in `docs/building.md`.
- **Q6** (SIGPIPE class at seven small sites): informational, inputs are far below the pipe buffer; recorded.
- **Q7** (`check.sh` temp leak on a failing `release.sh docs`): cleaned on the failure path.

All landed by the lead in the QA-report commit; the self-test, `shellcheck`, and `usage lint` were re-run.
