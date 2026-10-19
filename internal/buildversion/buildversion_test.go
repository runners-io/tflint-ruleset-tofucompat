package buildversion

import (
	"runtime/debug"
	"testing"
)

// RUN-1410: `go install …/cmd/tofucompat@v0.1.0` printed "tofucompat dev",
// because only GoReleaser stamps -X main.version. A module-mode build records
// its version in the build info, so an unstamped binary reads it from there.
func TestResolve(t *testing.T) {
	info := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) { return &debug.BuildInfo{Main: debug.Module{Version: v}}, true }
	}
	for name, tc := range map[string]struct {
		stamped, unstamped string
		read               func() (*debug.BuildInfo, bool)
		want               string
	}{
		"a release build keeps its stamp":            {"0.1.1", "dev", info("v9.9.9"), "0.1.1"},
		"go install @v0.1.1 reads the module":        {"dev", "dev", info("v0.1.1"), "0.1.1"},
		"the plugin's unstamped default is replaced": {"0.0.0", "0.0.0", info("v0.1.1"), "0.1.1"},
		"a pre-release tag keeps its suffix":         {"dev", "dev", info("v0.2.0-rc.1"), "0.2.0-rc.1"},
		"a checkout build stays unstamped":           {"dev", "dev", info("(devel)"), "dev"},
		"no build info stays unstamped":              {"dev", "dev", func() (*debug.BuildInfo, bool) { return nil, false }, "dev"},
		"an empty module version stays unstamped":    {"dev", "dev", info(""), "dev"},
	} {
		if got := resolve(tc.stamped, tc.unstamped, tc.read); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}
