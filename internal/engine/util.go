package engine

import (
	"fmt"
	"strings"
)

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// subtract returns a minus the members of b, preserving order.
func subtract(a, b []string) []string {
	drop := map[string]bool{}
	for _, x := range b {
		drop[x] = true
	}
	var out []string
	for _, x := range a {
		if !drop[x] {
			out = append(out, x)
		}
	}
	return out
}

func intersect(a, b []string) []string {
	var out []string
	for _, x := range a {
		if contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

// targetList expands a --target value into harness names.
func targetList(t string) ([]string, error) {
	switch t {
	case "both":
		return []string{"claude", "cursor"}, nil
	case "all":
		return []string{"claude", "cursor", "codex"}, nil
	case "claude":
		return []string{"claude"}, nil
	case "cursor":
		return []string{"cursor"}, nil
	case "codex":
		return []string{"codex"}, nil
	default:
		return nil, fmt.Errorf("invalid --target (claude|cursor|codex|both|all): %s", t)
	}
}

// ActiveBundles is the project's bundles minus the disabled ones, in order.
func (p Project) ActiveBundles() []string { return subtract(p.Bundles, p.Disabled) }

// BundlesDisplay renders a project's bundle list with disabled ones marked.
func (p Project) BundlesDisplay() string {
	disabled := map[string]bool{}
	for _, d := range p.Disabled {
		disabled[d] = true
	}
	var out []string
	for _, b := range p.Bundles {
		if disabled[b] {
			b += "(off)"
		}
		out = append(out, b)
	}
	return strings.Join(out, ", ")
}
