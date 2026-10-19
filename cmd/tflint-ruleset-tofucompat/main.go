// Command tflint-ruleset-tofucompat is the tofucompat rules as a tflint
// plugin. See package tflintrules for configuration.
package main

import (
	"github.com/runners-io/tflint-ruleset-tofucompat/internal/buildversion"
	"github.com/runners-io/tflint-ruleset-tofucompat/tflintrules"
	"github.com/terraform-linters/tflint-plugin-sdk/plugin"
)

// version is stamped at release time with -ldflags "-X main.version=...".
// tflint compares it with the `version` in the plugin block. A `go install`
// build has no stamp and reports the module version instead (RUN-1410).
var version = "0.0.0"

func main() {
	plugin.Serve(&plugin.ServeOpts{RuleSet: tflintrules.New(buildversion.Resolve(version, "0.0.0"))})
}
