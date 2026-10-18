package compat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every rule has testdata/rules/<id>/flag (must produce a finding of that rule
// on every version the rule applies to, and none on the versions it does not)
// and testdata/rules/<id>/clean (must produce no finding of ANY rule on ANY
// supported version). A rule with a mechanical fix also has fixed/, which is
// what its fix turns flag/ into. measure_test.go runs the same fixtures
// through real OpenTofu binaries.

const fixtureRoot = "testdata/rules"

func fixture(t *testing.T, id, kind string) string {
	t.Helper()
	return filepath.Join(fixtureRoot, id, kind)
}

func checkDir(t *testing.T, dir, version string, disabled ...string) *Result {
	t.Helper()
	res, err := CheckDir(dir, Options{OpenTofuVersion: version, Disabled: disabled})
	if err != nil {
		t.Fatalf("CheckDir(%s, %s): %v", dir, version, err)
	}
	return res
}

func count(res *Result, rule string) int {
	n := 0
	for _, f := range res.Findings {
		if f.Rule == rule {
			n++
		}
	}
	return n
}

func TestEveryRuleHasFixturesAndNoFixtureIsOrphaned(t *testing.T) {
	if len(Rules) < 20 {
		// A floor, so that a rule table that failed to build (or a test that
		// iterated an empty one) cannot pass by checking nothing.
		t.Fatalf("only %d rules; expected at least 20", len(Rules))
	}
	for _, r := range Rules {
		for _, kind := range []string{"flag", "clean"} {
			if _, err := os.Stat(fixture(t, r.ID, kind)); err != nil {
				t.Errorf("%s has no %s fixture: %v", r.ID, kind, err)
			}
		}
		_, err := os.Stat(fixture(t, r.ID, "fixed"))
		if r.HasFix() && err != nil {
			t.Errorf("%s offers a fix but has no fixed/ fixture", r.ID)
		}
		if !r.HasFix() && err == nil {
			t.Errorf("%s has a fixed/ fixture but offers no fix", r.ID)
		}
	}
	entries, err := os.ReadDir(fixtureRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if RuleByID(e.Name()) == nil {
			t.Errorf("fixture directory %s names no rule", e.Name())
		}
	}
}

func TestEveryRuleIsDescribed(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Rules {
		if seen[r.ID] {
			t.Errorf("duplicate rule id %s", r.ID)
		}
		seen[r.ID] = true
		if !strings.HasPrefix(r.ID, "tofucompat_") {
			t.Errorf("%s: ids start with tofucompat_ (tflint names rules <ruleset>_<rule>)", r.ID)
		}
		if r.Construct == "" || r.Message == "" || r.Remedy == "" || r.TerraformSince == "" || len(r.Sources) == 0 {
			t.Errorf("%s: Construct, Message, Remedy, TerraformSince and Sources are all required", r.ID)
		}
		if r.Severity != Error && r.Severity != Warning {
			t.Errorf("%s: severity %q", r.ID, r.Severity)
		}
		if r.measure.unmeasured != "" && r.Severity == Error {
			t.Errorf("%s: an unmeasured rule must be a Warning, since Error findings refuse uploads", r.ID)
		}
		if r.measure.none && r.Severity == Error {
			t.Errorf("%s: a rule OpenTofu never diagnoses cannot be an Error", r.ID)
		}
		if r.OpenTofuSince != "" {
			if _, err := parseVersion(r.OpenTofuSince); err != nil {
				t.Errorf("%s: %v", r.ID, err)
			}
		}
	}
}

func TestFlagFixtureIsFlaggedExactlyWhereTheRuleApplies(t *testing.T) {
	for _, r := range Rules {
		dir := fixture(t, r.ID, "flag")
		for _, v := range Supported {
			n := count(checkDir(t, dir, v), r.ID)
			applies := r.appliesTo(v)
			if applies && n == 0 {
				t.Errorf("%s: flag fixture not flagged on OpenTofu %s", r.ID, v)
			}
			if !applies && n != 0 {
				t.Errorf("%s: flagged on OpenTofu %s, which supports it from %s", r.ID, v, r.OpenTofuSince)
			}
		}
	}
}

func TestCleanFixtureIsCleanForEveryRuleOnEveryVersion(t *testing.T) {
	for _, r := range Rules {
		dir := fixture(t, r.ID, "clean")
		for _, v := range Supported {
			res := checkDir(t, dir, v)
			for _, f := range res.Findings {
				t.Errorf("%s clean fixture on %s: %s at %s:%d: %s", r.ID, v, f.Rule, f.File, f.Start.Line, f.Message)
			}
			if len(res.Skipped) != 0 {
				t.Errorf("%s clean fixture on %s skipped files: %+v", r.ID, v, res.Skipped)
			}
			if res.FilesChecked == 0 {
				t.Errorf("%s clean fixture on %s checked no files", r.ID, v)
			}
		}
	}
}

func TestFixTurnsFlagIntoFixedAndClearsTheFinding(t *testing.T) {
	for _, r := range Rules {
		if !r.HasFix() {
			continue
		}
		oldest := firstApplicable(r)
		flag := fixture(t, r.ID, "flag")
		res := checkDir(t, flag, oldest)
		fixable := 0
		for _, f := range res.Findings {
			if f.Rule == r.ID && f.Fix != nil {
				fixable++
				if f.Fix.Description == "" || len(f.Fix.Edits) == 0 {
					t.Errorf("%s: a fix with no description or no edits", r.ID)
				}
			}
		}
		if fixable == 0 {
			t.Errorf("%s: no finding on its flag fixture carries a fix", r.ID)
			continue
		}
		fixedDir := fixture(t, r.ID, "fixed")
		err := filepath.WalkDir(flag, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(flag, p)
			src, _ := os.ReadFile(p)
			got, err := ApplyFixes(filepath.ToSlash(rel), src, findingsOf(res, r.ID))
			if err != nil {
				t.Errorf("%s: %v", r.ID, err)
				return nil
			}
			want, err := os.ReadFile(filepath.Join(fixedDir, rel))
			if err != nil {
				t.Errorf("%s: fixed/ has no %s", r.ID, rel)
				return nil
			}
			if string(got) != string(want) {
				t.Errorf("%s: fixing %s gave\n%s\nwant\n%s", r.ID, rel, got, want)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if n := count(checkDir(t, fixedDir, oldest), r.ID); n != 0 {
			t.Errorf("%s: the fixed fixture still has %d findings of the rule", r.ID, n)
		}
	}
}

func findingsOf(res *Result, rule string) []Finding {
	var out []Finding
	for _, f := range res.Findings {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

// Every finding names its rule's documentation, a release, and a remedy: what
// a customer reading one line of CI output needs.
func TestFindingsCarryRemedyAndLink(t *testing.T) {
	for _, r := range Rules {
		v := firstApplicable(r)
		res := checkDir(t, fixture(t, r.ID, "flag"), v)
		for _, f := range findingsOf(res, r.ID) {
			if f.Remedy == "" || f.Link != r.Link() || f.OpenTofuVersion != v || f.Start.Line == 0 {
				t.Errorf("%s: incomplete finding %+v", r.ID, f)
			}
		}
	}
}

func firstApplicable(r *Rule) string {
	for _, v := range Supported {
		if r.appliesTo(v) {
			return v
		}
	}
	return Supported[0]
}
