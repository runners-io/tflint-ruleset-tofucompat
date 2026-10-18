package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rules = "../../compat/testdata/rules"

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestExitCodes(t *testing.T) {
	if code, out, _ := runCLI(t, "-opentofu-version", "1.12", rules+"/tofucompat_convert_function/flag"); code != 1 || !strings.Contains(out, "tofucompat_convert_function") || !strings.Contains(out, "-fix") {
		t.Errorf("flag fixture: exit %d, %s", code, out)
	}
	if code, out, _ := runCLI(t, rules+"/tofucompat_convert_function/flag"); code != 0 {
		t.Errorf("latest supports convert: exit %d, %s", code, out)
	}
	if code, out, _ := runCLI(t, "-opentofu-version", "1.6", rules+"/tofucompat_action_block/clean"); code != 0 {
		t.Errorf("clean fixture: exit %d, %s", code, out)
	}
	// Warnings pass unless -strict.
	q := rules + "/tofucompat_query_file/flag"
	if code, _, _ := runCLI(t, q); code != 0 {
		t.Errorf("warning-only: exit %d", code)
	}
	if code, _, _ := runCLI(t, "-strict", q); code != 1 {
		t.Errorf("warning-only with -strict: exit %d", code)
	}
	// Reading nothing is not passing.
	if code, _, stderr := runCLI(t, t.TempDir()); code != 2 || !strings.Contains(stderr, "no Terraform configuration") {
		t.Errorf("empty dir: exit %d, %s", code, stderr)
	}
	if code, _, _ := runCLI(t, "-opentofu-version", "9.9", "."); code != 2 {
		t.Errorf("unknown version: exit %d", code)
	}
}

func TestJSONIsMachineReadable(t *testing.T) {
	code, out, _ := runCLI(t, "-format", "json", "-opentofu-version", "1.9", rules+"/tofucompat_deprecated_argument/flag")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	var got []struct {
		Dir             string `json:"dir"`
		OpenTofuVersion string `json:"opentofu_version"`
		Findings        []struct {
			Rule string `json:"rule"`
			Fix  *struct {
				Edits []struct {
					NewText string `json:"new_text"`
				} `json:"edits"`
			} `json:"fix"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(got) != 1 || got[0].OpenTofuVersion != "1.9.4" || len(got[0].Findings) != 2 || got[0].Findings[0].Fix == nil {
		t.Errorf("%+v", got)
	}
}

func TestGitHubAnnotations(t *testing.T) {
	_, out, _ := runCLI(t, "-format", "github", "-opentofu-version", "1.10", rules+"/tofucompat_ephemeral_variable/flag")
	if !strings.HasPrefix(out, "::error file=") || !strings.Contains(out, "title=tofucompat_ephemeral_variable") {
		t.Errorf("%s", out)
	}
}

func TestFixRewritesInPlace(t *testing.T) {
	dir := t.TempDir()
	src, _ := os.ReadFile(rules + "/tofucompat_output_type/flag/main.tf")
	want, _ := os.ReadFile(rules + "/tofucompat_output_type/fixed/main.tf")
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := runCLI(t, "-fix", dir); code != 0 {
		t.Errorf("after fixing, exit %d: %s", code, out)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "main.tf"))
	if string(got) != string(want) {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestListRules(t *testing.T) {
	_, latest, _ := runCLI(t, "-list-rules")
	_, old, _ := runCLI(t, "-list-rules", "-opentofu-version", "1.6")
	if strings.Contains(latest, "tofucompat_convert_function") || !strings.Contains(old, "tofucompat_convert_function") {
		t.Errorf("latest:\n%s\n1.6:\n%s", latest, old)
	}
}
