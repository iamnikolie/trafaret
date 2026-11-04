package engine

import (
	"strings"
	"testing"
)

// --- engine version gate ---------------------------------------------------

func TestCheckEngine(t *testing.T) {
	cases := []struct {
		constraint, ver string
		ok              bool
	}{
		{"", "0.1.0", true},         // absent field: no check
		{"  ", "0.1.0", true},       // blank: no check
		{">=0.2.0", "dev", true},    // dev always passes
		{">=99.0.0", "dev", true},   // ... whatever the constraint
		{">=0.2.0", "0.2.0", true},  // boundary
		{">=0.2.0", "0.1.9", false}, // too old
		{">=0.2.0", "v0.3.1", true}, // v prefix on the binary
		{">=v0.2.0", "0.3.1", true}, // v prefix on the constraint
		{">=0.2", "0.2.0", true},    // X.Y means X.Y.0
		{">0.2.0", "0.2.0", false},  // strict
		{">0.2.0", "0.2.1", true},
		{"<1.0.0", "0.9.9", true},
		{"<1.0.0", "1.0.0", false},
		{"<=1.0.0", "1.0.0", true},
		{"0.2.1", "0.2.1", true}, // exact
		{"0.2.1", "0.2.2", false},
		{"=0.2.1", "0.2.1", true},
		{"==0.2.1", "0.2.0", false},
		{">=0.2.0, <1.0.0", "0.5.0", true}, // range
		{">=0.2.0, <1.0.0", "1.2.0", false},
		{">= 0.2.0", "0.2.0", true},    // space after the operator
		{">=0.2.0", "0.3.0-rc1", true}, // pre-release suffix ignored
		{">=0.2.0", "0.3.0-4-gabc123-dirty", true},
		{">=0.2.0", "abc1234", true}, // bare commit hash: not a version, treated as dev
		{">=1.0.0", "0.9.0+meta", false},
	}
	for _, c := range cases {
		err := checkEngine(c.constraint, c.ver)
		if c.ok && err != nil {
			t.Errorf("checkEngine(%q, %q) = %v, want pass", c.constraint, c.ver, err)
		}
		if !c.ok && err == nil {
			t.Errorf("checkEngine(%q, %q) passed, want an error", c.constraint, c.ver)
		}
	}
}

func TestCheckEngineErrorIsClear(t *testing.T) {
	err := checkEngine(">=2.0.0", "1.4.0")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{">=2.0.0", "1.4.0", "trafaret", manifestName} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestCheckEngineRejectsMalformedConstraint(t *testing.T) {
	for _, c := range []string{"banana", ">=", "^1.2.3", ">=1.x", "1.2.3.4", ">=1,"} {
		if err := checkEngine(c, "1.0.0"); err == nil {
			t.Errorf("malformed constraint %q must be an error", c)
		}
		// ... even for a dev build: a typo must not silently disable the gate
		if err := checkEngine(c, "dev"); err == nil {
			t.Errorf("malformed constraint %q must be an error on dev builds too", c)
		}
	}
}
