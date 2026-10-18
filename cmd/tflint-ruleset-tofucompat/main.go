// Command tflint-ruleset-tofucompat is the tofucompat rules as a tflint
// plugin. See package tflintrules for configuration.
package main

import (
	"github.com/runners-io/tflint-ruleset-tofucompat/tflintrules"
	"github.com/terraform-linters/tflint-plugin-sdk/plugin"
)

// version is stamped at release time with -ldflags "-X main.version=...".
// tflint compares it with the `version` in the plugin block.
var version = "0.0.0"

func main() {
	plugin.Serve(&plugin.ServeOpts{RuleSet: tflintrules.New(version)})
}
