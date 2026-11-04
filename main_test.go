package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexAgentConvertsMetadataAndInstructions(t *testing.T) {
	in := []byte(`---
name: reviewer
description: Reviews one change.
tools: Bash, Read, Grep
model: sonnet
---

Read the reviewer skill and report evidence.
`)

	got, err := codexAgent(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{
		`name = "reviewer"`,
		`description = "Reviews one change."`,
		`model = "gpt-5.6-terra"`,
		`model_reasoning_effort = "medium"`,
		`sandbox_mode = "read-only"`,
		`Read the reviewer skill and report evidence.`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("converted agent missing %q:\n%s", want, s)
		}
	}
}

func TestCodexAgentKeepsWritableAgentsWritable(t *testing.T) {
	in := []byte("---\nname: worker\ndescription: Implements a change.\ntools: Bash, Read, Edit, Write\nmodel: inherit\n---\n\nImplement it.\n")
	got, err := codexAgent(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "sandbox_mode") {
		t.Fatalf("writable agent unexpectedly constrained:\n%s", got)
	}
	if strings.Contains(string(got), "model =") {
		t.Fatalf("inheriting agent unexpectedly pins a model:\n%s", got)
	}
}

func TestCodexRenderIncludesReadableAndNativeAgentForms(t *testing.T) {
	p := testProject(t)
	files, err := render(p, testEngine(t), []string{"example"}, []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		filepath.Join(p.RepoRoot, ".agents", "agents", "example-reviewer.md"):  false,
		filepath.Join(p.RepoRoot, ".codex", "agents", "example-reviewer.toml"): false,
	}
	for _, f := range files {
		if _, ok := want[f.path]; ok {
			want[f.path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("Codex render missing %s", path)
		}
	}
}

func TestPartialTargetPrunePreservesOtherHarnesses(t *testing.T) {
	repo := t.TempDir()
	old := []string{
		".claude/skills/example/SKILL.md",
		".cursor/rules/example.mdc",
		".agents/skills/old/SKILL.md",
	}
	for _, rel := range old {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := json.Marshal(old)
	track := filepath.Join(repo, filepath.FromSlash(trackFile))
	if err := os.MkdirAll(filepath.Dir(track), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(track, b, 0o644); err != nil {
		t.Fatal(err)
	}

	newSet := map[string]bool{".agents/skills/new/SKILL.md": true}
	removed, err := pruneStale(repo, newSet, []string{"codex"}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || !strings.HasSuffix(filepath.ToSlash(removed[0].Path), ".agents/skills/old/SKILL.md") {
		t.Fatalf("unexpected removals: %#v", removed)
	}
	for _, rel := range old[:2] {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
			t.Errorf("other target artifact %s was removed: %v", rel, err)
		}
	}
}

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

// testProject is a fully-populated demo project, so tests do not depend on any
// particular manifest on disk.
func testProject(t *testing.T) Project {
	t.Helper()
	p := Project{
		Name:     "demo",
		RepoRoot: t.TempDir(),
		Bundles:  []string{"example", "example-db"},
		Targets:  []string{"claude", "cursor", "codex"},
		Tickets: Tickets{
			CLI: "fibery", Config: "acme", Space: "Acme", Host: "acme.example.com",
			Boards:    Boards{Tasks: "Tasks", Bugs: "Bugs"},
			States:    States{Backlog: "Backlog", Todo: "To do", InProgress: "In progress", InReview: "In review", Done: "To test"},
			Priority:  []string{"Critical", "High", "Medium", "Low"},
			RefScheme: RefScheme{Task: "Task-N", Bug: "Bug-N"},
			DependsOn: "Depends on: <ref>",
		},
		VCS:   VCS{DefaultBranch: "main", CommitConvention: "developer-style message"},
		Gates: Gates{Packages: []Package{{Name: "app", Dir: "app", Cmd: "make test"}}, LintBaseline: "new code clean"},
		Docs:  Docs{Map: "docs/MAP.md", ContextDir: "docs/context", SourceDir: "docs/source", Architect: "docs/ARCHITECT.md", SpecDir: "docs/specs", PlanDir: "docs/plans"},
		Stack: Stack{EnvPrefix: "DEMO_", ConventionsDocs: []string{"docs/source/dev-policy.md"}},
		Tools: Tools{Memory: "mem", DB: DB{CLI: "dbq", Config: "acme", Env: "dev", Schema: "demo", Tunnel: "true"}},
	}
	resolve(&p, Defaults{})
	return p
}

func testEngine(t *testing.T) *Engine {
	t.Helper()
	reg, err := loadBundles(embeddedTemplates())
	if err != nil {
		t.Fatal(err)
	}
	return &Engine{ConfigDir: t.TempDir(), Templates: embeddedTemplates(), Bundles: reg}
}
