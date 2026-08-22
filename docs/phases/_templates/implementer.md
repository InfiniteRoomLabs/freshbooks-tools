# Work order: implementer

Dispatch: `Agent(subagent_type: "general-purpose", model: "<sonnet|opus per GOAL.md>", name: "phase-<n>-impl")`. Fill every `<...>`. Pointers, not pasted text.

---

You are implementing **Phase <n> (<name>)** of `freshbooks-tools`, a public MIT Go monorepo. Work ONLY inside `<absolute repo or worktree path>` on branch `<branch>` (already created, clean). Do not touch other branches or worktrees.

## Read first (pointers, not pasted)

1. The oracle: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` sections <list>. Section 2 is locked; do not redesign.
2. Conventions: `CLAUDE.md` (toolchain, commits, green rule, parity contract, public-repo hygiene), `GOAL.md` stage 2 for this phase (the deliverable list and acceptance criteria).
3. **FreshBooks docs before coding:** https://www.freshbooks.com/api/start, then the page for every resource in this phase (`https://www.freshbooks.com/api/<resource>`, e.g. `/api/invoices`, `/api/clients`). They carry response examples the Postman collection lacks. Also `https://www.freshbooks.com/api/authentication` if you touch auth.
4. Inventory entries for this phase: `freshbooks/internal/inventory/testdata/inventory.json`, folders <list>. Every request you implement gets a `// inventory: <Folder>/<Request name>` comment on its method.
5. Exemplar to mirror in shape and style: `<path to the most recently shipped phase's code + tests>`.

## Deliverables

- <file or package> -- <exactly what it must contain, rule by rule>
- Tests: unit (table-driven, `httptest` fixtures under `testdata/`), integration (`//go:build integration`) for every cross-package seam this phase introduces: <list>. Coverage >= 90% for the module. `-race` clean.
- Doc comments on every exported identifier; package `doc.go` updated; runnable `Example*` functions where the spec shows usage.
- `<module>/CHANGELOG.md` `[Unreleased]` entry in the existing style.
- `mise run check` green on a clean tree. Always run tooling via `mise run ...`.
- Commit on `<branch>` in TDD-sized conventional commits (`feat(<scope>): ...`), each green. **Stage and commit in separate Bash calls.** Do NOT push, do NOT merge, do NOT create the GitHub repo -- the lead runs the gate and ships.

## Gotchas (these cost prior runs time)

- Two ID families (`AccountID` string vs `BusinessID` int64); accounting responses are enveloped, business-scoped ones are flat; refresh tokens are one-time-use. See `CLAUDE.md` Gotchas.
- Fixture IDs and names are synthetic. Never paste real account IDs, tokens, or names from docs or Postman into fixtures or tests.
- Docs are ASCII-only, no hard wraps.
- If the spec is wrong about something the API does, implement what the API does, add a `> **STATE AS OF <date>**` callout in the affected spec section in the same commit, and list the discrepancy in your report.
- <phase-specific gotchas>

## Reporting (both channels)

When done (gate green, committed, `git status --porcelain` empty): write the report to `docs/phases/<n>/reports/implementer.md` (commit it), send the same report with `SendMessage` to `team-lead` (report in `message`, not `summary`), AND return it as your final text. Report: files created/changed, test counts per package, coverage per module, the exact `mise run check` tail, `git log --oneline main..<branch>`, `git status --porcelain` output, inventory entries covered, and every spec discrepancy or ambiguity you hit and how you resolved it. If you are genuinely blocked, report the blocker the same way instead of guessing.
