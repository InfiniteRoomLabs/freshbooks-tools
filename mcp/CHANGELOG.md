# Changelog

All notable changes to `freshbooks-mcp` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `internal/tools`: 168 MCP tools, one per `freshbooks` client-library
  method (minus the 17 `All` iterators and the auth-owned
  `Authorization/Revoke Refresh Token`), built by a generic, data-driven
  `newSpec[In]` constructor rather than 168 hand-rolled handlers. Input
  schemas are computed once at package init via `jsonschema.ForType` with
  three type overrides (`freshbooks.Date`, `freshbooks.DateTime`,
  `freshbooks.ProfitLossLine`) that the SDK's default reflection cannot
  handle, and shared across every server this process builds through one
  `mcp.SchemaCache`. Every tool falls back to a configured default
  `account_id`/`business_id`/`business_uuid` and names the missing field
  when neither the call nor the default supplies it.
- `internal/config`: cobra flags with `FRESHBOOKS_MCP_*` env twins (flag
  over env over a built-in default, an empty env var treated as unset),
  the lib-wide `FRESHBOOKS_ACCESS_TOKEN`/`FRESHBOOKS_CLIENT_ID`/
  `FRESHBOOKS_CLIENT_SECRET`/`FRESHBOOKS_TOKEN_FILE`/
  `FRESHBOOKS_REFRESH_TOKEN` token environment (stdio only; HTTP mode's
  bearer comes from each request), and a redacting `String()`/`LogValue()`
  so a `Config` can never leak a secret through a log line or an error.
- `internal/server`: stdio (one process-lived `freshbooks.Client`) and
  stateless streamable HTTP (`mcp.StreamableHTTPOptions{Stateless: true}`,
  a fresh per-request client built from that request's bearer, `401` +
  `WWW-Authenticate` before any JSON-RPC parsing when the header is
  missing or malformed, `GET /healthz` unauthenticated).
- `cmd/freshbooks-mcp`: the cobra `serve`/`version`/`tools` command tree.
  `tools` prints the manifest (name, description, annotations, input
  schema) as JSON, sorted by name.
- Security: `identity_create_application`, `identity_applications`, and
  `identity_update_application` zero `client_secret` before it reaches a
  result; the four tokenization tools never echo their input into a
  result, an error, or a log.
- `docs/mcp.md`: install, stdio setup for Claude Desktop and Claude Code,
  HTTP setup with a `curl` example, the env/flag table, tool naming and
  scope defaults, the error shape, and the two security constraints.
