package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkModeSharesOneDatabase(t *testing.T) {
	p := Project{Tickets: Tickets{Space: "Development", DB: "Development/Work", Project: "Alpha",
		Boards: Boards{Tasks: "Tasks", Bugs: "Bugs"}}}
	resolve(&p, Defaults{Workspace: "/ws"})
	tk := p.Tickets
	if !tk.Work || tk.TasksDB != "Development/Work" || tk.BugsDB != "Development/Work" {
		t.Fatalf("work mode not resolved: %+v", tk)
	}
	want := map[string]string{
		"ProjectFilter": ` --filter "Development/Project=Alpha"`,
		"TaskFilter":    ` --filter "Development/Project=Alpha" --filter "Development/Type=Task"`,
		"BugFilter":     ` --filter "Development/Project=Alpha" --filter "Development/Type=Bug"`,
		"TaskSet":       ` "Development/Project=Alpha" "Development/Type=Task"`,
		"BugSet":        ` "Development/Project=Alpha" "Development/Type=Bug"`,
		"ProjectWhere":  `"q/where": ["=", ["Development/Project", "enum/name"], "$project"],`,
		"ProjectParams": ` --params '{"$project":"Alpha"}'`,
	}
	got := map[string]string{"ProjectFilter": tk.ProjectFilter, "TaskFilter": tk.TaskFilter, "BugFilter": tk.BugFilter,
		"TaskSet": tk.TaskSet, "BugSet": tk.BugSet, "ProjectWhere": tk.ProjectWhere, "ProjectParams": tk.ProjectParams}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestResolveLegacyModeLeavesWorkTokensEmpty(t *testing.T) {
	p := Project{Tickets: Tickets{Space: "Old", Boards: Boards{Tasks: "Tasks", Bugs: "Bugs"}}}
	resolve(&p, Defaults{Workspace: "/ws"})
	tk := p.Tickets
	if tk.Work || tk.TasksDB != "Old/Tasks" || tk.BugsDB != "Old/Bugs" {
		t.Fatalf("legacy mode changed: %+v", tk)
	}
	if tk.ProjectFilter+tk.TaskFilter+tk.BugFilter+tk.TaskSet+tk.BugSet+tk.ProjectWhere+tk.ProjectParams != "" {
		t.Fatalf("work tokens must be empty in legacy mode: %+v", tk)
	}
}

func TestLoadManifestEnforcesEngine(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "m.yaml")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	if _, err := loadManifest(write("engine: \">=0.2.0\"\nprojects: {}\n"), "0.1.0"); err == nil || !strings.Contains(err.Error(), ">=0.2.0") {
		t.Fatalf("a too-old binary must be refused, got %v", err)
	}
	if _, err := loadManifest(write("engine: \">=0.2.0\"\nprojects: {}\n"), "0.2.0"); err != nil {
		t.Fatalf("a satisfying binary must load: %v", err)
	}
	if _, err := loadManifest(write("projects: {}\n"), "0.1.0"); err != nil {
		t.Fatalf("no engine field: no check: %v", err)
	}
	if _, err := loadManifest(write("someFutureField: 1\nprojects: {}\n"), "0.1.0"); err != nil {
		t.Fatalf("unknown fields must be ignored: %v", err)
	}
	if _, err := loadManifest(write("engine: \">=99.0.0\"\nprojects: {}\n"), "dev"); err != nil {
		t.Fatalf("dev builds always pass: %v", err)
	}
	// an unquoted YAML float ("engine: 0.2") is still a usable constraint
	if _, err := loadManifest(write("engine: 0.2\nprojects: {}\n"), "0.3.0"); err == nil {
		t.Fatal("engine: 0.2 is an exact-version constraint; 0.3.0 does not satisfy it")
	}
}
