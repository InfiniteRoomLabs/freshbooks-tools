# Changelog

Repo-level changelog (process, CI, docs). Each module keeps its own: `freshbooks/CHANGELOG.md`, `mcp/CHANGELOG.md`, `cli/CHANGELOG.md`.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Phase 3 (MCP server) shipped: `freshbooks-mcp` with 168 tools (one per library method minus the 17 `All` iterators; 212 of the 213 inventory keys, the auth-owned revoke key excluded), stateless streamable HTTP and stdio transports, per-request bearer in HTTP mode, `serve`/`version`/`tools` commands, `docs/mcp.md` rewritten with per-transport setup (see `mcp/CHANGELOG.md`). Stage-1 premise findings recorded as a `STATE AS OF 2026-09-01` callout in spec section 6: go-sdk v1.7.0 current, three `jsonschema` type overrides required (`Date`, `DateTime`, `ProfitLossLine`), no `_all` tools. Phase 3 gate artifacts under `docs/phases/3/` (work order, definitive tool table, five lane reports, triage, fix report).
- Phase 2 (lib resources) shipped as four sequentially merged, individually gated batches: all 213 Postman inventory keys implemented (`todo 0`), 36 services, transport multipart upload + forced-https tokenization host override + shared retry core + path-segment input validation (see `freshbooks/CHANGELOG.md` for the library detail). Phase 2 review-gate artifacts (four work orders, per-batch lane reports, triages, QA reports) under `docs/phases/2/`. Spec section 3/5.1 `STATE AS OF 2026-09-01` callouts recording every docs-vs-collection conflict and the envelope-family resolutions; the live-conformance backlog moved to `docs/progress.md`.
- `CLAUDE.md`: agent-cleanup convention -- stop subagents via `TaskStop` as soon as their gate role completes (lanes after triage, implementers after merge, QA after its verdict).
- Phase 2 work orders `docs/phases/2/plan-{a,b,c,d}.md` and the 2026-09-01 stage-1 re-cut recorded in `GOAL.md`: 25 cross-folder duplicate requests (same method+URL in two or three Postman folders) are each owned by one batch and implemented as one canonical method with stacked `// inventory:` comments; effective batch loads a 51 / b 59 / c 47 / d 52.
- Phase 1 (lib core) shipped: the `freshbooks` library client, transport, errors, pagination, types, `auth/` package, and Identity service (see `freshbooks/CHANGELOG.md` for the library-level detail); live-verified OAuth facts recorded as `STATE AS OF 2026-08-23` callouts in spec section 3; sanitized live captures under `freshbooks/testdata/seed/`; `docs/authentication.md` and `docs/library.md` written for real.
- Phase 1 review-gate artifacts: work order, lead triage, and the implementer / code-review / simplification / security / QA reports under `docs/phases/1/`.
- Phase 0 scaffold: `go.work` with the `freshbooks`, `mcp`, and `cli` modules; the `freshbooks/internal/inventory` tool that normalizes the Postman collection into `inventory.json` and checks Go source against it via `// inventory:` comments; `mise.toml` tasks (`fmt-check`, `vet`, `lint`, `test`, `cover`, `vuln`, `inventory-check`, `actionlint`, `build`, `docs`, `check`) backed by `scripts/`; `.golangci.yml`; `.github/workflows/{ci,release}.yml` and per-module `.goreleaser.yaml`; `.github/dependabot.yml`; `README.md` and the `docs/*.md` stubs.
- Gitignored `fnox.toml` wiring for the registered FreshBooks dev app (`FRESHBOOKS_CLIENT_ID` / `FRESHBOOKS_CLIENT_SECRET`); documented in `CLAUDE.md` and `docs/progress.md`.
- Design spec `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` covering the library, MCP server, CLI, CI/release gates, and the GOAL.md-driven review process.
- FreshBooks public Postman collection (moved to `freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json` in the Phase 0 scaffold) as the endpoint inventory source.
- MIT license.
- Phase 0 review-gate artifacts: work order, lead triage, and the implementer / code-review / simplification / security / QA reports under `docs/phases/0/`.
- `GOAL.md` treadmill (phase 0 goal block + phase 1-5 roadmap), root `CLAUDE.md`, `docs/progress.md`, and the five review-gate work-order templates under `docs/phases/_templates/`.

### Changed

- Spec section 3/7: the FreshBooks developer portal rejects `http://localhost` redirect URIs; the CLI loopback flow now uses ephemeral self-signed TLS on `https://localhost:8765/callback` with a paste-the-URL fallback.
