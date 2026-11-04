package engine

import (
	"io/fs"
	"testing"
)

func TestEmbeddedTemplatesAreComplete(t *testing.T) {
	tmpl := embeddedTemplates()
	reg, err := loadBundles(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg) == 0 {
		t.Fatal("embedded bundles.yaml lists no bundles")
	}
	for _, b := range reg {
		for _, s := range b.Skills {
			if _, err := fs.Stat(tmpl, "skills/"+s+"/SKILL.md.tmpl"); err != nil {
				t.Errorf("bundle skill %q missing from the embedded templates: %v", s, err)
			}
		}
		for _, a := range b.Agents {
			if _, err := fs.Stat(tmpl, "agents/"+a+".md.tmpl"); err != nil {
				t.Errorf("bundle agent %q missing from the embedded templates: %v", a, err)
			}
		}
		for _, r := range b.CursorRules {
			if _, err := fs.Stat(tmpl, "cursor/rules/"+r+".mdc.tmpl"); err != nil {
				t.Errorf("bundle rule %q missing from the embedded templates: %v", r, err)
			}
		}
	}
}

func TestEveryBundleRendersFromEmbeddedTemplates(t *testing.T) {
	p := testProject(t)
	eng := testEngine(t)
	var all []string
	for name := range eng.Bundles {
		all = append(all, name)
	}
	files, err := render(p, eng, all, []string{"claude", "cursor", "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("nothing rendered")
	}
}
