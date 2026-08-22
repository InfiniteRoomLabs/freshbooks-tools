# freshbooks-tools -- Project Instructions for Claude

Go monorepo for the FreshBooks REST API: `freshbooks/` (client library), `mcp/` (stateless MCP server), `cli/` (cobra CLI). Public, MIT, an Infinite Room Labs portfolio piece that also runs IRL's own books. Design oracle: `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` (the "spec").

## Read-first

- **`docs/progress.md`** -- living status. Always read before starting work; update at every phase boundary.
- **`GOAL.md`** -- the current autonomous phase goal + full roadmap. This repo runs on the `/goal` treadmill: `/goal complete everything in @GOAL.md` ships one phase and retargets the file.
- **The spec** -- section 2 is locked (do not re-litigate), 3 is the API facts, 4-8 the designs, 9 the process. Read only the sections the current phase needs.

## Toolchain

- **Go >= 1.26**, always through `mise`: `mise run check` (the gate), `mise run test|lint|cover|build|inventory-check|docs`. Never bare `golangci-lint`/`goreleaser` (version drift); `go` itself is fine.
- **One module per directory** (`freshbooks/`, `mcp/`, `cli/`), joined by `go.work`. Run `go` commands from inside the module or with `-C <module>`.
- **Dependencies:** lib is stdlib-only (+ testify in tests). `mcp` adds go-sdk + cobra. `cli` adds cobra, pflag, yaml.v3, x/term. Adding anything else is a design decision: flag it.
- **Python helpers (scripts that need it):** `uv run` only. **JS (future Docusaurus):** `pnpm` only.
- **Secrets:** a gitignored `fnox.toml` maps `FRESHBOOKS_CLIENT_ID` / `FRESHBOOKS_CLIENT_SECRET` (the registered dev app) to Bitwarden. Run anything that needs them as `fnox exec -- <cmd>`; never export them in a shell, never write them to a file, never echo them (test resolution by length: `fnox exec -- sh -c 'echo ${#FRESHBOOKS_CLIENT_SECRET}'`). Outside contributors register their own app and set the two env vars however they like.

## Working conventions

- **Phases, not tasks.** One GOAL block = one phase = one branch `phase-<n>/<slug>` = one gate = one `--no-ff` merge. Phase 2 batches run as parallel worktrees under `.worktrees/` (gitignored); everything else is in-place branches.
- **Four-lane review gate before every merge:** code review, simplification, security (parallel, read-only) then QA (the only lane that runs `mise run check`). Templates in `docs/phases/_templates/`. Reports land in `docs/phases/<n>/reports/<lane>.md` and via `SendMessage` to `team-lead`. One fix commit, re-gate, merge.
- **Model rules (non-negotiable):** pass `model:` explicitly on every dispatch; reviewers are one tier above the implementer (sonnet -> opus, opus -> fable); haiku only for read-only sweeps that cannot hit a permission prompt. Check an agent's `tools:` before writing its prompt; `general-purpose` has everything.
- **TDD, green rule:** tests first; no `t.Skip` without an issue link; no committed `-run` filters; `-race` always; lint warnings are errors; coverage floor **90% per module** (`scripts/coverage-gate.sh`). Tag `t.Run` names with `[happy] [sad] [edge] [corner] [parity]` where it helps triage.
- **Parity contract:** every implemented endpoint carries `// inventory: <Folder>/<Request name>` on its lib method; `mise run inventory-check` fails on uncovered or double-covered inventory entries. MCP tools and CLI commands are checked against the same list.
- **Inferred vs confirmed:** API facts are CONFIRMED (observed live or in official docs), INFERRED (from examples), or TODO. When reality disagrees with the spec, the API wins -- add a `> **STATE AS OF YYYY-MM-DD**` callout in the affected spec section in the same commit.
- **Commits:** conventional, imperative, scoped (`feat(freshbooks): ...`, `feat(mcp): ...`, `feat(cli): ...`, `chore(ci): ...`, `docs: ...`). **Stage and commit in separate Bash calls** (agent-ops version guard); never `-a`/`-am`, never `--no-verify`. On `main`, `CHANGELOG.md` must be staged (agent-ops changelog guard -- a Claude hook, not a git hook). Co-author trailer per harness rules.
- **Changelogs:** Keep a Changelog, `[Unreleased]` on top. Root `CHANGELOG.md` = process/CI/docs; each module has its own for its releases. Release tags `freshbooks/vX.Y.Z`, `mcp/vX.Y.Z`, `cli/vX.Y.Z` must have a matching `## [X.Y.Z]` section.
- **Public-repo hygiene on every commit:** no vault item names, internal IPs/domains, real FreshBooks account/business IDs, tokens, or personal correspondents. Fixture IDs are synthetic. Run `scripts/redaction-check.sh` before committing.
- **Docs are ASCII-only and never hard-wrapped.** No smart quotes, em dashes, or arrows; use `--` and `->`.
- **Remotes:** `origin` = `git@github.com:InfiniteRoomLabs/freshbooks-tools.git` (public). A private `gitea` mirror remote may exist in `.git/config`; its hostname is internal and must never appear in tracked files. Check `git remote -v` before pushing a WIP branch.

## Key locations

| Concern | Path |
|---|---|
| Library client, options, transport, errors, pagination, types | `freshbooks/{client,options,transport,errors,page,types}.go` |
| OAuth + token source/store | `freshbooks/auth/` |
| One file per resource service | `freshbooks/<resource>.go` + `_test.go` |
| Inventory tool + normalized inventory + Postman source | `freshbooks/internal/inventory/` (+ `testdata/`) |
| Fixtures for httptest | `freshbooks/testdata/<resource>/*.json` |
| MCP entrypoint / config / server / tool registry | `mcp/cmd/freshbooks-mcp/`, `mcp/internal/{config,server,tools}/` |
| CLI entrypoint / command registry / output / auth flow | `cli/cmd/freshbooks/`, `cli/internal/{cmd,config,output,auth}/` |
| Gate, scripts, CI | `mise.toml`, `scripts/`, `.github/workflows/{ci,release}.yml`, `.golangci.yml` |
| Guides | `docs/*.md`; API reference on pkg.go.dev |
| Process | `GOAL.md`, `docs/progress.md`, `docs/phases/` |

## Locked design decisions (don't re-litigate)

Spec section 2 is authoritative. Headlines: monorepo with three modules and prefixed tags; no codegen (inventory tool + handcrafted); stdlib-only lib; distinct `AccountID`/`BusinessID` types; `TokenSource`/`TokenStore` with rotation write-back in the lib, login owned by the CLI, MCP is a token consumer; MCP stateless via go-sdk `StreamableHTTPOptions{Stateless: true}`; CLI outputs `json|yaml|table|name` (no jsonpath); docs as Markdown + pkg.go.dev (Docusaurus later); MIT.

## Commands you'll want

```bash
mise run check                       # full gate for every module + dirty-tree banner
mise run check -- freshbooks         # one module (once Phase 0 lands the task args)
mise run test                        # go test -race -coverprofile across the workspace
mise run cover                       # coverage gate (90%)
mise run inventory-check             # parity against the Postman inventory
go run ./freshbooks/internal/inventory -in freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json -out /tmp/inventory.json
mise run build                       # cross-compile matrix
go test -tags integration ./...      # cross-package seams (also run in CI)
FRESHBOOKS_LIVE=1 go test -tags live ./freshbooks/...   # read-only smoke vs a real account (never in CI)
scripts/branch-protection.sh         # (re)apply required checks on main via gh api
```

## Gotchas

- **Two API families, two ID types.** Accounting URLs take the string `account_id`; projects/timetracking/comments/auth take the integer `business_id`; ledger accounts take a `business_uuid`. Both come from `GET /auth/api/v1/users/me` -> `business_memberships[]`. Never "convert" one to the other.
- **Accounting responses are enveloped** (`{"response":{"result":{...}}}`) and errors are `response.errors[]`; business-scoped APIs are flat with a `meta` pagination object and a different error shape. The transport normalizes both; resource code never sees the envelope.
- **Refresh tokens are one-time-use.** A refresh returns a new refresh token and kills the old one. Anything that refreshes must persist through `TokenStore` before returning. Tests for this seam are mandatory.
- **Redirect URIs must be HTTPS -- the portal rejects `http://localhost`.** Registered dev redirect is `https://localhost:8765/callback`. The CLI loopback listener serves an ephemeral self-signed TLS cert on `127.0.0.1`, validates `state`, and always offers a paste-the-URL fallback.
- **Two sets of OAuth endpoints exist** (documented `api.freshbooks.com/auth/oauth/*` vs RFC 8414 metadata `auth.freshbooks.com/service/auth/oauth/*` with PKCE). Phase 1 decides; until then treat both as INFERRED.
- **The Postman collection is messy:** `{{accountId}}` vs `{{accountid}}`, a few hard-coded example account IDs instead of variables, a URL with an embedded newline, ~6 `my.freshbooks.com/service/api/...` internal endpoints. The inventory tool normalizes; do not hand-edit the collection.
- **Rate limits are undocumented.** Honor `Retry-After` on 429; default retry is 3 attempts with jittered backoff; tests use `WithClock`.
- **`gh` needs the org scope** to create the repo and set branch protection; `gh auth status` first.
- **Changelog guard + version guard are Claude hooks from agent-ops.** If a commit is blocked, read the message and fix the staging -- do not bypass.

## If you break something

- Gate red: read the first failing step's output; `mise run lint` and `mise run test` individually isolate it.
- Coverage dipped: `go tool cover -func=coverage.out | sort -k3 -n | head` shows the worst packages.
- Inventory check red: either add the `// inventory:` comment to the implementing method or add the entry to the ignore list with a reason -- never delete inventory entries.
- CI green locally, red on GitHub: check the `mise` tool pins match and that `go.work` is committed.
- Hook blocked a commit: it is doing its job; check what you staged.

## Credits

- FreshBooks API and Postman collection are published by FreshBooks (https://www.freshbooks.com/api/start). This project is not affiliated with FreshBooks.
- Process modelled on `hoyle-re`'s GOAL.md treadmill.
