package engine

import (
	"strings"
	"testing"
)

const renderManifest = "defaults:\n  workspace: {{WS}}\n  harnesses: [claude]\nprojects:\n  p:\n    repoRoot: p\n    skills: [s]\n    vars: {who: world, tracker: {kind: jira, key: WEB}}\n"

// Every .tmpl file of a skill sees the partials; every other file is copied
// byte for byte even when it contains {{ }}.
func TestPartialsAreParsedIntoEveryTmplAndNothingElse(t *testing.T) {
	e, _, _ := newConfig(t, renderManifest, map[string]string{
		"templates/partials/a.tmpl":        `{{define "greet"}}hello {{.Vars.who}}{{end}}`,
		"templates/partials/b.tmpl":        `{{/* only comments and defines count */}}{{define "ref"}}{{.Vars.tracker.key}}-1{{end}}stray text is ignored`,
		"templates/partials/notes.txt":     `{{define "greet"}}never loaded{{end}}`,
		"templates/skills/s/SKILL.md.tmpl": "---\nname: s\ndescription: d\n---\n{{template \"greet\" .}} / {{template \"ref\" .}}\n",
		"templates/skills/s/more.md.tmpl":  "{{template \"greet\" .}} again\n",
		"templates/skills/s/verbatim.md":   "{{template \"greet\" .}} stays literal\n",
	})
	got := renderEntry(t, e, "p")
	if !strings.HasSuffix(got[".claude/skills/s/SKILL.md"], "hello world / WEB-1\n") || got[".claude/skills/s/more.md"] != "hello world again\n" {
		t.Errorf("partials not applied:\n%q\n%q", got[".claude/skills/s/SKILL.md"], got[".claude/skills/s/more.md"])
	}
	if got[".claude/skills/s/verbatim.md"] != "{{template \"greet\" .}} stays literal\n" {
		t.Errorf("a non-.tmpl file must be copied byte for byte: %q", got[".claude/skills/s/verbatim.md"])
	}
	if _, ok := got[".claude/skills/s/SKILL.md.tmpl"]; ok {
		t.Error("the .tmpl suffix must be dropped")
	}
}

func TestOverlayPartialShadowsTheEmbeddedOne(t *testing.T) {
	manifest := "defaults:\n  workspace: {{WS}}\n  harnesses: [claude]\nprojects:\n  p:\n    repoRoot: p\n    skills: [s]\n    vars: {tracker: {kind: github, repo: a/b}}\n"
	e, _, _ := newConfig(t, manifest, map[string]string{
		"templates/skills/s/SKILL.md.tmpl": "---\nname: s\ndescription: d\n---\n{{template \"issueRef\" .}}\n",
		"templates/partials/tracker.tmpl":  `{{define "issueRef"}}OWN-{{.Vars.tracker.kind}}{{end}}`,
	})
	if got := renderEntry(t, e, "p")[".claude/skills/s/SKILL.md"]; !strings.HasSuffix(got, "OWN-github\n") {
		t.Errorf("the overlay partial at the same path must win: %q", got)
	}
	// without the overlay the embedded partial is there
	e, _, _ = newConfig(t, manifest, map[string]string{
		"templates/skills/s/SKILL.md.tmpl": "---\nname: s\ndescription: d\n---\n{{template \"issueRef\" .}}\n",
	})
	if got := renderEntry(t, e, "p")[".claude/skills/s/SKILL.md"]; !strings.HasSuffix(got, "#123\n") {
		t.Errorf("embedded partial: %q", got)
	}
}

func TestTemplateErrorsNameTheirFile(t *testing.T) {
	for name, c := range map[string]struct {
		files map[string]string
		want  string
	}{
		"parse error":      {map[string]string{"templates/skills/s/SKILL.md.tmpl": "---\nname: s\ndescription: d\n---\n{{if}}\n"}, "templates/skills/s/SKILL.md.tmpl"},
		"unknown template": {map[string]string{"templates/skills/s/SKILL.md.tmpl": "---\nname: s\ndescription: d\n---\n{{template \"nope\" .}}\n"}, `template "nope" not defined`},
		"partial parse error": {map[string]string{
			"templates/skills/s/SKILL.md": skillMD("s"),
			"templates/partials/bad.tmpl": "{{define \"x\"}}{{if}}{{end}}",
		}, "partials/bad.tmpl"},
	} {
		e, _, _ := newConfig(t, renderManifest, c.files)
		if _, err := renderEntryErr(t, e, "p"); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want an error mentioning %q, got %v", name, c.want, err)
		}
	}
}

func TestFrontmatterLintRunsOnEveryEmittedFile(t *testing.T) {
	cases := map[string]struct {
		files map[string]string
		want  string
	}{
		"skill without a description": {map[string]string{"templates/skills/s/SKILL.md": "---\nname: s\n---\nbody\n"}, `missing non-empty "description"`},
		"unquoted ' #'":               {map[string]string{"templates/skills/s/SKILL.md.tmpl": "---\nname: s\ndescription: Use {{.Vars.who}} # not a comment\n---\nbody\n"}, "contains ' #' in an unquoted value"},
		"not yaml":                    {map[string]string{"templates/skills/s/SKILL.md": "---\nname: s\ndescription: a: b\n---\nbody\n"}, "not valid YAML"},
		"unterminated":                {map[string]string{"templates/skills/s/SKILL.md": "---\nname: s\ndescription: d\n"}, "unterminated frontmatter"},
	}
	for name, c := range cases {
		e, _, _ := newConfig(t, renderManifest, c.files)
		if _, err := renderEntryErr(t, e, "p"); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want %q, got %v", name, c.want, err)
		}
	}
	// homes are linted too (v0.2 linted verbatim home files)
	e, _, _ := newConfig(t, "homes:\n  h:\n    harness: claude\n    dir: {{HOME}}/h\n", map[string]string{"global/skills/g/SKILL.md": "---\nname: g\n---\nbody\n"})
	if _, err := renderEntryErr(t, e, "home:h"); err == nil || !strings.Contains(err.Error(), "frontmatter") {
		t.Errorf("home lint: %v", err)
	}
}

// Rules are Markdown sources that Cursor gets as .mdc; Claude and Codex skip them.
func TestRulesRenderAsMDCOnlyForCursor(t *testing.T) {
	manifest := "defaults:\n  workspace: {{WS}}\nprojects:\n  p:\n    repoRoot: p\n    harnesses: [claude, codex]\n    rules: [r]\n"
	e, _, _ := newConfig(t, manifest, adapterFiles)
	if got := renderEntry(t, e, "p"); len(got) != 0 {
		t.Fatalf("no harness that takes rules: %v", paths(got))
	}
	e, _, _ = newConfig(t, strings.Replace(manifest, "[claude, codex]", "[cursor]", 1), map[string]string{"templates/rules/r.md": "---\ndescription: plain rule\nalwaysApply: false\n---\n\nverbatim {{ rule }}\n"})
	got := renderEntry(t, e, "p")
	if strings.Join(paths(got), ",") != ".cursor/rules/r.mdc" || !strings.Contains(got[".cursor/rules/r.mdc"], "verbatim {{ rule }}") {
		t.Fatalf("got %v", got)
	}
}

func TestTemplateFuncsDirectly(t *testing.T) {
	join := templateFuncs["join"].(func(any, string) (string, error))
	for _, c := range []struct {
		in   any
		want string
	}{
		{[]any{"a", 1, true}, "a, 1, true"}, {[]string{"x", "y"}, "x, y"}, {nil, ""}, {[]any{}, ""},
	} {
		if got, err := join(c.in, ", "); err != nil || got != c.want {
			t.Errorf("join(%v) = %q %v, want %q", c.in, got, err, c.want)
		}
	}
	if _, err := join("not a list", ","); err == nil {
		t.Error("join of a scalar must fail")
	}
	def := templateFuncs["default"].(func(any, any) any)
	for _, empty := range []any{nil, "", 0, false, []any{}, map[string]any{}} {
		if got := def("fallback", empty); got != "fallback" {
			t.Errorf("default over %#v = %v", empty, got)
		}
	}
	for _, set := range []any{"x", 1, true, []any{1}} {
		if got := def("fallback", set); got == "fallback" {
			t.Errorf("default over %#v fell back", set)
		}
	}
}
