# Progress

Living status doc. Read first, update at every phase boundary. Last updated: 2026-08-22.

## Current state

- Repo initialized on `main` with the design spec (`docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md`), MIT license, root `CHANGELOG.md`, `.gitignore`, the FreshBooks Postman collection (`docs/freshbooks.postman_collection.json`, pulled via `https://documenter.gw.postman.com/api/collections/3322108/S1ERwwza?segregateAuth=true&versionTag=latest`), `GOAL.md`, `CLAUDE.md`, and the five work-order templates under `docs/phases/_templates/`.
- No Go code yet. No GitHub repo yet (Phase 0 creates it).
- Spec status: sections 2-9 approved by Wes in chat on 2026-08-22. API facts in section 3 are a mix of CONFIRMED (auth flow, token lifetimes, scopes, base URL, inventory counts) and INFERRED (envelope/error shapes per family, date formats, which OAuth endpoint set accepts our app).

## Phase ledger

| Phase | Status | Branch / merge | Notes |
|---|---|---|---|
| 0 Scaffold | not started | `phase-0/scaffold` | see `GOAL.md` |
| 1 Lib core | not started | | |
| 2 Lib resources (a-d) | not started | | |
| 3 MCP | not started | | |
| 4 CLI | not started | | |
| 5 Release | not started | | |

## Discoveries

- 2026-08-22: FreshBooks has an unannounced first-party MCP (`mcp.freshbooks.com` resolves, `mcp:*` scopes in the OAuth metadata). Not a dependency; noted in spec Future Work.
- 2026-08-22: `auth.freshbooks.com/.well-known/oauth-authorization-server` advertises PKCE S256 and a second set of OAuth endpoints; Phase 1 stage 1 resolves which to use.

## Next action

Run `/goal complete everything in @GOAL.md` in a fresh session. It targets Phase 0 (scaffold).

## How to resume in a fresh session

1. Read this file, then `GOAL.md`, then `CLAUDE.md`.
2. `git status --porcelain` must be empty and `git log --oneline -5` should match the ledger above. If not, reconcile before starting.
3. Read only the spec sections the current phase names.
4. Start the goal.
