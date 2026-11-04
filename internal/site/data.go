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
	Bundles     []siteBundle
	Skills      []siteTemplate
	Agents      []siteTemplate
	Rules       []siteTemplate
	ProjSkills  []siteProjSkills
}

type siteProject struct {
	Name     string
	RepoRoot string
	Branch   string
	Space    string
	Targets  string
	Bundles  string
	Install  int
	Update   int
	Skip     int
	InSync   bool
	Err      string
}

type siteBundle struct {
	Name   string
	Skills string
	Agents string
	Rules  string
	UsedBy string
}

type siteTemplate struct {
	Name        string
	Description string
	Words       int
	Path        string
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

	// projects + live sync status (same comparison as `trafaret diff`)
	var names []string
	for n := range m.Projects {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := m.Projects[n]
		sp := siteProject{
			Name:     n,
			RepoRoot: p.RepoRoot,
			Branch:   p.VCS.DefaultBranch,
			Space:    p.Tickets.Space,
			Targets:  strings.Join(p.Targets, " + "),
			Bundles:  p.BundlesDisplay(),
		}
		if sp.Branch == "" {
			sp.Branch = "—"
		}
		if sp.Space == "" {
			sp.Space = "—"
		}
		if sp.Bundles == "" {
			sp.Bundles = "(verbatim sources only)"
		}
		res, err := eng.DryRun(p)
		if err != nil {
			sp.Err = err.Error()
		} else {
			for _, f := range res {
				switch f.Status {
				case "install":
					sp.Install++
				case "update", "remove":
					sp.Update++
				case "skip":
					sp.Skip++
				}
			}
			sp.InSync = sp.Install == 0 && sp.Update == 0
		}
		d.Projects = append(d.Projects, sp)
	}

	// bundle registry + reverse usage
	usedBy := map[string][]string{}
	for _, n := range names {
		for _, b := range m.Projects[n].Bundles {
			usedBy[b] = append(usedBy[b], n)
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
			Name:   bn,
			Skills: strings.Join(b.Skills, ", "),
			Agents: strings.Join(b.Agents, ", "),
			Rules:  strings.Join(b.CursorRules, ", "),
			UsedBy: strings.Join(usedBy[bn], ", "),
		})
	}

	// template catalog
	d.Skills = catalogDir(eng.Templates, "skills", "templates/skills", true)
	d.Agents = catalogFiles(eng.Templates, "agents", "templates/agents", ".md.tmpl")
	d.Rules = catalogFiles(eng.Templates, "cursor/rules", "templates/cursor/rules", ".mdc.tmpl")

	// per-project verbatim sources
	projFS := os.DirFS(eng.ProjectsDir())
	if entries, err := fs.ReadDir(projFS, "."); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skills := catalogDir(projFS, path.Join(e.Name(), "skills"), "projects/"+e.Name()+"/skills", false)
			if len(skills) > 0 {
				d.ProjSkills = append(d.ProjSkills, siteProjSkills{Project: e.Name(), Skills: skills})
			}
		}
	}
	return d, nil
}

// catalogDir lists <dir>/<name>/SKILL.md[.tmpl] entries of fsys with frontmatter
// metadata; label is the path prefix shown on the page.
func catalogDir(fsys fs.FS, dir, label string, tmpl bool) []siteTemplate {
	var out []siteTemplate
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		base := "SKILL.md"
		if tmpl {
			base += ".tmpl"
		}
		p := path.Join(dir, e.Name(), base)
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			continue
		}
		out = append(out, siteTemplate{
			Name:        e.Name(),
			Description: frontmatterValue(string(b), "description"),
			Words:       len(strings.Fields(string(b))),
			Path:        path.Join(label, e.Name(), base),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// catalogFiles lists flat <dir>/<name><suffix> entries of fsys.
func catalogFiles(fsys fs.FS, dir, label, suffix string) []siteTemplate {
	var out []siteTemplate
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		p := path.Join(dir, e.Name())
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			continue
		}
		out = append(out, siteTemplate{
			Name:        strings.TrimSuffix(e.Name(), suffix),
			Description: frontmatterValue(string(b), "description"),
			Words:       len(strings.Fields(string(b))),
			Path:        path.Join(label, e.Name()),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// frontmatterValue extracts a single-line scalar from a raw frontmatter block.
// Templates keep {{.Token}} expressions — shown as-is on purpose: the tokens
// ARE the documentation of what varies per project.
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
