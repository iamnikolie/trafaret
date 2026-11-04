package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// --- manifest types -------------------------------------------------------

// Manifest is manifest.yaml. The engine knows harness layouts and nothing else:
// every project-specific fact (a ticket board, a gate list, a docs layout) is a
// free value under vars:, read by templates as .Vars.
type Manifest struct {
	// Engine is an optional version constraint on the trafaret binary, e.g.
	// ">=0.3.0" or "0.3.1". Empty = no check. See checkEngine.
	Engine   string             `yaml:"engine"`
	Vars     map[string]any     `yaml:"vars"` // config-wide vars, the lowest layer
	Defaults Defaults           `yaml:"defaults"`
	Projects map[string]Project `yaml:"projects"`
	Homes    map[string]Home    `yaml:"homes"`
}

type Defaults struct {
	Workspace string         `yaml:"workspace"` // relative repoRoot values resolve against it
	Harnesses []string       `yaml:"harnesses"` // what a project renders for when it sets none
	Vars      map[string]any `yaml:"vars"`      // the middle layer: every project and home
	Extra     map[string]any `yaml:",inline" json:"-"`
}

// Project is one repository that receives skills, subagents and rules.
type Project struct {
	Name      string   `yaml:"-"`        // filled from the map key
	RepoRoot  string   `yaml:"repoRoot"` // resolved to absolute in place
	Harnesses []string `yaml:"harnesses"`
	Skills    []string `yaml:"skills"`
	Agents    []string `yaml:"agents"`
	Rules     []string `yaml:"rules"`
	Bundles   []string `yaml:"bundles"`
	Disabled  []string `yaml:"disabled"` // bundles declared but switched off; install prunes their artifacts
	// Vars is, after load, the merged map: config vars < defaults.vars < the project's own.
	Vars  map[string]any `yaml:"vars"`
	Extra map[string]any `yaml:",inline" json:"-"`
}

// Home is a per-user harness directory (~/.claude, ~/.codex, ~/.cursor, ...).
// It goes through the same render/adapter/write/track/prune pipeline as a
// project; only the layout differs: skills land in <dir>/skills, agents in
// <dir>/agents, Cursor rules in <dir>/rules.
type Home struct {
	Name    string `yaml:"-"`       // filled from the map key
	Harness string `yaml:"harness"` // claude | codex | cursor
	Dir     string `yaml:"dir"`     // the harness user dir; ~ is expanded
	// Skills is a list of skill names; omitted (nil) means every skill under
	// <config>/global/skills and <config>/agents/*/skills — what a home always
	// received. An explicit empty list means none. A skill from templates/ or
	// the embedded examples reaches a home only when named here or by a bundle.
	Skills  []string       `yaml:"skills"`
	Skip    []string       `yaml:"skip"` // names this home does not take (harness-specific ones)
	Agents  []string       `yaml:"agents"`
	Rules   []string       `yaml:"rules"`
	Bundles []string       `yaml:"bundles"`
	Vars    map[string]any `yaml:"vars"` // merged after load, like Project.Vars
	Extra   map[string]any `yaml:",inline" json:"-"`
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

// --- load + resolve -------------------------------------------------------

// loadManifest reads and resolves the manifest at path. version is the binary's
// version, checked against the manifest's optional `engine:` constraint.
func loadManifest(path, version string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := checkEngine(m.Engine, version); err != nil {
		return nil, err
	}
	if err := rejectRemoved(&m); err != nil {
		return nil, err
	}
	if m.Projects == nil {
		m.Projects = map[string]Project{}
	}
	if m.Homes == nil {
		m.Homes = map[string]Home{}
	}
	for name, p := range m.Projects {
		p.Name = name
		resolveProject(&p, m)
		m.Projects[name] = p
	}
	for name, h := range m.Homes {
		h.Name = name
		h.Dir = expandHome(h.Dir)
		h.Vars = mergeVars(m.Vars, m.Defaults.Vars, h.Vars)
		m.Homes[name] = h
	}
	return &m, nil
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func resolveProject(p *Project, m Manifest) {
	p.RepoRoot = expandHome(p.RepoRoot)
	if p.RepoRoot != "" && !filepath.IsAbs(p.RepoRoot) {
		p.RepoRoot = filepath.Join(m.Defaults.Workspace, p.RepoRoot)
	}
	if len(p.Harnesses) == 0 {
		p.Harnesses = m.Defaults.Harnesses
	}
	p.Vars = mergeVars(m.Vars, m.Defaults.Vars, p.Vars)
}

// removedKeys are the v0.2 manifest fields that no longer exist, with the v0.3
// replacement. They are rejected loudly: silently ignoring `tickets:` would
// surface later as a confusing "map has no entry for key" in a template.
var removedProjectKeys = map[string]string{
	"tickets": "move it under vars:",
	"run":     "move it under vars:",
	"gates":   "move it under vars:",
	"docs":    "move it under vars:",
	"stack":   "move it under vars:",
	"tools":   "move it under vars:",
	"vcs":     "move it under vars:",
	"e2e":     "move it under vars:",
	"targets": "renamed to harnesses:",
}

func rejectRemoved(m *Manifest) error {
	check := func(where string, extra map[string]any, keys map[string]string) error {
		var found []string
		for k := range extra {
			if _, ok := keys[k]; ok {
				found = append(found, k)
			}
		}
		sort.Strings(found)
		if len(found) == 0 {
			return nil
		}
		return fmt.Errorf("%s: %q was removed in trafaret 0.3 — %s (see docs/spec.md, Migration)", where, found[0], keys[found[0]])
	}
	defaultsKeys := map[string]string{
		"targets": "renamed to harnesses:",
		"tickets": "move it under defaults.vars:",
		"tools":   "move it under defaults.vars:",
	}
	if err := check("defaults", m.Defaults.Extra, defaultsKeys); err != nil {
		return err
	}
	for _, n := range sortedKeys(m.Projects) {
		if err := check("projects."+n, m.Projects[n].Extra, removedProjectKeys); err != nil {
			return err
		}
	}
	for _, n := range sortedKeys(m.Homes) {
		if err := check("homes."+n, m.Homes[n].Extra, map[string]string{"skillsDir": "use harness: and dir: (skills land in <dir>/skills)"}); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
