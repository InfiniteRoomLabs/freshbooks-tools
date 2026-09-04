# Phase 9 review -- simplification lane

Branch `phase-9/release-automation`, 5 commits `7738689..12b0e88` (`git diff main...phase-9/release-automation`). Scope: `scripts/release.sh` (941 lines), `scripts/release-selftest.sh` (536), the wiring in `scripts/check.sh` / `mise.toml`, and the D7 docs.

**PROPOSE ONLY.** Nothing here was edited, staged, committed, or executed; `scripts/release.sh` was read, never run. No finding changes the output contract (`release: OK/SKIP/FAIL <step>` text or step names), the subcommand/flag surface, resume or dry-run semantics, exit codes, or what the self-test asserts. No new dependencies -- one finding removes one (`jq`).

Verdict: the branch is in good shape. The step-level structure is right, the comments that carry evidence (the `local` same-statement scoping bug in `cut_lib`, the `commit_with_subject_exists` rationale, the `verify_after_cut` rationale, the `RELEASE_GO_BIN` / scratch-state notes) are all worth keeping and I propose keeping every one of them. What is left is mechanical duplication: three near-identical commit/push blocks, three near-identical CI-watch blocks, and a `cut_binary` whose body is a verbatim slice of `cut_lib`. Applying S1-S8 removes roughly 150 lines across the two scripts with no behavioural delta.

## On the suggested `run_step <name> <fn>` wrapper

I looked at this and recommend **against** a single generic wrapper. The steps are not uniform: some print SKIP from a postcondition and then continue, some print SKIP *instead of* a group of later steps, some emit two step lines (`cut-tag` + `cut-tag-push`) from one block, and `bump-check-<module>` emits one line for a five-command loop. A wrapper general enough to cover all of that ends up taking a skip-predicate, a dry-run message and a step-name list, i.e. more machinery than it removes. The duplication that actually exists is at a coarser grain -- whole *blocks*, not individual steps -- so S1-S3 propose three narrow helpers instead. That is the same amount of deduplication with far less indirection.

---

## S1 -- `cut_binary` duplicates `cut_lib`'s tag/watch tail verbatim -- APPLY-RECOMMENDED

`scripts/release.sh:701-720` and `scripts/release.sh:729-748`.

The 20 lines from the `ls-remote` skip check through `watch_run "cut-release-watch"` are character-identical in both functions. `cut_binary` is *nothing but* that block.

Before (twice):

```bash
if git -C "$repo_root" ls-remote --tags origin "refs/tags/$tag" 2>>"$LOG_FILE" | grep -q "$tag"; then
  step_skip "cut-tag-push" "$tag already on origin"
else
  confirm_first_push
  if ! git -C "$repo_root" rev-parse "$tag" >/dev/null 2>&1; then
    run_cmd git -C "$repo_root" tag -a "$tag" -m "$module $version" || step_fail "cut-tag" ...
  fi
  step_ok "cut-tag"
  run_cmd git -C "$repo_root" push origin "$tag" || step_fail "cut-tag-push" ...
  step_ok "cut-tag-push"
fi
if [ "$DRY_RUN" = true ]; then
  dry_echo "watch the Release workflow for $tag"
  step_skip "cut-release-watch" "dry-run"
else
  watch_run "cut-release-watch" "Release" "$tag"
fi
```

After -- one helper, called from both:

```bash
# tag_push_and_watch <module> <version> <tag> -- steps cut-tag,
# cut-tag-push, cut-release-watch. Shared by cut_lib and cut_binary.
tag_push_and_watch() { ...the block above, unchanged... }

cut_binary() {
  tag_push_and_watch "$1" "$2" "$3"
}
```

Behaviour-preserving: the moved text is byte-identical and the three parameters are the only free variables in it (`$LOG_FILE`, `$DRY_RUN`, `$repo_root` stay global). Step names, ordering and skip reasons are untouched.

Risk: low. Covered by the happy-path, resume, and release-fails probes, which assert `cut-tag-push` SKIP on resume and `cut-release-watch` FAIL, both through this block for all three modules.

## S2 -- three identical commit-and-push blocks -- APPLY-RECOMMENDED

`scripts/release.sh:675-692` (`cut-commit`/`cut-push`), `scripts/release.sh:839-856` (`bump-commit`/`bump-push`), `scripts/release.sh:901-918` (`all-ship-commit`/`all-ship-push`).

All three are the same shape and the step names are uniformly `<prefix>-commit` / `<prefix>-push`, so the prefix is the only variable besides the subject and the file list.

Before (three times, ~18 lines each):

```bash
local expect_subject="release($module): v$version"
if commit_with_subject_exists "$expect_subject"; then
  step_skip "cut-commit" "a commit \"$expect_subject\" is already on main"
else
  confirm_first_push
  run_cmd git -C "$repo_root" add "$module/CHANGELOG.md" CHANGELOG.md || step_fail "cut-commit" "git add to succeed" "failed"
  run_cmd git -C "$repo_root" commit -m "$expect_subject" || step_fail "cut-commit" "git commit to succeed" "failed"
  step_ok "cut-commit"
  confirm_first_push
  run_cmd git -C "$repo_root" push origin main || step_fail "cut-push" "git push origin main to succeed" "failed"
  step_ok "cut-push"
fi
```

After:

```bash
# commit_and_push <step-prefix> <subject> <path>... -- emits
# <prefix>-commit and <prefix>-push, or SKIPs both when the subject is
# already on main (D3). Staging and committing stay separate run_cmd
# calls: the agent-ops guards inspect the index between them.
commit_and_push() {
  local prefix="$1" subject="$2"
  shift 2
  if commit_with_subject_exists "$subject"; then
    step_skip "$prefix-commit" "a commit \"$subject\" is already on main"
    return 0
  fi
  confirm_first_push
  run_cmd git -C "$repo_root" add "$@"           || step_fail "$prefix-commit" "git add to succeed" "failed"
  run_cmd git -C "$repo_root" commit -m "$subject" || step_fail "$prefix-commit" "git commit to succeed" "failed"
  step_ok "$prefix-commit"
  confirm_first_push
  run_cmd git -C "$repo_root" push origin main   || step_fail "$prefix-push" "git push origin main to succeed" "failed"
  step_ok "$prefix-push"
}

commit_and_push cut "release($module): v$version" "$module/CHANGELOG.md" CHANGELOG.md
commit_and_push bump "$expect_subject" mcp/go.mod mcp/go.sum mcp/CHANGELOG.md cli/go.mod cli/go.sum cli/CHANGELOG.md CHANGELOG.md
commit_and_push all-ship "docs: ship v$lib_version" README.md
```

Behaviour-preserving: same commands in the same order, same two `confirm_first_push` calls (the second is a no-op once `CONFIRMED=true`), same step names and identical `expected/observed` strings. The skip path returns before the push exactly as the current `if/else` does.

Risk: low-medium -- it is the largest textual change of the set. Verify by diffing the `release:` lines of `all --dry-run` before and after; the resume probe already asserts `SKIP cut-commit`.

## S3 -- three identical "watch CI for HEAD" blocks -- APPLY-RECOMMENDED

`scripts/release.sh:692-699`, `scripts/release.sh:856-863`, `scripts/release.sh:918-926`.

Before (three times):

```bash
if [ "$DRY_RUN" = true ]; then
  dry_echo "watch CI on main for the release commit"   # "bump" / "ship" in the other two
  step_skip "cut-ci-watch" "dry-run"
else
  local sha
  sha=$(git -C "$repo_root" rev-parse HEAD)
  watch_run "cut-ci-watch" "CI" "main" "$sha"
fi
```

After:

```bash
# watch_head_ci <step> <what> -- watches the CI run for HEAD, or echoes
# the plan and SKIPs under --dry-run (where the commit was never made).
watch_head_ci() {
  if [ "$DRY_RUN" = true ]; then
    dry_echo "watch CI on main for the $2 commit"
    step_skip "$1" "dry-run"
    return 0
  fi
  watch_run "$1" "CI" "main" "$(git -C "$repo_root" rev-parse HEAD)"
}

watch_head_ci cut-ci-watch release
watch_head_ci bump-ci-watch bump
watch_head_ci all-ship-ci-watch ship
```

Behaviour-preserving: same step names, same SKIP reason, same `dry-run:` text (the noun is parametrised, and the self-test greps only `dry-run: git`). Also drops three `local sha` declarations that shadow nothing.

Risk: low.

## S4 -- `watch_run` hand-parses JSON with grep, and polls twice per tick -- APPLY-RECOMMENDED

`scripts/release.sh:305-313` (parse) and `scripts/release.sh:317-318` (poll).

Two separate issues in one function. First, the run list is fetched with `--jq '.[0] // empty'` and then re-parsed out of the resulting JSON with `grep -oE` + `sed`, in a file that has a `gh_jq` helper for exactly this. Second, each poll iteration makes **two** `gh run view` calls for two fields of the same object -- double the API calls, and a window where `status` and `conclusion` come from different reads.

Before:

```bash
run_json=$(read_cmd gh run list ... --json databaseId,headSha,conclusion,status --jq '.[0] // empty') || true
[ -z "$run_json" ] && step_fail ...
id=$(printf '%s' "$run_json" | grep -oE '"databaseId":[0-9]+' | grep -oE '[0-9]+')
sha=$(printf '%s' "$run_json" | grep -oE '"headSha":"[^"]*"' | sed -E 's/.*:"(.*)"/\1/')
...
status=$(gh_jq '.status' -- run view "$id" --json status)
conclusion=$(gh_jq '.conclusion' -- run view "$id" --json conclusion)
```

After:

```bash
run_json=$(read_cmd gh run list ... --json databaseId,headSha --jq '.[0] // empty | "\(.databaseId) \(.headSha)"') || true
[ -z "$run_json" ] && step_fail ...
read -r id sha <<<"$run_json"
...
read -r status conclusion <<<"$(gh_jq '"\(.status) \(.conclusion // "")"' -- run view "$id" --json status,conclusion)"
```

Behaviour-preserving: same values, same comparisons. `conclusion` is only read after `status = completed`, where it is non-null, so the `// ""` default only affects the discarded in-flight reads. The self-test's fake `gh` routes `--jq` through real `jq`, so both forms work against it unchanged. `conclusion,status` can drop out of the `run list --json` set -- they were already unused there.

Risk: low. Halves the API calls of a 20-minute poll loop.

## S5 -- `do_verify` shells out to external `jq` three times, contradicting its own header -- APPLY-RECOMMENDED

`scripts/release.sh:115` says "gh has an embedded jq; no external jq dependency", but `scripts/release.sh:435,437,439` run the real `jq` binary three times over one blob -- an undeclared runtime dependency the plan's Rules section explicitly asked to avoid ("prefer `gh --jq`").

Before:

```bash
json=$(read_cmd gh release view "$tag" --json isDraft,name,assets) || step_fail ...
isdraft=$(printf '%s' "$json" | jq -r '.isDraft' 2>>"$LOG_FILE") || step_fail "verify-release" "valid JSON from gh release view" "unparsable"
name=$(printf '%s' "$json" | jq -r '.name' 2>>"$LOG_FILE")      || step_fail ... (same)
assets=$(printf '%s' "$json" | jq -r '.assets | length' 2>>"$LOG_FILE") || step_fail ... (same)
```

After:

```bash
json=$(gh_jq '"\(.isDraft) \(.name) \(.assets | length)"' -- release view "$tag" --json isDraft,name,assets) ||
  step_fail "verify-release" "a published release for $tag" "gh release view failed"
read -r isdraft name assets <<<"$json"
```

Behaviour-preserving: the same three values, the same subsequent comparisons, one `gh` call instead of one `gh` plus three `jq`. The `name` field is compared against `$tag`, which never contains whitespace, so `read -r` splits correctly. The three identical "unparsable" `step_fail` arms collapse into the existing "gh release view failed" arm -- both were already reachable only when `gh` misbehaves, and the `verify-release` step name is unchanged either way.

Risk: low. Directly covered by the verify-fail probes.

## S6 -- three dead variables -- APPLY-RECOMMENDED

- `scripts/release.sh:399`: `local module tag readme=".README.md" tmp` -- `readme` is overwritten on the very next line by `readme="$repo_root/README.md"`. Drop the `.README.md` initialiser; it reads like a leftover and a dot-file target would be a real bug if the next line ever moved.
- `scripts/release.sh:376`: `sha=$(git -C "$repo_root" rev-parse HEAD)` in `cmd_preflight` is never read, and the `gh run list` on the next line asks for `headSha` and never uses it. Drop both. **Note for the lead (not a simplification):** the plan's preflight says CI must be green *for HEAD*; the code only checks the newest `main` run's conclusion. Deleting `sha` makes that gap visible rather than looking half-implemented -- adding the comparison is a behaviour change and belongs to the code-review lane's call, not this one.
- `scripts/release-selftest.sh:24` + nine `case_n=$((case_n + 1))` lines: `case_n` is incremented and never read. Either delete all ten lines, or -- better -- print it in the final summary (`release-selftest: OK (N cases)`), which costs one line and makes a silently-skipped probe visible.

Behaviour-preserving: unused values. Risk: nil.

## S7 -- `changelog_has_section`'s first grep is subsumed by its second -- APPLY-RECOMMENDED

`scripts/release.sh:138-140`.

```bash
# before
grep -qxF "## [$2]" "$1" 2>/dev/null || grep -qE "^## \\[$2\\]" "$1" 2>/dev/null
# after
grep -qE "^## \[$2\]" "$1" 2>/dev/null
```

Every line the `-qxF` exact-match arm can match (`## [1.2.3]` alone on a line) is also matched by the `^## \[1.2.3\]` prefix regex, which additionally catches the real-world `## [1.2.3] - 2026-09-03` that `changelog_cut_section` actually writes. The first arm is pure dead weight.

Behaviour-preserving: strictly identical result set. Risk: nil. (`$2` is a version of digits and dots, so the unescaped `.` in the regex is harmless -- unchanged from today either way.)

## S8 -- fake `gh` shim: duplicated conclusion selection, hand-built assets JSON -- APPLY-RECOMMENDED

`scripts/release-selftest.sh:91-113` (the `run list` and `run view` arms) repeat the same seven lines:

```bash
workflow=$(cat "$FAKE_GH_STATE/last-workflow" 2>/dev/null || echo CI)
if [ "$workflow" = "Release" ]; then
  conclusion="${FAKE_GH_RELEASE_CONCLUSION:-success}"
else
  conclusion="${FAKE_GH_CI_CONCLUSION:-success}"
fi
```

Hoist to one `conclusion_for_workflow()` above the `case`, called from both arms.

`scripts/release-selftest.sh:122-130` builds the assets array with a counter loop:

```bash
assets_json="["; n=0
while [ "$n" -lt "$assets" ]; do
  [ "$n" -gt 0 ] && assets_json+=","
  assets_json+="{\"name\":\"asset$n\"}"
  n=$((n + 1))
done
assets_json+="]"
json=$(printf '{"isDraft":false,"name":"%s","assets":%s}' "$tag" "$assets_json")
```

The shim already depends on `jq` (its `emit_json`), so this is one line:

```bash
json=$(jq -cn --arg tag "$tag" --argjson n "$assets" \
  '{isDraft:false, name:$tag, assets:[range($n)|{name:"asset\(.)"}]}')
```

Behaviour-preserving: identical JSON shape; only `.assets | length` and `.isDraft`/`.name` are ever read from it. No new dependency -- `jq` is already required by this shim. Risk: low; a mistake here fails the self-test loudly and locally.

## S9 -- the self-test repeats its "seed an Added changelog" preamble three times -- APPLY-RECOMMENDED

`scripts/release-selftest.sh:406-414`, `:433-441`, `:477-485` -- byte-identical (the fourth, `:385-390`, differs only in bullet text and has no commit):

```bash
{ changelog_module; echo; echo "### Added"; echo "- v1"; } >"$w/freshbooks/CHANGELOG.md"
git -C "$w" add -A
git -C "$w" commit -q -m "seed unreleased notes"
git -C "$w" push -q origin main
```

After: `seed_added_changelog "$w"` beside the existing `new_scratch_repo`, which is already the right pattern -- the builder itself is correctly called once per probe, not per assertion, so nothing else needs restructuring there.

Behaviour-preserving: same file content, same commit subject, same push. Risk: nil.

## S10 -- self-test probe boilerplate (`set +e` / `status=$?` / `set -e`) x9 -- OPTIONAL

Every probe repeats:

```bash
set +e
RELEASE_EXTRA_ENV=()
out=$(release_run "$w" ... 2>&1)
status=$?
set -e
```

A `capture()` wrapper (`capture "$w" preflight` setting `out`/`status`) removes ~27 lines. I am tagging this OPTIONAL rather than recommended because the assertions afterwards genuinely differ per probe (some also diff `ls-remote` output), so only the capture half factors cleanly, and the current form is at least explicit. If it is taken, also let `release_run` reset `RELEASE_EXTRA_ENV=()` on exit so the six defensive `RELEASE_EXTRA_ENV=()` resets can go.

Risk: low, but it touches every probe -- lower payoff per unit of churn than S1-S9.

## S11 -- a comment that misstates the code -- APPLY-RECOMMENDED

`scripts/release-selftest.sh:294`:

```bash
git -C "$work" init -q -b main >/dev/null 2>&1 || true # init -b main above already did this; harmless if already a repo
```

The `git init` "above" (`:273`) initialises `$origin`, the *bare* remote -- not `$work`. This line is the one and only init of the work tree, so the comment is wrong, and the `|| true` plus double redirect suppress the failure of a step everything downstream depends on. Replace with a plain `git -C "$work" init -q -b main` and a comment saying what it is (`# the work tree; $origin above is the bare remote`). This is the one comment in the branch I would change -- the rest carry evidence and should stay exactly as they are.

Behaviour-preserving: in the current call pattern the init always succeeds, so removing the swallow changes nothing except which failure mode is visible. Risk: nil.

## S12 -- `GO_BIN` is resolved eagerly on every invocation -- OPTIONAL

`scripts/release.sh:58`:

```bash
GO_BIN="${RELEASE_GO_BIN:-$(cd "$repo_root" && mise where go)/bin/go}"
```

This runs `mise where go` for `preflight` and `docs` too, neither of which uses `GO_BIN` -- including on every `mise run check`, via the new `readme-drift-check` step. Making it a memoised `go_bin()` called from `do_verify_proxy_pickup` / `do_verify_binary` / `do_bump` removes a subprocess from the gate.

Secondary reason, worth the lead's attention: because this is a top-level command substitution under `set -euo pipefail`, a `mise where go` failure kills the script *before any `release:` line is printed*, which is the one place the output contract can be violated without a `FAIL` line. Lazy resolution confines that to a step that can `step_fail` properly. Tagged OPTIONAL because the fix's value is mostly robustness, and the robustness half is the code-review lane's call.

## S13 -- `[ -z "$a" ] || [ -z "$b" ] && step_fail` -- OPTIONAL

`scripts/release.sh:590`:

```bash
[ -z "$module" ] || [ -z "$version" ] && step_fail "cut" "cut <module> <version>" "missing argument(s)"
```

This parses as `([ -z a ] || [ -z b ]) && step_fail` and does currently do the right thing (bash's `errexit` exemption for non-final commands in an AND-OR list is what keeps the both-args-present case from exiting silently). It is a well-known footgun that survives only by that exemption; the sibling guards in `cmd_bump` / `cmd_all` use the simple single-test form and are not exposed to it. Prefer:

```bash
if [ -z "$module" ] || [ -z "$version" ]; then
  step_fail "cut" "cut <module> <version>" "missing argument(s)"
fi
```

Behaviour-preserving in every reachable case. Risk: nil.

---

## Considered and rejected

- **`changelog_add_bullet`'s three-awk-pass split (`scripts/release.sh:233-264`) -- DO-NOT-APPLY.** It is the ugliest function in the branch (an awk for the head, `changelog_unreleased_section` for the middle, a third awk for the tail, plus a `sed` to trim leading blanks) and it *would* collapse into a single awk pass. But no test asserts the resulting changelog text anywhere -- the self-test checks step lines, tags and refs, never file content -- so a rewrite here would be unverifiable by the gate, on the one function whose output lands in a committed, human-read file. Not worth it inside a review-gate fix commit. (Flagging the coverage gap itself for QA's lane: a probe asserting the post-`cut` `freshbooks/CHANGELOG.md` body would be cheap and would unlock this.)
- **`cmd_verify` / `verify_after_cut` / `do_verify` three-layer split -- DO-NOT-APPLY.** Looks like indirection, is not: the comment at `scripts/release.sh:569-575` documents exactly why the `cut`-internal path must SKIP under `--dry-run` while the standalone subcommand must not. Keep the layers and the comment.
- **`derive_bump_kind`'s explicit `### Fixed` branch (`scripts/release.sh:175-177`) -- DO-NOT-APPLY.** Technically dead (the `echo patch` fallback below it returns the same answer), but it is the line that encodes the plan's "only `### Fixed` -> patch" rule, and the self-test asserts that rule by name. Deleting it would save one branch and lose the mapping between code and spec.
- **`local` scoping and `commit_with_subject_exists` comments -- KEEP VERBATIM.** Both record a bug that was actually hit (`local a=$1 b=$a`, and `git log -1` being wrong mid-`all`). These are the two highest-value comments in the file.
- **The unconditional `dry_echo` in `preflight` (`scripts/release.sh:390-392`)** prints a `dry-run:` line outside `--dry-run`, which reads odd against D2 -- but D8 requires printing the ruleset call unconditionally and the comment says so. Leave it.

## Would anything be simpler as a `mise` task?

Essentially no. The wiring is right: `[tasks.release]` and `[tasks.release-selftest]` are thin passthroughs, and `mise` appends the post-`--` arguments, so the plan's `"$@"` is unnecessary. The one gap is asymmetry rather than complexity: `readme-drift-check` is reachable as `scripts/check.sh readme-drift-check` and inside `check all`, but has no `[tasks.readme-drift-check]` of its own the way `redaction-selftest` and `release-selftest` do. A three-line task entry would make the new gate step individually runnable when it fires. OPTIONAL, and cosmetic.

Nothing in `release.sh` wants to move into `mise.toml` -- the step logic is inherently sequential shell with postcondition checks, which is exactly what a script is for.
