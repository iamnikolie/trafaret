package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// libFiles is a small library: two global skills, a role skill, a templates
// skill and a templates agent.
var libFiles = map[string]string{
	"global/skills/alpha/SKILL.md":           skillMD("alpha"),
	"global/skills/beta/SKILL.md":            skillMD("beta"),
	"agents/designer/skills/design/SKILL.md": skillMD("design"),
	"templates/skills/proc/SKILL.md.tmpl":    "---\nname: proc\ndescription: Use for proc in {{.Name}}.\n---\n\nbranch {{.Vars.branch}}\n",
	"templates/agents/rev.md.tmpl":           "---\nname: rev\ndescription: Reviewer.\ntools: Read\n---\n\nReview.\n",
	"templates/bundles.yaml":                 "tools:\n  skills: [proc]\n  agents: [rev]\n",
}

func homeManifest(home, extra string) string {
	return "vars: {branch: main}\nhomes:\n  h:\n    harness: claude\n    dir: " + home + "\n" + extra
}

// Omitted skills: means what it meant in v0.2 — every global and role skill —
// and NOT the templates overlay's skills or the embedded examples.
func TestHomeSkillsOmittedMeansGlobalOnly(t *testing.T) {
	e, _, home := newConfig(t, homeManifest("{{HOME}}/.claude", ""), libFiles)
	got := renderEntry(t, e, "home:h")
	want := "skills/alpha/SKILL.md\nskills/beta/SKILL.md\nskills/design/SKILL.md"
	if strings.Join(paths(got), "\n") != want {
		t.Fatalf("omitted skills: got %v, want global + role skills only", paths(got))
	}
	_ = home
}

// Overlay and example skills (and agents) reach a home when named, or by bundle.
func TestHomeExplicitListsAndBundles(t *testing.T) {
	e, _, _ := newConfig(t, homeManifest("{{HOME}}/.claude", "    skills: [alpha, proc, standup]\n    agents: [rev]\n    vars: {branch: trunk}\n"), libFiles)
	got := renderEntry(t, e, "home:h")
	if strings.Join(paths(got), "\n") != "agents/rev.md\nskills/alpha/SKILL.md\nskills/proc/SKILL.md\nskills/standup/SKILL.md" {
		t.Fatalf("explicit lists: got %v", paths(got))
	}
	if !strings.Contains(got["skills/proc/SKILL.md"], "branch trunk") || !strings.Contains(got["skills/proc/SKILL.md"], "proc in h.") {
		t.Errorf("a templates skill renders with the home's vars:\n%s", got["skills/proc/SKILL.md"])
	}

	e, _, _ = newConfig(t, homeManifest("{{HOME}}/.claude", "    skills: []\n    bundles: [tools]\n"), libFiles)
	got = renderEntry(t, e, "home:h")
	if strings.Join(paths(got), "\n") != "agents/rev.md\nskills/proc/SKILL.md" {
		t.Fatalf("bundle only (explicit empty skills): got %v", paths(got))
	}

	e, _, _ = newConfig(t, homeManifest("{{HOME}}/.claude", "    skills: []\n"), libFiles)
	if got := renderEntry(t, e, "home:h"); len(got) != 0 {
		t.Fatalf("skills: [] means none, got %v", paths(got))
	}
}

func TestHomeSkipLeavesOutNamedSkills(t *testing.T) {
	e, _, _ := newConfig(t, homeManifest("{{HOME}}/.claude", "    skip: [beta, design]\n"), libFiles)
	if got := renderEntry(t, e, "home:h"); strings.Join(paths(got), "\n") != "skills/alpha/SKILL.md" {
		t.Fatalf("want only alpha, got %v", paths(got))
	}
	// skip also removes what a bundle brings
	e, _, _ = newConfig(t, homeManifest("{{HOME}}/.claude", "    skills: []\n    bundles: [tools]\n    skip: [rev]\n"), libFiles)
	if got := renderEntry(t, e, "home:h"); strings.Join(paths(got), "\n") != "skills/proc/SKILL.md" {
		t.Fatalf("skip must apply to bundle agents, got %v", paths(got))
	}

	e, _, _ = newConfig(t, homeManifest("{{HOME}}/.claude", "    skip: [gamma]\n"), libFiles)
	if _, err := renderEntryErr(t, e, "home:h"); err == nil || !strings.Contains(err.Error(), `skip names no skill, agent or rule "gamma"`) {
		t.Fatalf("skip naming an unknown name must fail, got %v", err)
	}
	e, _, _ = newConfig(t, homeManifest("{{HOME}}/.claude", "    skills: [gamma]\n"), libFiles)
	if _, err := renderEntryErr(t, e, "home:h"); err == nil || !strings.Contains(err.Error(), `no skill "gamma"`) {
		t.Fatalf("an unknown skill must fail, got %v", err)
	}
}

func readTrackFile(t *testing.T, p string) []string {
	t.Helper()
	var out []string
	if err := json.Unmarshal([]byte(readFile(t, p)), &out); err != nil {
		t.Fatalf("%s: %v", p, err)
	}
	return out
}

// A home install tracks per written directory and prunes only what it wrote.
func TestHomeInstallPrunesOnlyWhatItWrote(t *testing.T) {
	manifest := homeManifest("{{HOME}}/.claude", "    skills: [alpha, beta, proc]\n    agents: [rev]\n")
	e, _, home := newConfig(t, manifest, libFiles)
	dir := filepath.Join(home, ".claude")
	foreign := filepath.Join(dir, "skills", ".system", "SKILL.md")
	foreignAgent := filepath.Join(dir, "agents", "mine.md")
	for _, f := range []string{foreign, foreignAgent} {
		writeTree(t, filepath.Dir(f), map[string]string{filepath.Base(f): "harness-owned"})
	}

	res := installOnly(t, e, InstallOptions{All: true})
	if c := statuses(res, "home:h"); c["install"] != 4 || len(c) != 1 {
		t.Fatalf("first install: %v", c)
	}
	if got := readTrackFile(t, filepath.Join(dir, "skills", ".ao-manifest.json")); strings.Join(got, ",") != "alpha/SKILL.md,beta/SKILL.md,proc/SKILL.md" {
		t.Errorf("skills track file: %v", got)
	}
	if got := readTrackFile(t, filepath.Join(dir, "agents", ".ao-manifest.json")); strings.Join(got, ",") != "rev.md" {
		t.Errorf("agents track file: %v", got)
	}
	if fileExists(filepath.Join(dir, "rules", ".ao-manifest.json")) {
		t.Error("a directory that never held anything gets no track file")
	}

	// unchanged: everything skips
	if c := statuses(installOnly(t, e, InstallOptions{All: true}), "home:h"); c["skip"] != 4 || len(c) != 1 {
		t.Fatalf("second install: %v", c)
	}

	// narrow the home: beta and the agent leave, nothing foreign is touched
	writeTree(t, e.ConfigDir, map[string]string{"manifest.yaml": strings.ReplaceAll(homeManifest("{{HOME}}/.claude", "    skills: [alpha, proc]\n"), "{{HOME}}", home)})
	res = installOnly(t, e, InstallOptions{All: true})
	var removed []string
	for _, f := range res[0].Files {
		if f.Status == "remove" {
			removed = append(removed, strings.TrimPrefix(f.Path, dir+"/"))
		}
	}
	if strings.Join(removed, ",") != "skills/beta/SKILL.md,agents/rev.md" {
		t.Fatalf("removed %v", removed)
	}
	for _, gone := range []string{filepath.Join(dir, "skills", "beta"), filepath.Join(dir, "agents", "rev.md")} {
		if fileExists(gone) {
			t.Errorf("%s should be pruned", gone)
		}
	}
	for _, kept := range []string{foreign, foreignAgent, filepath.Join(dir, "skills", "alpha", "SKILL.md")} {
		if !fileExists(kept) {
			t.Errorf("%s must survive", kept)
		}
	}
	if got := readTrackFile(t, filepath.Join(dir, "agents", ".ao-manifest.json")); len(got) != 0 {
		t.Errorf("agents track file should now be empty, got %v", got)
	}

	// --dry-run reports a removal without doing it
	writeTree(t, e.ConfigDir, map[string]string{"manifest.yaml": strings.ReplaceAll(homeManifest("{{HOME}}/.claude", "    skills: [alpha]\n"), "{{HOME}}", home)})
	res = installOnly(t, e, InstallOptions{All: true, DryRun: true})
	if c := statuses(res, "home:h"); c["remove"] != 1 || !fileExists(filepath.Join(dir, "skills", "proc", "SKILL.md")) {
		t.Fatalf("dry-run must report but not remove: %v", c)
	}
}

func TestHomeTakesRoleSkillsAndRejectsDuplicates(t *testing.T) {
	e, _, _ := newConfig(t, homeManifest("{{HOME}}/.claude", ""), map[string]string{
		"global/skills/alpha/SKILL.md":           skillMD("alpha"),
		"agents/designer/skills/review/SKILL.md": skillMD("review"),
	})
	if got := renderEntry(t, e, "home:h"); strings.Join(paths(got), "\n") != "skills/alpha/SKILL.md\nskills/review/SKILL.md" {
		t.Fatalf("want alpha + review, got %v", paths(got))
	}
	e, _, _ = newConfig(t, homeManifest("{{HOME}}/.claude", ""), map[string]string{
		"global/skills/alpha/SKILL.md":       skillMD("alpha"),
		"agents/other/skills/alpha/SKILL.md": skillMD("alpha"),
	})
	if _, err := renderEntryErr(t, e, "home:h"); err == nil || !strings.Contains(err.Error(), `skill "alpha" defined twice`) {
		t.Fatalf("a name in global/skills and agents/*/skills must fail, got %v", err)
	}
}

// A global skill is verbatim unless a file ends in .tmpl: those render with the
// home's vars, the rest is copied byte for byte (braces and all).
func TestGlobalSkillFilesRenderOnlyWhenTmpl(t *testing.T) {
	e, _, _ := newConfig(t, homeManifest("{{HOME}}/.claude", ""), map[string]string{
		"global/skills/g/SKILL.md":       "---\nname: g\ndescription: Use for g.\n---\n\nliteral {{ braces }} stay\n",
		"global/skills/g/ctx.md.tmpl":    "branch={{.Vars.branch}} dir={{.TargetRoot}}\n",
		"global/skills/g/scripts/run.sh": "#!/bin/sh\necho '{{.nope}}'\n",
	})
	got := renderEntry(t, e, "home:h")
	if !strings.Contains(got["skills/g/SKILL.md"], "literal {{ braces }} stay") || got["skills/g/scripts/run.sh"] != "#!/bin/sh\necho '{{.nope}}'\n" {
		t.Errorf("non-.tmpl files must be copied byte for byte:\n%v", got)
	}
	if !strings.HasPrefix(got["skills/g/ctx.md"], "branch=main dir=") || strings.Contains(got["skills/g/ctx.md.tmpl"], "x") {
		t.Errorf("a .tmpl file renders, suffix dropped: %v", paths(got))
	}
}

// A home installed by v0.2 (global skills copied verbatim, track file in the
// skills dir) reads as all-skip once its config is migrated.
func TestHomeMigratesFromV02State(t *testing.T) {
	manifest := "homes:\n  h:\n    harness: claude\n    dir: {{HOME}}/.claude\n    skip: [beta]\n"
	e, _, home := newConfig(t, manifest, map[string]string{
		"global/skills/alpha/SKILL.md":           skillMD("alpha"),
		"global/skills/alpha/scripts/helper.py":  "print('{{ not a template }}')\n",
		"global/skills/beta/SKILL.md":            skillMD("beta"),
		"agents/designer/skills/design/SKILL.md": skillMD("design"),
	})
	skills := filepath.Join(home, ".claude", "skills")
	// what v0.2.0 wrote
	writeTree(t, skills, map[string]string{
		"alpha/SKILL.md":          skillMD("alpha"),
		"alpha/scripts/helper.py": "print('{{ not a template }}')\n",
		"design/SKILL.md":         skillMD("design"),
		"beta/SKILL.md":           skillMD("beta"),
		".ao-manifest.json":       "[\n  \"alpha/SKILL.md\",\n  \"alpha/scripts/helper.py\",\n  \"beta/SKILL.md\",\n  \"design/SKILL.md\"\n]\n",
	})
	res := installOnly(t, e, InstallOptions{All: true, DryRun: true})
	c := statuses(res, "home:h")
	if c["skip"] != 3 || c["remove"] != 1 || len(c) != 2 {
		t.Fatalf("dry-run over a v0.2 install: %v (want 3 skip; beta was skipped by the config so it is removed)", c)
	}
	// without the skip, nothing at all changes
	writeTree(t, e.ConfigDir, map[string]string{"manifest.yaml": strings.ReplaceAll("homes:\n  h:\n    harness: claude\n    dir: {{HOME}}/.claude\n", "{{HOME}}", home)})
	c = statuses(installOnly(t, e, InstallOptions{All: true, DryRun: true}), "home:h")
	if c["skip"] != 4 || len(c) != 1 {
		t.Fatalf("a migrated home must be all-skip: %v", c)
	}
}

func TestHomeTargetAndWhatFilters(t *testing.T) {
	manifest := "homes:\n  hk:\n    harness: claude\n    dir: {{HOME}}/k\n    skills: [alpha]\n    bundles: [tools]\n  hc:\n    harness: codex\n    dir: {{HOME}}/c\n    skills: [alpha]\nvars: {branch: main}\n"
	e, _, home := newConfig(t, manifest, libFiles)

	res := installOnly(t, e, InstallOptions{All: true, Target: "codex", DryRun: true})
	if len(res) != 1 || res[0].Project != "home:hc" {
		t.Fatalf("--target codex must select only the codex home: %+v", res)
	}
	res = installOnly(t, e, InstallOptions{All: true, Target: "both", DryRun: true})
	if len(res) != 1 || res[0].Project != "home:hk" {
		t.Fatalf("--target both = claude+cursor: %+v", res)
	}

	// --what installs only that bundle's contents and prunes nothing
	installOnly(t, e, InstallOptions{Name: "hk"})
	writeTree(t, e.ConfigDir, map[string]string{"manifest.yaml": strings.ReplaceAll(strings.ReplaceAll(manifest, "{{HOME}}", home), "bundles: [tools]", "bundles: []")})
	res = installOnly(t, e, InstallOptions{Name: "hk", What: []string{"tools"}})
	if len(res) != 1 || len(res[0].Files) != 0 {
		t.Fatalf("--what naming a bundle the home no longer has installs nothing: %+v", res)
	}
	if !fileExists(filepath.Join(home, "k", "skills", "proc", "SKILL.md")) {
		t.Error("--what must not prune")
	}
	// a full install does prune the bundle's leftovers
	res = installOnly(t, e, InstallOptions{Name: "hk"})
	if c := statuses(res, "home:hk"); c["remove"] != 2 {
		t.Fatalf("full install must prune the dropped bundle: %v", c)
	}
}

func TestHomeNeedsAnAbsoluteDirAndKnownHarness(t *testing.T) {
	e, _, _ := newConfig(t, "homes:\n  a:\n    harness: claude\n    dir: relative/dir\n  b:\n    harness: vim\n    dir: /x\n", nil)
	m, _ := e.LoadManifest()
	rep := e.Validate(m)
	var msgs []string
	for _, p := range rep.Problems {
		msgs = append(msgs, p.Where+": "+p.Msg)
	}
	all := strings.Join(msgs, "\n")
	if !strings.Contains(all, `home:a: dir must be an absolute or ~/ path, got "relative/dir"`) || !strings.Contains(all, `home:b: harness "vim" is not one of claude|codex|cursor`) {
		t.Fatalf("validate: %v", msgs)
	}
	if _, err := e.Install(InstallOptions{Name: "a"}); err == nil || !strings.Contains(err.Error(), "dir must be") {
		t.Fatalf("install must refuse a relative dir: %v", err)
	}
	_ = os.Stdout
}
