package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// --- source namespace ---------------------------------------------------------
//
// Skills live in one namespace made of
//
//	templates/skills/<name>/        the templates overlay (merged over the examples by path)
//	<config>/global/skills/<name>/  more skills
//	<config>/agents/<role>/skills/<name>/
//	the embedded examples           (skills/<name>/ inside the binary)
//
// A name defined twice among the config sources (overlay, global, roles) is an
// error — the sources land side by side in one place. The embedded examples are
// shadowed by any config source of the same name. Agents and rules come from
// the template set only: templates/agents/<name>.md[.tmpl] and
// templates/rules/<name>.md[.tmpl].

// Skill origins, as reported in a SkillSource.
const (
	OriginTemplates = "templates" // the templates overlay on disk
	OriginExamples  = "examples"  // embedded in the binary
	OriginGlobal    = "global"    // <config>/global/skills
	OriginRole      = "role"      // <config>/agents/<role>/skills
)

// SkillSource is one skill of the namespace.
type SkillSource struct {
	Name   string
	Origin string // one of the Origin* constants
	// Dir is where the skill lives, relative to the config directory for config
	// sources ("global/skills/x") and to the embedded set for examples
	// ("examples/skills/x").
	Dir string
	// Global marks the sources a home takes when its skills: is omitted:
	// <config>/global/skills and <config>/agents/*/skills.
	Global bool

	fsys fs.FS // rooted at the skill directory
	abs  string
}

// Main returns the skill's entry file (SKILL.md.tmpl or SKILL.md) and its name.
func (s *SkillSource) Main() (string, []byte, error) {
	for _, n := range []string{"SKILL.md.tmpl", "SKILL.md"} {
		if b, err := fs.ReadFile(s.fsys, n); err == nil {
			return n, b, nil
		}
	}
	return "", nil, fmt.Errorf("skill %q has no SKILL.md or SKILL.md.tmpl", s.Name)
}

// FileSource is one agent or rule: a single Markdown file, rendered when it ends
// in .tmpl.
type FileSource struct {
	Name  string
	Label string // "templates/agents/x.md.tmpl" or "examples/agents/x.md.tmpl"
	Tmpl  bool
	path  string // inside the template set
	fsys  fs.FS
}

// Read returns the raw source.
func (f *FileSource) Read() ([]byte, error) { return fs.ReadFile(f.fsys, f.path) }

type partial struct{ name, text string }

// Namespace is everything that can be installed, resolved from the template set
// and the config directory.
type Namespace struct {
	Skills map[string]*SkillSource
	Agents map[string]*FileSource
	Rules  map[string]*FileSource

	partials []partial
}

// SkillNames returns the sorted skill names.
func (ns *Namespace) SkillNames() []string { return sortedKeys(ns.Skills) }

// AgentNames returns the sorted agent names.
func (ns *Namespace) AgentNames() []string { return sortedKeys(ns.Agents) }

// RuleNames returns the sorted rule names.
func (ns *Namespace) RuleNames() []string { return sortedKeys(ns.Rules) }

// globalSkillNames are the skills a home with skills: omitted receives.
func (ns *Namespace) globalSkillNames() []string {
	var out []string
	for n, s := range ns.Skills {
		if s.Global {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// Namespace resolves the source namespace of this engine.
func (e *Engine) Namespace() (*Namespace, error) {
	ns := &Namespace{
		Skills: map[string]*SkillSource{},
		Agents: map[string]*FileSource{},
		Rules:  map[string]*FileSource{},
	}

	// templates overlay (merged over the examples by path) and the examples
	var overlayNames []string
	if e.OverlayDir != "" {
		overlayNames = dirNames(os.DirFS(e.OverlayDir), "skills")
	}
	inOverlay := map[string]bool{}
	for _, n := range overlayNames {
		inOverlay[n] = true
	}
	for _, n := range dirNames(e.Templates, "skills") {
		sub, err := fs.Sub(e.Templates, path.Join("skills", n))
		if err != nil {
			return nil, err
		}
		s := &SkillSource{Name: n, fsys: sub}
		if inOverlay[n] {
			s.Origin, s.Dir, s.abs = OriginTemplates, "templates/skills/"+n, filepath.Join(e.OverlayDir, "skills", n)
		} else {
			s.Origin, s.Dir, s.abs = OriginExamples, "examples/skills/"+n, "(embedded) skills/"+n
		}
		ns.Skills[n] = s
	}

	// global skills and role skills: a name may appear once among all config
	// sources; an embedded example of the same name is shadowed
	addDir := func(dir, origin, label string) error {
		for _, n := range dirNames(os.DirFS(dir), ".") {
			abs := filepath.Join(dir, n)
			if prev, dup := ns.Skills[n]; dup && prev.Origin != OriginExamples {
				return fmt.Errorf("skill %q defined twice: %s and %s", n, prev.abs, abs)
			}
			ns.Skills[n] = &SkillSource{Name: n, Origin: origin, Dir: label + "/" + n, Global: true, fsys: os.DirFS(abs), abs: abs}
		}
		return nil
	}
	if err := addDir(filepath.Join(e.GlobalDir(), "skills"), OriginGlobal, "global/skills"); err != nil {
		return nil, err
	}
	agentsDir := filepath.Join(e.ConfigDir, "agents")
	for _, role := range dirNames(os.DirFS(agentsDir), ".") {
		if err := addDir(filepath.Join(agentsDir, role, "skills"), OriginRole, "agents/"+role+"/skills"); err != nil {
			return nil, err
		}
	}

	var err error
	if ns.Agents, err = e.fileSources("agents"); err != nil {
		return nil, err
	}
	if ns.Rules, err = e.fileSources("rules"); err != nil {
		return nil, err
	}

	// partials: every templates/partials/*.tmpl, parsed into each .tmpl render
	entries, _ := fs.ReadDir(e.Templates, "partials")
	for _, en := range entries {
		if en.IsDir() || !strings.HasSuffix(en.Name(), ".tmpl") {
			continue
		}
		b, err := fs.ReadFile(e.Templates, path.Join("partials", en.Name()))
		if err != nil {
			return nil, err
		}
		ns.partials = append(ns.partials, partial{name: "partials/" + en.Name(), text: string(b)})
	}
	// a broken partial is an error of the library, not of whichever template
	// happens to render first
	probe := template.New("partials").Funcs(templateFuncs)
	for _, p := range ns.partials {
		if _, err := probe.New(p.name).Parse(p.text); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p.name, err)
		}
	}
	return ns, nil
}

// fileSources lists the flat <dir>/<name>.md[.tmpl] entries of the template set.
func (e *Engine) fileSources(dir string) (map[string]*FileSource, error) {
	out := map[string]*FileSource{}
	entries, err := fs.ReadDir(e.Templates, dir)
	if err != nil {
		return out, nil // no such directory: none of this kind
	}
	for _, en := range entries {
		if en.IsDir() {
			continue
		}
		var name string
		tmpl := false
		switch {
		case strings.HasSuffix(en.Name(), ".md.tmpl"):
			name, tmpl = strings.TrimSuffix(en.Name(), ".md.tmpl"), true
		case strings.HasSuffix(en.Name(), ".md"):
			name = strings.TrimSuffix(en.Name(), ".md")
		default:
			continue
		}
		if prev, dup := out[name]; dup {
			return nil, fmt.Errorf("%s %q defined twice: %s and %s", strings.TrimSuffix(dir, "s"), name, prev.Label, e.sourceLabel(path.Join(dir, en.Name())))
		}
		out[name] = &FileSource{Name: name, Tmpl: tmpl, Label: e.sourceLabel(path.Join(dir, en.Name())), path: path.Join(dir, en.Name()), fsys: e.Templates}
	}
	return out, nil
}

// sourceLabel names where a template-set file comes from: the overlay when it
// has the file, the embedded examples otherwise.
func (e *Engine) sourceLabel(rel string) string {
	if e.OverlayDir != "" {
		if _, err := os.Stat(filepath.Join(e.OverlayDir, filepath.FromSlash(rel))); err == nil {
			return "templates/" + rel
		}
	}
	return "examples/" + rel
}

// dirNames lists the non-hidden subdirectories of dir in fsys.
func dirNames(fsys fs.FS, dir string) []string {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, en := range entries {
		if en.IsDir() && !strings.HasPrefix(en.Name(), ".") {
			out = append(out, en.Name())
		}
	}
	sort.Strings(out)
	return out
}
