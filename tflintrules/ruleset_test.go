package tflintrules

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/terraform-linters/tflint-plugin-sdk/hclext"

	"github.com/runners-io/tflint-ruleset-tofucompat/compat"
	"github.com/terraform-linters/tflint-plugin-sdk/helper"
	"github.com/terraform-linters/tflint-plugin-sdk/tflint"
)

func ruleNamed(t *testing.T, rs *RuleSet, id string) tflint.Rule {
	t.Helper()
	for _, r := range rs.Rules {
		if r.Name() == id {
			return r
		}
	}
	t.Fatalf("no rule %s", id)
	return nil
}

func TestEveryCompatRuleIsServed(t *testing.T) {
	rs := New("test")
	if len(rs.Rules) != len(compat.Rules) {
		t.Fatalf("serving %d rules, compat has %d", len(rs.Rules), len(compat.Rules))
	}
}

func TestEmitsAndFixesOnTheConfiguredVersion(t *testing.T) {
	rs := New("test")
	r := ruleNamed(t, rs, "tofucompat_convert_function")
	src := map[string]string{"main.tf": "output \"n\" {\n  value = convert(\"1\", number)\n}\n"}

	// Latest (1.13) has convert(): nothing to say.
	runner := helper.TestRunner(t, src)
	if err := r.Check(runner); err != nil {
		t.Fatal(err)
	}
	if len(runner.Issues) != 0 {
		t.Fatalf("latest: %+v", runner.Issues)
	}

	// 1.12 does not, and the fix is the exact rewrite.
	*rs.target = "1.12.7"
	runner = helper.TestRunner(t, src)
	if err := r.Check(runner); err != nil {
		t.Fatal(err)
	}
	if len(runner.Issues) != 1 || runner.Issues[0].Range.Start.Line != 2 {
		t.Fatalf("1.12: %+v", runner.Issues)
	}
	helper.AssertChanges(t, map[string]string{"main.tf": "output \"n\" {\n  value = tonumber(\"1\")\n}\n"}, runner.Changes())
}

func config(t *testing.T, v string) *hclext.BodyContent {
	t.Helper()
	e, diags := hclsyntax.ParseExpression([]byte(`"`+v+`"`), "x.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	return &hclext.BodyContent{Attributes: hclext.Attributes{"opentofu_version": {Name: "opentofu_version", Expr: e}}}
}

func TestPluginConfigSetsTheVersionAndRefusesUnknownOnes(t *testing.T) {
	rs := New("test")
	if *rs.target != compat.Latest {
		t.Errorf("default target %s", *rs.target)
	}
	if err := rs.ApplyConfig(config(t, "1.10")); err != nil || *rs.target != "1.10.10" {
		t.Errorf("1.10: %v %s", err, *rs.target)
	}
	if err := rs.ApplyConfig(config(t, "1.99")); err == nil {
		t.Error("1.99 accepted")
	}
}
