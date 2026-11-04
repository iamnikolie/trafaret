package engine

import "strings"

// ProjectSkills lists the skills a project installs, from its own skills: list
// and its active bundles — what `list` and the operator site show. (Agents and
// rules are not included.)
func (e *Engine) ProjectSkills(p Project) []string {
	it, _ := collectItems(e.Bundles, itemSet{skills: p.Skills}, p.ActiveBundles())
	return it.skills
}

// HomeSkills describes the skills of a home: "all" when skills: is omitted,
// else the listed and bundled names, with the skipped ones noted.
func (e *Engine) HomeSkills(h Home) string {
	var s string
	if h.Skills == nil {
		s = "all"
	} else {
		it, _ := collectItems(e.Bundles, itemSet{skills: h.Skills}, h.Bundles)
		s = strings.Join(it.skills, ",")
		if s == "" {
			s = "-"
		}
	}
	if len(h.Skip) > 0 {
		s += " (skip " + strings.Join(h.Skip, ",") + ")"
	}
	return s
}
