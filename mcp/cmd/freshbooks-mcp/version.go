package main

import "runtime/debug"

// devVersion is the placeholder main.version carries when the binary was
// built without -ldflags -X main.version=... (a plain `go build`, or a
// `go install .../cmd/freshbooks-mcp@<tag>` build, which never sets
// ldflags).
const devVersion = "0.0.0-dev"

// readBuildInfo is an injectable seam over debug.ReadBuildInfo so
// resolveVersion is unit-testable without needing a real `go install`
// build (D7). Safe as a package-level var only because no test in this
// package calls t.Parallel().
var readBuildInfo = debug.ReadBuildInfo

// resolveVersion falls back to the Go module version embedded by the Go
// toolchain (debug.ReadBuildInfo's Main.Version) when version is still
// the unbuilt-ldflags placeholder. A `go install .../mcp/cmd/freshbooks-mcp@v0.1.0`
// user then sees "v0.1.0"; a `go install ...@<sha>` user sees the
// pseudo-version. A goreleaser build (which always sets -X main.version)
// or any Main.Version of "" or "(devel)" (no module info, e.g. `go run`
// or a build outside GOPATH/module mode) leaves version untouched.
func resolveVersion(version string) string {
	if version != devVersion {
		return version
	}
	info, ok := readBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return version
	}
	return info.Main.Version
}
