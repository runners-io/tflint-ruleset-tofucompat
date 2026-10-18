// Package tflintrules serves the compat rules as a tflint ruleset plugin
// (RUN-1385). Every rule here is a thin wrapper: the matching, the versions
// and the fixes all live in package compat, so tflint, the tofucompat CLI and
// otfd cannot disagree about what a rule means.
//
// Configure in .tflint.hcl:
//
//	plugin "tofucompat" {
//	  enabled          = true
//	  opentofu_version = "1.10"   # default: latest
//	}
//
// What tflint cannot see: tflint hands a plugin the .tf files of one module at
// a time, so the rules about test files (.tftest.hcl), query and Stacks
// files, .tofu files and `import` blocks in child modules only report from
// the `tofucompat` CLI.
package tflintrules

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/runners-io/tflint-ruleset-tofucompat/compat"
	"github.com/terraform-linters/tflint-plugin-sdk/hclext"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

// Name is the plugin name: tflint requires the binary to be named
// tflint-ruleset-<Name>.
const Name = "tofucompat"

// RuleSet is the tofucompat ruleset.
type RuleSet struct {
	tflint.BuiltinRuleSet
	// target is the resolved OpenTofu version, shared by every rule.
	target *string
}

// New returns the ruleset at the given plugin version.
func New(version string) *RuleSet {
	target := compat.Latest
	rs := &RuleSet{target: &target}
	rs.Name = Name
	rs.Version = version
	for _, r := range compat.Rules {
		rs.Rules = append(rs.Rules, &rule{r: r, target: rs.target})
	}
	return rs
}

// ConfigSchema declares the plugin block's own attribute.
func (rs *RuleSet) ConfigSchema() *hclext.BodySchema {
	return &hclext.BodySchema{Attributes: []hclext.AttributeSchema{{Name: "opentofu_version"}}}
}

// ApplyConfig reads opentofu_version, refusing one the rules do not know
// rather than quietly checking against another.
func (rs *RuleSet) ApplyConfig(body *hclext.BodyContent) error {
	attr, ok := body.Attributes["opentofu_version"]
	if !ok {
		return nil
	}
	var v string
	if diags := gohcl.DecodeExpression(attr.Expr, nil, &v); diags.HasErrors() {
		return diags
	}
	resolved, err := compat.ResolveVersion(v)
	if err != nil {
		return fmt.Errorf("plugin %q: opentofu_version: %w", Name, err)
	}
	*rs.target = resolved
	return nil
}

type rule struct {
	tflint.DefaultRule
	r      *compat.Rule
	target *string
}

func (x *rule) Name() string  { return x.r.ID }
func (x *rule) Enabled() bool { return true }
func (x *rule) Link() string  { return x.r.Link() }
func (x *rule) Metadata() any { return nil }
func (x *rule) Severity() tflint.Severity {
	if x.r.Severity == compat.Warning {
		return tflint.WARNING
	}
	return tflint.ERROR
}

// Check runs compat over the module's files and emits this rule's findings.
func (x *rule) Check(runner tflint.Runner) error {
	files, err := runner.GetFiles()
	if err != nil {
		return err
	}
	src := make(map[string][]byte, len(files))
	for name, f := range files {
		src[name] = f.Bytes
	}
	res, err := compat.Check(src, compat.Options{OpenTofuVersion: *x.target})
	if err != nil {
		return err
	}
	for _, f := range res.Findings {
		if f.Rule != x.r.ID {
			continue
		}
		rng := hcl.Range{Filename: f.File, Start: hclPos(f.Start), End: hclPos(f.End)}
		msg := f.Message + " Fix: " + f.Remedy
		if f.Fix == nil {
			if err := runner.EmitIssue(x, msg, rng); err != nil {
				return err
			}
			continue
		}
		fix := f.Fix
		if err := runner.EmitIssueWithFix(x, msg, rng, func(fx tflint.Fixer) error {
			for _, e := range fix.Edits {
				r := hcl.Range{Filename: e.File, Start: hclPos(e.Start), End: hclPos(e.End)}
				if err := fx.ReplaceText(r, e.NewText); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func hclPos(p compat.Pos) hcl.Pos { return hcl.Pos{Line: p.Line, Column: p.Column, Byte: p.Byte} }
