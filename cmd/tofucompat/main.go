// Command tofucompat checks Terraform configuration for constructs a given
// OpenTofu release cannot run (RUN-1385).
//
//	tofucompat [flags] [DIR ...]
//
// Exit status: 0 when no Error finding was made (warnings do not fail unless
// -strict), 1 when one was, 2 when the check could not run. A run that checked
// no file at all exits 2, not 0: "found nothing" and "read nothing" are
// different answers.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/runners-io/tflint-ruleset-tofucompat/compat"
	"github.com/runners-io/tflint-ruleset-tofucompat/internal/buildversion"
)

// version is stamped at release time with -ldflags "-X main.version=...".
// A `go install …@vX.Y.Z` build has no stamp and reads the module version
// instead (RUN-1410); see buildVersion.
var version = "dev"

// buildVersion is the version this binary reports.
func buildVersion() string { return buildversion.Resolve(version, "dev") }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tofucompat", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		target    = fs.String("opentofu-version", "latest", `OpenTofu release to check against: "latest", a line such as "1.10", or a full version`)
		format    = fs.String("format", "text", `output format: "text", "json" or "github" (GitHub Actions annotations)`)
		fix       = fs.Bool("fix", false, "apply every mechanical fix in place, then report what is left")
		strict    = fs.Bool("strict", false, "exit 1 on warnings too")
		disable   = fs.String("disable", "", "comma-separated rule ids to leave out")
		listRules = fs.Bool("list-rules", false, "print the rules that apply to -opentofu-version and exit")
		showVer   = fs.Bool("version", false, "print the version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: tofucompat [flags] [DIR ...]\n\nChecks Terraform configuration under each DIR (default .) for constructs the chosen\nOpenTofu release cannot run. Rules: %s\n\n", compat.DocsBase)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVer {
		fmt.Fprintf(stdout, "tofucompat %s (rules measured against OpenTofu %s to %s)\n", buildVersion(), compat.Supported[0], compat.Latest)
		return 0
	}
	if *listRules {
		rs, err := compat.RulesFor(*target)
		if err != nil {
			fmt.Fprintln(stderr, "tofucompat:", err)
			return 2
		}
		for _, r := range rs {
			since := "no OpenTofu release"
			if r.OpenTofuSince != "" {
				since = "OpenTofu " + r.OpenTofuSince
			}
			fixable := ""
			if r.HasFix() {
				fixable = " [fixable]"
			}
			fmt.Fprintf(stdout, "%-40s %-7s %s (Terraform %s; %s)%s\n", r.ID, r.Severity, r.Construct, r.TerraformSince, since, fixable)
		}
		return 0
	}
	var disabled []string
	for _, d := range strings.Split(*disable, ",") {
		if d = strings.TrimSpace(d); d != "" {
			disabled = append(disabled, d)
		}
	}
	dirs := fs.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	opts := compat.Options{OpenTofuVersion: *target, Disabled: disabled}

	type dirResult struct {
		Dir string `json:"dir"`
		*compat.Result
	}
	var all []dirResult
	for _, dir := range dirs {
		res, err := compat.CheckDir(dir, opts)
		if err != nil {
			fmt.Fprintln(stderr, "tofucompat:", err)
			return 2
		}
		if *fix && res.Fixable() > 0 {
			if err := applyFixes(dir, res); err != nil {
				fmt.Fprintln(stderr, "tofucompat:", err)
				return 2
			}
			fmt.Fprintf(stderr, "tofucompat: applied %d fix(es) under %s\n", res.Fixable(), dir)
			if res, err = compat.CheckDir(dir, opts); err != nil {
				fmt.Fprintln(stderr, "tofucompat:", err)
				return 2
			}
		}
		all = append(all, dirResult{Dir: dir, Result: res})
	}

	errorsFound, warnings, checked := 0, 0, 0
	for _, d := range all {
		errorsFound += d.Errors()
		warnings += len(d.Findings) - d.Errors()
		checked += d.FilesChecked
	}
	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(all)
	case "github":
		for _, d := range all {
			for _, f := range d.Findings {
				level := "error"
				if f.Severity == compat.Warning {
					level = "warning"
				}
				fmt.Fprintf(stdout, "::%s file=%s,line=%d,col=%d,endLine=%d,title=%s::%s %s%s\n",
					level, ghEscapeProp(filepath.ToSlash(filepath.Join(d.Dir, f.File))), f.Start.Line, f.Start.Column, f.End.Line,
					ghEscapeProp(f.Rule), ghEscape(f.Message), ghEscape("Fix: "+f.Remedy), ghEscape(fixNote(f)))
			}
		}
		fmt.Fprintln(stdout, summary(all[0].OpenTofuVersion, errorsFound, warnings, checked))
	case "text":
		for _, d := range all {
			for _, s := range d.Skipped {
				fmt.Fprintf(stderr, "skipped %s: %s\n", filepath.Join(d.Dir, s.File), s.Reason)
			}
			for _, f := range d.Findings {
				fmt.Fprintf(stdout, "%s:%d:%d: %s: %s [%s]\n  fix: %s%s\n  see: %s\n",
					filepath.Join(d.Dir, f.File), f.Start.Line, f.Start.Column, f.Severity, f.Message, f.Rule, f.Remedy, fixNote(f), f.Link)
			}
		}
		fmt.Fprintln(stdout, summary(all[0].OpenTofuVersion, errorsFound, warnings, checked))
	default:
		fmt.Fprintf(stderr, "tofucompat: unknown -format %q\n", *format)
		return 2
	}
	if checked == 0 {
		fmt.Fprintln(stderr, "tofucompat: no Terraform configuration files were checked")
		return 2
	}
	if errorsFound > 0 || (*strict && warnings > 0) {
		return 1
	}
	return 0
}

func summary(v string, errs, warns, files int) string {
	return fmt.Sprintf("OpenTofu %s: %d error(s), %d warning(s) in %d file(s)", v, errs, warns, files)
}

func fixNote(f compat.Finding) string {
	if f.Fix == nil {
		return ""
	}
	return " (run with -fix to " + f.Fix.Description + ")"
}

// applyFixes rewrites each file that has fixable findings. A file is written
// only when every one of its edits applies; ApplyFixes refuses overlaps.
func applyFixes(dir string, res *compat.Result) error {
	byFile := map[string][]compat.Finding{}
	for _, f := range res.Findings {
		if f.Fix != nil {
			for _, e := range f.Fix.Edits {
				byFile[e.File] = append(byFile[e.File], f)
			}
		}
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, name := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out, err := compat.ApplyFixes(name, src, dedupe(byFile[name]))
		if err != nil {
			return err
		}
		if err := os.WriteFile(p, out, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// dedupe drops repeats: a finding with edits in two files is listed under
// each, and twice under one if it has two edits there.
func dedupe(fs []compat.Finding) []compat.Finding {
	seen := map[string]bool{}
	var out []compat.Finding
	for _, f := range fs {
		k := fmt.Sprintf("%s:%s:%d", f.Rule, f.File, f.Start.Byte)
		if !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	return out
}

// GitHub workflow-command escaping
// (https://github.com/actions/toolkit/blob/main/packages/core/src/command.ts).
func ghEscape(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func ghEscapeProp(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}
