# Getting started

## Register a FreshBooks app

1. Go to https://www.freshbooks.com/api/start and register a developer application. Note the client id and client secret it gives you.
2. Set the app's redirect URI to `https://localhost:8765/callback` (or another `https://localhost:<port>/callback` -- the developer portal rejects `http://localhost` outright; see `docs/authentication.md`).
3. Pick the scopes your use needs from https://www.freshbooks.com/api/scopes. `user:profile:read` is granted regardless of what you ask for; every other object needs an explicit `user:<object>:read` and/or `user:<object>:write`.
4. Export the two values so the examples below can use them:

```sh
export FRESHBOOKS_CLIENT_ID=<your client id>
export FRESHBOOKS_CLIENT_SECRET=<your client secret>
```

## First call from the library

```sh
go get github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks
```

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

func main() {
	// A static token you already have (from a prior CLI login, or one you
	// paste in for a quick experiment). For a program that should keep
	// itself logged in, use auth.NewTokenSource with a TokenStore instead
	// -- see docs/authentication.md.
	client, err := freshbooks.NewClient(
		freshbooks.WithTokenSource(auth.StaticTokenSource("<a FreshBooks access token>")),
	)
	if err != nil {
		log.Fatal(err)
	}

	memberships, err := client.Identity.Me(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range memberships {
		fmt.Printf("account=%s business=%d\n", m.AccountID, m.BusinessID)
	}
}
```

`Identity.Me` is the standard first call: it returns every account/business/business-uuid triple the token can act on, which is what every other service call needs as its scope argument. See `docs/library.md` for client options, pagination, and error handling.

## First call from the CLI

```sh
go install github.com/InfiniteRoomLabs/freshbooks-tools/cli/cmd/freshbooks@latest   # once v0.1.0 tags ship; mise run build until then

freshbooks auth login   # reads FRESHBOOKS_CLIENT_ID/FRESHBOOKS_CLIENT_SECRET from the environment set above
freshbooks identity me
```

`auth login` opens a browser to FreshBooks' consent screen, runs a loopback HTTPS listener to catch the redirect, and stores the resulting credentials under `$XDG_CONFIG_HOME/freshbooks/credentials/default.json` (mode 0600). `identity me` then lists the accounts/businesses that credential can reach. Save one as your default context so you do not have to pass `--account`/`--business` on every command:

```sh
freshbooks config set-context default --account <account-id> --business <business-id>
```

See `docs/cli.md` for the full command reference, contexts, and output formats.

## First call from the MCP server

```sh
go install github.com/InfiniteRoomLabs/freshbooks-tools/mcp/cmd/freshbooks-mcp@latest   # once v0.1.0 tags ship; mise run build until then
```

For Claude Desktop or Claude Code (stdio transport), add `freshbooks-mcp serve` with a token in its environment -- either a static access token, or the client id/secret/token-file trio for automatic rotation:

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

For a shared deployment, run the stateless HTTP transport on loopback behind a TLS-terminating reverse proxy and call it through the proxy. The server itself speaks plain HTTP and the bearer token travels in the clear, so never expose it without TLS in front:

```sh
freshbooks-mcp serve --transport http --addr 127.0.0.1:8080

curl -s https://mcp.example.com/mcp \
  -H "Authorization: Bearer $FRESHBOOKS_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"identity_whoami","arguments":{}}}'
```

`identity_whoami` needs no scope fields, so it is the fastest way to confirm a token works. See `docs/mcp.md` for the full transport, configuration, and tool-naming reference.

## Where to go next

- `docs/authentication.md` -- the OAuth2 flow in detail: PKCE, token rotation, revocation, where each component stores a token.
- `docs/library.md` -- client construction options, the `AccountID`/`BusinessID`/`BusinessUUID` split, pagination, retries, and error handling.
- `docs/mcp.md` -- transports, tool naming and scope resolution, the security constraints around card tokenization and OAuth secrets.
- `docs/cli.md` -- the full generated command reference, contexts, output formats, exit codes, and the `api` escape hatch.
- `docs/building.md` -- if you are contributing: the toolchain, the gate, and the release flow.
