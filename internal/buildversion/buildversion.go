// Package buildversion says which release a binary is (RUN-1410).
//
// GoReleaser stamps the version with -ldflags "-X main.version=...", and
// that stamp always wins. A binary built by `go install
// github.com/runners-io/tflint-ruleset-tofucompat/cmd/tofucompat@v0.1.1` has
// no stamp -- so v0.1.0 printed "tofucompat dev" -- but the Go toolchain
// records the module version it built in the binary's build info, and that
// is read instead. A build from a checkout records "(devel)", which is no
// release, so it keeps the unstamped default.
package buildversion

import (
	"runtime/debug"
	"strings"
)

// Resolve returns stamped when the release build set it (it differs from
// unstamped, the variable's default), else the module version this binary was
// built from without its leading "v", else unstamped.
func Resolve(stamped, unstamped string) string {
	return resolve(stamped, unstamped, debug.ReadBuildInfo)
}

func resolve(stamped, unstamped string, read func() (*debug.BuildInfo, bool)) string {
	if stamped != unstamped {
		return stamped
	}
	info, ok := read()
	if !ok || info == nil {
		return unstamped
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return unstamped
	}
	return strings.TrimPrefix(v, "v")
}
