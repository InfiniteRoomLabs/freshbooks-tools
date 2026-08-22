# Work order: code-review lane

Dispatch: `Agent(subagent_type: "general-purpose", model: "<one tier above the implementer>", name: "phase-<n>-review")`. Runs in parallel with the simplification and security lanes. Read-only.

---

You are the **code-review lane** of a four-lane review gate for branch `<branch>` in `<absolute path>` (`git diff main...<branch>`, commits `<shas>`). **READ-ONLY:** do not modify files, do not commit, and do NOT run `mise run check`, tests, or builds -- the QA lane owns the gate and concurrent runs collide on build and coverage outputs. You may run `git`, `grep`, and read files freely.

## Context

- Spec: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections <list>. Conventions: `CLAUDE.md`. Exemplar conventions: `<prior phase paths>`.
- FreshBooks docs for the resources in this phase: https://www.freshbooks.com/api/start and `/api/<resource>` pages -- compare the port's request/response structs against the documented examples.
- Inventory entries in scope: <folders>.

## Review for

- Correctness against the spec and the FreshBooks docs: wrong ID family, wrong URL template, envelope handling, pagination fields, query encoding of `search[...]`/`include[]`, error decoding per family, retry semantics, token rotation write-back ordering, context cancellation.
- Go canon: exported API shape (ctx first, options, typed IDs, error wrapping with `%w`, sentinels via `errors.Is`), no package-level mutable state, no goroutine leaks, `io.Closer` handling, zero-value usability.
- Doc comments present and accurate on every exported identifier; `Example*` functions compile and say what the spec says.
- Test smells: vacuous asserts, fixtures that mirror the implementation instead of the docs, missing sad/edge paths, `t.Skip`, non-deterministic time/random, tests that would still pass if the behaviour regressed.
- Convention drift from the exemplar (naming, file layout, changelog entry, `// inventory:` comments).

Confidence-filter: report only issues you are confident matter, each with `file:line`, why, and the concrete fix.

## Deliver

Verdict **APPROVE** or **REQUEST CHANGES**, findings numbered and tagged **BLOCKING** / **ADVISORY**. Write the report to `docs/phases/<n>/reports/code-review.md` (do not commit it; the lead will), send it with `SendMessage` to `team-lead` (full report in `message`, not `summary`), AND return it as your final text.
