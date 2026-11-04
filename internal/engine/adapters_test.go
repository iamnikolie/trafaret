package engine

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// adapterFiles is a tiny library that exercises every item kind: a skill with a
// nested rendered file, an agent that Codex converts, and a rule.
var adapterFiles = map[string]string{
	"templates/skills/s/SKILL.md.tmpl":     "---\nname: s\ndescription: Use for s.\n---\n\n{{.Harness}}|{{.Kind}}|{{.Name}}|{{.TargetRoot}}|{{.RepoRoot}}\n",
	"templates/skills/s/scripts/x.sh.tmpl": "#!/bin/sh\necho {{.Harness}}\n",
	"templates/skills/s/scripts/plain.sh":  "echo {{ not rendered }}\n",
	"templates/agents/a.md.tmpl":           "---\nname: a\ndescription: Agent for {{.Name}}.\ntools: Bash, Read\nmodel: sonnet\n---\n\nTargetRoot={{.TargetRoot}}\n",
	"templates/rules/r.md.tmpl":            "---\ndescription: Rule for {{.Name}}.\nalwaysApply: true\n---\n\nRule {{.Harness}}\n",
}

func paths(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestAdapterPathsForProjects(t *testing.T) {
	cases := []struct {
		harnesses string
		want      []string
	}{
		{"[claude]", []string{".claude/agents/a.md", ".claude/skills/s/SKILL.md", ".claude/skills/s/scripts/plain.sh", ".claude/skills/s/scripts/x.sh"}},
		{"[codex]", []string{".agents/agents/a.md", ".agents/skills/s/SKILL.md", ".agents/skills/s/scripts/plain.sh", ".agents/skills/s/scripts/x.sh", ".codex/agents/a.toml"}},
		{"[cursor]", []string{".claude/agents/a.md", ".claude/skills/s/SKILL.md", ".claude/skills/s/scripts/plain.sh", ".claude/skills/s/scripts/x.sh", ".cursor/rules/r.mdc"}},
		{"[claude, cursor]", []string{".claude/agents/a.md", ".claude/skills/s/SKILL.md", ".claude/skills/s/scripts/plain.sh", ".claude/skills/s/scripts/x.sh", ".cursor/rules/r.mdc"}},
		{"[claude, codex, cursor]", []string{
			".agents/agents/a.md", ".agents/skills/s/SKILL.md", ".agents/skills/s/scripts/plain.sh", ".agents/skills/s/scripts/x.sh",
			".claude/agents/a.md", ".claude/skills/s/SKILL.md", ".claude/skills/s/scripts/plain.sh", ".claude/skills/s/scripts/x.sh",
			".codex/agents/a.toml", ".cursor/rules/r.mdc"}},
	}
	for _, c := range cases {
		manifest := "defaults:\n  workspace: {{WS}}\nprojects:\n  p:\n    repoRoot: p\n    harnesses: " + c.harnesses + "\n    skills: [s]\n    agents: [a]\n    rules: [r]\n"
		e, _, _ := newConfig(t, manifest, adapterFiles)
		got := renderEntry(t, e, "p")
		if strings.Join(paths(got), "\n") != strings.Join(c.want, "\n") {
			t.Errorf("harnesses %s:\n got  %v\n want %v", c.harnesses, paths(got), c.want)
		}
	}
}

// The data a template sees differs per placement: .Harness is the harness the
// copy is rendered for, .TargetRoot the tree it lands in. The shared .claude
// tree is rendered once, by claude when claude is among the harnesses.
func TestAdapterTemplateDataPerPlacement(t *testing.T) {
	render := func(harnesses string) map[string]string {
		manifest := "defaults:\n  workspace: {{WS}}\nprojects:\n  p:\n    repoRoot: p\n    harnesses: " + harnesses + "\n    skills: [s]\n    agents: [a]\n    rules: [r]\n"
		e, ws, _ := newConfig(t, manifest, adapterFiles)
		got := renderEntry(t, e, "p")
		got["__ws"] = ws
		return got
	}
	all := render("[claude, codex, cursor]")
	repo := filepath.Join(all["__ws"], "p")
	if got, want := strings.TrimSpace(strings.SplitN(all[".claude/skills/s/SKILL.md"], "\n\n", 2)[1]), "claude|project|p|.claude|"+repo; got != want {
		t.Errorf("claude tree: %q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(strings.SplitN(all[".agents/skills/s/SKILL.md"], "\n\n", 2)[1]), "codex|project|p|.agents|"+repo; got != want {
		t.Errorf("codex tree: %q, want %q", got, want)
	}
	if !strings.Contains(all[".cursor/rules/r.mdc"], "Rule cursor") {
		t.Errorf("cursor rule: %q", all[".cursor/rules/r.mdc"])
	}
	if !strings.Contains(all[".agents/agents/a.md"], "TargetRoot=.agents") || !strings.Contains(all[".claude/agents/a.md"], "TargetRoot=.claude") {
		t.Errorf("agent TargetRoot per tree wrong:\n%s\n%s", all[".agents/agents/a.md"], all[".claude/agents/a.md"])
	}

	// Cursor alone renders the shared tree itself, as cursor
	cur := render("[cursor]")
	if !strings.HasPrefix(strings.TrimSpace(strings.SplitN(cur[".claude/skills/s/SKILL.md"], "\n\n", 2)[1]), "cursor|project|p|.claude|") {
		t.Errorf("cursor-only shared tree: %q", cur[".claude/skills/s/SKILL.md"])
	}
}

func TestAdapterPathsForHomes(t *testing.T) {
	cases := []struct {
		harness string
		want    []string
	}{
		{"claude", []string{"agents/a.md", "skills/s/SKILL.md", "skills/s/scripts/plain.sh", "skills/s/scripts/x.sh"}},
		{"codex", []string{"agents/a.toml", "skills/s/SKILL.md", "skills/s/scripts/plain.sh", "skills/s/scripts/x.sh"}},
		{"cursor", []string{"agents/a.md", "rules/r.mdc", "skills/s/SKILL.md", "skills/s/scripts/plain.sh", "skills/s/scripts/x.sh"}},
	}
	for _, c := range cases {
		manifest := "homes:\n  h:\n    harness: " + c.harness + "\n    dir: {{HOME}}/." + c.harness + "\n    skills: [s]\n    agents: [a]\n    rules: [r]\n"
		e, _, home := newConfig(t, manifest, adapterFiles)
		got := renderEntry(t, e, "home:h")
		if strings.Join(paths(got), "\n") != strings.Join(c.want, "\n") {
			t.Errorf("home %s:\n got  %v\n want %v", c.harness, paths(got), c.want)
		}
		// a home's TargetRoot is its absolute dir, RepoRoot is empty
		wantLine := c.harness + "|home|h|" + filepath.Join(home, "."+c.harness) + "|"
		if !strings.Contains(got["skills/s/SKILL.md"], wantLine+"\n") {
			t.Errorf("home %s data line: want %q in %q", c.harness, wantLine, got["skills/s/SKILL.md"])
		}
	}
}

// A home's Codex agent is the TOML form; there is no readable copy in a home.
func TestHomeAgentsConvertToCodexTOML(t *testing.T) {
	manifest := "homes:\n  c:\n    harness: codex\n    dir: {{HOME}}/.codex\n    agents: [a]\n    skills: []\n  k:\n    harness: claude\n    dir: {{HOME}}/.claude\n    agents: [a]\n    skills: []\n"
	e, _, home := newConfig(t, manifest, adapterFiles)
	installOnly(t, e, InstallOptions{All: true})
	toml := readFile(t, filepath.Join(home, ".codex", "agents", "a.toml"))
	for _, want := range []string{`name = "a"`, `description = "Agent for c."`, `model = "gpt-5.6-terra"`, `sandbox_mode = "read-only"`, "TargetRoot=" + filepath.Join(home, ".codex")} {
		if !strings.Contains(toml, want) {
			t.Errorf("home TOML lacks %q:\n%s", want, toml)
		}
	}
	md := readFile(t, filepath.Join(home, ".claude", "agents", "a.md"))
	if !strings.Contains(md, "description: Agent for k.") {
		t.Errorf("claude home agent not rendered for the home:\n%s", md)
	}
	if fileExists(filepath.Join(home, ".codex", "agents", "a.md")) {
		t.Error("a Codex home gets TOML only")
	}
}

func TestHarnessConditionalRender(t *testing.T) {
	files := map[string]string{
		"templates/skills/s/SKILL.md.tmpl": "---\nname: s\ndescription: d\n---\n{{if eq .Harness \"codex\"}}use the codex agent{{else}}dispatch the subagent{{end}}\n",
	}
	manifest := "defaults:\n  workspace: {{WS}}\nprojects:\n  p:\n    repoRoot: p\n    harnesses: [claude, codex]\n    skills: [s]\nhomes:\n  hc:\n    harness: codex\n    dir: {{HOME}}/c\n    skills: [s]\n  hk:\n    harness: claude\n    dir: {{HOME}}/k\n    skills: [s]\n"
	e, _, _ := newConfig(t, manifest, files)
	p := renderEntry(t, e, "p")
	if !strings.Contains(p[".agents/skills/s/SKILL.md"], "use the codex agent") || !strings.Contains(p[".claude/skills/s/SKILL.md"], "dispatch the subagent") {
		t.Errorf("project: %q / %q", p[".agents/skills/s/SKILL.md"], p[".claude/skills/s/SKILL.md"])
	}
	if !strings.Contains(renderEntry(t, e, "home:hc")["skills/s/SKILL.md"], "use the codex agent") ||
		!strings.Contains(renderEntry(t, e, "home:hk")["skills/s/SKILL.md"], "dispatch the subagent") {
		t.Error("homes must render per their own harness")
	}
}

func TestTargetListAndAdapterTable(t *testing.T) {
	for in, want := range map[string]string{
		"claude": "claude", "codex": "codex", "cursor": "cursor",
		"both": "claude,cursor", "all": "claude,codex,cursor",
	} {
		got, err := targetList(in)
		if err != nil || strings.Join(got, ",") != want {
			t.Errorf("targetList(%q) = %v %v, want %s", in, got, err, want)
		}
	}
	if _, err := targetList("vim"); err == nil || !strings.Contains(err.Error(), "invalid --target (claude|codex|cursor|both|all): vim") {
		t.Errorf("bad target: %v", err)
	}
	if err := checkHarnesses([]string{"claude", "emacs"}); err == nil || !strings.Contains(err.Error(), `"emacs"`) {
		t.Errorf("unknown harness: %v", err)
	}
	// every adapter owns at least one root, and every home layout is zone-prefixed
	for _, a := range adapters {
		if len(a.roots) == 0 {
			t.Errorf("%s has no roots", a.name)
		}
		for _, z := range homeZones(a.name) {
			if z != "skills" && z != "agents" && z != "rules" {
				t.Errorf("%s: unexpected home zone %q", a.name, z)
			}
		}
	}
	if !pathBelongsToTargets(".claude/skills/x", []string{"cursor"}) || pathBelongsToTargets(".agents/skills/x", []string{"claude", "cursor"}) ||
		!pathBelongsToTargets(".codex/agents/x.toml", []string{"codex"}) || pathBelongsToTargets("elsewhere/x", []string{"claude", "codex", "cursor"}) {
		t.Error("pathBelongsToTargets ownership wrong")
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := osReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
