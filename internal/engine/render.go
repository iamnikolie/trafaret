package engine

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// --- render ---------------------------------------------------------------

type outFile struct {
	path  string // absolute destination
	bytes []byte
	src   string // where it came from, for error messages
}

// tmplData is what every .tmpl file renders against.
type tmplData struct {
	Name       string         // project or home name
	Kind       string         // "project" | "home"
	Harness    string         // claude | codex | cursor: the harness this copy is rendered for
	RepoRoot   string         // project repo root; "" for a home
	TargetRoot string         // where the file lands: ".claude" (relative to the repo) or the absolute home dir
	Vars       map[string]any // merged vars
}

// templateFuncs are the helpers templates may call, beside text/template's own.
var templateFuncs = template.FuncMap{
	// dash joins words with hyphens: "team memory" -> "team-memory".
	"dash": func(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), " ", "-") },
	// join is strings.Join over a list of any element type (YAML lists are []any).
	"join": func(elems any, sep string) (string, error) {
		switch v := elems.(type) {
		case nil:
			return "", nil
		case []string:
			return strings.Join(v, sep), nil
		case []any:
			parts := make([]string, len(v))
			for i, x := range v {
				parts[i] = fmt.Sprint(x)
			}
			return strings.Join(parts, sep), nil
		}
		return "", fmt.Errorf("join: want a list, got %T", elems)
	},
	// default returns def when val is empty: nil, "", 0, false, an empty list or map.
	"default": func(def, val any) any {
		if isEmptyValue(val) {
			return def
		}
		return val
	},
	// has reports whether a dotted key path exists: has .Vars "tickets.db".
	"has": hasPath,
}

// itemSet is what one project or home installs: its own lists plus the contents
// of its active bundles.
type itemSet struct {
	skills, agents, rules []string
}

func uniqSorted(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// collectItems unions the direct lists with the contents of the active bundles.
func collectItems(reg map[string]Bundle, direct itemSet, active []string) (itemSet, error) {
	it := itemSet{
		skills: append([]string(nil), direct.skills...),
		agents: append([]string(nil), direct.agents...),
		rules:  append([]string(nil), direct.rules...),
	}
	for _, bn := range active {
		b, ok := reg[bn]
		if !ok {
			return itemSet{}, fmt.Errorf("unknown bundle %q (not in templates/%s)", bn, bundlesFile)
		}
		it.skills = append(it.skills, b.Skills...)
		it.agents = append(it.agents, b.Agents...)
		it.rules = append(it.rules, b.Rules...)
	}
	it.skills, it.agents, it.rules = uniqSorted(it.skills), uniqSorted(it.agents), uniqSorted(it.rules)
	return it, nil
}

// check reports the first name the namespace does not define.
func (ns *Namespace) check(it itemSet) error {
	for _, s := range it.skills {
		if _, ok := ns.Skills[s]; !ok {
			return fmt.Errorf("no skill %q (looked in templates/skills, global/skills, agents/*/skills and the embedded examples)", s)
		}
	}
	for _, a := range it.agents {
		if _, ok := ns.Agents[a]; !ok {
			return fmt.Errorf("no agent %q (templates/agents/%s.md or .md.tmpl)", a, a)
		}
	}
	for _, r := range it.rules {
		if _, ok := ns.Rules[r]; !ok {
			return fmt.Errorf("no rule %q (templates/rules/%s.md or .md.tmpl)", r, r)
		}
	}
	return nil
}

// entity is the thing being rendered for: a project or a home.
type entity struct {
	scope     scope
	name      string
	base      string // project repo root or home dir, absolute
	harnesses []string
	vars      map[string]any
	// verbatimDir, for a project, is <config>/projects/<name>: skills/ and
	// agents/ there are copied as they are, no template pass.
	verbatimDir string
}

func (en entity) kind() string {
	if en.scope == homeScope {
		return "home"
	}
	return "project"
}

// renderer accumulates the output of one entity.
type renderer struct {
	ns      *Namespace
	en      entity
	out     []outFile
	claimed map[string]int // destination (relative, slash) -> index into out
}

// render produces every artifact of one project or home for the given items:
// for each harness of the entity, each item is placed where the adapter says.
//
// Claude and Cursor share the project's .claude tree (Cursor reads it natively;
// a parallel .cursor copy would double-load every skill): the harness that
// reaches a destination first renders it, the other skips it.
func (ns *Namespace) render(en entity, it itemSet) ([]outFile, error) {
	r := &renderer{ns: ns, en: en, claimed: map[string]int{}}
	for _, ad := range adapters {
		if !contains(en.harnesses, ad.name) {
			continue
		}
		lay := ad.layout(en.scope)
		for _, s := range it.skills {
			if err := r.skill(ad, lay.skills, ns.Skills[s]); err != nil {
				return nil, err
			}
		}
		for _, a := range it.agents {
			if err := r.file(ad, "agent", lay.agents, ns.Agents[a]); err != nil {
				return nil, err
			}
		}
		for _, ru := range it.rules {
			if err := r.file(ad, "rule", lay.rules, ns.Rules[ru]); err != nil {
				return nil, err
			}
		}
	}
	if en.scope == projectScope && en.verbatimDir != "" {
		if err := r.verbatim(); err != nil {
			return nil, err
		}
	}

	// frontmatter lint: a rendered artifact whose YAML header silently truncates
	// (unquoted " #") or fails to parse must never reach a repo.
	for _, f := range r.out {
		if err := lintFrontmatter(f.path, f.bytes); err != nil {
			return nil, err
		}
	}
	sort.Slice(r.out, func(i, j int) bool { return r.out[i].path < r.out[j].path })
	return r.out, nil
}

func (r *renderer) data(ad adapter, pl placement) tmplData {
	d := tmplData{Name: r.en.name, Kind: r.en.kind(), Harness: ad.name, Vars: r.en.vars}
	if r.en.scope == homeScope {
		d.TargetRoot = r.en.base
	} else {
		d.RepoRoot, d.TargetRoot = r.en.base, pl.root
	}
	return d
}

// add records a file. A destination reached twice with the same bytes is the
// shared tree; with different bytes it is a conflict.
func (r *renderer) add(rel string, b []byte, src string) error {
	if i, ok := r.claimed[rel]; ok {
		if !bytes.Equal(r.out[i].bytes, b) {
			return fmt.Errorf("%s is produced twice with different content (%s and %s)", rel, r.out[i].src, src)
		}
		return nil
	}
	r.claimed[rel] = len(r.out)
	r.out = append(r.out, outFile{path: filepath.Join(r.en.base, filepath.FromSlash(rel)), bytes: b, src: src})
	return nil
}

// skill places every file of a skill directory, recursively: .tmpl files render
// (the suffix is dropped), every other file is copied byte for byte.
func (r *renderer) skill(ad adapter, pls []placement, s *SkillSource) error {
	for _, pl := range pls {
		dir := fmt.Sprintf(pl.path, s.Name)
		err := fs.WalkDir(s.fsys, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			dst := path.Join(dir, strings.TrimSuffix(p, ".tmpl"))
			if _, shared := r.claimed[dst]; shared {
				return nil
			}
			label := path.Join(s.Dir, p)
			b, err := fs.ReadFile(s.fsys, p)
			if err != nil {
				return err
			}
			if strings.HasSuffix(p, ".tmpl") {
				if b, err = r.ns.renderTmpl(label, b, r.data(ad, pl)); err != nil {
					return err
				}
			}
			return r.add(dst, b, label)
		})
		if err != nil {
			return fmt.Errorf("skill %q: %w", s.Name, err)
		}
	}
	return nil
}

// file places an agent or a rule. It renders once per distinct root (the root
// is what .TargetRoot says) and derives every placement from that render.
func (r *renderer) file(ad adapter, kind string, pls []placement, f *FileSource) error {
	rendered := map[string][]byte{}
	for _, pl := range pls {
		dst := fmt.Sprintf(pl.path, f.Name)
		if _, shared := r.claimed[dst]; shared {
			continue
		}
		b, ok := rendered[pl.root]
		if !ok {
			var err error
			if b, err = f.Read(); err != nil {
				return fmt.Errorf("read %s: %w", f.Label, err)
			}
			if f.Tmpl {
				if b, err = r.ns.renderTmpl(f.Label, b, r.data(ad, pl)); err != nil {
					return err
				}
			}
			rendered[pl.root] = b
		}
		if pl.convert != nil {
			var err error
			if b, err = pl.convert(b); err != nil {
				return fmt.Errorf("%s %s %q: %w", ad.name, kind, f.Name, err)
			}
		}
		if err := r.add(dst, b, f.Label); err != nil {
			return err
		}
	}
	return nil
}

// verbatim copies <config>/projects/<name>/{skills,agents}/** as they are — no
// template pass: hand-written skills may legitimately contain "{{". Skills go
// wherever the harness puts skills; .md agents go wherever it puts agents (a
// Codex agent becomes TOML), without the readable auxiliary copy.
func (r *renderer) verbatim() error {
	for _, ad := range adapters {
		if !contains(r.en.harnesses, ad.name) {
			continue
		}
		lay := ad.layout(projectScope)
		for _, kind := range []string{"skills", "agents"} {
			root := filepath.Join(r.en.verbatimDir, kind)
			if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
				continue
			}
			err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				b, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(root, p)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				src := path.Join("projects", r.en.name, kind, rel)
				if kind == "skills" {
					name, rest, _ := strings.Cut(rel, "/")
					for _, pl := range lay.skills {
						if err := r.add(path.Join(fmt.Sprintf(pl.path, name), rest), b, src); err != nil {
							return err
						}
					}
					return nil
				}
				if !strings.HasSuffix(rel, ".md") {
					return nil
				}
				name := strings.TrimSuffix(rel, ".md")
				for _, pl := range lay.agents {
					if pl.aux {
						continue
					}
					out := b
					if pl.convert != nil {
						if out, err = pl.convert(b); err != nil {
							return fmt.Errorf("%s project agent %q: %w", ad.name, rel, err)
						}
					}
					if err := r.add(fmt.Sprintf(pl.path, name), out, src); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("project sources %s/%s: %w", r.en.name, kind, err)
			}
		}
	}
	return nil
}

// renderTmpl executes one template source against data, with every partial
// parsed in. A key missing from a map is an error, not an empty string.
func (ns *Namespace) renderTmpl(label string, src []byte, data tmplData) ([]byte, error) {
	t := template.New(label).Option("missingkey=error").Funcs(templateFuncs)
	for _, p := range ns.partials {
		if _, err := t.New(p.name).Parse(p.text); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p.name, err)
		}
	}
	if _, err := t.Parse(string(src)); err != nil {
		return nil, fmt.Errorf("parse %s: %w", label, err)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", label, err)
	}
	return []byte(sb.String()), nil
}
