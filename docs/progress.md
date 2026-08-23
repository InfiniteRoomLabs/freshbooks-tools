# Progress

Living status doc. Read first, update at every phase boundary. Last updated: 2026-08-23 (Phase 1 shipped).

## Current state

- **Phase 1 (lib core) shipped 2026-08-23**, merge `98ea08c` on `main`. The `freshbooks` module now has: client with all 36 service fields pre-declared (Phase 2 batches only add files), single-path transport (three envelope families, retry 429/502/503/504 with jittered backoff + `Retry-After` capped by `MaxDelay`, cross-host `Authorization` strip, 10MB bounded reads), errors with sentinels, `Page[T]`/`All` iterator, types (`AccountID`/`BusinessID`/`BusinessUUID`, `Money`, `Date`/`DateTime` x3 wire formats, `VisState`, request options), `auth/` (PKCE S256, both endpoint sets with RFC 8414 default, single-flight refresh, rotation write-back before return with pending-save recovery, `FileStore` 0600 atomic + `MemoryStore`), `Identity.Me`/`Whoami`/`Register`, `(*Client).Do` escape hatch. Coverage: freshbooks 99.1% / auth 93.8% / module 95.4%. Inventory: implemented 4 (Authorization), todo 209.
- Stage 1 live-verified the OAuth premise with the registered dev app (two attended consent runs): both endpoint sets work identically, PKCE accepted, refresh rotation + one-time-use confirmed, revoke confirmed, expiry = `created_at + expires_in`. All recorded as `STATE AS OF 2026-08-23` callouts in spec section 3 + 5.1; sanitized captures at `freshbooks/testdata/seed/`.
- **Phase 0 (scaffold) shipped 2026-08-22**, merge `b3063ba` on `main`. Public repo `github.com/InfiniteRoomLabs/freshbooks-tools` with branch protection (`lib`/`mcp`/`cli` required checks, linear history, PRs only).
- Layout: `go.work` + `freshbooks/` (stdlib-only; `doc.go`, `Version`, `internal/inventory`), `mcp/` (version-printing `cmd/freshbooks-mcp`, `internal/{config,server,tools}` placeholders), `cli/` (cobra root with `version` + `completion`, `internal/{cmd,config,output,auth}`). Gate: `mise run check [-- <module>]` = fmt-check, vet, lint, test (-race, coverage, `-tags integration`), cover (90% floor; only `cmd/*/main.go` excluded and those hold one statement), vuln (govulncheck), inventory-check, actionlint, cross build. Coverage at ship: freshbooks 92.2%, mcp 100%, cli 100%.
- Inventory: `freshbooks/internal/inventory/testdata/inventory.json` = 213 entries from the Postman collection beside it; `ignore.list` = 0 ignore / 213 todo (4 `phase-1` for Authorization, 209 `phase-2`). Keys are `<Folder>/<Subfolder>/<Request name>`; name collisions carry ` (METHOD)` on every side. Family counts: accounting 133, business 33, payments 13, auth 11, ledger 7, uploads 6, events 5, internal 3 (`my.freshbooks.com` in the collection; verify the public host live in Phase 2), unknown 2 (`paid.freshbooks.com` tokenization).
- Toolchain pins (`mise.toml`): go 1.26.6, golangci-lint 2.13.1, goreleaser 2.17.1, actionlint 1.7.12, usage 6.0.0. `jdx/mise-action` SHA-pinned (v4.2.5). Dependabot watches gomod x3 + actions weekly; nothing watches `mise.toml` -- bump by hand.
- FreshBooks dev app registered 2026-08-22 (redirect `https://localhost:8765/callback`, all `user:*` scopes). Credentials resolve through the gitignored `fnox.toml` as `FRESHBOOKS_CLIENT_ID` / `FRESHBOOKS_CLIENT_SECRET` (`fnox exec -- ...`); verified resolving.
- Spec status: sections 2-9 approved by Wes in chat on 2026-08-22. Section 3 carries three `STATE AS OF 2026-08-22` callouts from Phase 0 (inventory count, Single Tax collisions, ledger_accounts family). Remaining INFERRED facts (envelope/error shapes per family, date formats, which OAuth endpoint set accepts our app, PKCE) are Phase 1 stage 1's job and need Wes for the browser consent step.

## Phase ledger

| Phase | Status | Branch / merge | Notes |
|---|---|---|---|
| 0 Scaffold | **SHIPPED 2026-08-22** | `phase-0/scaffold` -> `main` @ `b3063ba` | gate: review REQUEST CHANGES -> fixed; security BLOCK -> fixed; QA PASS. Reports in `docs/phases/0/reports/`, triage in `docs/phases/0/triage.md` |
| 1 Lib core | **SHIPPED 2026-08-23** | `phase-1/lib-core` -> `main` @ `98ea08c` | gate: review REQUEST CHANGES + security BLOCK + QA NEEDS WORK, all on one converged blocker (`Token.String` value-receiver redaction) -> fixed in `bfa265d`; triage in `docs/phases/1/triage.md`, no overrides |
| 2 Lib resources (a-d) | not started | `phase-2/{a,b,c,d}` | batches cut against real counts in `GOAL.md` Retarget: (a) Invoices 50; (b) Clients+Estimates+Expenses 50; (c) Projects+TimeTracking+MyTeam+Settings 61; (d) Accounting+Reports+Webhooks+Uploader+Tokenization 48 |
| 3 MCP | not started | | |
| 4 CLI | not started | | |
| 5 Release | not started | | carries the goreleaser tag-collision and changelog-section backlog below |

## Discoveries

- 2026-08-22: FreshBooks has an unannounced first-party MCP (`mcp.freshbooks.com` resolves, `mcp:*` scopes in the OAuth metadata). Not a dependency; noted in spec Future Work.
- 2026-08-22: `auth.freshbooks.com/.well-known/oauth-authorization-server` advertises PKCE S256 and a second set of OAuth endpoints; Phase 1 stage 1 resolves which to use.

- 2026-08-22 (Phase 0 stage 1): the Postman collection has 213 leaf requests in 14 folders / 22 subfolders, not 130 (the spec counted subfolders as requests). Spec section 3 carries the callout with per-folder counts. Inventory keys include the subfolder path. `Single Tax` under `Expenses` and under `Settings/Items and Services` are NOT duplicates -- each is a GET and a DELETE sharing one Postman name and URL; the inventory tool disambiguates both sides with a `" (METHOD)"` suffix, so the real collection normalizes to 213 distinct keys. Phase 2 batch split needs re-cutting: Invoices alone is 50 requests (incl. Payments, Retainers, Other Income).
- 2026-08-22: toolchain verified: go 1.26.5, mise 2026.5.15, gh logged in as org admin of InfiniteRoomLabs, GOPROXY resolves go-sdk v1.7.0 / cobra v1.10.2 / testify v1.12.1; mise can pin golangci-lint 2.13.1, goreleaser 2.17.1, actionlint 1.7.12.

- 2026-08-22 (Phase 0 gate, backlog): `release.yml` sets `GORELEASER_CURRENT_TAG` to the unprefixed `vX.Y.Z`, so `mcp/v0.1.0` and `cli/v0.1.0` would both publish to a release named `v0.1.0` -- Phase 5 must dry-run goreleaser per module and fix (`release.tag`/`name_template`, or `gh release create` against the real tag). goreleaser OSS has no `monorepo` key (Pro only). `scripts/changelog-section.sh` exits 1 silently when the section is missing -- give it a message in Phase 5.
- 2026-08-22 (Phase 0 gate, backlog): the `-tags integration` test pass is a no-op until a `//go:build integration` file exists; Phase 1's work order requires one for the refresh -> store write-back -> retry seam. `scripts/redaction-check.sh` scans staged files only, not commit messages. No `mise.lock` yet.
- 2026-08-22: a private Gitea mirror remote named `gitea` is configured locally (fetch-only, push disabled, per the org convention); the Gitea-side pull mirror is not created yet -- create it from the Gitea UI/API when wanted. `origin` is the public GitHub repo.

- 2026-08-23 (Phase 1 stage 1): both OAuth endpoint sets accept the dev app identically; token response carries undocumented `direct_buy_tokens`; refresh response `created_at` can equal the original exchange's (expiry = `created_at + expires_in`, not `now + expires_in`). Full detail in spec section 3's 2026-08-23 callout.
- 2026-08-23 (Phase 1 gate): retries are at-least-once (transport errors replay non-idempotent bodies) -- documented on `RetryPolicy` and in `docs/library.md`; idempotency gating deliberately deferred to Phase 2's write-heavy batches (triage F9).

## Next action

Run `/goal complete everything in @GOAL.md` in a fresh session. It targets Phase 2 (lib resources, four sonnet batches in parallel worktrees under `.worktrees/`, opus reviewers, one gate per batch, merged sequentially). Unattended. Open INFERRED items Phase 2 must confirm live or from docs: business-family bare `field=value` query encoding (first business-scoped list endpoint), `/events/` + `/payments/` + `/uploads/` envelope families (spec 5.1 callout), and the internal `my.freshbooks.com` endpoints' public paths.

## How to resume in a fresh session

1. Read this file, then `GOAL.md`, then `CLAUDE.md`.
2. `git status --porcelain` must be empty and `git log --oneline -5` should match the ledger above. If not, reconcile before starting.
3. Read only the spec sections the current phase names.
4. Start the goal.
