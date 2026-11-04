package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// --- install ----------------------------------------------------------------

// InstallOptions select what Install touches.
type InstallOptions struct {
	All    bool     // every project and every home in the manifest
	Name   string   // one project or home (ignored with All)
	Force  bool     // rewrite even unchanged (sha256-equal) files
	DryRun bool     // render and report, write nothing
	Target string   // claude|cursor|codex|both|all; "" keeps each project's targets
	What   []string // install only these bundles; a non-empty filter skips pruning
}

// ProjectResult is the per-file outcome for one project (or "home:<name>").
type ProjectResult struct {
	Project string       `json:"project"`
	Files   []FileStatus `json:"files"`
}

// Install renders and writes the selected projects and homes. Homes go first,
// then projects in name order; the first error stops the run.
func (e *Engine) Install(o InstallOptions) ([]ProjectResult, error) {
	m, err := e.LoadManifest()
	if err != nil {
		return nil, err
	}

	var names, homes []string
	if o.All {
		for n := range m.Projects {
			names = append(names, n)
		}
		sort.Strings(names)
		for n := range m.Homes {
			homes = append(homes, n)
		}
		sort.Strings(homes)
	} else if o.Name != "" {
		if _, ok := m.Homes[o.Name]; ok {
			homes = []string{o.Name}
		} else {
			names = []string{o.Name}
		}
	} else {
		return nil, errors.New("install: name a project or home, or pass --all")
	}

	var all []ProjectResult

	// homes take only --force/--dry-run: --target/--what are per-project knobs.
	globalDir := e.GlobalDir()
	for _, n := range homes {
		res, err := installHome(m.Homes[n], globalDir, o.Force, o.DryRun)
		if err != nil {
			return nil, err
		}
		all = append(all, ProjectResult{"home:" + n, res})
	}

	for _, n := range names {
		p, ok := m.Projects[n]
		if !ok {
			return nil, errors.New("no such project or home: " + n)
		}
		active := p.ActiveBundles()
		prune := true
		if len(o.What) > 0 {
			active = intersect(active, o.What)
			prune = false // partial install must not prune other bundles' artifacts
		}
		targets := p.Targets
		if o.Target != "" {
			if targets, err = targetList(o.Target); err != nil {
				return nil, err
			}
		}
		res, err := install(p, e, active, targets, o.Force, o.DryRun, prune, o.Target != "")
		if err != nil {
			return nil, err
		}
		all = append(all, ProjectResult{n, res})
	}
	return all, nil
}

// DryRun reports what a full install of p would do — the comparison behind
// `trafaret diff` and the operator site's sync status.
func (e *Engine) DryRun(p Project) ([]FileStatus, error) {
	return install(p, e, p.ActiveBundles(), p.Targets, false, true /* dry-run */, true /* prune report */, false /* full target set */)
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
// project and home, so token typos and frontmatter errors surface here and not
// at install time.
func (e *Engine) Validate(m *Manifest) Report {
	var rep Report
	report := func(proj, msg string) {
		rep.Problems = append(rep.Problems, Problem{proj, msg})
	}
	var names []string
	for n := range m.Projects {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := m.Projects[n]
		// required fields
		if p.RepoRoot == "" {
			report(n, "missing repoRoot")
		}
		for _, dn := range p.Disabled {
			if _, ok := e.Bundles[dn]; !ok {
				report(n, fmt.Sprintf("disabled lists unknown bundle %q", dn))
			}
			if !contains(p.Bundles, dn) {
				report(n, fmt.Sprintf("disabled bundle %q is not in bundles", dn))
			}
		}
		needs := func(block string) []string {
			var by []string
			for _, bn := range p.Bundles {
				if e.Bundles[bn].requires(block) {
					by = append(by, bn)
				}
			}
			return by
		}
		for _, bn := range p.Bundles {
			if _, ok := e.Bundles[bn]; !ok {
				report(n, fmt.Sprintf("bundle %q is not in templates/%s", bn, bundlesFile))
			}
		}
		if by := needs("db"); len(by) > 0 && (p.Tools.DB.CLI == "" || p.Tools.DB.Config == "" || p.Tools.DB.Tunnel == "") {
			report(n, fmt.Sprintf("bundle %s needs tools.db.cli, tools.db.config and tools.db.tunnel", strings.Join(by, ", ")))
		}
		if _, err := sessionContext(e.Bundles, p.ActiveBundles()); err != nil {
			report(n, err.Error())
		}
		// only bundles whose templates read the ticket protocol need it, so a repo
		// without a ticket board can still carry e.g. a read-only database skill.
		if len(needs("tickets")) > 0 {
			if p.Tickets.Config == "" {
				report(n, "missing tickets.config")
			}
			if p.Tickets.Space == "" {
				report(n, "missing tickets.space")
			}
			if (p.Tickets.DB == "") != (p.Tickets.Project == "") {
				report(n, "tickets.db and tickets.project go together (Work mode)")
			}
			if p.Tickets.DB != "" && !strings.HasPrefix(p.Tickets.DB, p.Tickets.Space+"/") {
				report(n, fmt.Sprintf("tickets.db %q must be in tickets.space %q", p.Tickets.DB, p.Tickets.Space))
			}
			if _, ok := runProfiles[p.Run.Profile]; !ok {
				report(n, fmt.Sprintf("run.profile %q is not one of careful|balanced|fast", p.Run.Profile))
			}
			if p.Run.Order != "priority" && p.Run.Order != "cheap-first" {
				report(n, fmt.Sprintf("run.order %q is not one of priority|cheap-first", p.Run.Order))
			}
			if p.Run.MaxRounds < 1 || p.Run.MaxRounds > 5 {
				report(n, fmt.Sprintf("run.maxRounds %d is outside 1..5", p.Run.MaxRounds))
			}
		} else if len(p.Bundles) == 0 {
			if _, err := os.Stat(filepath.Join(e.ProjectsDir(), n)); err != nil {
				report(n, "no bundles and no projects/"+n+" sources — nothing to install")
			}
		}
		// render every active template to catch token/parse errors
		if _, err := render(p, e, p.Bundles, p.Targets); err != nil {
			report(n, "render error: "+err.Error())
		}
	}
	var homeNames []string
	for n := range m.Homes {
		homeNames = append(homeNames, n)
	}
	sort.Strings(homeNames)
	globalDir := e.GlobalDir()
	for _, n := range homeNames {
		if _, ok := m.Projects[n]; ok {
			report("home:"+n, "name collides with a project — `trafaret install "+n+"` would be ambiguous")
		}
		if _, err := renderHome(m.Homes[n], globalDir); err != nil {
			report("home:"+n, "render error: "+err.Error())
		}
	}
	rep.Projects, rep.Homes = len(names), len(homeNames)
	return rep
}
