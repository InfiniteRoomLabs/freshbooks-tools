// Command freshbooks-mcp is a stateless Model Context Protocol server that
// exposes the freshbooks client library as MCP tools. See docs/mcp.md in
// the repository root for setup and transport details (Phase 3 adds the
// serve command; this scaffold only prints the version).
package main

import "os"

// version is the current release version of freshbooks-mcp. It is
// overwritten at build time via -ldflags -X main.version=....
var version = "0.0.0-dev"

func main() {
	os.Exit(run(os.Stdout, os.Stderr, version))
}
