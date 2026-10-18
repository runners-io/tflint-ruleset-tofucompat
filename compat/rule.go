package compat

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// DocsBase is the published page that documents every rule
// (apps/web/content/docs/infrastructure-runs/opentofu-compatibility.md).
const DocsBase = "https://www.runners.io/docs/infrastructure-runs/opentofu-compatibility"

type kindSet uint8

func kinds(ks ...fileKind) kindSet {
	var s kindSet
	for _, k := range ks {
		s |= 1 << k
	}
	return s
}

func (s kindSet) has(k fileKind) bool { return s&(1<<k) != 0 }

// Rule is one construct OpenTofu runs late, differently, or never.
type Rule struct {
	// ID is stable: it is what a customer disables, what the tflint plugin
	// names, and what the docs page anchors.
	ID string
	// Construct names the thing as a customer would say it.
	Construct string
	Severity  Severity
	// TerraformSince is the Terraform release that added the construct.
	TerraformSince string
	// OpenTofuSince is the first OpenTofu release that runs it, "" for none.
	// Measured, not read: see measure_test.go.
	OpenTofuSince string
	// Message says what goes wrong on a release without the construct.
	Message string
	// Remedy says how to fix it by hand.
	Remedy string
	// FixDescription, when set, says what the mechanical fix does. A rule
	// with one sets Fix on its findings where the rewrite applies.
	FixDescription string
	// Sources are the documents the rule rests on.
	Sources []string

	// appliesFrom, when set, is the first release on which the rule means
	// anything: before it the construct's container is itself refused, and
	// another rule says so.
	appliesFrom string

	kinds   kindSet
	measure measurement
	block   func(fc *fileCtx, r *Rule, n node)
	attr    func(fc *fileCtx, r *Rule, parent node, a *hclsyntax.Attribute)
	expr    func(fc *fileCtx, r *Rule, n hclsyntax.Node)
	file    func(fc *fileCtx, r *Rule)
}

// link is the rule's anchor on DocsBase. The page gives each rule a heading
// that is its id in backticks, and the docs renderer's slugify drops
// backticks and underscores (apps/web/lib/docs/markdown.ts), so the anchor is
// the id without underscores. The page's own test holds the two together.
func (r *Rule) link() string { return DocsBase + "#" + strings.ReplaceAll(r.ID, "_", "") }

// Link is the rule's documentation URL.
func (r *Rule) Link() string { return r.link() }

// HasFix reports whether the rule offers a mechanical fix.
func (r *Rule) HasFix() bool { return r.FixDescription != "" }

// RulesFor returns the rules that apply to an OpenTofu version: the version's
// rule set.
func RulesFor(v string) ([]*Rule, error) {
	target, err := ResolveVersion(v)
	if err != nil {
		return nil, err
	}
	var out []*Rule
	for _, r := range Rules {
		if r.appliesTo(target) {
			out = append(out, r)
		}
	}
	return out, nil
}

// --- matcher constructors -------------------------------------------------

// onBlock reports every block at an exact type path.
func onBlock(path string) func(*fileCtx, *Rule, node) {
	return func(fc *fileCtx, r *Rule, n node) {
		if n.is(path) {
			fc.report(r, n.block.TypeRange, nil)
		}
	}
}

// onAttr reports an attribute by name directly inside blocks at a type path,
// optionally only when pred holds. fix may be nil.
func onAttr(path, name string, pred func(fc *fileCtx, a *hclsyntax.Attribute) bool, fix func(fc *fileCtx, parent node, a *hclsyntax.Attribute) *Fix) func(*fileCtx, *Rule, node, *hclsyntax.Attribute) {
	return func(fc *fileCtx, r *Rule, parent node, a *hclsyntax.Attribute) {
		if a.Name != name || !parent.is(path) {
			return
		}
		if pred != nil && !pred(fc, a) {
			return
		}
		var f *Fix
		if fix != nil {
			f = fix(fc, parent, a)
		}
		fc.report(r, a.NameRange, f)
	}
}

// onCall reports calls to a function by exact name. fix may be nil.
func onCall(name string, fix func(fc *fileCtx, call *hclsyntax.FunctionCallExpr) *Fix) func(*fileCtx, *Rule, hclsyntax.Node) {
	return func(fc *fileCtx, r *Rule, n hclsyntax.Node) {
		call, ok := n.(*hclsyntax.FunctionCallExpr)
		if !ok || call.Name != name {
			return
		}
		var f *Fix
		if fix != nil {
			f = fix(fc, call)
		}
		fc.report(r, call.NameRange, f)
	}
}

// isTrue reports whether an attribute's expression is the literal true.
func isTrue(_ *fileCtx, a *hclsyntax.Attribute) bool {
	lit, ok := a.Expr.(*hclsyntax.LiteralValueExpr)
	return ok && lit.Val.IsKnown() && !lit.Val.IsNull() && lit.Val.Type().FriendlyName() == "bool" && lit.Val.True()
}

// removeAttrLine deletes an attribute's whole line, including its newline,
// when the attribute is alone on it; otherwise just the attribute.
func removeAttr(fc *fileCtx, a *hclsyntax.Attribute) Edit {
	start, end := a.SrcRange.Start.Byte, a.SrcRange.End.Byte
	ls := start
	for ls > 0 && (fc.src[ls-1] == ' ' || fc.src[ls-1] == '\t') {
		ls--
	}
	le := end
	for le < len(fc.src) && (fc.src[le] == ' ' || fc.src[le] == '\t') {
		le++
	}
	if (ls == 0 || fc.src[ls-1] == '\n') && (le == len(fc.src) || fc.src[le] == '\n' || fc.src[le] == '\r') {
		if le < len(fc.src) && fc.src[le] == '\r' {
			le++
		}
		if le < len(fc.src) && fc.src[le] == '\n' {
			le++
		}
		start, end = ls, le
	}
	rng := hcl.Range{Filename: fc.name, Start: offsetPos(fc.src, start), End: offsetPos(fc.src, end)}
	return fc.edit(rng, "")
}

// offsetPos converts a byte offset into a Pos.
func offsetPos(src []byte, off int) hcl.Pos {
	line, col := 1, 1
	for i := 0; i < off && i < len(src); i++ {
		if src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return hcl.Pos{Line: line, Column: col, Byte: off}
}

// appliesTo reports whether the rule is in the rule set for target.
func (r *Rule) appliesTo(target string) bool {
	if r.appliesFrom != "" && !supportedIn(r.appliesFrom, target) {
		return false
	}
	return !supportedIn(r.OpenTofuSince, target)
}
