package engine

import (
	"testing"
)

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
