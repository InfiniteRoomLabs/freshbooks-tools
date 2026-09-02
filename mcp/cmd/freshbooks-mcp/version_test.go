package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	t.Run("[happy] a real ldflags version is returned unchanged", func(t *testing.T) {
		if got := resolveVersion("v1.2.3"); got != "v1.2.3" {
			t.Errorf("resolveVersion(%q) = %q, want unchanged", "v1.2.3", got)
		}
	})

	t.Run("[happy] falls back to the module pseudo-version when unbuilt", func(t *testing.T) {
		orig := readBuildInfo
		defer func() { readBuildInfo = orig }()
		readBuildInfo = func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, true
		}
		if got := resolveVersion(devVersion); got != "v0.1.0" {
			t.Errorf("resolveVersion(devVersion) = %q, want %q", got, "v0.1.0")
		}
	})

	t.Run("[sad] ReadBuildInfo returning false leaves the placeholder", func(t *testing.T) {
		orig := readBuildInfo
		defer func() { readBuildInfo = orig }()
		readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
		if got := resolveVersion(devVersion); got != devVersion {
			t.Errorf("resolveVersion(devVersion) = %q, want unchanged %q", got, devVersion)
		}
	})

	t.Run("[edge] a (devel) Main.Version leaves the placeholder", func(t *testing.T) {
		orig := readBuildInfo
		defer func() { readBuildInfo = orig }()
		readBuildInfo = func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true
		}
		if got := resolveVersion(devVersion); got != devVersion {
			t.Errorf("resolveVersion(devVersion) = %q, want unchanged %q", got, devVersion)
		}
	})

	t.Run("[edge] an empty Main.Version leaves the placeholder", func(t *testing.T) {
		orig := readBuildInfo
		defer func() { readBuildInfo = orig }()
		readBuildInfo = func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: ""}}, true
		}
		if got := resolveVersion(devVersion); got != devVersion {
			t.Errorf("resolveVersion(devVersion) = %q, want unchanged %q", got, devVersion)
		}
	})
}
