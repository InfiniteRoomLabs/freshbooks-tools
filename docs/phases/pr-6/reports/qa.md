# PR #6 QA / reality-check lane -- `fix/redaction-resolver-env`

Subject: `fix/redaction-resolver-env` at `cda678d`, two commits ahead of `main` (`d6e038d` fix, `cda678d` review-gate fix commit). Run from the worktree `.worktrees/redaction-env`. Nothing in the tree was modified except this report.

## Verdict: PASS

Everything the two commit messages claim is in the tree, `mise run check` is green end to end, all five behaviour probes and both static checks behave exactly as documented, and the branch introduces no new operator-identifying string. Four ADVISORY findings below; none of them blocks the merge. The default verdict was NEEDS WORK -- it moved because every claim was independently reproduced, not because the lane reports said so.

## 1. Gate: green, tree clean

```
$ cd <operator home>/projects/infinite-room-labs/freshbooks-tools/.worktrees/redaction-env
$ mise run check
...
check.sh: all OK
GATE_EXIT=0
```

No dirty-tree banner fired (the only "dirty" string in the whole gate log is `release-selftest: PASS preflight FAILs on a dirty tree`, an assertion name). `git status --porcelain` was empty before the gate and empty again after it (the gate's `dist/` and `site/build/` output is gitignored); it is non-empty now only because this report exists, which `scripts/check.sh` excludes via `docs/phases/*/reports/*`.

Environment precondition, confirmed without printing the value: `mise env` from this worktree does export `REDACTION_TERMS_RESOLVER`, its value is an absolute path, and that path is an existing file. So the operator's gitignored `mise.local.toml [env]` block does reach a worktree under `.worktrees/` -- the claim `cda678d` makes and `CLAUDE.md:79` documents.

### `redaction-selftest` -- all 21 probes, both private-term probes armed

```
== redaction-selftest ==
redaction-selftest: PASS 7-digit integer fails, naming file:line
redaction-selftest: PASS the sweep reaches re-seeded fixtures
redaction-selftest: PASS a real id wearing a synthetic uuid tail fails
redaction-selftest: PASS an entirely decimal uuid-shaped token fails
redaction-selftest: PASS the synthetic uuid convention passes
redaction-selftest: PASS a genuine hex uuid passes
redaction-selftest: PASS a microsecond timestamp passes
redaction-selftest: PASS a space-separated instant passes
redaction-selftest: PASS a 700NN synthetic id passes (below the sweep threshold)
redaction-selftest: PASS an allowlisted filler number passes
redaction-selftest: PASS an all-zero run passes
redaction-selftest: PASS a short stub term fails in both modes
redaction-selftest: PASS a long stub term fails in both modes, mid-identifier
redaction-selftest: PASS a pre-existing number survives an unrelated edit
redaction-selftest: PASS a new number in a baselined file still fails
redaction-selftest: PASS a new file has no baseline
redaction-selftest: PASS an unusable range exits 2
redaction-selftest: PASS an unset REDACTION_TERMS_RESOLVER no-ops with the notice
redaction-selftest: PASS a relative REDACTION_TERMS_RESOLVER exits 2
redaction-selftest: PASS a short private term fails in both modes
redaction-selftest: PASS a long private term fails in both modes
redaction-selftest: OK
```

Both new probes are present and passing. The two private-term probes ran for real -- neither `redaction-selftest: NOTICE no short private term available` nor its long-term twin appears anywhere in the gate log, so the absolute-path guard added at `scripts/redaction-selftest.sh:334` admits this machine's resolver rather than skipping past it.

### Tail of the gate

```
== cover: cli ==
coverage-gate: .../cli/coverage.out total = 91.6% (floor 90%)
coverage-gate: PASS
== vuln: cli ==
No vulnerabilities found.
== inventory-check: cli (skipped -- only freshbooks has an inventory) ==
== build ==
build: mcp ./cmd/freshbooks-mcp -> .../dist/freshbooks-mcp_linux_amd64
...
== redaction-selftest ==   (above)
== release-selftest ==     (19 PASS)
== site-build ==
[SUCCESS] Generated static files in "build".
site-build: OK
check.sh: all OK
```

Module coverage: freshbooks 91.9%, cli 91.6%, mcp all packages at or above the floor. `inventory-check` reports implemented 213, uncovered 0, double-covered 0.

## 2. Commit claims vs the tree

### `d6e038d`

| # | Claim | Status | Evidence |
|---|---|---|---|
| 1 | `redaction-check.sh` reads `REDACTION_TERMS_RESOLVER` | MET | `scripts/redaction-check.sh:41` |
| 2 | the selftest reads it too | MET | `scripts/redaction-selftest.sh:21` |
| 3 | the selftest passes its stub through the same variable | MET | `scripts/redaction-selftest.sh:56-57` |
| 4 | no-op behaviour for outside contributors unchanged | MET | `scripts/redaction-check.sh:51-54` still exits 0 with a notice; probe (b) and (d) below |
| 5 | the hardcoded operator home path is gone from both files | MET | `git grep 'projects/infinite-room-labs' main` hits `scripts/redaction-check.sh:33`, `scripts/redaction-selftest.sh:21,35,36`; the same grep at HEAD hits neither file |

### `cda678d`

| # | Claim | Status | Evidence |
|---|---|---|---|
| 1 | relative resolver refused with exit 2, not resolved against cwd | MET | `scripts/redaction-check.sh:43-49`; probe (c) |
| 2 | header names the variable and the mise task | MET | `scripts/redaction-check.sh:8-11` |
| 3 | header says "outside this repo", not "private repo" (security A4) | MET | `scripts/redaction-check.sh:9` |
| 4 | `mise.toml` gains `[tasks.redaction-check]` | MET | `mise.toml:64-66` |
| 5 | `CLAUDE.md` points at `mise run redaction-check` | MET | `CLAUDE.md:30` |
| 6 | `README.md` points at `mise run redaction-check` | MET | `README.md:66` |
| 7 | `CLAUDE.md` Gotchas bullet: located via the var, missing var looks like a pass | MET | `CLAUDE.md:79` |
| 8 | verified `mise run` applies the root `[env]` block from a worktree under `.worktrees/` | MET | probe (a) below, and the `mise env` check in section 1 |
| 9 | one `stub_env` helper replaces the five stub invocations | MET | helper at `scripts/redaction-selftest.sh:55-58`; the five call sites it replaced are `:63`, `:125`, `:146`, `:234`, `:292` |
| 10 | the stub resolver moved out of the fake HOME (review A6) | MET | `scripts/redaction-selftest.sh:36-38` builds it at `$scratch/resolver/`, and `:37` no longer mkdirs a resolver-shaped tree under `$stub_home` |
| 11 | header and HOME comment corrected (review A1, simplify 5) | MET | `scripts/redaction-selftest.sh:8-12` and `:26-31` |
| 12 | the real-resolver guard requires an absolute path | MET | `scripts/redaction-selftest.sh:334` (`[ "${resolver#/}" != "$resolver" ]`) |
| 13 | two new probes (unset no-ops, relative exits 2) | MET | `scripts/redaction-selftest.sh:302-329`; both PASS in the gate output above |
| 14 | README "needs neither" made explicit (review A3) | MET | `README.md:66` -- "needs neither the resolver nor network access (it does still need `usage`)" |
| 15 | CHANGELOG entry under `[Unreleased] / ### Changed` | MET | `CHANGELOG.md:45`, first bullet of the section |
| 16 | lane reports under `docs/phases/pr-6/reports/` | MET | `code-review.md`, `simplify.md`, `security.md` |
| 17 | accepted-not-changed: `docs/phases/0/plan.md:73` and `docs/phases/8/reports/qa.md:229` still spell the old path | MET (accurately described) | `git grep -n 'projects/infinite-room-labs' HEAD` hits exactly those two plus the three self-referential mentions in `docs/phases/pr-6/reports/security.md` |

Nothing claimed is missing. Two OPTIONAL simplify items (`simplify.md:57` trim the comment block, `simplify.md:97` drop the redundant `-z` guard) were not applied; both were explicitly labelled optional, and the commit message does not claim them.

## 3. Behaviour probes

**(a) the documented operator invocation**

```
$ mise run redaction-check -- --range main..HEAD
[redaction-check] $ scripts/redaction-check.sh --range main..HEAD
redaction-check: clean
exit=0
```

This also proves `mise run <task> -- <args>` forwards the flags to the script, which the `CLAUDE.md:79` gate-lane recipe depends on.

**(b) unset variable no-ops**

```
$ env -u REDACTION_TERMS_RESOLVER scripts/redaction-check.sh --range main..HEAD
redaction-check: term list not configured (optional for outside contributors)
exit=0
```

**(c) relative value refused**

```
$ REDACTION_TERMS_RESOLVER=relative/x.py scripts/redaction-check.sh
redaction-check: REDACTION_TERMS_RESOLVER must be an absolute path: relative/x.py
exit=2
```

**(d) absolute but missing file no-ops**

```
$ REDACTION_TERMS_RESOLVER=/nonexistent/x.py scripts/redaction-check.sh
redaction-check: term list not configured (optional for outside contributors)
exit=0
```

**(e) shellcheck**

```
$ shellcheck -S warning -e SC1008,SC2154 scripts/*.sh
(no output)
exit=0
```

**(f) usage lint**

```
$ usage lint scripts/redaction-check.sh
No issues found.
$ usage lint scripts/redaction-selftest.sh
No issues found.
```

Incidental observation, not a finding: in an agent Bash shell the variable is already present, inherited from the mise-activated login shell this session was launched from. Probe (b) therefore needed the explicit `env -u`. This matches, rather than contradicts, `CLAUDE.md:79` ("a bare `scripts/redaction-check.sh` only sees it in a mise-activated shell").

## 4. Public-repo hygiene of the branch

Commands (run at both refs; only file:line was printed, never the matched content):

```
git grep -n -i -- "<operator username>" HEAD   /  main
git grep -n --    "/home/"               HEAD   /  main
git grep -n --    "projects/infinite-room-labs" HEAD / main
```

| Term | `main` | `HEAD` | Delta |
|---|---|---|---|
| operator username | 1 (`docs/phases/3/reports/security.md:135`) | 1 (same file:line) | none |
| `/home/` | 7 | 8 | +1: `docs/phases/pr-6/reports/security.md:15`, which is the string `$scratch/home/resolver/...` inside a sentence about the stub -- not an operator path |
| `projects/infinite-room-labs` | 7 | 6 | **-1 net**: the branch removes 4 (`scripts/redaction-check.sh:33`, `scripts/redaction-selftest.sh:21,35,36`) and adds 3, all inside `docs/phases/pr-6/reports/security.md` (`:13`, `:22`, `:73`) where the security lane quotes the pre-existing docs hits it is reporting |

The three lane reports under `docs/phases/pr-6/reports/` contain **zero** occurrences of the operator username and **zero** occurrences of the literal operator home path (`grep -rn -i "<username>" docs/phases/pr-6/reports/` -> 0; `grep -rn "/home/<username>" docs/phases/pr-6/reports/` -> 0). They refer to it as `<operator username>` throughout.

`mise.local.toml` -- the file that actually carries the operator path -- is ignored (`.gitignore:19`) and is not tracked anywhere in the repo (`git ls-files | grep mise.local` -> empty). That is the load-bearing check for this whole change and it holds.

Net effect: the branch strictly reduces the operator-layout surface of the public tree. No new leak.

## Findings

### 1. ADVISORY -- `README.md:66`: "no-ops unless ... absolute path" is wrong for one of the three cases

Expected: the only public documentation of `REDACTION_TERMS_RESOLVER` describes what the script actually does with a bad value. Observed: it says the check "no-ops unless `REDACTION_TERMS_RESOLVER` names a term-list resolver by absolute path", which reads as "anything that is not an absolute-path resolver -> no-op". A *relative* value does not no-op; it exits 2 and fails the caller (`scripts/redaction-check.sh:43-49`, probe (c)). Unset and absolute-but-missing do no-op. An outside contributor who follows a blog post and sets `REDACTION_TERMS_RESOLVER=./terms.py` gets a hard exit 2 that the README told them would be a no-op. Suggested wording: "... it no-ops when `REDACTION_TERMS_RESOLVER` is unset or names a file that is not present, and refuses a relative value outright."

Not blocking: the failure mode is loud, exits with a distinct message naming the variable, and cannot produce a false pass.

### 2. ADVISORY -- `docs/phases/pr-6/reports/security.md:14`: "No `<operator username>` anywhere in the tree, on either branch" is false

Expected: a security lane's grep claim to match the tree. Observed: `git grep -i -- "<username>" HEAD` returns exactly one hit, `docs/phases/3/reports/security.md:135`, where the username appears verbatim in a list of terms that lane grepped for. The same hit exists on `main`, so it is pre-existing and **not** introduced by this PR -- but the report asserts its absence rather than listing it, and `security.md:23` separately describes that same file:line as referencing "the operator's home path ... abstractly", which is not what line 135 does.

Impact is a stale claim in a frozen lane report, plus one pre-existing username mention already published in a public repo. Out of this PR's scope to fix (the same rationale the commit message applies to `docs/phases/0/plan.md:73`), but the lead should know the sweeping "on either branch" claim does not hold, and may want a follow-up scrub of `docs/phases/3/reports/security.md:135` alongside the two paths already deferred.

### 3. ADVISORY -- `docs/phases/pr-6/reports/security.md:15`: describes the stub resolver at a path it does not use

Expected: the security report's description of the fix to match the code. Observed: it says `scripts/redaction-selftest.sh:36` "builds its stub at `$scratch/home/resolver/...`". The actual line 36 is `stub_resolver="$scratch/resolver/resolve-redaction-terms.py"` -- deliberately *outside* `$stub_home` (`$scratch/home`), which is the whole point of code-review A6. The report's own conclusion is right (no repo-shaped path); the path it quotes is not. Cosmetic, in a frozen report.

### 4. ADVISORY (nit) -- `scripts/redaction-check.sh:11`: one comment line breaks the block's wrap

Expected: the header comment block to wrap consistently, as the surrounding lines do at roughly 72 columns. Observed: line 11 runs to 96 characters ("Operators run it as `mise run redaction-check`. scripts/redaction-selftest.sh is this script's regression test"), because the inserted sentence was not re-flowed. No functional effect; `shellcheck` and `usage lint` are both clean. Mentioned only because a future editor of that block will re-wrap it and produce diff noise.

## What I did not check

- CI on GitHub. `mise run check` is green locally; the branch has not been observed running through `.github/workflows/ci.yml`.
- The contents of the private term list, or whether it is complete. Out of lane scope; the probes only assert that the control is armed and that terms drawn from it are matched.
- The two pre-existing path mentions the commit message explicitly accepts (`docs/phases/0/plan.md:73`, `docs/phases/8/reports/qa.md:229`). Confirmed present and unchanged; deliberately not re-litigated.

## Commands run

```
git show cda678d --stat
git diff main...HEAD [--stat, and scoped to scripts/ mise.toml CLAUDE.md README.md CHANGELOG.md]
mise run check
mise env | grep REDACTION_TERMS_RESOLVER   (existence / absolute / is-a-file only; value never printed)
mise run redaction-check -- --range main..HEAD
env -u REDACTION_TERMS_RESOLVER scripts/redaction-check.sh --range main..HEAD
REDACTION_TERMS_RESOLVER=relative/x.py scripts/redaction-check.sh
REDACTION_TERMS_RESOLVER=/nonexistent/x.py scripts/redaction-check.sh
shellcheck -S warning -e SC1008,SC2154 scripts/*.sh
usage lint scripts/redaction-check.sh ; usage lint scripts/redaction-selftest.sh
git grep -n -i -- "<username>" HEAD | main
git grep -n -- "/home/" HEAD | main
git grep -n -- "projects/infinite-room-labs" HEAD | main
git check-ignore -v mise.local.toml ; git ls-files | grep mise.local
git status --porcelain   (before and after the gate)
```
