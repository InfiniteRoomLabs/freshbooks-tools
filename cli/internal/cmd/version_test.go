package cmd

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

	buildInfo := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"[happy] falls back to the module pseudo-version when unbuilt", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, true, "v0.1.0"},
		{"[sad] ReadBuildInfo returning false leaves the placeholder", nil, false, devVersion},
		{"[edge] a (devel) Main.Version leaves the placeholder", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true, devVersion},
		{"[edge] an empty Main.Version leaves the placeholder", &debug.BuildInfo{Main: debug.Module{Version: ""}}, true, devVersion},
	}
	for _, tt := range buildInfo {
		t.Run(tt.name, func(t *testing.T) {
			orig := readBuildInfo
			defer func() { readBuildInfo = orig }()
			readBuildInfo = func() (*debug.BuildInfo, bool) { return tt.info, tt.ok }
			if got := resolveVersion(devVersion); got != tt.want {
				t.Errorf("resolveVersion(devVersion) = %q, want %q", got, tt.want)
			}
		})
	}
}
