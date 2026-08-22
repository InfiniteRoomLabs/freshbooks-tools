# GOAL.md -- autonomous next-phase goal

Run with `/goal complete everything in @GOAL.md`. Each run ships exactly one phase of `freshbooks-tools` through the pipeline: verify premise -> implement -> four-lane review gate -> merge -> ship -> retarget this file. The design oracle is `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` (the "spec"); `CLAUDE.md` holds the conventions and dispatch rules; `docs/progress.md` is where we are.

**Shipped so far:** nothing -- the repo holds only the spec, license, changelog, and Postman inventory. Next default target: **Phase 0 (scaffold)**, then Phase 1 (lib core). Full roadmap in *Retarget* at the bottom.

---

## Goal (current target: Phase 0 -- scaffold)

> **Goal -- don't stop until Phase 0 (scaffold) ships:**
>
> 1. **Verify the premise before building.** Read the spec sections 2, 4, 5.2, 8, 9 and `CLAUDE.md`. Confirm on this machine: `go version` is >= 1.26, `mise` is installed, `gh auth status` is logged in with access to the `InfiniteRoomLabs` org, and `GOPROXY` can resolve `github.com/modelcontextprotocol/go-sdk@v1.7.0`, `github.com/spf13/cobra`, `github.com/stretchr/testify`, `golangci-lint`, `goreleaser` (try `mise use` for the last two; pin exact versions, never `latest`). Re-read `docs/freshbooks.postman_collection.json`'s top-level folders and confirm the 14-folder / ~130-request inventory the spec assumes. Anything wrong: fix the spec with a `> **STATE AS OF YYYY-MM-DD**` callout in the affected section, note it in `docs/progress.md`, and continue.
> 2. **Implement** on branch `phase-0/scaffold` by dispatching ONE implementer (`general-purpose`, `model: "sonnet"`, work order from `docs/phases/_templates/implementer.md`, pointers to spec sections 4, 5.2, 8, 10). Deliverables, all in one branch:
>    - `go.work` + three modules: `freshbooks/` (`doc.go` with the package overview + a `Version` const), `mcp/` (`cmd/freshbooks-mcp/main.go` printing version, `internal/` placeholder packages), `cli/` (`cmd/freshbooks/main.go` cobra root with `version` + `completion`), each with `go.mod` (module paths per spec section 2), `CHANGELOG.md` (Keep a Changelog, `[Unreleased]`), and at least one test so coverage is measurable.
>    - **Inventory tool** `freshbooks/internal/inventory`: parses the Postman collection, normalizes paths exactly as spec 5.2 says (lower-camel variables, synthetic IDs, `my.freshbooks.com/service/api` -> public path, whitespace), emits `inventory.json` (committed under `freshbooks/internal/inventory/testdata/` together with the collection moved there from `docs/`), and implements `-check <pkg...>` that scans Go source for `// inventory: <folder>/<name>` comments and fails on uncovered or double-covered entries. Allow a `//go:inventory-ignore <folder>/<name> <reason>` list file for requests we deliberately do not implement (hard-coded-ID duplicates, `my.freshbooks.com` internal endpoints). Unit tests with fixtures; this is the only "real" code in Phase 0 and must itself clear the 90% floor.
>    - `mise.toml`: tool pins (`go`, `golangci-lint`, `goreleaser`) and tasks `fmt-check`, `vet`, `lint`, `test` (`-race -coverprofile`), `cover` (runs `scripts/coverage-gate.sh 90`), `inventory-check`, `build` (cross matrix `{linux,darwin,windows} x {amd64,arm64}`), `check` (all of the above, per module, then prints a dirty-tree banner if `git status --porcelain` is non-empty), `docs` (regenerates `docs/cli.md` via cobra/doc once the CLI exists -- a stub now).
>    - `.golangci.yml` (errcheck, govet, staticcheck, revive with the exported-doc rule, gosec, misspell; warnings are errors), `scripts/coverage-gate.sh`, `scripts/changelog-section.sh`, `scripts/branch-protection.sh` (uses `gh api`; required checks `lib`, `mcp`, `cli`; PRs only; linear history), `scripts/redaction-check.sh` (greps staged content against the list produced by `~/projects/infinite-room-labs/agent-ops/scripts/resolve-redaction-terms.py`; documented as optional for outside contributors).
>    - `.github/workflows/ci.yml` (triggers `pull_request`, `push` to `main`, `workflow_call`; jobs `lib` -> `mcp`, `cli` with `needs: lib`; each runs `mise run check` for its module via `jdx/mise-action`), `.github/workflows/release.yml` (tag push `freshbooks/v*`, `mcp/v*`, `cli/v*`; guards: strict semver, tag is ancestor of `origin/main`, module changelog has the `## [X.Y.Z]` section; calls `ci.yml`; goreleaser for `mcp`/`cli` with `.goreleaser.yaml` per module; GitHub release with the changelog section as body; lib gets release + notes only), `.github/dependabot.yml` (gomod + actions, weekly).
>    - `README.md` (what/why, module table, install placeholders, **contributor agent setup: agent-ops marketplace for the changelog guard**, links to docs + pkg.go.dev), and stub docs with real headings: `docs/getting-started.md`, `building.md`, `authentication.md`, `library.md`, `mcp.md`, `cli.md`, `agentic-transformation.md` (this one written for real: how the collection was pulled -- the `documenter.gw.postman.com/api/collections/<id>/<slug>` trick -- the inventory tool, the work orders, the gate).
>    - Acceptance: `mise run check` green in all three modules on a clean tree; `go run ./internal/inventory -check ./...` passes with every entry in the ignore list or marked TODO-by-phase; CI workflow files pass `actionlint` if available.
> 3. **Four-lane gate** -- dispatch code-review, simplification, and security lanes in parallel (read-only), then the QA lane (the only lane that runs `mise run check`); all `general-purpose`, **`model: "opus"`** (one tier above the sonnet implementer). Work orders from `docs/phases/_templates/`. Reports to `docs/phases/0/reports/<lane>.md` AND via `SendMessage` to `team-lead`. Triage, ONE fix commit `fix(scaffold): apply the review-gate findings`, re-run `mise run check`, confirm `git status --porcelain` is empty.
> 4. **Ship** -- merge `phase-0/scaffold` into `main` with `--no-ff` (body summarises gate results); update root `CHANGELOG.md` + `docs/progress.md`; **create the public GitHub repo `InfiniteRoomLabs/freshbooks-tools`** (`gh repo create --public --source . --push`; this goal explicitly authorizes it), run `scripts/branch-protection.sh`, add the Gitea mirror remote per `CLAUDE.md`, push `main`. Confirm the `ci.yml` run on `main` is green (`gh run watch`). Then **retarget THIS file to Phase 1** (see *Self-advance*) and commit it with the docs.
>
> **Done when:** the scaffold is merged and pushed, CI is green on GitHub, `mise run check` is green locally on a clean tree, the inventory check passes, `docs/progress.md` + changelogs are updated, **and `GOAL.md` is retargeted to Phase 1.** **Stop only** for a genuine decision you + an advisor skill (`superpowers:brainstorming`, `make-decision`, `deep-think`) cannot confidently make, or for a permission the harness will not grant.

### Self-advance (do this as the last Ship step, every run)

Leave `GOAL.md` pointed at the **next** phase so the following `/goal` run is paste-ready:
- Update the **"Shipped so far"** line and strike the shipped row in the **Retarget table** (mark it `SHIPPED`, add the merge sha and one-line lesson).
- Rewrite the **`## Goal` heading + block** for the next phase: stage 1 is always "verify the premise" against the spec sections that phase depends on; stage 2 names the branch, the implementer model (from the Retarget table), the work-order template, and the concrete deliverables with acceptance criteria; stage 3 is the gate with the reviewer model one tier above the implementer; stage 4 is ship + retarget.
- Append anything you learned to **Lessons** (process traps only, not feature notes -- those go in `docs/progress.md` and the spec callouts).
- **Next default after Phase 0: Phase 1 (lib core)** -- implementer `opus`, reviewers `fable`; branch `phase-1/lib-core`; deliverables = spec section 5.1 minus the resource services: `client.go`, `options.go`, `transport.go` (envelope unwrap, family-specific error decoding, retry/backoff, `Retry-After`), `errors.go`, `page.go` (`Page[T]` + `iter.Seq2`), `types.go` (`AccountID`, `BusinessID`, `BusinessUUID`, `Money`, `Date`, `DateTime`, enums), `auth/` (PKCE auth-code URL, exchange, refresh with rotation write-back, revoke, `TokenSource`, `TokenStore`, `FileStore` 0600 atomic, `MemoryStore`, single-flight refresh), `identity.go` (`Identity.Me`), **all service struct types pre-declared as empty fields on `Client`** so Phase 2 batches only add files, `testdata/` fixture loader, `WithClock`. Stage 1 of Phase 1 must confirm live (with a throwaway dev app, read-only) which OAuth endpoints work -- documented `api.freshbooks.com/auth/oauth/*` vs metadata `auth.freshbooks.com/service/auth/oauth/*` -- and whether PKCE is accepted; record the answer in spec section 3 with a callout. Then Phase 2 (resources, 4 parallel sonnet batches in worktrees).

---

## Lessons from prior runs (read before starting)

Seeded from `hoyle-re` (the reference implementation of this loop). Append, never rewrite.

- **Verify the premise before implementing.** Two of three hoyle-re autonomous runs found the goal's own premise wrong. Here the premise is the spec's reading of the FreshBooks API (section 3) and the Postman inventory; the API's actual behaviour wins over the spec, and the spec gets a `STATE AS OF` callout in the same run.
- **Model tiers are pinned in agent definitions, not inherited.** Omitting `model:` on an `Agent` call uses whatever the agent file pins (several agent-ops agents pin haiku). Pass `model:` explicitly on every dispatch, and keep reviewers one tier above the implementer (sonnet -> opus, opus -> fable). Never use haiku for anything that could hit a permission prompt -- it has no auto mode and stalls the run.
- **Check the agent's `tools:` list before writing its prompt.** Most agent-ops review agents lack `Bash` or `SendMessage` and will appear to end silently. `general-purpose` (tools `*`) works for every lane.
- **Tell agents HOW to deliver.** A named background agent's final text does not reach the lead by itself. Instruct it to call `SendMessage` to **`team-lead`** (not `main`) with the full report in `message` (not `summary`), AND to write the same report to `docs/phases/<n>/reports/<lane>.md`.
- **Only the QA lane runs the gate.** Concurrent `mise run check` runs in one tree collide on build and coverage outputs. Code-review, simplification, and security lanes are read-only (`git`, `grep`, read).
- **A green HEAD says nothing about a dirty working tree.** Verify `git status --porcelain` is empty before reporting a gate result or claiming a fix landed.
- **The changelog guard is a Claude hook, not a git hook.** Commits on `main` are blocked unless `CHANGELOG.md` is staged. Write the changelog entry with the commit, not after. The version guard also requires `git add` and `git commit` as **separate Bash calls** (no `-a`/`-am`, no `&&` chaining).
- **Bash `cd` persists across tool calls.** Use absolute paths or `git -C <dir>` for worktree and module commands; never rely on the cwd a previous call left behind.
- **Public repo from commit 1.** Run `scripts/redaction-check.sh` before every commit once it exists; until then grep staged content for vault item names, internal IPs and hostnames, real account/business IDs, tokens, and personal names. Fixture IDs are synthetic.

## Reference

- **Spec:** `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md`. Section 2 is locked; sections 3-8 are the build oracle; section 9 is this process.
- **Conventions + dispatch rules:** `CLAUDE.md`. **Status:** `docs/progress.md`.
- **Work orders:** `docs/phases/_templates/{implementer,code-review,simplify,security,qa}.md`. Phase artifacts: `docs/phases/<n>/reports/`.
- **Inventory:** `docs/freshbooks.postman_collection.json` (Phase 0 moves it under `freshbooks/internal/inventory/testdata/`); 14 folders: Authorization 4, Clients 8, Invoices 21, Expenses 21, Estimates 8, Time Tracking 7, Projects 15, My Team 7, Reports 15, Accounting 4, Uploader 3, Webhooks 5, Settings 6, Tokenization 6.
- **FreshBooks docs:** https://www.freshbooks.com/api/start (overview), `/api/authentication`, `/api/scopes`, and one page per resource (`/api/invoices`, `/api/clients`, `/api/expenses`, `/api/projects`, `/api/timetracking`, ...) -- these carry the response examples the Postman collection lacks. Every implementer reads the pages for its phase before coding.
- **Toolchain:** everything through `mise run <task>` / `mise exec --`. `mise run check` is the gate.
- **Exemplars to mirror:** within this repo, the most recently shipped phase. Outside: the official FreshBooks Python/Node SDKs (resource naming), `google/go-github` (service-struct client shape), `kubectl`/`gh` (CLI ergonomics), the go-sdk `examples/` directory (stateless streamable HTTP).

## Retarget / roadmap

One row per phase; strike and mark `SHIPPED` as they land. Effort is relative; "attended" means the lead should have Wes available.

| Phase | Branch | Implementer -> Reviewers | Effort | Notes |
|---|---|---|---|---|
| **0 Scaffold** | `phase-0/scaffold` | sonnet -> opus | low-med | go.work, 3 modules, mise, lint, CI/release, scripts, inventory tool, docs stubs, public repo + branch protection |
| 1 Lib core | `phase-1/lib-core` | opus -> fable | med-high | client/transport/errors/page/types/auth/identity; service fields pre-declared; live OAuth endpoint check (stage 1) |
| 2 Lib resources | `phase-2/{a,b,c,d}` in `.worktrees/` | sonnet x4 -> opus | med (volume) | (a) Clients+Invoices+Estimates, (b) Expenses+Payments+Accounting+Reports, (c) Projects+Time Tracking+My Team+Settings, (d) Webhooks+Uploader+Tokenization+Authorization remainder. One gate per batch, merge sequentially |
| 3 MCP server | `phase-3/mcp` | sonnet -> opus | med | go-sdk v1.7.0 stateless HTTP + stdio; one tool per lib method; parity test; `mcp.md` |
| 4 CLI | `phase-4/cli` | sonnet -> opus | med | cobra tree from registry; loopback PKCE login; contexts; outputs; parity test; `cli.md` via cobra/doc |
| 5 Release hardening | `phase-5/release` | sonnet -> opus | low | docs pass, `getting-started`, goreleaser dry-runs, then tags `freshbooks/v0.1.0`, `mcp/v0.1.0`, `cli/v0.1.0` -- **tag pushes attended** (they publish releases) |

**Batch option:** Phases 3 and 4 are independent once Phase 2 is merged; an attended run may dispatch both implementers in separate worktrees and gate them separately.

**Alternative phase (any time after Phase 1):** live-API conformance pass -- run the `live` build-tag suite against a sandbox account, upgrade every INFERRED fact in spec section 3 to CONFIRMED or correct it. Attended (needs Wes's FreshBooks credentials).
