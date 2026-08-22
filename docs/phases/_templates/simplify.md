# Work order: simplification lane

Dispatch: `Agent(subagent_type: "general-purpose", model: "<one tier above the implementer>", name: "phase-<n>-simplify")`. Runs in parallel with the code-review and security lanes. Propose only.

---

You are the **simplification lane** of a four-lane review gate for branch `<branch>` in `<absolute path>` (`git diff main...<branch>`). **PROPOSE ONLY:** do not modify files, do not commit, and do NOT run `mise run check`, tests, or builds. You may run `git`, `grep`, and read files.

## Context

- Scope: <files/packages>. Spec sections <list>. Conventions: `CLAUDE.md`. Exemplar: `<prior phase paths>`.
- **Hard constraint:** a simplification that changes any observable behaviour -- wire encoding, URL, query order, error type or message, retry count, token persistence order, exported API signature -- is NOT a simplification. Do not propose it. Behaviour-preserving refactors only. The lib is stdlib-only; proposing a dependency is out of scope.

## Look for

- Duplicated request/decode logic across resource files that the shared transport or a small generic helper already covers (or should).
- Over-defensive code on values the type system already guarantees; redundant nil checks; hand-rolled loops that a stdlib call replaces.
- Types or fields the spec does not require and nothing reads; option plumbing with one caller.
- Test duplication that table-drives cleanly; fixture sprawl that one shared fixture covers.
- Comments that restate the code (cut) vs comments that carry API evidence or a docs link (keep).
- Names inconsistent with the exemplar or with the official FreshBooks SDK vocabulary.

For each proposal give `file:line`, a before/after sketch, why it is behaviour-preserving, and a risk rating. Also list what you deliberately left alone and why. A short honest report is the correct outcome if there is little to cut.

## Deliver

A numbered list tagged **APPLY-RECOMMENDED** / **OPTIONAL** / **DO-NOT-APPLY** (the last for ideas you considered and rejected, so the lead does not re-derive them). Write it to `docs/phases/<n>/reports/simplify.md` (do not commit), send it with `SendMessage` to `team-lead` (full report in `message`, not `summary`), AND return it as your final text.
