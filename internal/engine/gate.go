package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// --- engine version gate ----------------------------------------------------

// checkEngine enforces the manifest's optional `engine:` field against the
// binary's version. An empty constraint means no check; an unstamped ("dev") or
// otherwise non-numeric binary version always passes. Accepted forms: an exact
// version ("0.2.1", "v0.2.1") or one or more comma-separated comparisons
// (">=0.2.0", ">0.2", "<1.0.0", "<=0.9", "=0.2.1"; "X.Y" means X.Y.0). A
// pre-release or build suffix on either side ("-rc1", "+meta") is ignored.
// A constraint this binary cannot parse is an error: a typo must not silently
// disable the gate.
func checkEngine(constraint, ver string) error {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return nil
	}
	reqs, err := parseConstraint(constraint)
	if err != nil {
		return fmt.Errorf("manifest engine %q: %w", constraint, err)
	}
	have, ok := parseVersion(ver)
	if !ok {
		return nil // dev / unstamped build
	}
	for _, r := range reqs {
		if !r.holds(have) {
			return fmt.Errorf("manifest requires trafaret %s, this binary is %s — upgrade trafaret (or relax engine: in %s)", constraint, ver, manifestName)
		}
	}
	return nil
}

type semver [3]int

func (a semver) cmp(b semver) int {
	for i := range a {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}

type requirement struct {
	op  string // >=, >, <=, <, =
	ver semver
}

func (r requirement) holds(have semver) bool {
	c := have.cmp(r.ver)
	switch r.op {
	case ">=":
		return c >= 0
	case ">":
		return c > 0
	case "<=":
		return c <= 0
	case "<":
		return c < 0
	default:
		return c == 0
	}
}

func parseConstraint(s string) ([]requirement, error) {
	var out []requirement
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		op := "="
		for _, cand := range []string{">=", "<=", "==", ">", "<", "="} {
			if strings.HasPrefix(part, cand) {
				op, part = cand, strings.TrimSpace(strings.TrimPrefix(part, cand))
				break
			}
		}
		if op == "==" {
			op = "="
		}
		v, ok := parseVersion(part)
		if !ok {
			return nil, fmt.Errorf("%q is not a version like X.Y.Z (operators: >= > <= < =)", part)
		}
		out = append(out, requirement{op: op, ver: v})
	}
	return out, nil
}

// parseVersion reads "X.Y.Z" or "X.Y" (optionally "v"-prefixed, optionally with
// a -prerelease / +build suffix). Anything else — "dev", a bare commit hash —
// is not a version.
func parseVersion(s string) (semver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return semver{}, false
	}
	var v semver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}, false
		}
		v[i] = n
	}
	return v, true
}
