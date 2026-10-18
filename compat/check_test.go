package compat

import (
	"strings"
	"testing"
)

func check(t *testing.T, version string, files map[string]string) *Result {
	t.Helper()
	in := map[string][]byte{}
	for k, v := range files {
		in[k] = []byte(v)
	}
	res, err := Check(in, Options{OpenTofuVersion: version})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func rules(res *Result) string {
	var ids []string
	for _, f := range res.Findings {
		ids = append(ids, f.Rule)
	}
	return strings.Join(ids, ",")
}

// The false refusals the design rule exists to prevent: each of these is a
// construct that LOOKS like a rule's target and is not.
func TestLookalikesAreNotFlagged(t *testing.T) {
	cases := map[string]string{
		// `action` is a nested block in real provider schemas; only a
		// top-level one is a Terraform 1.14 action.
		"nested action": `resource "aws_wafv2_web_acl" "x" {
  rule {
    action {
      block {}
    }
  }
}`,
		"ephemeral attribute in a resource": `resource "x_y" "z" {
  ephemeral = true
}`,
		"moved within one type": `moved {
  from = aws_instance.a
  to   = aws_instance.b
}`,
		"moved between modules": `moved {
  from = module.a
  to   = module.b
}`,
		"validation on its own variable, with a for iterator": `variable "v" {
  type = list(string)
  validation {
    condition     = alltrue([for s in var.v : length(s) > 0])
    error_message = "x"
  }
}`,
		"module source literal": `module "m" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.13.0"
}`,
		"import at the root of a directory nobody calls": `import {
  to = aws_instance.a
  id = "i-1"
}`,
		"store block in another resource type": `resource "x_y" "z" {
  store {}
}`,
		"use_lockfile in a non-s3 backend": `terraform {
  backend "custom" {
    use_lockfile = true
  }
}`,
		"cloud block with hostname": `terraform {
  cloud {
    hostname     = "tf.runners.io"
    organization = "acme"
  }
}`,
		"a satisfied required_version": `terraform {
  required_version = ">= 1.5.0"
}`,
		"a _wo attribute outside a resource": `variable "password_wo" {
  type = string
}`,
	}
	for name, src := range cases {
		res := check(t, "1.6", map[string]string{"main.tf": src})
		if len(res.Findings) != 0 {
			t.Errorf("%s: flagged %s", name, rules(res))
		}
	}
}

func TestDefaultsToLatestAndResolvesLines(t *testing.T) {
	for in, want := range map[string]string{"": Latest, "latest": Latest, "LATEST": Latest, "1.10": "1.10.10", "1.7.2": "1.7.2", "v1.9.0": "1.9.0"} {
		got, err := ResolveVersion(in)
		if err != nil || got != want {
			t.Errorf("ResolveVersion(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"1.5.7", "1.99", "1.14.0", "2.0.0", "banana", "1"} {
		if v, err := ResolveVersion(bad); err == nil {
			t.Errorf("ResolveVersion(%q) = %q; want an error", bad, v)
		}
	}
	res := check(t, "", map[string]string{"main.tf": `output "x" { value = convert("1", number) }`})
	if res.OpenTofuVersion != Latest || len(res.Findings) != 0 {
		t.Errorf("latest: %s %s", res.OpenTofuVersion, rules(res))
	}
}

func TestRuleSetsShrinkAsVersionsRise(t *testing.T) {
	prev := len(Rules) + 1
	for _, v := range Supported {
		rs, err := RulesFor(v)
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) > prev {
			t.Errorf("%s has %d rules, more than the release before (%d)", v, len(rs), prev)
		}
		prev = len(rs)
	}
	latest, _ := RulesFor("")
	oldest, _ := RulesFor("1.6")
	if len(latest) >= len(oldest) {
		t.Errorf("latest has %d rules, 1.6 has %d", len(latest), len(oldest))
	}
}

func TestWhatIsNotReadIsSkippedNotPassed(t *testing.T) {
	res := check(t, "", map[string]string{
		"main.tf.json": `{"resource": {}}`,
		"broken.tf":    `resource "x" {`,
		"README.md":    "# hi",
	})
	if len(res.Skipped) != 2 || res.FilesChecked != 0 {
		t.Errorf("skipped %+v, checked %d", res.Skipped, res.FilesChecked)
	}
}

// OpenTofu 1.8+ reads x.tofu INSTEAD of x.tf in the same directory, so x.tf's
// Terraform-only constructs do not run there.
func TestTofuFileShadowsTheSameNamedTFFile(t *testing.T) {
	files := map[string]string{
		"main.tf":   `action "x" "y" {}`,
		"main.tofu": `resource "terraform_data" "y" {}`,
	}
	if got := rules(check(t, "1.13", files)); got != "" {
		t.Errorf("1.13 flagged a shadowed main.tf: %s", got)
	}
	if got := rules(check(t, "1.7", files)); got != "tofucompat_action_block" {
		t.Errorf("1.7 reads main.tf (no .tofu files before 1.8); got %q", got)
	}
}

func TestRequiredVersionIsJudgedAgainstTheTarget(t *testing.T) {
	src := map[string]string{"main.tf": `terraform {
  required_version = "~> 1.9.0"
}`}
	if got := rules(check(t, "1.9", src)); got != "" {
		t.Errorf("1.9.4 satisfies ~> 1.9.0; got %s", got)
	}
	if got := rules(check(t, "1.10", src)); got != "tofucompat_required_version" {
		t.Errorf("1.10.10 does not satisfy ~> 1.9.0; got %q", got)
	}
	if got := rules(check(t, "1.12", src)); got != "" {
		t.Errorf("1.12 ignores required_version in .tf files; got %s", got)
	}
}

func TestDisabledRulesAreLeftOutAndUnknownOnesRefused(t *testing.T) {
	in := map[string][]byte{"main.tf": []byte(`action "x" "y" {}`)}
	res, err := Check(in, Options{Disabled: []string{"tofucompat_action_block"}})
	if err != nil || len(res.Findings) != 0 {
		t.Errorf("disabled rule still reported: %v %s", err, rules(res))
	}
	if _, err := Check(in, Options{Disabled: []string{"tofucompat_typo"}}); err == nil {
		t.Error("an unknown rule name was accepted")
	}
}

func TestOverlappingFixesAreRefused(t *testing.T) {
	src := []byte("abcdef")
	f := []Finding{
		{Fix: &Fix{Edits: []Edit{{File: "a", Start: Pos{Byte: 1}, End: Pos{Byte: 4}, NewText: "X"}}}},
		{Fix: &Fix{Edits: []Edit{{File: "a", Start: Pos{Byte: 3}, End: Pos{Byte: 5}, NewText: "Y"}}}},
	}
	if _, err := ApplyFixes("a", src, f); err == nil {
		t.Error("overlapping edits were applied")
	}
	got, err := ApplyFixes("a", src, f[:1])
	if err != nil || string(got) != "aXef" {
		t.Errorf("got %q %v", got, err)
	}
}

func TestConvertFixOnlyForExactEquivalents(t *testing.T) {
	res := check(t, "1.12", map[string]string{"main.tf": `locals {
  a = convert(var.x, number)
  b = convert(var.x, list(string))
  c = "${convert(var.y, bool)}"
}`})
	var fixed, unfixed int
	for _, f := range res.Findings {
		if f.Fix != nil {
			fixed++
		} else {
			unfixed++
		}
	}
	if fixed != 2 || unfixed != 1 {
		t.Errorf("fixed %d unfixed %d; want 2 and 1 (list(string) has no exact rewrite)", fixed, unfixed)
	}
	out, err := ApplyFixes("main.tf", []byte(`locals {
  a = convert(var.x, number)
  b = convert(var.x, list(string))
  c = "${convert(var.y, bool)}"
}`), res.Findings)
	if err != nil || !strings.Contains(string(out), "a = tonumber(var.x)") || !strings.Contains(string(out), `"${tobool(var.y)}"`) || !strings.Contains(string(out), "convert(var.x, list(string))") {
		t.Errorf("got %s, %v", out, err)
	}
}
