package engine

import (
	"io/fs"
	"strings"
	"testing"
)

const nsManifest = `defaults:
  workspace: {{WS}}
  harnesses: [claude]
projects:
  p:
    repoRoot: p
    skills: [standup]
`

func TestNamespaceUnionsAllSources(t *testing.T) {
	e, _, _ := newConfig(t, nsManifest, map[string]string{
		"templates/skills/t1/SKILL.md":              skillMD("t1"),
		"global/skills/g1/SKILL.md":                 skillMD("g1"),
		"agents/designer/skills/r1/SKILL.md":        skillMD("r1"),
		"agents/designer/AGENT.md":                  "boot document, never installed",
		"templates/agents/a1.md.tmpl":               "---\nname: a1\ndescription: d\n---\n",
		"templates/agents/a2.md":                    "---\nname: a2\ndescription: d\n---\n",
		"templates/rules/r1.md.tmpl":                "---\ndescription: d\nalwaysApply: true\n---\n",
		"templates/partials/p.tmpl":                 `{{define "x"}}x{{end}}`,
		"templates/partials/ignored.txt":            "not a partial",
		"global/skills/.hidden/SKILL.md":            skillMD("hidden"),
		"agents/designer/skills/.DS_Store/SKILL.md": skillMD("ds"),
	})
	ns, err := e.Namespace()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for n, s := range ns.Skills {
		got[n] = s.Origin
	}
	want := map[string]string{
		"t1": OriginTemplates, "g1": OriginGlobal, "r1": OriginRole,
		"ship": OriginExamples, "standup": OriginExamples,
	}
	if len(got) != len(want) {
		t.Fatalf("skills %v, want %v", got, want)
	}
	for n, o := range want {
		if got[n] != o {
			t.Errorf("skill %s origin %q, want %q", n, got[n], o)
		}
	}
	if ns.Skills["g1"].Dir != "global/skills/g1" || ns.Skills["r1"].Dir != "agents/designer/skills/r1" || ns.Skills["t1"].Dir != "templates/skills/t1" || ns.Skills["ship"].Dir != "examples/skills/ship" {
		t.Errorf("source dirs wrong: %+v", ns.Skills)
	}
	if strings.Join(ns.AgentNames(), ",") != "a1,a2,gatekeeper" {
		t.Errorf("agents = %v", ns.AgentNames())
	}
	if strings.Join(ns.RuleNames(), ",") != "r1,repo-stance" {
		t.Errorf("rules = %v", ns.RuleNames())
	}
	if len(ns.partials) != 2 { // the embedded tracker.tmpl and the overlay's p.tmpl
		t.Errorf("partials = %v", ns.partials)
	}
}

// A name defined twice among the config sources is an error; the message names
// both directories.
func TestNamespaceCollisionIsAnError(t *testing.T) {
	cases := map[string]map[string]string{
		"overlay and global": {
			"templates/skills/dup/SKILL.md": skillMD("dup"),
			"global/skills/dup/SKILL.md":    skillMD("dup"),
		},
		"global and role": {
			"global/skills/dup/SKILL.md":          skillMD("dup"),
			"agents/designer/skills/dup/SKILL.md": skillMD("dup"),
		},
		"two roles": {
			"agents/a/skills/dup/SKILL.md": skillMD("dup"),
			"agents/b/skills/dup/SKILL.md": skillMD("dup"),
		},
		"overlay and role": {
			"templates/skills/dup/SKILL.md.tmpl":  skillMD("dup"),
			"agents/designer/skills/dup/SKILL.md": skillMD("dup"),
		},
	}
	for name, files := range cases {
		e, _, _ := newConfig(t, nsManifest, files)
		_, err := e.Namespace()
		if err == nil || !strings.Contains(err.Error(), `skill "dup" defined twice`) {
			t.Errorf("%s: want a duplicate error, got %v", name, err)
		}
		// validate reports it instead of crashing
		m, _ := e.LoadManifest()
		rep := e.Validate(m)
		found := false
		for _, p := range rep.Problems {
			found = found || strings.Contains(p.Msg, `skill "dup" defined twice`)
		}
		if !found {
			t.Errorf("%s: validate did not report the duplicate: %+v", name, rep.Problems)
		}
	}

	// the same agent as .md and .md.tmpl is ambiguous too
	e, _, _ := newConfig(t, nsManifest, map[string]string{
		"templates/agents/x.md":      "---\nname: x\ndescription: d\n---\n",
		"templates/agents/x.md.tmpl": "---\nname: x\ndescription: d\n---\n",
	})
	if _, err := e.Namespace(); err == nil || !strings.Contains(err.Error(), `agent "x" defined twice`) {
		t.Errorf("agent defined as .md and .md.tmpl: got %v", err)
	}
}

// The embedded examples are shadowed by a config source of the same name: no
// error, the config source wins, and the example vanishes from the namespace.
func TestExamplesAreShadowedByConfigSources(t *testing.T) {
	for name, path := range map[string]string{
		"global": "global/skills/standup/SKILL.md",
		"role":   "agents/designer/skills/standup/SKILL.md",
	} {
		e, _, _ := newConfig(t, nsManifest, map[string]string{
			path: "---\nname: standup\ndescription: Use for the config's own standup.\n---\n\nOWN STANDUP\n",
		})
		ns, err := e.Namespace()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if ns.Skills["standup"].Origin == OriginExamples {
			t.Fatalf("%s: the example must be shadowed", name)
		}
		got := renderEntry(t, e, "p")
		if !strings.Contains(got[".claude/skills/standup/SKILL.md"], "OWN STANDUP") {
			t.Errorf("%s: the config source must win:\n%s", name, got[".claude/skills/standup/SKILL.md"])
		}
	}

	// the templates overlay is the odd one out: it merges over an example by path
	e, _, _ := newConfig(t, nsManifest, map[string]string{
		"templates/skills/standup/SKILL.md": "---\nname: standup\ndescription: Overlaid standup.\n---\n\nOVERLAID\n",
		"templates/skills/standup/extra.md": "extra\n",
	})
	ns, err := e.Namespace()
	if err != nil || ns.Skills["standup"].Origin != OriginTemplates {
		t.Fatalf("overlay over an example: %v %+v", err, ns.Skills["standup"])
	}
	got := renderEntry(t, e, "p")
	if !strings.Contains(got[".claude/skills/standup/SKILL.md"], "OVERLAID") || got[".claude/skills/standup/extra.md"] != "extra\n" {
		t.Errorf("overlay must win and add files, got:\n%s", sortedPaths(got))
	}
}

func TestEmbeddedTemplatesAreComplete(t *testing.T) {
	tmpl := embeddedTemplates()
	reg, err := loadBundles(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg) == 0 {
		t.Fatal("embedded bundles.yaml lists no bundles")
	}
	e, _, _ := newConfig(t, nsManifest, nil)
	ns, err := e.Namespace()
	if err != nil {
		t.Fatal(err)
	}
	for bn, b := range reg {
		if err := ns.check(itemSet{skills: b.Skills, agents: b.Agents, rules: b.Rules}); err != nil {
			t.Errorf("bundle %s: %v", bn, err)
		}
	}
	// every embedded skill has an entry file; the partial exists
	for _, n := range ns.SkillNames() {
		if _, _, err := ns.Skills[n].Main(); err != nil {
			t.Error(err)
		}
	}
	if _, err := fs.Stat(tmpl, "partials/tracker.tmpl"); err != nil {
		t.Error(err)
	}
	// the demo is not embedded
	if _, err := fs.Stat(tmpl, "demo"); err == nil {
		t.Error("examples/demo must not be embedded in the binary")
	}
}

// The 0.2 layout is reported by validate instead of being silently ignored.
func TestValidateFlagsTheLegacyCursorRulesLayout(t *testing.T) {
	e, _, _ := newConfig(t, nsManifest, map[string]string{"templates/cursor/rules/x.mdc.tmpl": "---\ndescription: d\n---\n"})
	m, _ := e.LoadManifest()
	found := false
	for _, p := range e.Validate(m).Problems {
		found = found || strings.Contains(p.Msg, "templates/cursor/ is the 0.2 layout")
	}
	if !found {
		t.Error("validate must flag templates/cursor/")
	}
}
