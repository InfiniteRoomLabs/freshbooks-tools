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
  result; the four tokenization tools and `identity_update_application`
  are registered through a sensitive-input path that validates and decodes
  arguments itself, so neither this module nor the SDK's argument
  validator ever quotes their input back into a result, an error, or a
  log (review-gate finding; the generic SDK validator does quote a
  malformed value for the other 163 tools, which is what a model needs to
  self-correct).
- Review-gate hardening: `--transport http` rejects a default scope from
  `FRESHBOOKS_ACCOUNT_ID`/`FRESHBOOKS_BUSINESS_ID`/`FRESHBOOKS_BUSINESS_UUID`
  (multi-tenant confused-scope hazard; stdio only), validates `--path`
  (leading `/`) and `--addr` (`host:port`), rejects a malformed
  `FRESHBOOKS_BUSINESS_ID` at startup instead of silently zeroing it, sets
  `CrossOriginProtection` explicitly, returns HTTP 400 (not a tool-less
  server) when per-request client construction fails, and builds the
  logger once per process.
- `docs/mcp.md`: install, stdio setup for Claude Desktop and Claude Code,
  HTTP setup with a `curl` example, the env/flag table, tool naming and
  scope defaults, the error shape, and the two security constraints.

### Fixed

- `bearerToken` now matches the `Bearer` Authorization scheme
  case-insensitively (RFC 7235 section 2.1: `auth-scheme` is
  case-insensitive), so a client sending `bearer <token>` or
  `BEARER <token>` is accepted exactly like `Bearer <token>`.
- Test hardening from the Phase 3 gate's advisory findings:
  `payment_options_save_credit_card`'s schema-invalid redaction case now
  plants and checks for a real sensitive value (it previously asserted
  nothing); `TestSensitiveToolsNeverEchoInput`'s well-typed subtest now
  asserts `!IsError` before checking for a leak; its log-containment
  check is skipped (with an explanatory comment) when the captured log
  buffer is empty, since an empty buffer proves nothing was logged, not
  that redaction worked; and the round-trip test's page/per_page query
  assertions now check the exact `page=7`/`per_page=13` key=value pair
  instead of the digit appearing anywhere in the query string.
