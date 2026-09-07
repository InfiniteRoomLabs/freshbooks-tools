# Security lane -- PR #6 (`fix/redaction-resolver-env`, commit d6e038d)

Verdict: **PASS** (no blocking findings; 4 advisories)

Scope: `git diff main...fix/redaction-resolver-env` -- 4 files, ~35 lines: `scripts/redaction-check.sh`, `scripts/redaction-selftest.sh`, `README.md`, `CHANGELOG.md`. Read-only review; no gate, tests, or selftest were run. Work order `docs/phases/_templates/security.md` items 1-7 (Go/OAuth/transport/supply-chain) do not apply -- this diff touches no Go code, no dependencies, and no workflow files. Item 8 (public-repo hygiene) plus the new trust boundary is the whole review.

## Summary

The change is a straight hygiene win. The operator's home-directory layout is gone from both scripts, the env var replaces it cleanly, the selftest swaps the stub resolver and the stub `uv` together (never one without the other), and the fail-open behavior is identical in kind to what it replaced. The new "env var names a file that gets executed" boundary does not meaningfully widen exposure, because both places that execute it -- CI and an operator's `mise run check` -- already execute PR-supplied code by design (`go test`, the Docusaurus site build). The advisories below are about the control going quietly dark in worktrees and in non-mise shells, which is a coverage regression rather than a vulnerability.

## (a) Operator home layout -- clean in the diff, two pre-existing mentions elsewhere

`git grep` over the whole branch tree for `<operator username>`, `projects/infinite-room-labs`, `agent-ops`, `$HOME`, `resolve-redaction-terms`:

- **The two scripts are clean.** `scripts/redaction-check.sh:36` is now `resolver="${REDACTION_TERMS_RESOLVER:-}"`; `scripts/redaction-selftest.sh:21` is the same; `scripts/redaction-selftest.sh:36` builds its stub at `$scratch/home/resolver/...` with no repo-shaped path. No `$HOME` remains in either file (`scripts/redaction-selftest.sh:45-46` mentions `$HOME` only in a comment about the mise shim, which is generic and reveals nothing).
- **No `<operator username>` anywhere in the tree**, on either branch.
- Surviving `agent-ops` mentions (`CLAUDE.md:28,78`, `GOAL.md:43-44`, `README.md:65`, spec 301/322/349/357) name the **public** marketplace repo `github.com/InfiniteRoomLabs/agent-ops`, which `README.md:65` links deliberately. Not a leak.
- The only `$HOME` left in `scripts/` is `scripts/release.sh:53` (`RELEASE_LOCAL_BIN="${RELEASE_LOCAL_BIN:-$HOME/.local/bin}"`) -- a generic XDG-ish default, no operator layout. Out of scope and fine.

**Pre-existing on `main`, not findings of this diff** (they predate the branch; `git grep ... main` confirms each):

- `docs/phases/0/plan.md:73` -- spells out `~/projects/infinite-room-labs/agent-ops/scripts/resolve-redaction-terms.py` verbatim.
- `docs/phases/8/reports/qa.md:229` -- same path, in the QA narrative.
- `docs/phases/3/reports/security.md:135` and `docs/phases/8/reports/fix.md:56-58` -- reference "the operator's home path" and `$HOME` abstractly; harmless.

The first two are the same string this PR just removed from the scripts, still sitting in tracked docs. See A1.

## (b) `REDACTION_TERMS_RESOLVER` as an execution vector -- real boundary, negligible delta

The variable names a file that reaches `uv run` in two places:

- `scripts/redaction-check.sh:43` -- `terms_raw=$(cd "$(dirname "$resolver")" && uv run "$(basename "$resolver")")`, guarded by `-n "$resolver"` and `-f "$resolver"` at line 38.
- `scripts/redaction-selftest.sh:298` -- the same call, guarded at line 297 by `-n "$resolver"`, `-f "$resolver"`, and `command -v uv`.

**Can a PR set it in the committed `mise.toml` and run code?** Mechanically yes: `mise.toml` is tracked, `mise.local.toml` is gitignored (`.gitignore:19`), and a PR could add an `[env]` block plus a payload file, then rely on `mise run <task>` exporting it. `mise.toml` on this branch has no `[env]` block at all, so the addition would be visible in review.

**Does that reach CI?** Only through the selftest. `.github/workflows/ci.yml:1-8` triggers on `pull_request` and `push: [main]` with `permissions: contents: read`; the `repo-wide` job (ci.yml:74) runs `mise run repo-wide` -> `scripts/check.sh repo-wide` -> `scripts/redaction-selftest.sh`. `scripts/redaction-check.sh` itself is **never** invoked by `mise.toml` or any workflow (only `[tasks.redaction-selftest]`, mise.toml:64-66, exists). So the reachable sink is `redaction-selftest.sh:298`, and only if `command -v uv` succeeds -- `uv` is not in `mise.toml [tools]` and is not part of the standard GitHub runner image, so the block is expected to be skipped in CI today. That is a coincidence, not a control.

**Is it a real change in exposure?** Practically no.

- Baseline: a `pull_request` run already executes arbitrary attacker-authored code -- `mise run check -- <module>` runs `go test` on the PR's own source, and `repo-wide` runs the Docusaurus/pnpm site build. Adding "and it might `uv run` a PR-supplied file" grants no capability the runner did not already hand over.
- Blast radius is bounded the right way: `permissions: contents: read`, and a fork `pull_request` run gets no repository secrets and a read-only token. Nothing to steal beyond public source.
- Old behavior was not "safe by design" either -- it was safe by accident: `$HOME/projects/...` simply does not exist on a runner, so the branch never fired.

**Local operator risk** is the more realistic version: a maintainer checks out a contributor branch and runs `mise run check`, and the committed `mise.toml [env]` would point the selftest at a PR file with the operator's own `uv` present. Same argument applies -- that same command already runs the PR's `go test`. `mise.local.toml` takes precedence over `mise.toml`, so an IRL operator's own value wins anyway; a maintainer without that file would not be protected.

Advisories A2 and A3 harden this cheaply.

## (c) Fail-open -- unchanged in kind, narrower in reach

`scripts/redaction-check.sh:38-41`: unset or missing file prints `redaction-check: term list not configured (optional for outside contributors)` and exits 0. That is exactly the old contract (main's `redaction-check.sh:35-38` did the same for a missing file), so the gate's fail-open property is not new, and it is the right call for a check that must be a no-op for outside contributors. Documenting it in `CLAUDE.md` as the lead intends is the correct resolution.

What *is* new is how easily the variable goes missing for an IRL operator:

- **Worktrees.** `mise.local.toml` is gitignored, so it exists in the main checkout but **not** in `.worktrees/redaction-env` (verified: `ls` finds it at the repo root, not in the worktree). `mise activate` resolves config from the cwd upward, so a shell started inside a worktree sees only the tracked `mise.toml`, which has no `[env]`. Phase batches and gate lanes run in worktrees. Under the old code the resolver was found in every worktree unconditionally.
- **Non-mise shells.** `scripts/redaction-check.sh` has no mise task and is invoked directly (`scripts/redaction-check.sh --range main..<branch>`), exactly as the spec at `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md:357` describes. `mise.local.toml [env]` only materializes under `mise run` / `mise exec` / an activate *hook*; the agent Bash environment documented in the global harness config uses `mise activate bash --shims`, which prepends shims to PATH and does **not** apply `[env]`. In this session the variable happened to be present, inherited from a parent shell that was activated in the main checkout -- inheritance, not configuration.
- `env -u` is never applied to the variable anywhere in the tree; the selftest's `env` invocations only drop `BASH_ENV`.

Net: the check can now silently no-op in the exact contexts (worktrees, direct invocation, a fresh terminal opened in a worktree) where it was previously reliable, and the notice on stdout looks identical to a normal outside-contributor run. See A2.

## (d) Stub vs real resolver -- no crossing possible

Confirmed. The stub resolver path and the stub `uv` are always set in the same `env` invocation, and never separately:

- `scripts/redaction-selftest.sh:55` (preflight), `:117` (stub probes), `:138` (smoke), `:226` (staged runs), `:283` (bad-range) each pass `PATH="$stub_bin:$PATH"` **and** `REDACTION_TERMS_RESOLVER="$stub_resolver"` together. There is no invocation that sets one without the other.
- `stub_bin` is never prepended to the ambient `PATH` of the selftest process itself, so the top-level real-resolver block at `:297-309` and the `real` probe branch at `:119` run with the ambient `PATH` (real `uv`) and the ambient variable (real resolver).
- The preflight at `:55-59` hard-fails if the stub `uv` is shadowed, so a stub probe cannot silently degrade into a real-`uv` run.
- The stub resolver is a zero-byte file (`: >"$stub_resolver"`, `:38`); it is never executed, because the stub `uv` shim (`:39-43`) ignores its argument and prints the two nonsense terms. So "the stub resolver run by the real `uv`" cannot happen either -- reaching that would require the real `uv` on PATH while the variable points at the stub, and no code path sets the variable to the stub without also swapping PATH.

## Findings

**A1 -- ADVISORY -- the removed path still lives in tracked docs.**
`docs/phases/0/plan.md:73`, `docs/phases/8/reports/qa.md:229` (both pre-existing on `main`, unchanged by this PR). Evidence: `git grep -n 'projects/infinite-room-labs/agent-ops' main -- docs/` returns exactly these two, spelling out `~/projects/<...>/agent-ops/scripts/resolve-redaction-terms.py`. The PR scrubs the scripts but leaves the same string published in the docs it is scrubbing them for, so the stated goal ("the public file no longer carries a home-directory layout") is only half true at the repo level. Fix: in a follow-up (phase reports are historical records -- do not rewrite them silently), replace the two occurrences with `<the resolver path, now $REDACTION_TERMS_RESOLVER>` and note the edit, or accept them explicitly as historical. Not blocking: the path is a conventional `~/projects/<org>/<repo>` layout naming a public repo, with no username, host, or secret in it.

**A2 -- ADVISORY -- the check goes silently dark in worktrees and non-mise shells.**
`scripts/redaction-check.sh:38-41`. Evidence: `mise.local.toml` is gitignored (`.gitignore:19`) and is absent from `.worktrees/redaction-env` while present at the repo root; `mise.toml` on this branch has no `[env]` block; `redaction-check.sh` is not a mise task, so the documented invocation (`scripts/redaction-check.sh --range ...`, spec:357) runs outside mise's env layer entirely. A gate lane or pre-commit run in a worktree therefore prints "term list not configured" and exits 0, indistinguishable from clean. Fix (pick one): (i) add a `[tasks.redaction-check]` to `mise.toml` so the check always runs through mise's env layer; (ii) have the CLAUDE.md note tell operators to symlink or copy `mise.local.toml` into each worktree as part of worktree setup; or (iii) add an opt-in strict switch -- `REDACTION_TERMS_REQUIRED=1` (or a `--require-terms` flag) making an unset/missing resolver exit 2 -- and set it wherever an IRL operator or gate lane runs the check, keeping the default no-op for outside contributors. (iii) is the one that actually makes "missing" distinguishable from "pass".

**A3 -- ADVISORY -- nothing prevents `mise.toml` from supplying the variable.**
`mise.toml` (no `[env]` block today), reaching `scripts/redaction-selftest.sh:297-298`. Evidence: `.github/workflows/ci.yml:3-4` runs on `pull_request`; ci.yml:74 runs `mise run repo-wide` -> the selftest; the selftest's real-resolver block executes `uv run` on whatever the ambient variable names. A PR adding `[env] REDACTION_TERMS_RESOLVER = "payload.py"` plus that file would reach the sink if `uv` is on PATH. Delta over the status quo is near zero (the same job already builds and tests PR-authored code, `permissions: contents: read`, no secrets on fork PRs), which is why this is advisory. Fix, cheapest first: reject a non-absolute path in both scripts (`case "$resolver" in /*) ;; *) echo "redaction-check: REDACTION_TERMS_RESOLVER must be an absolute path" >&2; exit 2 ;; esac`) -- this alone kills the in-repo-payload shape; optionally also refuse a resolver located inside the repo working tree.

**A4 -- ADVISORY -- the comment calls the resolver's home a "private repo".**
`scripts/redaction-check.sh:33-35` says "The resolver lives in a sibling private repo", while `README.md:65` links `InfiniteRoomLabs/agent-ops` as a public marketplace that contributors are told to install. Either the resolver is not in agent-ops, or "private" is wrong. Not a leak either way -- it is a public-facing accuracy nit that could mislead a contributor into thinking they cannot obtain the term list. Fix: reword to "lives outside this repo" and drop the privacy claim, since the point is only that the path is not this repo's business.

## Items 1-7 of the work order

Not applicable and verified as such: the diff adds no Go code, no dependency (`go.sum` untouched), no workflow change, no credential handling, no network path, no new user-controlled input reaching a shell other than the one analyzed in (b). No secret, token, account id, business id, internal hostname, or IP appears anywhere in the four changed files.
