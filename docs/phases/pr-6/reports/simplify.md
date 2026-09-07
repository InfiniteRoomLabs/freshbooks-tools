# Simplification lane -- PR #6 (`fix/redaction-resolver-env`, commit d6e038d)

Scope: `scripts/redaction-check.sh`, `scripts/redaction-selftest.sh`, the README line, the CHANGELOG entry. Propose-only: nothing was modified, nothing committed, no gate/test/build/selftest was run.

Verdict up front: **this is a clean hygiene diff and there is very little to cut.** One proposal is worth applying (a five-call helper in the selftest that this very commit had to edit five times); the rest are cosmetic or explicitly rejected. Line numbers are post-change (as the files stand on the branch).

---

## 1. APPLY-RECOMMENDED -- collapse the five stub-env invocations into one helper

`scripts/redaction-selftest.sh:55,117,138,226,283`

The same six-token env prefix appears verbatim five times, and this commit had to touch all five to add one variable. That is the strongest argument for the helper: it is not speculative future-proofing, the diff already paid the five-site tax once.

Before (five copies, one shown):

```
resolved_uv=$(env -u BASH_ENV HOME="$stub_home" PATH="$stub_bin:$PATH" REDACTION_TERMS_RESOLVER="$stub_resolver" bash -c 'command -v uv')
...
out=$(cd "$dir" && env -u BASH_ENV HOME="$stub_home" PATH="$stub_bin:$PATH" REDACTION_TERMS_RESOLVER="$stub_resolver" ./redaction-check.sh "${args[@]}" 2>&1)
```

After -- define once, immediately below the `stub_usage` symlink (`:49`) and above the preflight (`:51`):

```
# stub_env runs a command in the stub environment: the fake HOME, the stub
# PATH, and the stub resolver. BASH_ENV is dropped because it re-activates
# mise's shims and re-prepends them to PATH, which would shadow the stub uv.
stub_env() {
  env -u BASH_ENV HOME="$stub_home" PATH="$stub_bin:$PATH" \
    REDACTION_TERMS_RESOLVER="$stub_resolver" "$@"
}
```

Call sites become:

```
resolved_uv=$(stub_env bash -c 'command -v uv')
out=$(cd "$dir" && stub_env ./redaction-check.sh "${args[@]}" 2>&1)
if ! smoke_out=$(cd "$smoke_dir" && stub_env ./redaction-check.sh 2>&1); then
staged_out=$(cd "$1" && stub_env ./redaction-check.sh 2>&1)
bad_out=$(cd "$bad_dir" && stub_env ./redaction-check.sh --range no-such-ref..HEAD 2>&1)
```

Why it is behaviour-preserving:

- `stub_home` (`:33`), `stub_bin` (`:34`) and `stub_resolver` (`:36`) are each assigned exactly once and never reassigned, so call-time expansion inside the function yields the identical strings the inline copies do.
- The function's last (only) command is `env`, so its exit status is `env`'s -- unchanged for the `set +e` regions at `:115-122`, `:224-228` and `:282-285`, and for the `if !` at `:138`.
- Command substitution, the preceding `cd`, and `2>&1` all stay at the call site, so the subshell and redirection semantics are untouched.
- `"$@"` preserves argument boundaries, including the empty-array case `"${args[@]}"` under `set -u` (bash 4.4+; the script already relies on this at `:110-114`).
- No exit code, printed notice, probe, or stub-environment variable changes. The set of probes that run and what they exercise is identical.

Secondary win: the "why `env -u BASH_ENV`" rationale currently lives only at `:51-54`, four call sites away from three of its users. The helper gives it one home.

Risk: **low.** The script under change is itself the regression test for `redaction-check.sh`, and `mise run check` already runs it once per gate -- so the QA lane verifies this at zero marginal cost. Only failure mode would be defining the helper above `:36`; the placement above is deliberate.

## 2. OPTIONAL -- trim the new comment block in `redaction-check.sh` from three lines to two

`scripts/redaction-check.sh:33-35`

Before:

```
# The resolver lives in a sibling private repo; point REDACTION_TERMS_RESOLVER
# at it (for Infinite Room Labs operators, in a gitignored mise.local.toml
# [env] block). This file is public, so the path is never hardcoded here.
```

After:

```
# Point REDACTION_TERMS_RESOLVER at the private term-list resolver (operators:
# a gitignored mise.local.toml [env] block). This file is public: no hardcoding.
```

The header comment at `:8-10` already tells the reader the resolver "lives in a sibling private repo" and that its absence is a clean exit-0, so the first clause is a restatement. The "this file is public, so the path is never hardcoded" sentence is the one line worth keeping -- it is the invariant that stops a future edit from regressing, not a description of the code.

Behaviour-preserving: comment only. Risk: **none.** Honest assessment: three lines is not actually too long for a comment that encodes a security invariant. Take it or leave it.

## 3. OPTIONAL -- the header at `:8-10` still says "isn't present", but the notice now says "not configured"

`scripts/redaction-check.sh:8-10` vs `:39`

The header describes one case ("if the resolver script isn't present ... this exits 0 with a notice") while the guard at `:38` now covers two (unset, or set-but-missing) and the notice text changed accordingly to `term list not configured`. One word closes the gap:

```
-# so `mise install` first), and if the resolver script isn't present (it
-# lives in a sibling private repo) this exits 0 with a notice instead of
-# failing.
+# so `mise install` first), and if the resolver script isn't configured (it
+# lives in a sibling private repo, located via REDACTION_TERMS_RESOLVER) this
+# exits 0 with a notice instead of failing.
```

Behaviour-preserving: comment only. Risk: **none.** This is docs-accuracy rather than simplification, but it is a line the diff arguably should have touched and did not.

## 4. OPTIONAL -- the `-z` guards are redundant against `-f`

`scripts/redaction-check.sh:38` and `scripts/redaction-selftest.sh:297`

`test -f ""` is false, so `[ ! -f "$resolver" ]` alone is exactly equivalent to `[ -z "$resolver" ] || [ ! -f "$resolver" ]`, and `[ -f "$resolver" ]` alone is exactly equivalent to `[ -n "$resolver" ] && [ -f "$resolver" ]`. Both `resolver` variables are `:-`-defaulted, so `set -u` is not in play.

```
-if [ -z "$resolver" ] || [ ! -f "$resolver" ]; then
+if [ ! -f "$resolver" ]; then
...
-if [ -n "$resolver" ] && [ -f "$resolver" ] && command -v uv >/dev/null 2>&1; then
+if [ -f "$resolver" ] && command -v uv >/dev/null 2>&1; then
```

Behaviour-preserving: identical truth table for every input including the empty string. Risk: **low.**

My honest lean is **do not bother**. The explicit `-z` names the "operator never set it" case, which is the whole point of this commit and is exactly what the new notice text ("not configured") says. Deleting it trades a readable statement of intent for two saved tokens. Listed because it is a genuine redundancy and the lead should not have to re-derive that it was considered.

## 5. OPTIONAL -- say why `HOME=` is still overridden, now that the resolver is not found under it

`scripts/redaction-selftest.sh:26-30`

The rewritten comment explains the resolver hand-off but leaves the `HOME="$stub_home"` override unexplained at the point of definition. A reader arriving after this commit reasonably asks "the resolver comes from an env var now -- why is there still a fake HOME?" The answer is real and lives four lines further down at `:44-49` (it breaks mise's shim resolution, which is why `stub_usage` is symlinked onto the stub PATH), plus a second reason that is documented nowhere (it insulates the throwaway repos' `git` invocations *inside* `redaction-check.sh` from the operator's global gitconfig). Half a sentence:

```
 # Stub environment: a stub resolver handed to redaction-check.sh through
 # REDACTION_TERMS_RESOLVER, and a `uv` shim that prints the stub term
-# list instead of running it. Both nonsense strings, one below the 8-char
+# list instead of running it. The fake HOME stays -- see the `usage` note
+# below, and it keeps the operator's global gitconfig out of the throwaway
+# repos. Both nonsense strings, one below the 8-char
 # short-term threshold (word-boundary path) and one above it (fixed-string
 # substring path).
```

Behaviour-preserving: comment only. Risk: **none.**

A structural variant -- move the stub resolver out of `$stub_home` to `$scratch/resolver/...`, so `stub_home` becomes purely the HOME-insulation knob -- is also behaviour-preserving (the path is passed by value), but it churns three more lines for a marginal conceptual gain. Not recommended.

---

## Deliberately left alone

1. **`HOME="$stub_home"` itself -- DO-NOT-APPLY.** It looks vestigial now that the resolver is not found under `$HOME`, and it is not. Two roles survive: (a) it breaks mise shim resolution, which is precisely why the `stub_usage` symlink at `:48-49` exists -- removing HOME would silently re-route the stub environment through the real mise shims, changing what the probes exercise; (b) the `git diff` / `git show` / `git rev-list` calls made *inside* `redaction-check.sh` run under the stub HOME and therefore ignore the operator's `~/.gitconfig`, so a global setting could otherwise change what the script sees per-machine. Note that the `git -C "$dir"` calls in `new_repo()` already run outside the stub env, so this insulation is asymmetric -- but tightening or loosening that is a behaviour change, not a simplification.
2. **The `stub_usage` symlink (`:44-49`) -- DO-NOT-APPLY.** Cascades from item 1; only removable if HOME goes, and HOME should not.
3. **`probe()`'s sixth `stub|real` parameter -- DO-NOT-APPLY.** It still earns its keep: the two `real` probes at `:311-323` must run *without* the prefix so they see the ambient resolver and the real `uv`. Collapsing the branch at `:116-120` would delete the only probes that exercise the private term list.
4. **The digit sweep, UUID exemption, baseline cache, and the F21 sequence (`redaction-check.sh:56-244`, `redaction-selftest.sh:147-292`).** Untouched by this diff and carrying dense Phase 7/8 gate provenance in their comments. Out of scope for a hygiene fix, and the comments are the "API evidence" kind the work order says to keep, not the restating kind.
5. **The CHANGELOG entry's length.** It is long, and so is every one of its neighbours -- this file's house style is one dense paragraph per change, and the entry is accurate about what changed and where the operator config now lives. Shortening it would make it inconsistent, not simpler.
6. **The README line.** One sentence, states the new contract exactly, no cut available.
7. **Not my lane, but flagging so it is not lost:** the CHANGELOG says "the public file no longer carries a home-directory layout", which is true of `scripts/redaction-check.sh`. Two tracked docs still spell the old operator path out in full -- `docs/phases/0/plan.md:73` and `docs/phases/8/reports/qa.md:229`. They are historical phase records and the path is a generic `~/projects/<public-org>/...` layout rather than a secret, so this may well be a deliberate no-op; but if the intent of the commit was "this repo does not describe the operator's home directory", the job is not finished. Routing to the security / code-review lanes rather than proposing an edit here.

## Summary

| # | Tag | File | Effort |
|---|---|---|---|
| 1 | APPLY-RECOMMENDED | `scripts/redaction-selftest.sh` (5 sites + 1 helper) | ~10 lines net -5 |
| 2 | OPTIONAL | `scripts/redaction-check.sh:33-35` | comment |
| 3 | OPTIONAL | `scripts/redaction-check.sh:8-10` | comment |
| 4 | OPTIONAL (lean skip) | `redaction-check.sh:38`, `redaction-selftest.sh:297` | 2 lines |
| 5 | OPTIONAL | `scripts/redaction-selftest.sh:26-30` | comment |

Only #1 changes code. If the lead wants the PR to stay a pure one-concern hygiene fix, deferring #1 to a follow-up is defensible -- but it costs nothing extra to verify here, because the selftest is the gate step that proves it.
