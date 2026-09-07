# Code review lane -- PR #6, `fix/redaction-resolver-env`

Branch: `fix/redaction-resolver-env` (one commit, `d6e038d`). Files: `scripts/redaction-check.sh`, `scripts/redaction-selftest.sh`, `README.md`, `CHANGELOG.md`. Read-only review; no gate, tests, or selftest run (QA lane owns those).

## Verdict

**APPROVE** -- six advisory findings, none blocking.

The mechanism is correct. `resolver="${REDACTION_TERMS_RESOLVER:-}"` is `set -u`-safe; the `[ -z "$resolver" ] || [ ! -f "$resolver" ]` guard covers both unset and dangling values; every stub invocation in the selftest now passes the variable explicitly, which overrides an ambient value the same way the old `HOME=` override did; and the "real resolver" probe path (`env_kind = real`, which deliberately runs with no `env` prefix) inherits the same variable the selftest's own guard at `scripts/redaction-selftest.sh:297` tested, so guard and probe cannot disagree. The stub preflight at `scripts/redaction-selftest.sh:138-145` still requires `redaction-check: clean` rather than the no-op notice, so it continues to prove the stub resolver was actually found and the term list actually loaded -- the non-vacuity property the Phase 8 lesson demanded survives the change intact. `shellcheck -S warning` reports only the pre-existing SC1008 on the `usage` shebang for both scripts. `git ls-files` confirms `mise.local.toml` is untracked and `.gitignore:19` ignores it. No occurrence of the old operator path remains in any file this PR is responsible for; the only surviving mentions are in `docs/phases/0/plan.md:73` and `docs/phases/8/reports/qa.md:229`, which are frozen phase artifacts and should not be rewritten.

## Findings

### 1. ADVISORY -- `scripts/redaction-selftest.sh:8-12`: the file header still describes the old delivery mechanism

The header says the term-scan probes run "against a stub resolver (a fake `$HOME` plus a `uv` shim on PATH)". The fake `$HOME` is no longer how the stub resolver is found -- it is handed over through `REDACTION_TERMS_RESOLVER`. The diff correctly updated the inline comment at `:26-30` but left the header, so the two now contradict each other, and the header is the one a reader hits first.

**Fix:** reword to "a stub resolver passed through `REDACTION_TERMS_RESOLVER` plus a `uv` shim on PATH". Keep the `$HOME` mention only where it is still true (`:44-49`, the mise-shim note).

### 2. ADVISORY -- `scripts/redaction-check.sh:8-10`: the header describes the opt-out condition without naming its control

The header says "if the resolver script isn't present (it lives in a sibling private repo) this exits 0 with a notice instead of failing". After this change the condition is "if `REDACTION_TERMS_RESOLVER` is unset, or names a file that does not exist". A reader of the header alone cannot tell how to arm the check; the answer is 25 lines further down at `:33-36`.

**Fix:** amend the header clause to "if `REDACTION_TERMS_RESOLVER` is unset or does not name an existing file (see below) this exits 0 with a notice instead of failing".

### 3. ADVISORY -- `README.md:66`: "needs neither" now reads as a false claim about `usage`

The sentence is: "needs `usage` on your PATH ... and is otherwise optional for outside contributors: it no-ops unless `REDACTION_TERMS_RESOLVER` points at a term-list resolver. Its own regression test, `scripts/redaction-selftest.sh`, runs in the gate and needs neither."

The two nearest antecedents for "neither" are now `usage` and `REDACTION_TERMS_RESOLVER`, and the selftest definitively needs `usage`: it is its own interpreter (`#!/usr/bin/env -S usage bash`, `scripts/redaction-selftest.sh:1`) and it additionally resolves the binary at `:48` (`mise which usage || command -v usage`), exiting non-zero if it cannot. The `CHANGELOG.md:46` version of the same claim is unambiguous because it names its two items ("needs neither the private term list nor network"); the README's is not, and this diff edited the clause immediately preceding it, which made the ambiguity worse.

**Fix:** make the items explicit, e.g. "... runs in the gate and needs neither the resolver nor network access (it does still need `usage`)."

### 4. ADVISORY -- the check is now only armed under mise, and nothing in the repo invokes it that way

`mise.toml` has a `[tasks.redaction-selftest]` but no `[tasks.redaction-check]`, and `CLAUDE.md:30` instructs "Run `scripts/redaction-check.sh` before committing" -- a bare invocation. `mise.local.toml [env]` reaches a bare invocation only when mise is hook-activated in the calling shell. It does not reach one under `mise activate --shims` (a static PATH prepend, which loads no `[env]`), a git hook launched by an editor or GUI, or a fresh shell without activation. In each of those the check now prints "term list not configured" and exits 0 where it was previously armed unconditionally by the hardcoded absolute path.

This is the intended trade for getting the operator path out of a public file, and it fails loudly enough to be noticed in an interactive run, so it is not blocking. But the repo currently offers no invocation that is guaranteed to carry the variable, and the one it documents is the fragile one.

**Fix:** add to `mise.toml`

```toml
[tasks.redaction-check]
description = "Grep staged content (or a --range diff) against the private redaction term list"
run = "scripts/redaction-check.sh"
```

and change `CLAUDE.md:30` and the `README.md:66` guidance to `mise run redaction-check` (`mise run redaction-check -- --range main..HEAD` for the gate lanes), so the `[env]` block is applied by construction rather than by shell state.

### 5. ADVISORY -- `scripts/redaction-check.sh:36-43`: a relative `REDACTION_TERMS_RESOLVER` resolves against the caller's CWD

`[ -f "$resolver" ]` and the later `cd "$(dirname "$resolver")"` both interpret a relative value against wherever the script was invoked from. The old hardcoded value was absolute by construction, so this failure mode is new. It is not hypothetical inside this PR's own test: the selftest evaluates the value at `scripts/redaction-selftest.sh:297` from the repo root, while `probe()` runs `cd "$dir" && ./redaction-check.sh` from a scratch repo (`:119`). A relative value therefore passes the selftest's guard, arms the two "real term" probes, and then fails them with `exit 0, want 1` -- a misleading failure that points at the probe rather than at the misconfiguration.

**Fix:** reject non-absolute values explicitly, next to the existing guard:

```sh
case "$resolver" in
  "" | /*) ;;
  *) echo "redaction-check: REDACTION_TERMS_RESOLVER must be an absolute path" >&2; exit 2 ;;
esac
```

Exit 2 matches the existing "misconfigured, not clean" status already used for an unusable `--range` at `:28-31`.

### 6. ADVISORY (nit) -- `scripts/redaction-selftest.sh:33-37`: `stub_home` keeps a resolver-shaped subtree it no longer needs

`HOME="$stub_home"` is still load-bearing -- it sandboxes the probes from the operator's real home and is what the mise-shim note at `:44-49` is about -- so it should stay. But the stub resolver no longer has any reason to live *inside* it; `$stub_home/resolver/resolve-redaction-terms.py` is a vestige of the layout the old code required, and reads as though the path still mattered.

**Fix:** move it out of the fake home -- `stub_resolver="$scratch/resolver/resolve-redaction-terms.py"` with a matching `mkdir -p "$scratch/resolver"` -- leaving `mkdir -p "$stub_home" "$stub_bin"` for the sandbox alone. Cosmetic; the behaviour is identical either way.

## Considered and dismissed

- **No probe covers the new "unset / missing resolver" branch.** Real, but low value: the stub preflight at `:138-145` asserts `redaction-check: clean`, which already fails if the configured path stops being found, so the branch that matters (configured -> terms actually loaded) is covered from the other side. A probe asserting `REDACTION_TERMS_RESOLVER= -> exit 0 + "term list not configured"` would be cheap if the fix pass is touching the file anyway, but it is not worth a round trip on its own.
- **`[ -n "$resolver" ]` at `:297` is redundant** with the `[ -f "$resolver" ]` that follows (`[ -f "" ]` is false). Explicit and harmless; leaving it to the simplification lane.
- **`env_kind = real` probes do not pass `env -u BASH_ENV`** while the stub ones do. Pre-existing and correct: the real probes are supposed to run in the operator's genuine environment, which is exactly where `BASH_ENV`/mise shims belong.
- **A worktree outside the main checkout would not see `mise.local.toml`.** True, but it is a property of where the operator puts worktrees, not of this diff, and `CLAUDE.md` already fixes worktrees under `.worktrees/`.
- **Stale references to the old hardcoded path in `docs/phases/0/plan.md` and `docs/phases/8/reports/qa.md`.** Frozen phase artifacts describing what was true at the time. Correctly left alone.
