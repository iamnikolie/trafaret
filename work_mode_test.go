package main

import "testing"

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
