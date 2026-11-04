package engine

import (
	"io/fs"
	"os"
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
		"cursor/rules/r.mdc.tmpl":    {Data: []byte("top rule")},
	}
	l := layerFS(top, base)

	if err := fstest.TestFS(l,
		"skills/alpha/SKILL.md.tmpl", "skills/alpha/extra.md", "skills/beta/SKILL.md.tmpl",
		"skills/gamma/SKILL.md.tmpl", "agents/x.md.tmpl", "cursor/rules/r.mdc.tmpl"); err != nil {
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
	cfg := writeManifest(t, filepath.Join(evalDir(t, t.TempDir()), "cfg"))
	overlay := filepath.Join(cfg, "templates")
	write := func(rel, body string) {
		path := filepath.Join(overlay, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// replaces a built-in skill file, adds a sibling file next to it
	write("skills/example-tickets/SKILL.md.tmpl", "---\nname: example-tickets\ndescription: overlaid for {{.Name}}\n---\nOVERLAY BODY {{.Tickets.Space}}\n")
	write("skills/example-tickets/notes.md.tmpl", "notes for {{.Name}}\n")

	e, err := New(Options{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if e.OverlayDir != overlay {
		t.Fatalf("overlay not discovered: %q", e.OverlayDir)
	}

	p := testProject(t)
	files, err := render(p, e, []string{"example"}, []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range files {
		if strings.Contains(filepath.ToSlash(f.path), "/skills/example-tickets/") {
			got[filepath.Base(f.path)] = string(f.bytes)
		}
	}
	if !strings.Contains(got["SKILL.md"], "OVERLAY BODY Acme") || !strings.Contains(got["SKILL.md"], "overlaid for demo") {
		t.Errorf("overlay SKILL.md did not win:\n%s", got["SKILL.md"])
	}
	if got["notes.md"] != "notes for demo\n" {
		t.Errorf("overlay-only file not rendered: %q", got["notes.md"])
	}

	// the built-in set alone renders the original
	files, err = render(p, testEngine(t), []string{"example"}, []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if filepath.Base(f.path) == "SKILL.md" && strings.Contains(string(f.bytes), "OVERLAY BODY") {
			t.Fatal("embedded render must not see the overlay")
		}
	}
}

func TestTemplatesFlagOverridesOverlayDir(t *testing.T) {
	cfg := writeManifest(t, filepath.Join(evalDir(t, t.TempDir()), "cfg"))
	if err := os.MkdirAll(filepath.Join(cfg, "templates", "skills", "example-db"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "templates", "skills", "example-db", "SKILL.md.tmpl"), []byte("FROM CONFIG OVERLAY"), 0o644); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(filepath.Join(custom, "skills", "example-db"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(custom, "skills", "example-db", "SKILL.md.tmpl"), []byte("FROM FLAG OVERLAY"), 0o644); err != nil {
		t.Fatal(err)
	}

	e, err := New(Options{Config: cfg, Templates: custom})
	if err != nil {
		t.Fatal(err)
	}
	b, err := fs.ReadFile(e.Templates, "skills/example-db/SKILL.md.tmpl")
	if err != nil || string(b) != "FROM FLAG OVERLAY" {
		t.Fatalf("--templates must replace <config>/templates: %q %v", b, err)
	}

	// an empty --templates directory disables the config's overlay entirely
	empty := t.TempDir()
	e, err = New(Options{Config: cfg, Templates: empty})
	if err != nil {
		t.Fatal(err)
	}
	b, err = fs.ReadFile(e.Templates, "skills/example-db/SKILL.md.tmpl")
	if err != nil || strings.Contains(string(b), "OVERLAY") {
		t.Fatalf("empty --templates must leave only the embedded file: %q %v", b, err)
	}

	if _, err := New(Options{Config: cfg, Templates: filepath.Join(empty, "missing")}); err == nil || !strings.Contains(err.Error(), "--templates") {
		t.Fatalf("a missing --templates dir must be an error, got %v", err)
	}
}
