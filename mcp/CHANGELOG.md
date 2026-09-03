# Changelog

All notable changes to `freshbooks-mcp` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] - 2026-09-03

### Changed

- Requires `freshbooks` v0.3.0: expense and ledger-account results carry the seventeen newly modelled keys; the new `time_entries_list_with_totals` tool (see Added) returns the page plus the account's `total_logged`/`total_unbilled` totals.

### Added

- `time_entries_list_with_totals`: a 169th tool wrapping the new `freshbooks` `TimeEntriesService.ListWithTotals`, over the same wire endpoint as `time_entries_list` -- keyless, like `identity_whoami`, since it carries no inventory key of its own.

## [0.1.1] - 2026-09-03

### Changed

- Requires `freshbooks` v0.2.0: tool results now carry the unified Stripe gateway connection (`stripe_unified`), the full paginated expense-vendor list (an empty account encodes `[]`), typed ledger-account taxonomy objects (`{"name": ...}` types, numeric sub-type ids with `base_number`), `identity_uuid`/`language` on staff members, and `sort` on business-family pages -- all corrected against live captures in the Phase 7 conformance pass.

## [0.1.0] - 2026-09-02

### Added

- `internal/tools`: 168 MCP tools, one per `freshbooks` client-library method (minus the 17 `All` iterators and the auth-owned `Authorization/Revoke Refresh Token`), built by a generic, data-driven `newSpec[In]` constructor rather than 168 hand-rolled handlers. Input schemas are computed once at package init via `jsonschema.ForType` with three type overrides (`freshbooks.Date`, `freshbooks.DateTime`, `freshbooks.ProfitLossLine`) that the SDK's default reflection cannot handle, and shared across every server this process builds through one `mcp.SchemaCache`. Every tool falls back to a configured default `account_id`/`business_id`/`business_uuid` and names the missing field when neither the call nor the default supplies it.
- `internal/server`: stdio (one process-lived `freshbooks.Client`) and stateless streamable HTTP (`mcp.StreamableHTTPOptions{Stateless: true}`, a fresh per-request client built from that request's bearer, `401` + `WWW-Authenticate` before any JSON-RPC parsing when the header is missing or malformed, `GET /healthz` unauthenticated). `bearerToken` matches the `Bearer` Authorization scheme case-insensitively (RFC 7235 section 2.1), so `bearer <token>` and `BEARER <token>` are accepted exactly like `Bearer <token>`. `--transport http` rejects a default scope from `FRESHBOOKS_ACCOUNT_ID`/`FRESHBOOKS_BUSINESS_ID`/`FRESHBOOKS_BUSINESS_UUID` (a multi-tenant confused-scope hazard; those three are stdio-only), validates `--path` (leading `/`) and `--addr` (`host:port`), sets `CrossOriginProtection` explicitly, returns HTTP 400 (not a tool-less server) when per-request client construction fails, and builds the logger once per process.
- `internal/config`: cobra flags with `FRESHBOOKS_MCP_*` env twins (flag over env over a built-in default, an empty env var treated as unset), the lib-wide `FRESHBOOKS_ACCESS_TOKEN`/`FRESHBOOKS_CLIENT_ID`/`FRESHBOOKS_CLIENT_SECRET`/`FRESHBOOKS_TOKEN_FILE`/`FRESHBOOKS_REFRESH_TOKEN` token environment (stdio only; HTTP mode's bearer comes from each request), a malformed `FRESHBOOKS_BUSINESS_ID` rejected at startup instead of silently zeroed, and a redacting `String()`/`LogValue()` so a `Config` can never leak a secret through a log line or an error.
- `cmd/freshbooks-mcp`: the cobra `serve`/`version`/`tools` command tree. `tools` prints the manifest (name, description, annotations, input schema) as JSON, sorted by name. `version` falls back to the Go module version (`debug.ReadBuildInfo`) when built without `-ldflags -X main.version=...`, so a `go install .../freshbooks-mcp@<tag>` build reports the tag instead of a placeholder.
- Security: `identity_create_application`, `identity_applications`, and `identity_update_application` zero `client_secret` before it reaches a result; the four tokenization tools and `identity_update_application` are registered through a sensitive-input path that validates and decodes arguments itself, so neither this module nor the SDK's argument validator ever quotes their input back into a result, an error, or a log (the generic SDK validator does quote a malformed value for the other 163 tools, which is what a model needs to self-correct).
- `docs/mcp.md`: install, stdio setup for Claude Desktop and Claude Code, HTTP setup with a `curl` example, the env/flag table, tool naming and scope defaults, the error shape, and the two security constraints.
- Release: goreleaser builds per-platform archives (`{linux,darwin,windows} x {amd64,arm64}`) with `GOWORK=off` (so the binary embeds the same lib pseudo-version a `go install` user gets), a shared `checksums.txt`, and an SPDX 2.3 JSON SBOM per archive; `gh release create --verify-tag` publishes them onto the module's prefixed tag, since goreleaser OSS cannot release a `mcp/vX.Y.Z`-shaped tag itself.
