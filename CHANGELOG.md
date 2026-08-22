# Changelog

Repo-level changelog (process, CI, docs). Each module keeps its own: `freshbooks/CHANGELOG.md`, `mcp/CHANGELOG.md`, `cli/CHANGELOG.md`.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Phase 0 scaffold: `go.work` with the `freshbooks`, `mcp`, and `cli` modules; the `freshbooks/internal/inventory` tool that normalizes the Postman collection into `inventory.json` and checks Go source against it via `// inventory:` comments; `mise.toml` tasks (`fmt-check`, `vet`, `lint`, `test`, `cover`, `vuln`, `inventory-check`, `actionlint`, `build`, `docs`, `check`) backed by `scripts/`; `.golangci.yml`; `.github/workflows/{ci,release}.yml` and per-module `.goreleaser.yaml`; `.github/dependabot.yml`; `README.md` and the `docs/*.md` stubs.
- Gitignored `fnox.toml` wiring for the registered FreshBooks dev app (`FRESHBOOKS_CLIENT_ID` / `FRESHBOOKS_CLIENT_SECRET`); documented in `CLAUDE.md` and `docs/progress.md`.
- Design spec `docs/superpowers/specs/2026-08-22-freshbooks-tools-design.md` covering the library, MCP server, CLI, CI/release gates, and the GOAL.md-driven review process.
- FreshBooks public Postman collection (moved to `freshbooks/internal/inventory/testdata/freshbooks.postman_collection.json` in the Phase 0 scaffold) as the endpoint inventory source.
- MIT license.
- `GOAL.md` treadmill (phase 0 goal block + phase 1-5 roadmap), root `CLAUDE.md`, `docs/progress.md`, and the five review-gate work-order templates under `docs/phases/_templates/`.

### Changed

- Spec section 3/7: the FreshBooks developer portal rejects `http://localhost` redirect URIs; the CLI loopback flow now uses ephemeral self-signed TLS on `https://localhost:8765/callback` with a paste-the-URL fallback.
