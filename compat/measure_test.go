package compat

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// THE ARRIVAL VERSIONS ARE MEASURED, NOT READ.
//
// Changelogs and migration guides disagree with each other and with the
// binaries -- RUN-1385's research found two places where OpenTofu's own 1.8/1.9
// migration guides are stale, and one (provider::terraform:: functions) where
// the changelog's version is not when the function became usable in a
// Terraform-authored module. So each rule's must-flag fixture is run through a
// real OpenTofu binary of every supported line, and the outcome is recorded in
// testdata/matrix.json. TestRulesAgreeWithTheMeasuredMatrix (always run, no
// binaries needed) then holds the rule table to that record.
//
// Re-measure (OpenTofu only; this never runs a terraform binary):
//
//	TOFUCOMPAT_TOFU_BINS=<dir holding <version>/tofu for every Supported version> \
//	  go test ./compat/ -run TestMeasureWithRealOpenTofu -update-matrix -timeout 60m
//
// It downloads providers from registry.opentofu.org for the few fixtures that
// declare one, and its s3/cloud fixtures make one refused request each to AWS
// and app.terraform.io with no credentials.

var updateMatrix = flag.Bool("update-matrix", false, "rewrite testdata/matrix.json from TestMeasureWithRealOpenTofu")

const matrixFile = "testdata/matrix.json"

// Outcome is what one binary did with one fixture.
type outcome string

const (
	refused  outcome = "refused"  // the rule's refusal diagnostic appeared
	accepted outcome = "accepted" // every command in the phase succeeded (init: no refusal)
)

type measuredRule struct {
	Phase string             `json:"phase"`
	Flag  map[string]outcome `json:"flag"`
	Clean map[string]outcome `json:"clean"`
}

type matrix struct {
	MeasuredAt string                   `json:"measured_at"`
	Versions   []string                 `json:"versions"`
	Rules      map[string]*measuredRule `json:"rules"`
}

func loadMatrix(t *testing.T) *matrix {
	t.Helper()
	b, err := os.ReadFile(matrixFile)
	if err != nil {
		t.Fatalf("reading %s: %v", matrixFile, err)
	}
	var m matrix
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parsing %s: %v", matrixFile, err)
	}
	return &m
}

// measuredSince is the first version from which the flag fixture is accepted
// on that version and every later one, or "" if the newest still refuses it.
// A refusal AFTER an acceptance is an error: arrival is monotonic, and a
// fixture that flips back is measuring something else.
func measuredSince(versions []string, flag map[string]outcome) (string, error) {
	for _, v := range versions {
		if o := flag[v]; o != refused && o != accepted {
			return "", fmt.Errorf("version %s: outcome %q", v, o)
		}
	}
	idx := len(versions)
	for idx > 0 && flag[versions[idx-1]] == accepted {
		idx--
	}
	for _, v := range versions[:idx] {
		if flag[v] != refused {
			return "", fmt.Errorf("accepted on %s, then refused on a later release", v)
		}
	}
	if idx == len(versions) {
		return "", nil
	}
	return versions[idx], nil
}

func sameLine(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	va, _ := parseVersion(a)
	vb, _ := parseVersion(b)
	return va[0] == vb[0] && va[1] == vb[1]
}

func TestRulesAgreeWithTheMeasuredMatrix(t *testing.T) {
	m := loadMatrix(t)
	if strings.Join(m.Versions, ",") != strings.Join(Supported, ",") {
		t.Fatalf("matrix measured %v, Supported is %v: re-measure", m.Versions, Supported)
	}
	measured := 0
	for _, r := range Rules {
		mr, ok := m.Rules[r.ID]
		if r.measure.unmeasured != "" {
			if ok {
				t.Errorf("%s says it is unmeasured but the matrix has it", r.ID)
			}
			continue
		}
		if !ok {
			t.Errorf("%s is not in %s: re-measure", r.ID, matrixFile)
			continue
		}
		measured++
		for _, v := range m.Versions {
			if mr.Clean[v] != accepted {
				t.Errorf("%s: clean fixture was %q on OpenTofu %s", r.ID, mr.Clean[v], v)
			}
		}
		if r.measure.none {
			for _, v := range m.Versions {
				if !r.appliesTo(v) {
					continue
				}
				if mr.Flag[v] != accepted {
					t.Errorf("%s: OpenTofu %s diagnosed a construct the rule says it ignores (%q)", r.ID, v, mr.Flag[v])
				}
			}
			continue
		}
		var judged []string
		for _, v := range m.Versions {
			if r.appliesFrom == "" || supportedIn(r.appliesFrom, v) {
				judged = append(judged, v)
			}
		}
		since, err := measuredSince(judged, mr.Flag)
		if err != nil {
			t.Errorf("%s: %v", r.ID, err)
			continue
		}
		if !sameLine(since, r.OpenTofuSince) {
			t.Errorf("%s: OpenTofuSince is %q but the binaries first accept it on %q", r.ID, r.OpenTofuSince, since)
		}
	}
	for id := range m.Rules {
		if RuleByID(id) == nil {
			t.Errorf("matrix holds %s, which is no rule", id)
		}
	}
	if measured < 20 {
		t.Errorf("only %d rules measured", measured)
	}
}

// A control for the agreement test: a matrix that disagrees with a rule must
// be caught, or the test above could be passing by comparing nothing.
func TestMeasuredSinceDetectsArrivalAndNonMonotonicity(t *testing.T) {
	vs := []string{"1.6.3", "1.7.10", "1.8.11"}
	got, err := measuredSince(vs, map[string]outcome{"1.6.3": refused, "1.7.10": accepted, "1.8.11": accepted})
	if err != nil || got != "1.7.10" {
		t.Errorf("got %q, %v; want 1.7.10", got, err)
	}
	got, err = measuredSince(vs, map[string]outcome{"1.6.3": refused, "1.7.10": refused, "1.8.11": refused})
	if err != nil || got != "" {
		t.Errorf("got %q, %v; want none", got, err)
	}
	if _, err := measuredSince(vs, map[string]outcome{"1.6.3": accepted, "1.7.10": refused, "1.8.11": accepted}); err == nil {
		t.Error("a non-monotonic arrival was accepted")
	}
	if _, err := measuredSince(vs, map[string]outcome{"1.6.3": accepted, "1.7.10": refused, "1.8.11": refused}); err == nil {
		t.Error("an acceptance followed by refusals was accepted")
	}
	if _, err := measuredSince(vs, map[string]outcome{"1.6.3": "failed: network", "1.7.10": accepted, "1.8.11": accepted}); err == nil {
		t.Error("an unexplained failure was accepted")
	}
	if sameLine("1.13.0", "1.12.7") || !sameLine("1.13.0", "1.13.1") || sameLine("", "1.6.3") {
		t.Error("sameLine")
	}
}

func TestMeasureWithRealOpenTofu(t *testing.T) {
	bins := os.Getenv("TOFUCOMPAT_TOFU_BINS")
	if bins == "" {
		t.Skip("set TOFUCOMPAT_TOFU_BINS to measure against real OpenTofu binaries")
	}
	cache := t.TempDir()
	m := &matrix{MeasuredAt: time.Now().UTC().Format(time.RFC3339), Versions: Supported, Rules: map[string]*measuredRule{}}
	for _, r := range Rules {
		if r.measure.unmeasured != "" {
			continue
		}
		phase := r.measure.phase
		if r.measure.none {
			phase = "validate"
		}
		mr := &measuredRule{Phase: phase, Flag: map[string]outcome{}, Clean: map[string]outcome{}}
		m.Rules[r.ID] = mr
		var refusal *regexp.Regexp
		if r.measure.refusal != "" {
			refusal = regexp.MustCompile(r.measure.refusal)
		}
		for _, v := range Supported {
			bin := filepath.Join(bins, v, "tofu")
			mr.Flag[v] = runTofu(t, bin, phase, fixture(t, r.ID, "flag"), refusal, cache)
			mr.Clean[v] = runTofu(t, bin, phase, fixture(t, r.ID, "clean"), refusal, cache)
			t.Logf("%-40s %-8s flag=%s clean=%s", r.ID, v, mr.Flag[v], mr.Clean[v])
		}
	}
	if *updateMatrix {
		b, _ := json.MarshalIndent(m, "", "  ")
		if err := os.WriteFile(matrixFile, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// runTofu runs one fixture through one OpenTofu binary in a scratch copy.
func runTofu(t *testing.T, bin, phase, dir string, refusal *regexp.Regexp, cache string) outcome {
	t.Helper()
	work := t.TempDir()
	if err := os.CopyFS(work, os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	var steps [][]string
	switch phase {
	case "validate":
		steps = [][]string{{"init", "-backend=false"}, {"validate"}}
	case "plan":
		steps = [][]string{{"init", "-backend=false"}, {"validate"}, {"plan", "-lock=false"}}
	case "init":
		steps = [][]string{{"init"}}
	case "test":
		steps = [][]string{{"init", "-backend=false"}, {"test"}}
	default:
		t.Fatalf("unknown phase %q", phase)
	}
	var out bytes.Buffer
	for _, s := range steps {
		cmd := exec.Command(bin, append(s, "-no-color")...)
		cmd.Dir = work
		cmd.Env = []string{
			"HOME=" + work, "PATH=/usr/bin:/bin",
			"TF_IN_AUTOMATION=1", "TF_INPUT=0", "CHECKPOINT_DISABLE=1",
			"TF_PLUGIN_CACHE_DIR=" + cache,
			// The s3 fixtures need SOME credentials to get past the SDK's
			// own check to the argument decoding being measured.
			"AWS_ACCESS_KEY_ID=measure", "AWS_SECRET_ACCESS_KEY=measure", "AWS_EC2_METADATA_DISABLED=true",
		}
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		if refusal != nil && refusal.Match(out.Bytes()) {
			return refused
		}
		if err != nil {
			if phase == "init" {
				// init reaches the network for a backend; anything other than
				// the refusal means the construct itself was accepted.
				return accepted
			}
			first := out.String()
			if i := strings.Index(first, "Error:"); i >= 0 {
				first = first[i:]
			}
			if len(first) > 160 {
				first = first[:160]
			}
			return outcome("failed: " + strings.ReplaceAll(first, "\n", " "))
		}
	}
	return accepted
}
