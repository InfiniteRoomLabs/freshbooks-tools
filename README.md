# freshbooks-tools

A Go toolkit for the [FreshBooks](https://www.freshbooks.com/api/start) REST API: a handcrafted client library, a stateless MCP server, and a kubectl-style CLI, all covering the same endpoint surface. Built because nothing in Go targets the current OAuth2 REST API, existing MCP implementations are TypeScript/Python and stateful, and [Infinite Room Labs](https://github.com/InfiniteRoomLabs) wanted portfolio-grade code it owns to run its own books.

Not affiliated with FreshBooks. MIT licensed.

## Modules

This is a Go workspace (`go.work`) with three independently versioned, independently tagged modules.

| Module | Path | Binary/import | Status |
|---|---|---|---|
| Library | `freshbooks/` | `github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks` | pre-release |
| MCP server | `mcp/` | `freshbooks-mcp` | pre-release |
| CLI | `cli/` | `freshbooks` | pre-release |

API reference for the library: [pkg.go.dev/github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks](https://pkg.go.dev/github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks).

## Install

Prebuilt binaries land on the [Releases page](https://github.com/InfiniteRoomLabs/freshbooks-tools/releases) once the first tags ship (see `docs/progress.md` for status). Until then, build from source:

```sh
go install github.com/InfiniteRoomLabs/freshbooks-tools/mcp/cmd/freshbooks-mcp@latest
go install github.com/InfiniteRoomLabs/freshbooks-tools/cli/cmd/freshbooks@latest
```

To use the library in your own module:

```sh
go get github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks
```

## Docs

- [Getting started](docs/getting-started.md) -- create a FreshBooks app, first call from each component
- [Authentication](docs/authentication.md) -- OAuth2 flow, token lifetimes, rotation
- [Library](docs/library.md) -- client, options, errors, pagination
- [MCP server](docs/mcp.md) -- transports, tool manifest, client setup
- [CLI](docs/cli.md) -- command reference
- [Building](docs/building.md) -- mise, the check gate, branch protection, release flow
- [Agentic transformation](docs/agentic-transformation.md) -- how this repo is built: the Postman collection, the inventory tool, the review-gate process

## Contributing

- Go >= 1.26, installed via [mise](https://mise.jdx.dev/): `mise install`.
- The gate: `mise run check` (or `mise run check -- <module>` for one module). See `docs/building.md` for what it runs.
- Commits: [Conventional Commits](https://www.conventionalcommits.org/), imperative mood, scoped (`feat(freshbooks): ...`, `feat(mcp): ...`, `feat(cli): ...`, `chore(ci): ...`, `docs: ...`).
- If you use Claude Code, install the [agent-ops marketplace](https://github.com/InfiniteRoomLabs/agent-ops) (`/plugin marketplace add InfiniteRoomLabs/agent-ops`) -- it provides the changelog guard that keeps each module's `CHANGELOG.md` honest.
- `scripts/redaction-check.sh` (a pre-commit hygiene check) is optional for outside contributors; it no-ops if the internal term list it looks for isn't present.
