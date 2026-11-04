package site

import (
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/iamnikolie/trafaret/internal/engine"
)

// --- view model -------------------------------------------------------------

type siteData struct {
	GeneratedAt string
	RepoDir     string
	Projects    []siteProject
	Homes       []siteHome
	Bundles     []siteBundle
	Skills      []siteSkill
	Agents      []siteTemplate
	Rules       []siteTemplate
	ProjSkills  []siteProjSkills
	SourceErr   string // the source namespace could not be built (a duplicate name, ...)
}

// siteSync is the live sync status of a project or a home: the same comparison
// as `trafaret diff`.
type siteSync struct {
	Install int
	Update  int
	Skip    int
	InSync  bool
	Err     string
}

type siteProject struct {
	Name      string
	RepoRoot  string
	Harnesses string
	Bundles   string
	Skills    string
	siteSync
}

type siteHome struct {
	Name    string
	Harness string
	Dir     string
	Bundles string
	Skills  string
	siteSync
}

type siteBundle struct {
	Name     string
	Skills   string
	Agents   string
	Rules    string
	Requires string
	UsedBy   string
}

type siteTemplate struct {
	Name        string
	Description string
	Words       int
	Path        string // where the source lives, relative to the config dir (or the embedded set)
}

// siteSkill is a skill of the namespace with the source directory it comes from.
type siteSkill struct {
	siteTemplate
	Origin string // templates | examples | global | role
}

type siteProjSkills struct {
	Project string
	Skills  []siteTemplate
}

// --- data gathering ---------------------------------------------------------

func collectSiteData(eng *engine.Engine) (*siteData, error) {
	m, err := eng.LoadManifest()
	if err != nil {
		return nil, err
	}
	d := &siteData{
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		RepoDir:     eng.ConfigDir,
	}

	sync := func(res []engine.FileStatus, err error) siteSync {
		var s siteSync
		if err != nil {
			s.Err = err.Error()
			return s
		}
		for _, f := range res {
			switch f.Status {
			case "install":
				s.Install++
			case "update", "remove":
				s.Update++
			case "skip":
				s.Skip++
			}
		}
		s.InSync = s.Install == 0 && s.Update == 0
		return s
	}

	// projects + live sync status
	var names []string
	for n := range m.Projects {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := m.Projects[n]
		sp := siteProject{
			Name:      n,
			RepoRoot:  p.RepoRoot,
			Harnesses: strings.Join(p.Harnesses, " + "),
			Bundles:   p.BundlesDisplay(),
			Skills:    strings.Join(eng.ProjectSkills(p), ", "),
		}
		if sp.Bundles == "" {
			sp.Bundles = "—"
		}
		if sp.Skills == "" {
			sp.Skills = "—"
		}
		sp.siteSync = sync(eng.DryRunProject(p))
		d.Projects = append(d.Projects, sp)
	}

	// homes + live sync status
	var homeNames []string
	for n := range m.Homes {
		homeNames = append(homeNames, n)
	}
	sort.Strings(homeNames)
	for _, n := range homeNames {
		h := m.Homes[n]
		sh := siteHome{
			Name:    n,
			Harness: h.Harness,
			Dir:     h.Dir,
			Bundles: strings.Join(h.Bundles, ", "),
			Skills:  eng.HomeSkills(h),
		}
		if sh.Bundles == "" {
			sh.Bundles = "—"
		}
		sh.siteSync = sync(eng.DryRunHome(h))
		d.Homes = append(d.Homes, sh)
	}

	// bundle registry + reverse usage
	usedBy := map[string][]string{}
	for _, n := range names {
		for _, b := range m.Projects[n].Bundles {
			usedBy[b] = append(usedBy[b], n)
		}
	}
	for _, n := range homeNames {
		for _, b := range m.Homes[n].Bundles {
			usedBy[b] = append(usedBy[b], "home:"+n)
		}
	}
	var bnames []string
	for bn := range eng.Bundles {
		bnames = append(bnames, bn)
	}
	sort.Strings(bnames)
	for _, bn := range bnames {
		b := eng.Bundles[bn]
		d.Bundles = append(d.Bundles, siteBundle{
			Name:     bn,
			Skills:   strings.Join(b.Skills, ", "),
			Agents:   strings.Join(b.Agents, ", "),
			Rules:    strings.Join(b.Rules, ", "),
			Requires: strings.Join(b.Requires, ", "),
			UsedBy:   strings.Join(usedBy[bn], ", "),
		})
	}

	// source catalog: every skill of the namespace with the directory it comes from
	ns, err := eng.Namespace()
	if err != nil {
		d.SourceErr = err.Error()
	} else {
		for _, name := range ns.SkillNames() {
			s := ns.Skills[name]
			st := siteSkill{Origin: s.Origin}
			st.Name, st.Path = name, s.Dir
			if main, b, err := s.Main(); err == nil {
				st.Description = frontmatterValue(string(b), "description")
				st.Words = len(strings.Fields(string(b)))
				st.Path = path.Join(s.Dir, main)
			}
			d.Skills = append(d.Skills, st)
		}
		d.Agents = fileCatalog(ns.AgentNames(), ns.Agents)
		d.Rules = fileCatalog(ns.RuleNames(), ns.Rules)
	}

	// per-project verbatim sources
	projFS := os.DirFS(eng.ProjectsDir())
	if entries, err := fs.ReadDir(projFS, "."); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skills := catalogDir(projFS, path.Join(e.Name(), "skills"), "projects/"+e.Name()+"/skills")
			if len(skills) > 0 {
				d.ProjSkills = append(d.ProjSkills, siteProjSkills{Project: e.Name(), Skills: skills})
			}
		}
	}
	return d, nil
}

func fileCatalog(names []string, src map[string]*engine.FileSource) []siteTemplate {
	var out []siteTemplate
	for _, n := range names {
		f := src[n]
		t := siteTemplate{Name: n, Path: f.Label}
		if b, err := f.Read(); err == nil {
			t.Description = frontmatterValue(string(b), "description")
			t.Words = len(strings.Fields(string(b)))
		}
		out = append(out, t)
	}
	return out
}

// catalogDir lists the <dir>/<name>/SKILL.md entries of fsys with frontmatter
// metadata; label is the path prefix shown on the page.
func catalogDir(fsys fs.FS, dir, label string) []siteTemplate {
	var out []siteTemplate
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := path.Join(dir, e.Name(), "SKILL.md")
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			continue
		}
		out = append(out, siteTemplate{
			Name:        e.Name(),
			Description: frontmatterValue(string(b), "description"),
			Words:       len(strings.Fields(string(b))),
			Path:        path.Join(label, e.Name(), "SKILL.md"),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// frontmatterValue extracts a single-line scalar from a raw frontmatter block.
// Templates keep their {{...}} expressions — shown as-is on purpose: they ARE
// the documentation of what varies per project.
func frontmatterValue(s, key string) string {
	if !strings.HasPrefix(s, "---\n") {
		return ""
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return ""
	}
	for _, line := range strings.Split(s[4:4+end], "\n") {
		if v, ok := strings.CutPrefix(line, key+": "); ok {
			return strings.Trim(v, `"`)
		}
	}
	return ""
}
