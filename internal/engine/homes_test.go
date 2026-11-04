package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeSkipLeavesOutNamedSkills(t *testing.T) {
	global := t.TempDir()
	for _, s := range []string{"alpha", "beta"} {
		dir := filepath.Join(global, "skills", s)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + s + "\ndescription: Use for " + s + ".\n---\n\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	home := Home{Name: "test", SkillsDir: t.TempDir(), Skip: []string{"beta"}}
	files, err := renderHome(home, global)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].path != filepath.Join(home.SkillsDir, "alpha", "SKILL.md") {
		t.Fatalf("want only alpha, got %+v", files)
	}

	home.Skip = []string{"gamma"}
	if _, err := renderHome(home, global); err == nil {
		t.Fatal("skip naming an unknown skill must fail")
	}
}

func TestHomeInstallPrunesOnlyWhatItWrote(t *testing.T) {
	global := t.TempDir()
	for _, s := range []string{"alpha", "beta"} {
		dir := filepath.Join(global, "skills", s)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + s + "\ndescription: Use for " + s + ".\n---\n\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	home := Home{Name: "test", SkillsDir: t.TempDir()}
	foreign := filepath.Join(home.SkillsDir, ".system", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("harness-owned"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := installHome(home, global, false, false); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"alpha", "beta"} {
		if _, err := os.Stat(filepath.Join(home.SkillsDir, s, "SKILL.md")); err != nil {
			t.Fatalf("%s not installed: %v", s, err)
		}
	}

	home.Skills = []string{"alpha"}
	res, err := installHome(home, global, false, false)
	if err != nil {
		t.Fatal(err)
	}
	removed := 0
	for _, f := range res {
		if f.Status == "remove" {
			removed++
		}
	}
	if removed != 1 {
		t.Fatalf("want 1 removal, got %+v", res)
	}
	if _, err := os.Stat(filepath.Join(home.SkillsDir, "beta")); !os.IsNotExist(err) {
		t.Fatalf("beta dir should be pruned, stat err = %v", err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("harness-owned file must survive: %v", err)
	}

	home.Skills = []string{"gamma"}
	if _, err := renderHome(home, global); err == nil {
		t.Fatal("unknown global skill must be an error")
	}
}

func TestHomeTakesAgentSkillsAndRejectsDuplicates(t *testing.T) {
	root := t.TempDir()
	global := filepath.Join(root, "global")
	write := func(dir, name string) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: Use for " + name + ".\n---\n\nBody.\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(global, "skills", "alpha"), "alpha")
	write(filepath.Join(root, "agents", "designer", "skills", "design-review"), "design-review")
	home := Home{Name: "test", SkillsDir: t.TempDir()}
	files, err := renderHome(home, global)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].path != filepath.Join(home.SkillsDir, "alpha", "SKILL.md") ||
		files[1].path != filepath.Join(home.SkillsDir, "design-review", "SKILL.md") {
		t.Fatalf("want alpha + design-review, got %+v", files)
	}

	write(filepath.Join(root, "agents", "other", "skills", "alpha"), "alpha")
	if _, err := renderHome(home, global); err == nil {
		t.Fatal("a skill name defined in global/skills and in agents/*/skills must fail")
	}
}
