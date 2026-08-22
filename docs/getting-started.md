# Getting started

## Register a FreshBooks app

How to create a developer app on the FreshBooks portal, which scopes to request, and where to find your client ID/secret and redirect URI. Lands in Phase 1, alongside the auth package that consumes them.

## First call from the library

A minimal `go get` + `TokenSource` + `Identity.Me` example. Lands in Phase 1.

## First call from the CLI

`freshbooks auth login` followed by a read-only command like `freshbooks identity me`. Lands in Phase 4.

## First call from the MCP server

Running `freshbooks-mcp serve` and connecting a client (Claude Desktop, `claude.ai`, or a raw `curl` to the streamable HTTP endpoint). Lands in Phase 3.

## Where to go next

Pointers to `authentication.md`, `library.md`, `mcp.md`, and `cli.md` once each has real content.
