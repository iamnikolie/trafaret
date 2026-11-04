package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// --- install ----------------------------------------------------------------

// InstallOptions select what Install touches.
type InstallOptions struct {
	All    bool     // every project and every home in the manifest
	Name   string   // one project or home (ignored with All)
	Force  bool     // rewrite even unchanged (sha256-equal) files
	DryRun bool     // render and report, write nothing
	Target string   // claude|cursor|codex|both|all; "" keeps each project's harnesses
	What   []string // install only these bundles; a non-empty filter skips pruning
}

// ProjectResult is the per-file outcome for one project (or "home:<name>").
type ProjectResult struct {
	Project string       `json:"project"`
	Files   []FileStatus `json:"files"`
}

// Install renders and writes the selected projects and homes. Homes go first,
// then projects in name order; the first error stops the run.
//
// --target overrides a project's harnesses for the run and selects the homes
// whose harness it names (a home belongs to exactly one harness); --what
// installs only the named bundles' contents and skips pruning.
func (e *Engine) Install(o InstallOptions) ([]ProjectResult, error) {
	m, err := e.LoadManifest()
	if err != nil {
		return nil, err
	}
	ns, err := e.Namespace()
	if err != nil {
		return nil, err
	}

	var names, homes []string
	if o.All {
		names, homes = sortedKeys(m.Projects), sortedKeys(m.Homes)
	} else if o.Name != "" {
		if _, ok := m.Homes[o.Name]; ok {
			homes = []string{o.Name}
		} else {
			names = []string{o.Name}
		}
	} else {
		return nil, errors.New("install: name a project or home, or pass --all")
	}

	var targets []string
	if o.Target != "" {
		if targets, err = targetList(o.Target); err != nil {
			return nil, err
		}
	}
	filter := func(bundles []string) (active []string, direct, prune bool) {
		if len(o.What) > 0 {
			return intersect(bundles, o.What), false, false // a partial install must not prune other bundles' artifacts
		}
		return bundles, true, true
	}

	var all []ProjectResult
	for _, n := range homes {
		h := m.Homes[n]
		if targets != nil && !contains(targets, h.Harness) {
			continue
		}
		active, direct, prune := filter(h.Bundles)
		res, err := e.installHome(ns, h, active, direct, o.Force, o.DryRun, prune)
		if err != nil {
			return nil, fmt.Errorf("home %s: %w", n, err)
		}
		all = append(all, ProjectResult{"home:" + n, res})
	}

	for _, n := range names {
		p, ok := m.Projects[n]
		if !ok {
			return nil, errors.New("no such project or home: " + n)
		}
		active, direct, prune := filter(p.ActiveBundles())
		harnesses := p.Harnesses
		if targets != nil {
			harnesses = targets
		}
		res, err := e.installProject(ns, p, active, harnesses, direct, o.Force, o.DryRun, prune, targets != nil)
		if err != nil {
			return nil, fmt.Errorf("project %s: %w", n, err)
		}
		all = append(all, ProjectResult{n, res})
	}
	return all, nil
}

// DryRunProject reports what a full install of p would do — the comparison
// behind `trafaret diff` and the operator site's sync status.
func (e *Engine) DryRunProject(p Project) ([]FileStatus, error) {
	ns, err := e.Namespace()
	if err != nil {
		return nil, err
	}
	return e.installProject(ns, p, p.ActiveBundles(), p.Harnesses, true, false, true /* dry-run */, true /* prune report */, false /* full harness set */)
}

// DryRunHome is DryRunProject for a home.
func (e *Engine) DryRunHome(h Home) ([]FileStatus, error) {
	ns, err := e.Namespace()
	if err != nil {
		return nil, err
	}
	return e.installHome(ns, h, h.Bundles, true, false, true, true)
}

// --- validate ---------------------------------------------------------------

// Problem is one finding of Validate; Where is a project name or "home:<name>".
type Problem struct {
	Where string
	Msg   string
}

// Report is the result of Validate.
type Report struct {
	Problems []Problem
	Projects int // projects checked
	Homes    int // homes checked
}

// Validate checks the manifest against the template set and renders every
// project and home, so template typos, missing vars and frontmatter errors
// surface here and not at install time.
func (e *Engine) Validate(m *Manifest) Report {
	var rep Report
	report := func(where, msg string) {
		rep.Problems = append(rep.Problems, Problem{where, msg})
	}

	// the sources themselves
	if _, err := fs.Stat(e.Templates, "cursor"); err == nil {
		report("templates", "templates/cursor/ is the 0.2 layout; Cursor rules now live in templates/rules/<name>.md[.tmpl] (and bundles list them under rules:)")
	}
	ns, err := e.Namespace()
	if err != nil {
		report("templates", err.Error())
		ns = nil
	}
	if ns != nil {
		for _, bn := range sortedKeys(e.Bundles) {
			b := e.Bundles[bn]
			if err := ns.check(itemSet{skills: b.Skills, agents: b.Agents, rules: b.Rules}); err != nil {
				report("bundle "+bn, err.Error())
			}
		}
	}

	names := sortedKeys(m.Projects)
	homeNames := sortedKeys(m.Homes)

	// requires: every var path must exist in the merged vars of the carrier
	checkRequires := func(where string, bundles []string, vars map[string]any) {
		for _, bn := range bundles {
			for _, req := range e.Bundles[bn].Requires {
				if !hasPath(vars, req) {
					report(where, fmt.Sprintf("bundle %s requires var %q, which is not set (vars: / defaults.vars / the entry's own vars:)", bn, req))
				}
			}
		}
	}

	for _, n := range names {
		p := m.Projects[n]
		if p.RepoRoot == "" {
			report(n, "missing repoRoot")
		}
		if err := checkHarnesses(p.Harnesses); err != nil {
			report(n, err.Error())
		}
		if len(p.Harnesses) == 0 {
			report(n, "no harnesses (set harnesses: or defaults.harnesses)")
		}
		for _, dn := range p.Disabled {
			if _, ok := e.Bundles[dn]; !ok {
				report(n, fmt.Sprintf("disabled lists unknown bundle %q", dn))
			}
			if !contains(p.Bundles, dn) {
				report(n, fmt.Sprintf("disabled bundle %q is not in bundles", dn))
			}
		}
		unknownBundle := false
		for _, bn := range p.Bundles {
			if _, ok := e.Bundles[bn]; !ok {
				report(n, fmt.Sprintf("bundle %q is not in templates/%s", bn, bundlesFile))
				unknownBundle = true
			}
		}
		checkRequires(n, p.Bundles, p.Vars)
		if _, err := sessionContext(e.Bundles, p.ActiveBundles()); err != nil {
			report(n, err.Error())
		}
		if len(p.Bundles) == 0 && len(p.Skills) == 0 && len(p.Agents) == 0 && len(p.Rules) == 0 {
			if _, err := os.Stat(filepath.Join(e.ProjectsDir(), n)); err != nil {
				report(n, "no bundles, skills, agents or rules and no projects/"+n+" sources — nothing to install")
			}
		}
		if ns == nil || unknownBundle {
			continue
		}
		// render everything the project declares to catch template and var errors
		it, err := e.projectItems(ns, p, p.Bundles, true)
		if err != nil {
			report(n, err.Error())
			continue
		}
		if _, err := ns.render(e.projectEntity(p, p.Harnesses), it); err != nil {
			report(n, "render error: "+err.Error())
		}
	}

	for _, n := range homeNames {
		h := m.Homes[n]
		where := "home:" + n
		if _, ok := m.Projects[n]; ok {
			report(where, "name collides with a project — `trafaret install "+n+"` would be ambiguous")
		}
		if _, ok := findAdapter(h.Harness); !ok {
			report(where, fmt.Sprintf("harness %q is not one of %s", h.Harness, strings.Join(adapterNames(), "|")))
			continue
		}
		if h.Dir == "" || !filepath.IsAbs(h.Dir) {
			report(where, fmt.Sprintf("dir must be an absolute or ~/ path, got %q", h.Dir))
			continue
		}
		unknownBundle := false
		for _, bn := range h.Bundles {
			if _, ok := e.Bundles[bn]; !ok {
				report(where, fmt.Sprintf("bundle %q is not in templates/%s", bn, bundlesFile))
				unknownBundle = true
			}
		}
		checkRequires(where, h.Bundles, h.Vars)
		if ns == nil || unknownBundle {
			continue
		}
		it, err := e.homeItems(ns, h, h.Bundles, true)
		if err != nil {
			report(where, err.Error())
			continue
		}
		if _, err := ns.render(homeEntity(h), it); err != nil {
			report(where, "render error: "+err.Error())
		}
	}
	rep.Projects, rep.Homes = len(names), len(homeNames)
	return rep
}
