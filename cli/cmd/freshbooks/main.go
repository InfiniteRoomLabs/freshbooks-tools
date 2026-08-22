// Command freshbooks is a kubectl-style CLI for the FreshBooks REST API,
// backed by the freshbooks client library. See docs/cli.md in the
// repository root for the command reference (generated once Phase 4 adds
// the resource commands; this scaffold only wires version and completion).
package main

import (
	"os"

	"github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/cmd"
)

// version is the current release version of the freshbooks CLI. It is
// overwritten at build time via -ldflags -X main.version=....
var version = "0.0.0-dev"

func main() {
	os.Exit(cmd.Run(os.Args[1:], os.Stdout, os.Stderr, version))
}
