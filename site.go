// trafaret serve — a local, live-rendered manual for the library trafaret manages.
//
// The page is rendered from manifest.yaml + templates (built in, plus the
// <config>/templates overlay) + projects/ on EVERY request: an agent's edit to any
// of the on-disk files is, by construction, an edit to this site. Nothing is
// published anywhere — the server binds localhost only.
package main

import (
	_ "embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

//go:embed docs/site-assets/mermaid.min.js
var mermaidJS []byte

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

func collectSiteData(eng *Engine) (*siteData, error) {
	m, err := loadManifest(eng.Manifest)
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
			Bundles:  bundlesDisplay(p),
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
		res, err := install(p, eng, subtract(p.Bundles, p.Disabled), p.Targets, false, true /* dry-run */, true /* prune report */, false /* full target set */)
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

// --- server -----------------------------------------------------------------

func cmdServe(eng *Engine, opt options) {
	addr := opt.addr
	if addr == "" {
		addr = "127.0.0.1:4477"
	}
	if !strings.Contains(addr, ":") {
		addr = "127.0.0.1:" + addr
	}
	page := template.Must(template.New("site").Parse(siteSkeleton))
	langs := map[string]map[string]template.HTML{"en": siteEN, "ru": siteRU}

	servePage := func(w http.ResponseWriter, lang, slug string, t map[string]template.HTML) {
		d, err := collectSiteData(eng)
		if err != nil {
			http.Error(w, "collect: "+err.Error(), 500)
			return
		}
		pd := sitePageData{siteData: d, Lang: lang, Page: slug, T: t}
		for i, sp := range sitePages {
			link := sitePageLink{Slug: sp.Slug, Title: t[sp.TitleKey], No: fmt.Sprintf("%02d", i+1)}
			pd.PageList = append(pd.PageList, link)
			if sp.Slug == slug {
				if i > 0 {
					prev := sitePageLink{Slug: sitePages[i-1].Slug, Title: t[sitePages[i-1].TitleKey]}
					pd.Prev = &prev
				}
				if i < len(sitePages)-1 {
					next := sitePageLink{Slug: sitePages[i+1].Slug, Title: t[sitePages[i+1].TitleKey]}
					pd.Next = &next
				}
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, pd); err != nil {
			fmt.Fprintln(os.Stderr, "trafaret: render site:", err)
		}
	}

	http.HandleFunc("/mermaid.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write(mermaidJS)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// forwards to the language saved in localStorage (default EN)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(langIndex))
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		t, ok := langs[parts[0]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if len(parts) == 1 {
			http.Redirect(w, r, "/"+parts[0]+"/overview", http.StatusFound)
			return
		}
		slug := parts[1]
		known := false
		for _, sp := range sitePages {
			if sp.Slug == slug {
				known = true
				break
			}
		}
		if !known || len(parts) > 2 {
			http.NotFound(w, r)
			return
		}
		servePage(w, parts[0], slug, t)
	})

	fmt.Printf("trafaret site → http://%s  (live: re-rendered from %s on every refresh; Ctrl-C to stop)\n", addr, eng.ConfigDir)
	check(http.ListenAndServe(addr, nil))
}
