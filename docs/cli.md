# CLI

## Command reference

Generated from `cobra/doc` once the resource command tree exists (`mise run docs`). Currently the tree has only `freshbooks version` and `freshbooks completion {bash,fish,zsh,powershell}`. Full reference lands in Phase 4.

## Auth commands

`freshbooks auth login|status|logout|token`, including the loopback PKCE flow and its `--no-browser` paste-a-URL fallback. Lands in Phase 4.

## Resource commands

The generic `<resource> {list,get,create,update,delete}` shape plus resource-specific verbs (`invoices send`, `invoices pdf`, ...), generated from the same inventory contract that gates the library and MCP server. Lands in Phase 4.

## Config and contexts

`freshbooks config view|contexts|use-context|set-context`, `$XDG_CONFIG_HOME/freshbooks/config.yaml`, and the flag > env > config precedence. Lands in Phase 4.

## Output formats and automation

`-o json|yaml|table|name`, exit code conventions (0 ok, 1 API/runtime error, 2 usage, 3 auth, 4 not found), and `--dry-run`. Lands in Phase 4.
