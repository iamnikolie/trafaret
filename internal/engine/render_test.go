package engine

import (
	"path/filepath"
	"strings"
	"testing"
)

// --- tokens that replaced private prose ------------------------------------

func TestMemoryNameAndExtraFieldsTokens(t *testing.T) {
	renderOne := func(p Project, skill, file string) string {
		t.Helper()
		files, err := render(p, testEngine(t), []string{"example"}, []string{"claude"})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(filepath.ToSlash(f.path), skill+"/"+file) {
				return string(f.bytes)
			}
		}
		t.Fatalf("%s/%s not rendered", skill, file)
		return ""
	}

	// default: neutral wording, nothing deployment-specific
	p := testProject(t)
	impl := renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(impl, "`mem` memory CLI") || !strings.Contains(impl, "learnings back to memory.") {
		t.Errorf("default memory wording not rendered:\n%s", grepLines(impl, "memory"))
	}

	// configured through defaults, inherited by the project
	p = testProject(t)
	p.Tools.MemoryName = ""
	p.Tickets.DB, p.Tickets.Project = "Acme/Work", "Widgets"
	p.Tickets.ExtraFields = ""
	resolve(&p, Defaults{
		Tickets: struct {
			ExtraFields string `yaml:"extraFields"`
		}{ExtraFields: "`Acme/Seat` (who holds it)"},
		Tools: struct {
			MemoryName string `yaml:"memoryName"`
		}{MemoryName: "team memory"},
	})
	impl = renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(impl, "`mem` team-memory CLI") || !strings.Contains(impl, "learnings back to team memory.") {
		t.Errorf("memoryName default not applied:\n%s", grepLines(impl, "memory"))
	}
	wt := renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(wt, "(merge request URL), `Acme/Seat` (who holds it).\n") {
		t.Errorf("extraFields default not applied:\n%s", grepLines(wt, "merge request"))
	}

	// a project's own value wins over the default
	p.Tools.MemoryName = "own memory"
	p.Tickets.ExtraFields = "`Acme/Own` (own field)"
	resolve(&p, Defaults{})
	wt = renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(wt, "(merge request URL), `Acme/Own` (own field).\n") {
		t.Errorf("project extraFields must win:\n%s", grepLines(wt, "merge request"))
	}

	// no extra fields at all: the sentence still closes cleanly
	p.Tickets.ExtraFields = ""
	resolve(&p, Defaults{})
	wt = renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(wt, "(merge request URL).\n") {
		t.Errorf("empty extraFields must collapse:\n%s", grepLines(wt, "merge request"))
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
