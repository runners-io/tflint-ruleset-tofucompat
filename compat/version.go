package compat

import (
	"fmt"
	"strconv"
	"strings"
)

// Supported lists the OpenTofu minor lines a rule set exists for, oldest
// first, each with the newest patch release the rules were MEASURED against
// (testdata/matrix.json, produced by `go test -run TestMeasuredMatrix` with
// real OpenTofu binaries -- see measure_test.go).
//
// A rule set for a version is not a hand-written list. It is every rule whose
// OpenTofuSince is later than that version, so adding a release is: append it
// here, re-measure, and let the matrix test say which rules' arrival versions
// were wrong.
var Supported = []string{
	"1.6.3",
	"1.7.10",
	"1.8.11",
	"1.9.4",
	"1.10.10",
	"1.11.14",
	"1.12.7",
	"1.13.2",
}

// Latest is the newest OpenTofu release the rules know about, and the default
// when no version is asked for. A customer who does not choose is checked
// against the release we want them on.
var Latest = Supported[len(Supported)-1]

type version [3]int

func parseVersion(s string) (version, error) {
	var v version
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return v, fmt.Errorf("%q is not an OpenTofu version: want MAJOR.MINOR or MAJOR.MINOR.PATCH", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, fmt.Errorf("%q is not an OpenTofu version: %q is not a number", s, p)
		}
		v[i] = n
	}
	return v, nil
}

func (v version) less(o version) bool {
	for i := range v {
		if v[i] != o[i] {
			return v[i] < o[i]
		}
	}
	return false
}

func (v version) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

// ResolveVersion turns what a customer asked for into the version the rules
// are evaluated against.
//
//   - "" and "latest" mean Latest.
//   - "1.10" means the newest supported patch of 1.10, so a customer can name a
//     line without knowing its patch level.
//   - A full version is used as given, provided it is at least 1.6.0
//     (OpenTofu's first release).
//
// A version NEWER than Latest is refused rather than silently treated as
// Latest. Every rule with an arrival version would then be judged against a
// release nobody measured, and the honest answer for a release we have not
// measured is "update the tool", not a confident report.
func ResolveVersion(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "latest") {
		return Latest, nil
	}
	v, err := parseVersion(s)
	if err != nil {
		return "", err
	}
	if v.less(version{1, 6, 0}) {
		return "", fmt.Errorf("OpenTofu %s does not exist: the first release is 1.6.0", s)
	}
	latest, _ := parseVersion(Latest)
	if strings.Count(strings.TrimPrefix(s, "v"), ".") == 1 {
		for i := len(Supported) - 1; i >= 0; i-- {
			sv, _ := parseVersion(Supported[i])
			if sv[0] == v[0] && sv[1] == v[1] {
				return Supported[i], nil
			}
		}
		return "", fmt.Errorf("OpenTofu %s is not a line these rules know; known lines are 1.6 to %d.%d", s, latest[0], latest[1])
	}
	if latest.less(v) {
		return "", fmt.Errorf("OpenTofu %s is newer than %s, the newest release these rules were measured against; update tofucompat", v, Latest)
	}
	return v.String(), nil
}

// supportedIn reports whether a construct that arrived in OpenTofu `since`
// is available in `target`. An empty since means no OpenTofu release has it.
func supportedIn(since, target string) bool {
	if since == "" {
		return false
	}
	s, err := parseVersion(since)
	if err != nil {
		panic(fmt.Sprintf("rule table: bad OpenTofuSince %q: %v", since, err))
	}
	t, err := parseVersion(target)
	if err != nil {
		panic(fmt.Sprintf("bad target version %q: %v", target, err))
	}
	return !t.less(s)
}
