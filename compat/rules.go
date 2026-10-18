package compat

import (
	"strings"

	goversion "github.com/hashicorp/go-version"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// The sources the rules cite. Each rule's OpenTofuSince is additionally
// MEASURED against real OpenTofu binaries (measure_test.go,
// testdata/matrix.json); where the changelogs and the binaries disagree, the
// binaries win and the disagreement is written on the rule.
const (
	srcTofuChangelog  = "https://github.com/opentofu/opentofu/blob/main/CHANGELOG.md"
	srcTofuMigration  = "https://opentofu.org/docs/intro/migration/"
	srcTFChangelog    = "https://github.com/hashicorp/terraform/blob/main/CHANGELOG.md"
	srcTofuEphemeral  = "https://opentofu.org/docs/language/ephemerality/"
	srcTofuFunctions  = "https://opentofu.org/docs/language/functions/"
	srcTFActions      = "https://developer.hashicorp.com/terraform/language/block/action"
	srcTFQuery        = "https://developer.hashicorp.com/terraform/language/block/tfquery/list"
	srcTFConvert      = "https://developer.hashicorp.com/terraform/language/functions/convert"
	srcTofuImport     = "https://opentofu.org/docs/language/import/"
	srcTofuRemoved    = "https://opentofu.org/docs/language/resources/syntax/#removing-resources"
	srcTofuMoved      = "https://opentofu.org/docs/language/modules/develop/refactoring/"
	srcTofuValidation = "https://opentofu.org/docs/language/values/variables/#custom-validation-rules"
	srcTofuProviderFn = "https://opentofu.org/docs/language/functions/#provider-defined-functions"
	srcTofuOutputs    = "https://opentofu.org/docs/language/values/outputs/"
	srcTofuVariables  = "https://opentofu.org/docs/language/values/variables/"
	srcTofuFiles      = "https://opentofu.org/docs/language/files/"
)

// measurement says how measure_test.go decides whether an OpenTofu binary
// refuses a rule's must-flag fixture.
type measurement struct {
	// phase is the deepest command run: "validate" (init -backend=false,
	// validate) or "plan" (then plan, with local state).
	phase string
	// refusal matches the binary's output when it refuses the construct.
	// Matching the diagnostic, not just the exit code, is what stops a
	// fixture that fails for an unrelated reason (a network blip, a typo)
	// from reading as "this release refuses the construct".
	refusal string
	// none: the construct produces no diagnostic on any release, because
	// OpenTofu never reads it (a file type it ignores). Measured as "passes
	// everywhere", and the rule is a Warning.
	none bool
	// unmeasured, when set, says why no binary measurement exists. Such a
	// rule must be a Warning: it never refuses an upload.
	unmeasured string
}

func ruleList() []*Rule {
	cfg := kinds(kindConfig)
	return []*Rule{
		{
			ID:             "tofucompat_action_block",
			Construct:      "`action` block",
			Severity:       Error,
			TerraformSince: "1.14",
			OpenTofuSince:  "",
			Message:        "A top-level `action` block (Terraform 1.14 actions) is rejected at init with \"Unsupported block type\".",
			Remedy:         "Remove the `action` block and run the operation another way: a provisioner, a `terraform_data` resource with `local-exec`, or a step in your pipeline after apply.",
			Sources:        []string{srcTFActions, srcTofuChangelog},
			kinds:          cfg,
			block:          onBlock("action"),
			measure:        measurement{phase: "validate", refusal: `Blocks of type "action" are not expected here`},
		},
		{
			ID:             "tofucompat_action_trigger",
			Construct:      "`action_trigger` block in `lifecycle`",
			Severity:       Error,
			TerraformSince: "1.14",
			OpenTofuSince:  "",
			Message:        "An `action_trigger` block inside a resource's `lifecycle` (Terraform 1.14 actions) is rejected at init with \"Unsupported block type\".",
			Remedy:         "Remove the `action_trigger` block, together with the `action` it invokes.",
			Sources:        []string{srcTFActions, srcTofuChangelog},
			kinds:          cfg,
			block:          onBlock("resource/lifecycle/action_trigger"),
			measure:        measurement{phase: "validate", refusal: `Blocks of type "action_trigger" are not expected here`},
		},
		{
			ID:             "tofucompat_convert_function",
			Construct:      "`convert()` function",
			Severity:       Error,
			TerraformSince: "1.15",
			OpenTofuSince:  "1.13.0",
			Message:        "`convert()` fails the PLAN with \"Invalid reference\", a diagnostic that does not name the function.",
			Remedy:         "Use the type-specific conversion function instead: `tonumber`, `tostring`, `tobool`, `tolist`, `toset` or `tomap`, or set the OpenTofu version to 1.13 or later.",
			FixDescription: "Rewrites `convert(x, number|string|bool)` to `tonumber(x)`, `tostring(x)` or `tobool(x)`, which convert exactly the same values. Calls converting to a collection or object type are left alone, because `tolist` and friends do not convert the elements.",
			Sources:        []string{srcTFConvert, srcTofuFunctions, srcTofuChangelog},
			kinds:          kinds(kindConfig, kindTest),
			expr:           onCall("convert", convertFix),
			measure:        measurement{phase: "plan", refusal: `Invalid reference`},
		},
		{
			ID:             "tofucompat_ephemeral_resource",
			Construct:      "`ephemeral` resource block",
			Severity:       Error,
			TerraformSince: "1.10",
			OpenTofuSince:  "1.11.0",
			Message:        "A top-level `ephemeral` block (an ephemeral resource) is rejected at init with \"Unsupported block type\".",
			Remedy:         "Set the OpenTofu version to 1.11 or later. On an older release, read the value with a `data` source instead, accepting that it is then stored in the plan and state.",
			Sources:        []string{srcTofuEphemeral, srcTofuChangelog},
			kinds:          cfg,
			block:          onBlock("ephemeral"),
			measure:        measurement{phase: "validate", refusal: `Blocks of type "ephemeral" are not expected here`},
		},
		{
			ID:             "tofucompat_ephemeral_variable",
			Construct:      "`ephemeral` argument on a variable",
			Severity:       Error,
			TerraformSince: "1.10",
			OpenTofuSince:  "1.11.0",
			Message:        "`ephemeral = true` on a `variable` is rejected with \"Unsupported argument\".",
			Remedy:         "Set the OpenTofu version to 1.11 or later. On an older release, remove `ephemeral` and mark the variable `sensitive = true`, accepting that its value is then stored in the plan and state.",
			Sources:        []string{srcTofuEphemeral, srcTofuVariables},
			kinds:          cfg,
			attr:           onAttr("variable", "ephemeral", nil, nil),
			measure:        measurement{phase: "validate", refusal: `An argument named "ephemeral" is not expected here`},
		},
		{
			ID:             "tofucompat_ephemeral_output",
			Construct:      "`ephemeral` argument on an output",
			Severity:       Error,
			TerraformSince: "1.10",
			OpenTofuSince:  "1.11.0",
			Message:        "`ephemeral = true` on an `output` is rejected with \"Unsupported argument\".",
			Remedy:         "Set the OpenTofu version to 1.11 or later. On an older release, remove `ephemeral` and mark the output `sensitive = true`, accepting that its value is then stored in state.",
			Sources:        []string{srcTofuEphemeral, srcTofuOutputs},
			kinds:          cfg,
			attr:           onAttr("output", "ephemeral", nil, nil),
			measure:        measurement{phase: "validate", refusal: `An argument named "ephemeral" is not expected here`},
		},
		{
			ID:             "tofucompat_ephemeralasnull_function",
			Construct:      "`ephemeralasnull()` function",
			Severity:       Error,
			TerraformSince: "1.10",
			OpenTofuSince:  "1.11.0",
			Message:        "`ephemeralasnull()` is rejected with \"Call to unknown function\".",
			Remedy:         "Set the OpenTofu version to 1.11 or later.",
			Sources:        []string{srcTofuFunctions, srcTofuEphemeral},
			kinds:          kinds(kindConfig, kindTest),
			expr:           onCall("ephemeralasnull", nil),
			measure:        measurement{phase: "validate", refusal: `Call to unknown function`},
		},
		{
			ID:             "tofucompat_terraform_applying",
			Construct:      "`terraform.applying`",
			Severity:       Error,
			TerraformSince: "1.10",
			OpenTofuSince:  "1.11.0",
			Message:        "`terraform.applying` is rejected with \"Invalid \\\"terraform\\\" attribute\".",
			Remedy:         "Set the OpenTofu version to 1.11 or later.",
			Sources:        []string{srcTofuEphemeral, srcTofuChangelog},
			kinds:          kinds(kindConfig, kindTest),
			expr:           onTraversal("terraform", "applying"),
			measure:        measurement{phase: "validate", refusal: `Invalid "terraform" attribute`},
		},
		{
			ID:             "tofucompat_import_for_each",
			Construct:      "`for_each` in an `import` block",
			Severity:       Error,
			TerraformSince: "1.7",
			OpenTofuSince:  "1.7.0",
			Message:        "`for_each` inside an `import` block is rejected with \"Unsupported argument\".",
			Remedy:         "Set the OpenTofu version to 1.7 or later, or write one `import` block per instance.",
			Sources:        []string{srcTofuImport, srcTofuChangelog},
			kinds:          cfg,
			attr:           onAttr("import", "for_each", nil, nil),
			measure:        measurement{phase: "validate", refusal: `An argument named "for_each" is not expected here`},
		},
		{
			ID:             "tofucompat_removed_block",
			Construct:      "`removed` block",
			Severity:       Error,
			TerraformSince: "1.7",
			OpenTofuSince:  "1.7.0",
			Message:        "A `removed` block is rejected at init with \"Unsupported block type\".",
			Remedy:         "Set the OpenTofu version to 1.7 or later, or remove the block and run `tofu state rm` once instead.",
			Sources:        []string{srcTofuRemoved, srcTofuChangelog},
			kinds:          cfg,
			block:          onBlock("removed"),
			measure:        measurement{phase: "validate", refusal: `Blocks of type "removed" are not expected here`},
		},
		{
			// Measured: OpenTofu 1.7 accepts a bare `removed` block but not
			// one carrying `lifecycle` or `provisioner`; both arrive in 1.10.
			// Terraform 1.7 REQUIRED `lifecycle { destroy = false }`, so a
			// Terraform-authored `removed` block very often has one.
			ID:             "tofucompat_removed_lifecycle",
			Construct:      "`lifecycle` or `provisioner` in a `removed` block",
			Severity:       Error,
			TerraformSince: "1.7",
			OpenTofuSince:  "1.10.0",
			Message:        "A `lifecycle` or `provisioner` block inside `removed` is rejected with \"Unsupported block type\".",
			Remedy:         "Set the OpenTofu version to 1.10 or later. On 1.7 to 1.9, `lifecycle { destroy = false }` can be deleted: a bare `removed` block already forgets the resource without destroying it.",
			FixDescription: "Deletes a `lifecycle` block whose only content is `destroy = false`, which is what a bare `removed` block does anyway on OpenTofu 1.7 to 1.9. A `lifecycle` with `destroy = true` or any other content, and every `provisioner`, is left for a human.",
			Sources:        []string{srcTofuRemoved, srcTofuChangelog},
			kinds:          cfg,
			block:          removedLifecycle,
			measure:        measurement{phase: "validate", refusal: `Blocks of type "(lifecycle|provisioner|removed)" are not expected here`},
		},
		{
			ID:             "tofucompat_moved_cross_type",
			Construct:      "`moved` between resource types",
			Severity:       Error,
			TerraformSince: "1.8",
			OpenTofuSince:  "1.10.0",
			Message:        "A `moved` block whose `from` and `to` are different resource types fails the plan with \"Resource type mismatch\".",
			Remedy:         "Set the OpenTofu version to 1.10 or later, or replace the move with a `removed` block for the old resource and an `import` block for the new one.",
			Sources:        []string{srcTofuMoved, srcTofuChangelog},
			kinds:          cfg,
			block:          movedCrossType,
			measure:        measurement{phase: "plan", refusal: `Resource type mismatch`},
		},
		{
			ID:             "tofucompat_validation_cross_reference",
			Construct:      "variable validation referring to other values",
			Severity:       Error,
			TerraformSince: "1.9",
			OpenTofuSince:  "1.9.0",
			Message:        "A `validation` condition that refers to anything but its own variable is rejected with \"Invalid reference in variable validation\".",
			Remedy:         "Set the OpenTofu version to 1.9 or later, or move the check into a `check` block or a resource `precondition`.",
			Sources:        []string{srcTofuValidation, srcTofuChangelog},
			kinds:          cfg,
			block:          validationCrossRef,
			measure:        measurement{phase: "validate", refusal: `Invalid reference in variable validation`},
		},
		{
			ID:             "tofucompat_provider_function",
			Construct:      "provider-defined function (`provider::...`)",
			Severity:       Error,
			TerraformSince: "1.8",
			OpenTofuSince:  "1.7.0",
			Message:        "OpenTofu 1.6 cannot parse a `provider::` function name and stops with \"Invalid single-argument block definition\" or \"Missing newline after argument\".",
			Remedy:         "Set the OpenTofu version to 1.7 or later.",
			Sources:        []string{srcTofuProviderFn, srcTofuChangelog},
			kinds:          kinds(kindConfig, kindTest),
			expr:           onCallPrefix("provider::"),
			measure:        measurement{phase: "validate", refusal: `Invalid single-argument block definition|Missing newline after argument|Extra characters after|Invalid expression`},
		},
		{
			ID:             "tofucompat_templatestring_function",
			Construct:      "`templatestring()` function",
			Severity:       Error,
			TerraformSince: "1.9",
			OpenTofuSince:  "1.7.0",
			Message:        "`templatestring()` is rejected with \"Call to unknown function\".",
			Remedy:         "Set the OpenTofu version to 1.7 or later, or move the template into a file and use `templatefile()`.",
			Sources:        []string{srcTofuFunctions, srcTofuChangelog},
			kinds:          kinds(kindConfig, kindTest),
			expr:           onCall("templatestring", nil),
			measure:        measurement{phase: "validate", refusal: `Call to unknown function`},
		},
		{
			ID:             "tofucompat_issensitive_function",
			Construct:      "`issensitive()` function",
			Severity:       Error,
			TerraformSince: "1.8",
			OpenTofuSince:  "1.7.0",
			Message:        "`issensitive()` is rejected with \"Call to unknown function\".",
			Remedy:         "Set the OpenTofu version to 1.7 or later.",
			Sources:        []string{srcTofuFunctions, srcTofuChangelog},
			kinds:          kinds(kindConfig, kindTest),
			expr:           onCall("issensitive", nil),
			measure:        measurement{phase: "validate", refusal: `Call to unknown function`},
		},
		{
			ID:             "tofucompat_deprecated_argument",
			Construct:      "`deprecated` argument on a variable or output",
			Severity:       Error,
			TerraformSince: "1.15",
			OpenTofuSince:  "1.10.0",
			Message:        "`deprecated` on a `variable` or `output` is rejected with \"Unsupported argument\".",
			Remedy:         "Set the OpenTofu version to 1.10 or later, or delete the argument: it only prints a notice and changes no value.",
			FixDescription: "Deletes the `deprecated` argument. It only prints a notice to callers; no value changes.",
			Sources:        []string{srcTofuVariables, srcTofuOutputs, srcTofuChangelog},
			kinds:          cfg,
			attr:           deprecatedAttr,
			measure:        measurement{phase: "validate", refusal: `An argument named "deprecated" is not expected here`},
		},
		{
			ID:             "tofucompat_output_type",
			Construct:      "`type` argument on an output",
			Severity:       Error,
			TerraformSince: "1.15",
			OpenTofuSince:  "",
			Message:        "A `type` constraint on an `output` is rejected with \"Unsupported argument\".",
			Remedy:         "Delete the `type` argument. It only checks the value's type; the value itself is unchanged.",
			FixDescription: "Deletes the output's `type` argument. The output's value is unchanged; only the type check goes.",
			Sources:        []string{srcTofuOutputs, srcTFChangelog},
			kinds:          cfg,
			attr: onAttr("output", "type", nil, func(fc *fileCtx, _ node, a *hclsyntax.Attribute) *Fix {
				return &Fix{Description: "delete the output's `type` argument", Edits: []Edit{removeAttr(fc, a)}}
			}),
			measure: measurement{phase: "validate", refusal: `An argument named "type" is not expected here`},
		},
		{
			ID:             "tofucompat_query_file",
			Construct:      "`.tfquery.hcl` file",
			Severity:       Warning,
			TerraformSince: "1.14",
			OpenTofuSince:  "",
			Message:        "OpenTofu does not read `.tfquery.hcl` files and has no `query` command, so the `list` blocks in this file are silently ignored. Plans and applies are not affected.",
			Remedy:         "Nothing to do for plans and applies. Run `terraform query` locally if you rely on it.",
			Sources:        []string{srcTFQuery, srcTofuFiles},
			kinds:          kinds(kindOther),
			file:           onFileSuffix(".tfquery.hcl"),
			measure:        measurement{none: true},
		},
		{
			ID:             "tofucompat_stacks_file",
			Construct:      "Terraform Stacks file (`.tfcomponent.hcl`, `.tfdeploy.hcl`, `.tfstack.hcl`)",
			Severity:       Warning,
			TerraformSince: "1.13",
			OpenTofuSince:  "",
			Message:        "Terraform Stacks is an HCP Terraform feature OpenTofu does not implement; this file is silently ignored.",
			Remedy:         "Model each stack component as a workspace with its own working directory.",
			Sources:        []string{srcTofuFiles, srcTFChangelog},
			kinds:          kinds(kindOther),
			file: func(fc *fileCtx, r *Rule) {
				for _, s := range []string{".tfcomponent.hcl", ".tfdeploy.hcl", ".tfstack.hcl"} {
					onFileSuffix(s)(fc, r)
				}
			},
			measure: measurement{none: true},
		},
		{
			// Measured: 1.6 to 1.11 check `required_version` in every .tf
			// file against the OPENTOFU version and stop at init; 1.12 and
			// later ignore it in .tf files (it constrains Terraform there) and
			// honour it only in .tofu files. So a Terraform pin such as
			// ">= 1.14.0" stops every OpenTofu before 1.12.
			ID:             "tofucompat_required_version",
			Construct:      "`required_version` that the OpenTofu version does not satisfy",
			Severity:       Error,
			TerraformSince: "0.12",
			OpenTofuSince:  "1.12.0",
			Message:        "OpenTofu before 1.12 checks `required_version` against its OWN version number, so a Terraform version pin it does not satisfy stops init with \"Unsupported OpenTofu Core version\".",
			Remedy:         "Set the OpenTofu version to 1.12 or later, which ignores `required_version` in .tf files; or widen the constraint so it also admits the OpenTofu version; or put an OpenTofu-specific constraint in a .tofu file.",
			Sources:        []string{"https://opentofu.org/docs/language/settings/", srcTofuChangelog},
			kinds:          cfg,
			attr:           requiredVersion,
			measure:        measurement{phase: "validate", refusal: `Unsupported OpenTofu Core version`},
		},
		{
			ID:             "tofucompat_import_in_child_module",
			Construct:      "`import` block in a child module",
			Severity:       Error,
			TerraformSince: "1.16",
			OpenTofuSince:  "",
			Message:        "An `import` block in a module called by another is rejected with \"Invalid import configuration: Import blocks are only allowed in the root module\".",
			Remedy:         "Move the `import` block to the root module and prefix its `to` address with the module path, e.g. `to = module.network.aws_vpc.main`.",
			Sources:        []string{srcTofuImport, srcTFChangelog},
			kinds:          cfg,
			block: func(fc *fileCtx, r *Rule, n node) {
				if n.is("import") && fc.mod.childDirs[fc.dir()] {
					fc.report(r, n.block.TypeRange, nil)
				}
			},
			measure: measurement{phase: "validate", refusal: `An import block was detected in "module\.`},
		},
		{
			ID:             "tofucompat_lifecycle_destroy",
			Construct:      "`destroy` in a resource's `lifecycle`",
			Severity:       Error,
			TerraformSince: "1.16",
			OpenTofuSince:  "1.12.0",
			Message:        "`destroy` inside a resource's `lifecycle` is rejected with \"Unsupported argument\".",
			Remedy:         "Set the OpenTofu version to 1.12 or later.",
			Sources:        []string{"https://opentofu.org/docs/language/meta-arguments/lifecycle/", srcTofuChangelog},
			kinds:          cfg,
			attr:           onAttr("resource/lifecycle", "destroy", nil, nil),
			measure:        measurement{phase: "validate", refusal: `An argument named "destroy" is not expected here`},
		},
		{
			ID:             "tofucompat_module_source_expression",
			Construct:      "module `source` or `version` that is not a literal string",
			Severity:       Error,
			TerraformSince: "1.15",
			OpenTofuSince:  "1.8.0",
			Message:        "A module `source` or `version` built from a variable or local is rejected at init with \"Variables not allowed\".",
			Remedy:         "Set the OpenTofu version to 1.8 or later, which evaluates these early, or write the value as a literal string.",
			Sources:        []string{"https://opentofu.org/docs/language/modules/sources/", srcTofuChangelog},
			kinds:          cfg,
			attr: func(fc *fileCtx, r *Rule, parent node, a *hclsyntax.Attribute) {
				if !parent.is("module") || (a.Name != "source" && a.Name != "version") {
					return
				}
				if _, ok := literalString(a.Expr); !ok {
					fc.report(r, a.NameRange, nil)
				}
			},
			measure: measurement{phase: "validate", refusal: `Variables not allowed`},
		},
		{
			ID:             "tofucompat_terraform_data_store",
			Construct:      "`store` block in `terraform_data`",
			Severity:       Error,
			TerraformSince: "1.16",
			OpenTofuSince:  "",
			Message:        "A `store` block inside a `terraform_data` resource is rejected with \"Unsupported block type\".",
			Remedy:         "Keep the value in the resource's `input` argument and read it back from `output`.",
			Sources:        []string{"https://opentofu.org/docs/language/resources/tf-data/", srcTFChangelog},
			kinds:          cfg,
			block: func(fc *fileCtx, r *Rule, n node) {
				if n.is("resource/store") {
					if res := n.parent(); res != nil && len(res.Labels) == 2 && res.Labels[0] == "terraform_data" {
						fc.report(r, n.block.TypeRange, nil)
					}
				}
			},
			measure: measurement{phase: "validate", refusal: `Blocks of type "store" are not expected here`},
		},
		{
			// Measured: with the declaration, 1.7 and later run these; without
			// it, every release answers "Unknown function provider". Terraform
			// needs no declaration, so a Terraform-authored config has none.
			ID:             "tofucompat_builtin_provider_function",
			Construct:      "`provider::terraform::` function without declaring the built-in provider",
			Severity:       Error,
			TerraformSince: "1.8",
			OpenTofuSince:  "",
			Message:        "OpenTofu runs `provider::terraform::` functions only when the module declares the built-in provider; without it every release fails with \"Unknown function provider\".",
			Remedy:         "Add `terraform = { source = \"terraform.io/builtin/terraform\" }` to this module's `required_providers` (OpenTofu 1.7 or later).",
			Sources:        []string{srcTofuProviderFn, srcTFChangelog},
			kinds:          cfg,
			expr: func(fc *fileCtx, r *Rule, n hclsyntax.Node) {
				call, ok := n.(*hclsyntax.FunctionCallExpr)
				if ok && strings.HasPrefix(call.Name, "provider::terraform::") && !fc.mod.builtinTerraform[fc.dir()] {
					fc.report(r, call.NameRange, nil)
				}
			},
			measure: measurement{phase: "validate", refusal: `Unknown function provider|Missing newline after argument|Invalid single-argument block definition`},
		},
		{
			ID:             "tofucompat_cloud_block_hostname",
			Construct:      "`cloud` block without `hostname`",
			Severity:       Error,
			TerraformSince: "1.1",
			OpenTofuSince:  "",
			Message:        "Terraform defaults the `cloud` block's `hostname` to app.terraform.io; OpenTofu has no default and stops init with \"Hostname is required for the cloud backend\".",
			Remedy:         "Set `hostname` in the `cloud` block (or the TF_CLOUD_HOSTNAME environment variable) to the service you run on.",
			FixDescription: "Adds `hostname = \"app.terraform.io\"`, the host Terraform used implicitly, so the block means what it meant under Terraform. Change it to the service you run on.",
			Sources:        []string{"https://opentofu.org/docs/cli/cloud/settings/", srcTofuChangelog},
			kinds:          cfg,
			block:          cloudHostname,
			measure:        measurement{phase: "init", refusal: `Hostname is required for the cloud backend`},
		},
		{
			ID:             "tofucompat_unsupported_backend",
			Construct:      "backend OpenTofu does not have",
			Severity:       Error,
			TerraformSince: "1.12",
			OpenTofuSince:  "",
			Message:        "OpenTofu has no `oci` backend and stops init with \"Unsupported backend type\".",
			Remedy:         "Move the state to a backend OpenTofu supports, such as `s3` (OCI Object Storage has an S3-compatible API) or `http`, with `tofu init -migrate-state`.",
			Sources:        []string{"https://opentofu.org/docs/language/settings/backends/configuration/", srcTFChangelog},
			kinds:          cfg,
			block: func(fc *fileCtx, r *Rule, n node) {
				if n.is("terraform/backend") && len(n.block.Labels) == 1 && n.block.Labels[0] == "oci" {
					fc.report(r, n.block.LabelRanges[0], nil)
				}
			},
			measure: measurement{phase: "init", refusal: `Unsupported backend type`},
		},
		{
			ID:             "tofucompat_s3_use_lockfile",
			Construct:      "`use_lockfile` in the `s3` backend",
			Severity:       Error,
			TerraformSince: "1.10",
			OpenTofuSince:  "1.10.0",
			Message:        "`use_lockfile` in the `s3` backend is rejected at init with \"Unsupported argument\".",
			Remedy:         "Set the OpenTofu version to 1.10 or later, or lock with `dynamodb_table` instead.",
			Sources:        []string{"https://opentofu.org/docs/language/settings/backends/s3/", srcTofuChangelog},
			kinds:          cfg,
			attr: func(fc *fileCtx, r *Rule, parent node, a *hclsyntax.Attribute) {
				if a.Name == "use_lockfile" && parent.is("terraform/backend") && len(parent.block.Labels) == 1 && parent.block.Labels[0] == "s3" {
					fc.report(r, a.NameRange, nil)
				}
			},
			measure: measurement{phase: "init", refusal: `An argument named "use_lockfile" is not expected here`},
		},
		{
			// Measured on 1.7.10, 1.10.10 and 1.13.1: a bare `removed` block
			// plans "will be removed from the OpenTofu state but will not be
			// destroyed". Terraform's default for the same block is to destroy.
			// Nothing fails, which is why this is the dangerous one.
			ID:             "tofucompat_removed_without_lifecycle",
			Construct:      "`removed` block without `lifecycle { destroy = ... }`",
			Severity:       Warning,
			TerraformSince: "1.7",
			OpenTofuSince:  "",
			Message:        "Without an explicit `lifecycle { destroy = ... }`, OpenTofu FORGETS the resource (removes it from state and leaves it running), where Terraform destroys it.",
			Remedy:         "Say which you mean: add `lifecycle { destroy = true }` to destroy it (OpenTofu 1.10 or later), or `lifecycle { destroy = false }` to forget it. On OpenTofu 1.7 to 1.9, which cannot express either, a bare `removed` block forgets; destroy the resource by deleting its block instead.",
			Sources:        []string{srcTofuRemoved, "https://developer.hashicorp.com/terraform/language/block/removed"},
			kinds:          cfg,
			appliesFrom:    "1.7.0", // before 1.7 the `removed` block itself is refused (tofucompat_removed_block)
			block: func(fc *fileCtx, r *Rule, n node) {
				if !n.is("removed") {
					return
				}
				for _, b := range n.block.Body.Blocks {
					if b.Type == "lifecycle" {
						if _, ok := b.Body.Attributes["destroy"]; ok {
							return
						}
					}
				}
				fc.report(r, n.block.TypeRange, nil)
			},
			measure: measurement{none: true},
		},
		{
			// Not measured against a binary: it needs a provider that
			// declares write-only attributes, and the providers that do are
			// too large to fetch per release. Hence a Warning, which never
			// refuses an upload. The `_wo` suffix is the naming convention
			// HashiCorp's providers use for write-only attributes.
			ID:             "tofucompat_write_only_attribute",
			Construct:      "write-only resource attribute (`*_wo`)",
			Severity:       Warning,
			TerraformSince: "1.11",
			OpenTofuSince:  "1.11.0",
			Message:        "This attribute is named like a write-only attribute (`_wo` suffix). Providers refuse write-only attributes on OpenTofu before 1.11 (\"WriteOnly Attribute Not Allowed\").",
			Remedy:         "Set the OpenTofu version to 1.11 or later. On an older release use the attribute without `_wo`, accepting that its value is then stored in state.",
			Sources:        []string{"https://opentofu.org/docs/language/ephemerality/write-only-attributes/", srcTofuChangelog},
			kinds:          cfg,
			attr: func(fc *fileCtx, r *Rule, parent node, a *hclsyntax.Attribute) {
				if len(parent.path) >= 1 && parent.path[0] == "resource" && strings.HasSuffix(a.Name, "_wo") {
					fc.report(r, a.NameRange, nil)
				}
			},
			measure: measurement{unmeasured: "needs a provider with write-only attributes"},
		},
		{
			ID:             "tofucompat_test_mocks",
			Construct:      "`mock_provider` or `override_*` in a test file",
			Severity:       Warning,
			TerraformSince: "1.7",
			OpenTofuSince:  "1.8.0",
			Message:        "`tofu test` rejects `mock_provider`, `override_resource`, `override_data` and `override_module` with \"Unsupported block type\". Plans and applies are not affected.",
			Remedy:         "Set the OpenTofu version to 1.8 or later to run these tests.",
			Sources:        []string{"https://opentofu.org/docs/cli/commands/test/", srcTofuChangelog},
			kinds:          kinds(kindTest),
			block: func(fc *fileCtx, r *Rule, n node) {
				for _, p := range []string{"mock_provider", "override_resource", "override_data", "override_module"} {
					if n.is(p) {
						fc.report(r, n.block.TypeRange, nil)
					}
				}
			},
			measure: measurement{phase: "test", refusal: `Unsupported block type`},
		},
		{
			ID:             "tofucompat_test_run_options",
			Construct:      "`parallel` or `state_key` in a test `run` block",
			Severity:       Warning,
			TerraformSince: "1.11",
			OpenTofuSince:  "",
			Message:        "`tofu test` rejects `parallel` and `state_key` in a `run` block with \"Unsupported argument\". Plans and applies are not affected.",
			Remedy:         "Remove `parallel` and `state_key`; OpenTofu runs test `run` blocks one after another against one state per module.",
			FixDescription: "Deletes the `parallel` or `state_key` argument. Tests then run one after another, which is how OpenTofu runs them anyway.",
			Sources:        []string{"https://opentofu.org/docs/cli/commands/test/", srcTFChangelog},
			kinds:          kinds(kindTest),
			attr: func(fc *fileCtx, r *Rule, parent node, a *hclsyntax.Attribute) {
				if parent.is("run") && (a.Name == "parallel" || a.Name == "state_key") {
					fc.report(r, a.NameRange, &Fix{Description: "delete `" + a.Name + "`", Edits: []Edit{removeAttr(fc, a)}})
				}
			},
			measure: measurement{phase: "test", refusal: `Unsupported argument`},
		},
	}
}

// Rules is every rule, in a stable order.
var Rules = ruleList()

var ruleByID = func() map[string]*Rule {
	m := make(map[string]*Rule, len(Rules))
	for _, r := range Rules {
		m[r.ID] = r
	}
	return m
}()

// RuleByID returns a rule, or nil.
func RuleByID(id string) *Rule { return ruleByID[id] }

// --- rule-specific matchers -----------------------------------------------

func onTraversal(root, attr string) func(*fileCtx, *Rule, hclsyntax.Node) {
	return func(fc *fileCtx, r *Rule, n hclsyntax.Node) {
		st, ok := n.(*hclsyntax.ScopeTraversalExpr)
		if !ok || len(st.Traversal) < 2 || st.Traversal.RootName() != root {
			return
		}
		if a, ok := st.Traversal[1].(hcl.TraverseAttr); ok && a.Name == attr {
			fc.report(r, st.Traversal.SourceRange(), nil)
		}
	}
}

func onCallPrefix(prefix string) func(*fileCtx, *Rule, hclsyntax.Node) {
	return func(fc *fileCtx, r *Rule, n hclsyntax.Node) {
		call, ok := n.(*hclsyntax.FunctionCallExpr)
		if ok && strings.HasPrefix(call.Name, prefix) {
			fc.report(r, call.NameRange, nil)
		}
	}
}

func onFileSuffix(suffix string) func(*fileCtx, *Rule) {
	return func(fc *fileCtx, r *Rule) {
		if strings.HasSuffix(fc.name, suffix) {
			end := offsetPos(fc.src, 0)
			fc.report(r, hcl.Range{Filename: fc.name, Start: end, End: end}, nil)
		}
	}
}

// convertFix rewrites convert(x, number|string|bool) to the matching to*
// function: the two convert exactly the same values, so the meaning holds.
func convertFix(fc *fileCtx, call *hclsyntax.FunctionCallExpr) *Fix {
	if len(call.Args) != 2 || call.ExpandFinal {
		return nil
	}
	typ, ok := call.Args[1].(*hclsyntax.ScopeTraversalExpr)
	if !ok || len(typ.Traversal) != 1 {
		return nil
	}
	fn := map[string]string{"number": "tonumber", "string": "tostring", "bool": "tobool"}[typ.Traversal.RootName()]
	if fn == "" {
		return nil
	}
	arg := fc.text(call.Args[0].Range())
	return &Fix{
		Description: "rewrite `convert(…, " + typ.Traversal.RootName() + ")` as `" + fn + "(…)`",
		Edits:       []Edit{fc.edit(call.Range(), fn+"("+arg+")")},
	}
}

func deprecatedAttr(fc *fileCtx, r *Rule, parent node, a *hclsyntax.Attribute) {
	if a.Name != "deprecated" || !(parent.is("variable") || parent.is("output")) {
		return
	}
	fc.report(r, a.NameRange, &Fix{Description: "delete the `deprecated` argument", Edits: []Edit{removeAttr(fc, a)}})
}

func removedLifecycle(fc *fileCtx, r *Rule, n node) {
	if !n.is("removed/lifecycle") && !n.is("removed/provisioner") {
		return
	}
	var fix *Fix
	if n.block.Type == "lifecycle" && len(n.block.Body.Blocks) == 0 && len(n.block.Body.Attributes) == 1 {
		if a, ok := n.block.Body.Attributes["destroy"]; ok {
			if lit, ok := a.Expr.(*hclsyntax.LiteralValueExpr); ok && lit.Val.IsKnown() && !lit.Val.IsNull() && lit.Val.False() {
				fix = &Fix{
					Description: "delete `lifecycle { destroy = false }`: a bare `removed` block forgets the resource without destroying it",
					Edits:       []Edit{removeBlock(fc, n.block)},
				}
			}
		}
	}
	fc.report(r, n.block.TypeRange, fix)
}

// movedCrossType reports a moved block whose from and to are resource
// addresses of different types. Module moves and same-type moves are not
// reported: only the TYPE change is the late construct.
func movedCrossType(fc *fileCtx, r *Rule, n node) {
	if !n.is("moved") {
		return
	}
	from, ok1 := n.block.Body.Attributes["from"]
	to, ok2 := n.block.Body.Attributes["to"]
	if !ok1 || !ok2 {
		return
	}
	ft, fok := resourceType(from.Expr)
	tt, tok := resourceType(to.Expr)
	if fok && tok && ft != tt {
		fc.report(r, n.block.TypeRange, nil)
	}
}

// resourceType returns the type of a moved/import address's LAST resource
// step, e.g. aws_instance for module.a.aws_instance.b[0]. It reports false for
// an address that ends in a module, or is not a plain traversal.
func resourceType(e hclsyntax.Expression) (string, bool) {
	st, ok := e.(*hclsyntax.ScopeTraversalExpr)
	if !ok {
		return "", false
	}
	var names []string
	for _, t := range st.Traversal {
		switch s := t.(type) {
		case hcl.TraverseRoot:
			names = append(names, s.Name)
		case hcl.TraverseAttr:
			names = append(names, s.Name)
		}
	}
	// Strip module.<name> pairs.
	for len(names) >= 2 && names[0] == "module" {
		names = names[2:]
	}
	if len(names) == 0 || names[0] == "data" {
		if len(names) == 3 {
			return "data." + names[1], true
		}
		return "", false
	}
	if len(names) != 2 {
		return "", false
	}
	return names[0], true
}

// validationCrossRef reports a variable validation condition that refers to
// a value other than the variable itself (var.<other>, local.*, resources...).
// References to the variable itself, to built-in functions' arguments and to
// for-expression iterators are not reported.
func validationCrossRef(fc *fileCtx, r *Rule, n node) {
	if !n.is("variable/validation") {
		return
	}
	v := n.parent()
	if v == nil || len(v.Labels) != 1 {
		return
	}
	owner := v.Labels[0]
	cond, ok := n.block.Body.Attributes["condition"]
	if !ok {
		return
	}
	for _, t := range cond.Expr.Variables() {
		root := t.RootName()
		if root == "var" {
			if len(t) >= 2 {
				if a, ok := t[1].(hcl.TraverseAttr); ok && a.Name == owner {
					continue
				}
			}
		} else if !referenceRoots[root] {
			// A for-expression iterator or a function-local name: not a
			// reference to another configuration value.
			continue
		}
		fc.report(r, t.SourceRange(), nil)
		return
	}
}

// referenceRoots are the names that refer to configuration values. Anything
// else at the root of a traversal in a condition is a local symbol such as a
// for-expression iterator.
var referenceRoots = map[string]bool{
	"var": true, "local": true, "module": true, "data": true, "path": true,
	"terraform": true, "count": true, "each": true, "self": true,
}

// removeBlock deletes a block's whole lines.
func removeBlock(fc *fileCtx, b *hclsyntax.Block) Edit {
	start, end := b.Range().Start.Byte, b.Range().End.Byte
	for start > 0 && (fc.src[start-1] == ' ' || fc.src[start-1] == '\t') {
		start--
	}
	if end < len(fc.src) && fc.src[end] == '\r' {
		end++
	}
	if end < len(fc.src) && fc.src[end] == '\n' {
		end++
	}
	rng := hcl.Range{Filename: fc.name, Start: offsetPos(fc.src, start), End: offsetPos(fc.src, end)}
	return fc.edit(rng, "")
}

// requiredVersion reports a `required_version` constraint the target version
// does not satisfy. A constraint that is not a literal string, or that does
// not parse, is left alone.
func requiredVersion(fc *fileCtx, r *Rule, parent node, a *hclsyntax.Attribute) {
	if a.Name != "required_version" || !parent.is("terraform") {
		return
	}
	if strings.HasSuffix(fc.name, ".tofu") {
		// A .tofu file's constraint is written for OpenTofu on purpose.
		return
	}
	s, ok := literalString(a.Expr)
	if !ok {
		return
	}
	c, err := goversion.NewConstraint(s)
	if err != nil {
		return
	}
	v, err := goversion.NewVersion(fc.target)
	if err != nil || c.Check(v) {
		return
	}
	fc.report(r, a.NameRange, nil)
}

// cloudHostname reports a `cloud` block with no hostname, and offers to add
// the one Terraform used implicitly.
func cloudHostname(fc *fileCtx, r *Rule, n node) {
	if !n.is("terraform/cloud") {
		return
	}
	if _, ok := n.block.Body.Attributes["hostname"]; ok {
		return
	}
	// Insert on its own line after the opening brace, indented like the
	// block's first line plus two spaces.
	open := n.block.OpenBraceRange
	indent := lineIndent(fc.src, n.block.TypeRange.Start.Byte) + "  "
	at := hcl.Range{Filename: fc.name, Start: open.End, End: open.End}
	fix := &Fix{
		Description: "add `hostname = \"app.terraform.io\"` to the `cloud` block",
		Edits:       []Edit{fc.edit(at, "\n"+indent+`hostname = "app.terraform.io"`)},
	}
	fc.report(r, n.block.TypeRange, fix)
}

// lineIndent returns the whitespace at the start of the line holding off.
func lineIndent(src []byte, off int) string {
	ls := off
	for ls > 0 && src[ls-1] != '\n' {
		ls--
	}
	e := ls
	for e < len(src) && (src[e] == ' ' || src[e] == '\t') {
		e++
	}
	return string(src[ls:e])
}
