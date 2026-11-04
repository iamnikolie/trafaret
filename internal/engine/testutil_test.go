package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree writes files (slash path -> body) under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// skillMD is a minimal valid SKILL.md body.
func skillMD(name string) string {
	return "---\nname: " + name + "\ndescription: Use for " + name + ".\n---\n\nBody of " + name + ".\n"
}

// newConfig creates a config directory holding manifest.yaml plus files (paths
// relative to the config directory) and returns an Engine over it. "{{WS}}" in
// the manifest is replaced with a fresh workspace directory (returned), and
// "{{HOME}}" with a fresh directory for homes.
func newConfig(t *testing.T, manifest string, files map[string]string) (e *Engine, ws, home string) {
	t.Helper()
	root := evalDir(t, t.TempDir())
	ws, home = filepath.Join(root, "ws"), filepath.Join(root, "home")
	for _, d := range []string{ws, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest = strings.ReplaceAll(strings.ReplaceAll(manifest, "{{WS}}", ws), "{{HOME}}", home)
	cfg := filepath.Join(root, "cfg")
	writeTree(t, cfg, map[string]string{"manifest.yaml": manifest})
	writeTree(t, cfg, files)
	e, err := New(Options{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	return e, ws, home
}

// installOnly runs a real install and returns the results.
func installOnly(t *testing.T, e *Engine, o InstallOptions) []ProjectResult {
	t.Helper()
	res, err := e.Install(o)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// statuses counts the statuses of one result entry.
func statuses(res []ProjectResult, name string) map[string]int {
	out := map[string]int{}
	for _, r := range res {
		if r.Project == name {
			for _, f := range r.Files {
				out[f.Status]++
			}
		}
	}
	return out
}

// renderEntry renders a project (or "home:<name>") fully in memory and returns
// path-relative-to-its-base -> content.
func renderEntry(t *testing.T, e *Engine, name string) map[string]string {
	t.Helper()
	out, err := renderEntryErr(t, e, name)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// renderEntryErr is renderEntry that returns the render error instead of failing.
func renderEntryErr(t *testing.T, e *Engine, name string) (map[string]string, error) {
	t.Helper()
	m, err := e.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	ns, err := e.Namespace()
	if err != nil {
		return nil, err
	}
	var files []outFile
	var base string
	if h, ok := strings.CutPrefix(name, "home:"); ok {
		hm := m.Homes[h]
		it, err := e.homeItems(ns, hm, hm.Bundles, true)
		if err != nil {
			return nil, err
		}
		if files, err = ns.render(homeEntity(hm), it); err != nil {
			return nil, err
		}
		base = hm.Dir
	} else {
		p := m.Projects[name]
		it, err := e.projectItems(ns, p, p.ActiveBundles(), true)
		if err != nil {
			return nil, err
		}
		if files, err = ns.render(e.projectEntity(p, p.Harnesses), it); err != nil {
			return nil, err
		}
		base = p.RepoRoot
	}
	out := map[string]string{}
	for _, f := range files {
		rel, _ := filepath.Rel(base, f.path)
		out[filepath.ToSlash(rel)] = string(f.bytes)
	}
	return out, nil
}

// tmplOut renders one skill template body for a project whose own vars are
// varsYAML (a YAML mapping body, unindented) and returns the rendered text
// after the frontmatter.
func tmplOut(t *testing.T, varsYAML, body string) (string, error) {
	t.Helper()
	manifest := "defaults:\n  workspace: {{WS}}\n  harnesses: [claude]\nprojects:\n  p:\n    repoRoot: p\n    skills: [t]\n    vars:\n"
	for _, l := range strings.Split(strings.TrimRight(varsYAML, "\n"), "\n") {
		manifest += "      " + l + "\n"
	}
	e, _, _ := newConfig(t, manifest, map[string]string{
		"templates/skills/t/SKILL.md.tmpl": "---\nname: t\ndescription: test skill\n---\n" + body,
	})
	got, err := renderEntryErr(t, e, "p")
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(got[".claude/skills/t/SKILL.md"], "---\nname: t\ndescription: test skill\n---\n"), nil
}

func sortedPaths(m map[string]string) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sortStringsLocal(ks)
	return strings.Join(ks, "\n")
}

func sortStringsLocal(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func grepLines(s, sub string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func osReadFile(p string) ([]byte, error) { return os.ReadFile(p) }
