# MCP server

## Transports

`freshbooks-mcp serve --transport stdio|http`: stdio for a single-user local process reading token env vars, and stateless streamable HTTP (`mcp.StreamableHTTPOptions{Stateless: true}`) for multi-client deployments where each request carries its own bearer token. Lands in Phase 3.

## Tool manifest

One tool per library method, named `{service_snake}_{verb}` (e.g. `invoices_list`), with `freshbooks-mcp tools` printing the manifest as JSON for docs and parity tests. Lands in Phase 3.

## Client setup

Connecting from Claude Desktop, the `claude.ai` connector UI, and a raw `curl` against the streamable HTTP endpoint. Lands in Phase 3.

## Configuration

Cobra flags with `FRESHBOOKS_MCP_*` env twins, no config file, `--log-level`/`--log-format`. Lands in Phase 3.

## Errors

How library `*Error` values map to MCP tool-call errors versus JSON-RPC transport errors. Lands in Phase 3.
