// Package compat checks Terraform-language configuration for constructs a
// given OpenTofu release cannot run, or runs differently (RUN-1385).
//
// # Why this exists
//
// runners.io executes customers' Terraform-authored configurations with
// OpenTofu (services/tfwarp/CLAUDE.md, hard constraint 1). The two languages
// are close but not equal: Terraform adds constructs OpenTofu gains a release
// or two later, or never. OpenTofu does not stop on Terraform's
// `required_version` pin (measured, docs/architecture/opentofu-compatibility.md),
// so nothing tells a customer before their run fails, and some failures name
// nothing the customer wrote. This package says, per construct and per
// OpenTofu release, what will not work and what to do instead.
//
// # How a rule set is versioned
//
// Every rule carries OpenTofuSince: the first OpenTofu release that runs the
// construct, or "" when none does. The rule set for a release is every rule
// that release does not yet support. The arrival versions are not taken on
// trust from changelogs: measure_test.go runs each rule's must-flag fixture
// through a real OpenTofu binary of every supported line and checks that the
// binary refuses it exactly where the rule says it will (testdata/matrix.json).
//
// # The design rule: a false finding is worse than a miss
//
// The same rules gate uploads on runners.io (internal/preflight), where an
// Error finding refuses the configuration outright. So every rule matches an
// exact nesting path, only Error-severity rules are ever used to refuse, and
// anything this package cannot read (JSON configuration, a file that does not
// parse) is reported as skipped rather than as clean.
//
// This package has no dependency on tflint or on runners.io. It is used by the
// `tofucompat` CLI, by the tflint plugin, by otfd's upload preflight and pull
// request check, and by the migration draft (RUN-1386).
package compat

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// Severity says what a finding costs the customer.
type Severity string

const (
	// Error: the OpenTofu release refuses the configuration (at init, validate
	// or plan). Only these refuse an upload on runners.io.
	Error Severity = "error"
	// Warning: the configuration runs, but something in it is silently ignored
	// or behaves differently from Terraform.
	Warning Severity = "warning"
)

// Pos is a position in a file. Byte is a zero-based offset; Line and Column
// are one-based, as editors show them.
type Pos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
	Byte   int `json:"byte"`
}

// Edit replaces the bytes [Start.Byte, End.Byte) of File with NewText.
type Edit struct {
	File    string `json:"file"`
	Start   Pos    `json:"start"`
	End     Pos    `json:"end"`
	NewText string `json:"new_text"`
}

// Fix is a mechanical rewrite that makes a finding go away. It is offered only
// where the rewrite keeps the configuration's meaning on the target release;
// Description says exactly what it changes.
type Fix struct {
	Description string `json:"description"`
	Edits       []Edit `json:"edits"`
}

// Finding is one construct, at one place, that the target release cannot run.
type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	File     string   `json:"file"`
	Start    Pos      `json:"start"`
	End      Pos      `json:"end"`
	// Message says what is incompatible, with which release.
	Message string `json:"message"`
	// Remedy says how to fix it by hand.
	Remedy string `json:"remedy"`
	// OpenTofuVersion is the release the configuration was checked against.
	OpenTofuVersion string `json:"opentofu_version"`
	// OpenTofuSince is the first release that supports the construct, or ""
	// when none does yet.
	OpenTofuSince string `json:"opentofu_since,omitempty"`
	// Fix is present only when a mechanical rewrite exists.
	Fix  *Fix   `json:"fix,omitempty"`
	Link string `json:"link,omitempty"`
}

// Skipped is a file that was not checked, and why. A file in this list has
// NOT been shown to be compatible.
type Skipped struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// Result is the outcome of one check.
type Result struct {
	OpenTofuVersion string    `json:"opentofu_version"`
	Findings        []Finding `json:"findings"`
	Skipped         []Skipped `json:"skipped,omitempty"`
	FilesChecked    int       `json:"files_checked"`
}

// Errors counts the Error-severity findings.
func (r *Result) Errors() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == Error {
			n++
		}
	}
	return n
}

// Fixable counts the findings that carry a mechanical fix.
func (r *Result) Fixable() int {
	n := 0
	for _, f := range r.Findings {
		if f.Fix != nil {
			n++
		}
	}
	return n
}

// Options configure a check.
type Options struct {
	// OpenTofuVersion is the release to check against: "" or "latest" for
	// Latest, "1.10" for the newest supported 1.10 patch, or a full version.
	OpenTofuVersion string
	// Disabled names rules to leave out.
	Disabled []string
}

// fileKind is which of the language's file types a file is.
type fileKind int

const (
	kindConfig fileKind = iota // .tf, .tofu
	kindTest                   // .tftest.hcl, .tofutest.hcl
	kindOther                  // a Terraform file type OpenTofu does not read at all
)

// classify returns the kind of a file by name, and false for a file that is not
// configuration at all.
func classify(name string) (fileKind, bool) {
	base := filepath.Base(name)
	switch {
	case strings.HasSuffix(base, ".tftest.hcl"), strings.HasSuffix(base, ".tofutest.hcl"):
		return kindTest, true
	case strings.HasSuffix(base, ".tfquery.hcl"),
		strings.HasSuffix(base, ".tfcomponent.hcl"),
		strings.HasSuffix(base, ".tfdeploy.hcl"),
		strings.HasSuffix(base, ".tfstack.hcl"):
		return kindOther, true
	case strings.HasSuffix(base, ".tf"), strings.HasSuffix(base, ".tofu"):
		return kindConfig, true
	}
	return 0, false
}

// IsCandidate reports whether a file name is one this package reads, so a
// caller collecting files (from a tarball, say) can skip the rest unread.
func IsCandidate(name string) bool {
	if _, ok := classify(name); ok {
		return true
	}
	return strings.HasSuffix(name, ".tf.json") || strings.HasSuffix(name, ".tofu.json")
}

// Check checks a set of files, keyed by slash-separated path relative to the
// configuration root.
func Check(files map[string][]byte, opts Options) (*Result, error) {
	target, err := ResolveVersion(opts.OpenTofuVersion)
	if err != nil {
		return nil, err
	}
	disabled := make(map[string]bool, len(opts.Disabled))
	for _, d := range opts.Disabled {
		if _, ok := ruleByID[d]; !ok {
			return nil, fmt.Errorf("no rule named %q", d)
		}
		disabled[d] = true
	}
	var active []*Rule
	for _, r := range Rules {
		if disabled[r.ID] || !r.appliesTo(target) {
			continue
		}
		active = append(active, r)
	}

	res := &Result{OpenTofuVersion: target, Findings: []Finding{}}
	mod := survey(files)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if strings.HasSuffix(name, ".tf.json") || strings.HasSuffix(name, ".tofu.json") {
			// hclsyntax cannot read JSON configuration, and a JSON file that
			// scanned to zero findings would read as checked.
			res.Skipped = append(res.Skipped, Skipped{File: name, Reason: "JSON configuration is not checked"})
			continue
		}
		kind, ok := classify(name)
		if !ok {
			continue
		}
		if kind == kindConfig && strings.HasSuffix(name, ".tf") && supportedIn("1.8.0", target) {
			// OpenTofu 1.8+ reads foo.tofu INSTEAD OF foo.tf when both are in
			// one directory, so foo.tf is not what runs and its constructs
			// are not the customer's problem.
			if _, shadowed := files[strings.TrimSuffix(name, ".tf")+".tofu"]; shadowed {
				res.Skipped = append(res.Skipped, Skipped{File: name, Reason: "a .tofu file of the same name replaces it"})
				continue
			}
		}
		if kind == kindConfig && strings.HasSuffix(name, ".tofu") && !supportedIn("1.8.0", target) {
			res.Skipped = append(res.Skipped, Skipped{File: name, Reason: ".tofu files are read from OpenTofu 1.8"})
			continue
		}
		fc := &fileCtx{name: name, src: files[name], kind: kind, target: target, mod: mod}
		if kind == kindOther {
			res.FilesChecked++
			for _, r := range active {
				if r.file != nil {
					r.file(fc, r)
				}
			}
			res.Findings = append(res.Findings, fc.findings...)
			continue
		}
		f, diags := hclsyntax.ParseConfig(fc.src, name, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			// The engine's own syntax diagnostic has a line and a caret, which
			// beats anything we could say.
			res.Skipped = append(res.Skipped, Skipped{File: name, Reason: "does not parse: " + diags.Errs()[0].Error()})
			continue
		}
		body, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		res.FilesChecked++
		fc.walkBody(body, nil, nil, active)
		for _, r := range active {
			if r.file != nil {
				r.file(fc, r)
			}
		}
		res.Findings = append(res.Findings, fc.findings...)
	}

	sort.SliceStable(res.Findings, func(i, j int) bool {
		a, b := res.Findings[i], res.Findings[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Start.Byte != b.Start.Byte {
			return a.Start.Byte < b.Start.Byte
		}
		return a.Rule < b.Rule
	})
	return res, nil
}

// CheckDir checks every configuration file under root, recursively, skipping
// .terraform (downloaded modules the customer did not write) and hidden
// directories.
func CheckDir(root string, opts Options) (*Result, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (d.Name() == ".terraform" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !IsCandidate(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = src
		return nil
	})
	if err != nil {
		return nil, err
	}
	return Check(files, opts)
}

// fileCtx is one file being checked.
type fileCtx struct {
	name     string
	src      []byte
	kind     fileKind
	target   string
	mod      *moduleFacts
	findings []Finding
}

// dir is the slash-separated directory of the file, "." for the root.
func (fc *fileCtx) dir() string { return path.Dir(fc.name) }

// moduleFacts are facts about the whole set of files that a rule looking at
// one file needs: which directories are CHILD modules, and which directories
// declare the built-in `terraform` provider.
type moduleFacts struct {
	// childDirs are directories some checked file calls as a local module
	// (`source = "./x"`). A directory nobody calls is a root module, even
	// when it is not the top directory: a repository holding several stacks
	// side by side has several roots.
	childDirs map[string]bool
	// builtinTerraform are directories whose required_providers declare
	// source = "terraform.io/builtin/terraform".
	builtinTerraform map[string]bool
}

func survey(files map[string][]byte) *moduleFacts {
	m := &moduleFacts{childDirs: map[string]bool{}, builtinTerraform: map[string]bool{}}
	for name, src := range files {
		if k, ok := classify(name); !ok || k != kindConfig {
			continue
		}
		f, diags := hclsyntax.ParseConfig(src, name, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			continue
		}
		body, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		dir := path.Dir(name)
		for _, b := range body.Blocks {
			switch b.Type {
			case "module":
				a, ok := b.Body.Attributes["source"]
				if !ok {
					continue
				}
				src, ok := literalString(a.Expr)
				if ok && (strings.HasPrefix(src, "./") || strings.HasPrefix(src, "../")) {
					m.childDirs[path.Clean(path.Join(dir, src))] = true
				}
			case "terraform":
				for _, rp := range b.Body.Blocks {
					if rp.Type != "required_providers" {
						continue
					}
					if a, ok := rp.Body.Attributes["terraform"]; ok {
						if obj, ok := a.Expr.(*hclsyntax.ObjectConsExpr); ok {
							for _, item := range obj.Items {
								k, _ := literalKey(item.KeyExpr)
								v, _ := literalString(item.ValueExpr)
								if k == "source" && strings.TrimPrefix(v, "terraform.io/") == "builtin/terraform" {
									m.builtinTerraform[dir] = true
								}
							}
						}
					}
				}
			}
		}
	}
	return m
}

// literalString returns the value of an expression that is a plain quoted
// string with no interpolation.
func literalString(e hclsyntax.Expression) (string, bool) {
	t, ok := e.(*hclsyntax.TemplateExpr)
	if !ok || len(t.Parts) != 1 {
		return "", false
	}
	lit, ok := t.Parts[0].(*hclsyntax.LiteralValueExpr)
	if !ok || !lit.Val.IsKnown() || lit.Val.IsNull() || lit.Val.Type() != cty.String {
		return "", false
	}
	return lit.Val.AsString(), true
}

// literalKey returns an object key written as a bare word or a quoted string.
func literalKey(e hclsyntax.Expression) (string, bool) {
	if k, ok := e.(*hclsyntax.ObjectConsKeyExpr); ok {
		if st, ok := k.Wrapped.(*hclsyntax.ScopeTraversalExpr); ok && len(st.Traversal) == 1 {
			return st.Traversal.RootName(), true
		}
		return literalString(k.Wrapped)
	}
	return "", false
}

func pos(p hcl.Pos) Pos { return Pos{Line: p.Line, Column: p.Column, Byte: p.Byte} }

func (fc *fileCtx) report(r *Rule, rng hcl.Range, fix *Fix) {
	msg := r.Message
	if r.OpenTofuSince == "" {
		msg += fmt.Sprintf(" No OpenTofu release supports it (checked against %s).", fc.target)
	} else {
		msg += fmt.Sprintf(" OpenTofu supports it from %s; this configuration is checked against %s.", r.OpenTofuSince, fc.target)
	}
	fc.findings = append(fc.findings, Finding{
		Rule:            r.ID,
		Severity:        r.Severity,
		File:            fc.name,
		Start:           pos(rng.Start),
		End:             pos(rng.End),
		Message:         msg,
		Remedy:          r.Remedy,
		OpenTofuVersion: fc.target,
		OpenTofuSince:   r.OpenTofuSince,
		Fix:             fix,
		Link:            r.link(),
	})
}

func (fc *fileCtx) text(rng hcl.Range) string {
	return string(fc.src[rng.Start.Byte:rng.End.Byte])
}

func (fc *fileCtx) edit(rng hcl.Range, newText string) Edit {
	return Edit{File: fc.name, Start: pos(rng.Start), End: pos(rng.End), NewText: newText}
}

// node is where the walker is: the block types from the file's root down.
type node struct {
	path  []string
	block *hclsyntax.Block // the innermost block, nil at the root
	// stack is every enclosing block, outermost first, ending with block.
	stack []*hclsyntax.Block
}

// parent returns the block enclosing n's block, or nil.
func (n node) parent() *hclsyntax.Block {
	if len(n.stack) < 2 {
		return nil
	}
	return n.stack[len(n.stack)-2]
}

func (n node) is(path string) bool { return pathMatch(n.path, path) }

// pathMatch compares a block-type path with a pattern such as
// "resource/lifecycle/action_trigger". A "*" segment matches any one block
// type. The match is EXACT in depth: `action` matches only a top-level block,
// never `resource "aws_wafv2_web_acl" { rule { action {} } }`.
func pathMatch(path []string, pattern string) bool {
	segs := strings.Split(pattern, "/")
	if pattern == "" {
		segs = nil
	}
	if len(segs) != len(path) {
		return false
	}
	for i, s := range segs {
		if s != "*" && s != path[i] {
			return false
		}
	}
	return true
}

func (fc *fileCtx) walkBody(body *hclsyntax.Body, path []string, stack []*hclsyntax.Block, rules []*Rule) {
	parent := node{path: path, stack: stack}
	if len(stack) > 0 {
		parent.block = stack[len(stack)-1]
	}
	for _, attr := range body.Attributes {
		for _, r := range rules {
			if r.attr != nil && r.kinds.has(fc.kind) {
				r.attr(fc, r, parent, attr)
			}
		}
		fc.walkExpr(attr.Expr, rules)
	}
	for _, blk := range body.Blocks {
		child := append(append([]string(nil), path...), blk.Type)
		childStack := append(append([]*hclsyntax.Block(nil), stack...), blk)
		n := node{path: child, block: blk, stack: childStack}
		for _, r := range rules {
			if r.block != nil && r.kinds.has(fc.kind) {
				r.block(fc, r, n)
			}
		}
		fc.walkBody(blk.Body, child, childStack, rules)
	}
}

type exprWalker struct {
	fc    *fileCtx
	rules []*Rule
}

func (w exprWalker) Enter(n hclsyntax.Node) hcl.Diagnostics {
	for _, r := range w.rules {
		if r.expr != nil && r.kinds.has(w.fc.kind) {
			r.expr(w.fc, r, n)
		}
	}
	return nil
}

func (exprWalker) Exit(hclsyntax.Node) hcl.Diagnostics { return nil }

func (fc *fileCtx) walkExpr(e hclsyntax.Expression, rules []*Rule) {
	// Walk, not a type switch: a call inside a template, a conditional or a
	// for-expression is found too.
	hclsyntax.Walk(e, exprWalker{fc: fc, rules: rules})
}

// ApplyFixes applies the edits of the given findings that belong to file, and
// returns the new contents. Overlapping edits are refused rather than applied
// in some order: two fixes that touch the same bytes need a human.
func ApplyFixes(file string, src []byte, findings []Finding) ([]byte, error) {
	var edits []Edit
	for _, f := range findings {
		if f.Fix == nil {
			continue
		}
		for _, e := range f.Fix.Edits {
			if e.File == file {
				edits = append(edits, e)
			}
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].Start.Byte > edits[j].Start.Byte })
	out := append([]byte(nil), src...)
	prevStart := len(src) + 1
	for _, e := range edits {
		if e.Start.Byte < 0 || e.End.Byte > len(src) || e.Start.Byte > e.End.Byte {
			return nil, fmt.Errorf("%s: edit %d-%d is outside the file", file, e.Start.Byte, e.End.Byte)
		}
		if e.End.Byte > prevStart {
			return nil, fmt.Errorf("%s: two fixes overlap at byte %d; apply them one at a time", file, e.Start.Byte)
		}
		out = append(out[:e.Start.Byte], append([]byte(e.NewText), out[e.End.Byte:]...)...)
		prevStart = e.Start.Byte
	}
	return out, nil
}
