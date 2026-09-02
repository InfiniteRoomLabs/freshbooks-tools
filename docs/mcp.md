# MCP server

`freshbooks-mcp` is a stateless Model Context Protocol server: 168 tools, one per `freshbooks` client-library method (minus the 17 `All` iterators and the auth-owned `Authorization/Revoke Refresh Token`, which the MCP never touches -- see "Errors and security" below). Every call goes straight through to the FreshBooks API; nothing is cached, sessioned, or written to disk except the optional stdio token file you configure yourself.

## Install

```
go install github.com/InfiniteRoomLabs/freshbooks-tools/mcp/cmd/freshbooks-mcp@latest
```

This installs the `freshbooks-mcp` binary to `$(go env GOPATH)/bin`. `freshbooks-mcp version` confirms it landed; `freshbooks-mcp tools` prints the full tool manifest as JSON.

## Transports

`freshbooks-mcp serve --transport stdio|http` (default `stdio`).

- **stdio** -- a single-user local process. It owns one FreshBooks client for its whole lifetime, built from the token environment described below. This is what Claude Desktop and Claude Code launch as a subprocess.
- **http** -- stateless streamable HTTP (`mcp.StreamableHTTPOptions{Stateless: true}`). No `Mcp-Session-Id` is ever read or written; every request is authenticated and served independently, and a fresh FreshBooks client is built per request from that request's own bearer token. Suitable for a shared deployment behind TLS, or a `claude.ai` custom connector.

## Stdio setup

### Claude Desktop

Add an entry to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "freshbooks": {
      "command": "freshbooks-mcp",
      "args": ["serve"],
      "env": {
        "FRESHBOOKS_ACCESS_TOKEN": "<a FreshBooks access token>",
        "FRESHBOOKS_ACCOUNT_ID": "<your account id>",
        "FRESHBOOKS_BUSINESS_ID": "<your business id>"
      }
    }
  }
}
```

### Claude Code

```
claude mcp add freshbooks -- freshbooks-mcp serve
```

Set the token/scope environment variables in the shell `claude` runs from, or with `claude mcp add --env`.

## HTTP setup

Run `freshbooks-mcp serve --transport http --addr 0.0.0.0:8080` behind a TLS-terminating reverse proxy -- the server itself speaks plain HTTP; nothing about stateless mode changes what carries the bearer token over the wire, so it must not be exposed without TLS in front of it. Every request under `--path` (default `/mcp`) requires `Authorization: Bearer <token>`; a missing or malformed header gets `401` with `WWW-Authenticate: Bearer` before any JSON-RPC parsing happens. `GET /healthz` is unauthenticated and always `200`.

A `claude.ai` custom connector needs a reachable HTTPS URL and a bearer header -- point it at the proxy in front of `freshbooks-mcp`, not at the bare HTTP process.

A raw `curl` example, `initialize` followed by a tool call (`Accept: application/json, text/event-stream` is required by the streamable-HTTP spec; `Stateless: true` means no `Mcp-Session-Id` ever comes back, so there is nothing to echo on the second request):

```
curl -s https://mcp.example.com/mcp \
  -H "Authorization: Bearer $FRESHBOOKS_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'

curl -s https://mcp.example.com/mcp \
  -H "Authorization: Bearer $FRESHBOOKS_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"identity_whoami","arguments":{}}}'
```

`identity_whoami` needs no scope at all and is always available, so it is the fastest way to confirm a token works and to discover the `account_id`/`business_id`/`business_uuid` to use for everything else.

## Configuration

Cobra flags on `serve`, each with a `FRESHBOOKS_MCP_*` env twin (flag beats env beats the built-in default; an env var explicitly set to `""` is treated as unset, not as a chosen empty value). No config file.

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `--transport` | `FRESHBOOKS_MCP_TRANSPORT` | `stdio` | `stdio` or `http` |
| `--addr` | `FRESHBOOKS_MCP_ADDR` | `127.0.0.1:8080` | listen address in http mode |
| `--path` | `FRESHBOOKS_MCP_PATH` | `/mcp` | URL path the MCP endpoint is served on in http mode |
| `--log-level` | `FRESHBOOKS_MCP_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `--log-format` | `FRESHBOOKS_MCP_LOG_FORMAT` | `text` | `json` or `text` |

Logs always go to stderr -- stdout is the stdio transport's wire format, and writing a log line there would corrupt the protocol stream.

The token and default-scope environment is deliberately **not** `FRESHBOOKS_MCP_*`: these are the same lib-wide names `freshbooks/auth` and the CLI use, so a shell that already exports them for other tools needs nothing extra.

| Variable | Meaning |
|---|---|
| `FRESHBOOKS_ACCESS_TOKEN` | a static bearer token, used as-is with no refresh. Stdio only. |
| `FRESHBOOKS_CLIENT_ID`, `FRESHBOOKS_CLIENT_SECRET`, `FRESHBOOKS_TOKEN_FILE` | together select the rotating path: a `freshbooks/auth.FileStore` at `FRESHBOOKS_TOKEN_FILE` plus `auth.NewTokenSource` for automatic refresh and rotation write-back. Stdio only. |
| `FRESHBOOKS_REFRESH_TOKEN` | seeds `FRESHBOOKS_TOKEN_FILE` the *first* time it is used -- only when no token is stored there yet. Never read again after that. |
| `FRESHBOOKS_ACCOUNT_ID`, `FRESHBOOKS_BUSINESS_ID`, `FRESHBOOKS_BUSINESS_UUID` | the default scope a tool call falls back to when it omits the corresponding field. Stdio only -- `serve --transport http` refuses to start if any of the three is set. A shared, multi-tenant HTTP deployment has no single default scope to fall back to: every caller supplies its own, and silently redirecting one who omits it to the operator's own account is exactly the confused-scope hazard this rejects (worst case, a destructive tool like `identity_delete_business`, which takes only a scope field). |
| `FRESHBOOKS_BASE_URL` | override the FreshBooks API root. Tests and future sandboxes; leave unset in production. |

Stdio mode requires either `FRESHBOOKS_ACCESS_TOKEN` or the complete `FRESHBOOKS_CLIENT_ID`/`FRESHBOOKS_CLIENT_SECRET`/`FRESHBOOKS_TOKEN_FILE` trio, and fails immediately with a message naming what is missing if neither is present. HTTP mode ignores this whole block: its bearer comes from each request's `Authorization` header, request by request, and nothing about it is cached.

## Tool naming and scope

Every tool is named `{service_field_snake}_{method_snake}` -- the `freshbooks.Client` field and the method on it, snake-cased (`invoices_list`, `reports_download_invoice_details_csv`, `payment_options_fb_pay_tokenize`). `freshbooks-mcp tools` prints the full manifest (name, description, annotations, input schema) as JSON, sorted by name; `docs/phases/3/tools.md` is the human-readable table it is generated to match, including which lib method and inventory key(s) each tool carries.

Every tool that needs one takes optional `account_id` (string), `business_id` (integer), and/or `business_uuid` (string) fields. Omit them to use the server's configured default scope (`FRESHBOOKS_ACCOUNT_ID`/`FRESHBOOKS_BUSINESS_ID`/`FRESHBOOKS_BUSINESS_UUID`); supply them per call to override it. If a required scope field is present in neither the call nor the server's defaults, the tool returns an error result naming exactly which field is missing -- never a generic failure.

Annotations follow the MCP hints: `readOnlyHint` on list/get/search/report tools, `destructiveHint` on delete/archive tools, `idempotentHint` on update/verify/undelete tools, and `openWorldHint` on all of them (FreshBooks is an external service, never a closed domain). The 17 `All`-iterator conveniences the lib exposes are not tools -- an unbounded page walk is the wrong shape for a model context; use the paginated `*_list` tool's `page`/`per_page` fields instead. Four `*_list` tools have no `page`/`per_page` fields, because the FreshBooks endpoint behind them takes none: `retainers_list`, `ledger_accounts_list`, `staff_list`, and `service_rates_list` always return their full result in one call.

## Errors and security

A `*freshbooks.Error` from the API maps to an `isError: true` tool result whose content is `{"status": <http status>, "code": <errno>, "message": ..., "field": ..., "family": ...}`; any other error (a network failure, invalid input, a missing scope field) becomes `isError: true` with that error's plain text. Tool handlers never panic and never return a Go error for an API failure -- doing so would surface as a JSON-RPC protocol error and hide the failure from the model instead of reporting it.

For most tools, a malformed argument (the wrong JSON type for a field, a value that fails the schema) is rejected by the MCP SDK's own schema validator before this module's handler ever runs, and that validator's error message quotes the offending value back into the `isError` result -- useful for a model correcting its own mistake, and harmless for a non-sensitive field.

Two constraints are structural, not just documented, and both hold even against a malformed call:

- **OAuth application secrets.** `identity_create_application`, `identity_applications`, and `identity_update_application` all return `freshbooks.Application`, whose `client_secret` field is zeroed (and therefore omitted from the wire -- the field is `omitempty`) before the value reaches a tool result. A live OAuth credential has no business in a stateless, model-facing response. `identity_update_application` also takes a `client_secret` as *input* (FreshBooks requires the current one to authorize the edit), so it is one of the five tools the next paragraph covers.
- **Card tokenization and secret input, end to end.** `payment_options_fb_pay_tokenize`, `payment_options_stripe_tokenize`, `payment_options_stripe_create_setup_intent`, `payment_options_save_credit_card`, and `identity_update_application` never echo their input into a result, an error, or a log -- including a malformed call. These five are registered through a hand-written validator instead of the generic SDK path described above: on any schema-invalid or undecodable input, they return one fixed, name-only message with nothing from the input interpolated, rather than letting the generic validator quote the bad value back. On a well-formed call, results carry only what the lib returns, errors are formatted from the lib's own (already-redacted) error, and this module never formats a tool's input struct with `%v`/`%+v` anywhere. `payment_options_fb_pay_tokenize` and `payment_options_stripe_tokenize` also never send a card number to the regular FreshBooks API host -- they post straight to FreshBooks' tokenization host over HTTPS, per `freshbooks.PaymentOptionsService`'s own doc comments.

The four tokenization tools carry PCI-sensitive data. `freshbooks-mcp` never stores or logs it, but it also does not (and cannot) make you PCI compliant on its own -- treat any client or transcript that carries this data with the same care you would the card itself, and prefer running the HTTP transport behind infrastructure you already trust with cardholder data if a client will call these tools automatically.

Authorization/Revoke Refresh Token is the one inventory entry with no matching tool: `auth.Config.Revoke` lives in the lib, and revocation is a login-lifecycle concern the CLI owns, not something a stateless MCP server should expose as a callable action.
