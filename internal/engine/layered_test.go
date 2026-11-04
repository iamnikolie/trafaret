package engine

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// --- templates overlay -----------------------------------------------------

func TestLayeredFSIsAValidFS(t *testing.T) {
	base := fstest.MapFS{
		"skills/alpha/SKILL.md.tmpl": {Data: []byte("base alpha")},
		"skills/alpha/extra.md":      {Data: []byte("base extra")},
		"skills/beta/SKILL.md.tmpl":  {Data: []byte("base beta")},
		"agents/x.md.tmpl":           {Data: []byte("base x")},
	}
	top := fstest.MapFS{
		"skills/alpha/SKILL.md.tmpl": {Data: []byte("top alpha")},
		"skills/gamma/SKILL.md.tmpl": {Data: []byte("top gamma")},
		"rules/r.md.tmpl":            {Data: []byte("top rule")},
	}
	l := layerFS(top, base)

	if err := fstest.TestFS(l,
		"skills/alpha/SKILL.md.tmpl", "skills/alpha/extra.md", "skills/beta/SKILL.md.tmpl",
		"skills/gamma/SKILL.md.tmpl", "agents/x.md.tmpl", "rules/r.md.tmpl"); err != nil {
		t.Fatal(err)
	}

	read := func(name string) string {
		b, err := fs.ReadFile(l, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(b)
	}
	if got := read("skills/alpha/SKILL.md.tmpl"); got != "top alpha" {
		t.Errorf("overlay file must win, got %q", got)
	}
	if got := read("skills/alpha/extra.md"); got != "base extra" {
		t.Errorf("untouched base sibling must remain, got %q", got)
	}
	if got := read("skills/gamma/SKILL.md.tmpl"); got != "top gamma" {
		t.Errorf("overlay-only file must be added, got %q", got)
	}
	if got := read("skills/beta/SKILL.md.tmpl"); got != "base beta" {
		t.Errorf("base-only file must remain, got %q", got)
	}

	entries, err := fs.ReadDir(l, "skills")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "alpha,beta,gamma" {
		t.Errorf("merged directory listing = %v", names)
	}
}

func TestOverlayReplacesAndAddsInRender(t *testing.T) {
	manifest := "defaults:\n  workspace: {{WS}}\n  harnesses: [claude]\nprojects:\n  p:\n    repoRoot: p\n    bundles: [ship]\n    vars: {branch: main, lang: Go, company: Acme, reviewers: 1, gates: [], tracker: {kind: github, repo: a/b}}\n"
	// the overlay replaces one embedded skill file and adds a sibling next to it
	e, _, _ := newConfig(t, manifest, map[string]string{
		"templates/skills/ship/SKILL.md.tmpl": "---\nname: ship\ndescription: overlaid for {{.Name}}\n---\nOVERLAY BODY {{.Vars.branch}}\n",
		"templates/skills/ship/notes.md.tmpl": "notes for {{.Name}}\n",
	})
	if e.OverlayDir == "" {
		t.Fatal("overlay not discovered")
	}
	got := renderEntry(t, e, "p")
	if !strings.Contains(got[".claude/skills/ship/SKILL.md"], "OVERLAY BODY main") || !strings.Contains(got[".claude/skills/ship/SKILL.md"], "overlaid for p") {
		t.Errorf("overlay SKILL.md did not win:\n%s", got[".claude/skills/ship/SKILL.md"])
	}
	if got[".claude/skills/ship/notes.md"] != "notes for p\n" {
		t.Errorf("overlay-only file not rendered: %q", got[".claude/skills/ship/notes.md"])
	}
	// the embedded sibling the overlay did not touch is still there
	if _, ok := got[".claude/skills/ship/scripts/gates.sh"]; !ok {
		t.Errorf("untouched embedded file must remain, got:\n%s", sortedPaths(got))
	}

	// the embedded set alone renders the original
	e2, _, _ := newConfig(t, manifest, nil)
	if strings.Contains(renderEntry(t, e2, "p")[".claude/skills/ship/SKILL.md"], "OVERLAY BODY") {
		t.Fatal("embedded render must not see the overlay")
	}
}

func TestTemplatesFlagOverridesOverlayDir(t *testing.T) {
	cfg := writeManifest(t, filepath.Join(evalDir(t, t.TempDir()), "cfg"))
	writeTree(t, filepath.Join(cfg, "templates"), map[string]string{"skills/standup/SKILL.md": "FROM CONFIG OVERLAY"})
	custom := filepath.Join(t.TempDir(), "custom")
	writeTree(t, custom, map[string]string{"skills/standup/SKILL.md": "FROM FLAG OVERLAY"})

	e, err := New(Options{Config: cfg, Templates: custom})
	if err != nil {
		t.Fatal(err)
	}
	b, err := fs.ReadFile(e.Templates, "skills/standup/SKILL.md")
	if err != nil || string(b) != "FROM FLAG OVERLAY" {
		t.Fatalf("--templates must replace <config>/templates: %q %v", b, err)
	}

	// an empty --templates directory disables the config's overlay entirely
	empty := t.TempDir()
	e, err = New(Options{Config: cfg, Templates: empty})
	if err != nil {
		t.Fatal(err)
	}
	b, err = fs.ReadFile(e.Templates, "skills/standup/SKILL.md")
	if err != nil || strings.Contains(string(b), "OVERLAY") || !strings.Contains(string(b), "# standup") {
		t.Fatalf("empty --templates must leave only the embedded file: %q %v", b, err)
	}

	if _, err := New(Options{Config: cfg, Templates: filepath.Join(empty, "missing")}); err == nil || !strings.Contains(err.Error(), "--templates") {
		t.Fatalf("a missing --templates dir must be an error, got %v", err)
	}
}
