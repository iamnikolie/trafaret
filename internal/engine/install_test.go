package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// projFiles: a bundle with a session context, a script and a plain skill.
var projFiles = map[string]string{
	"templates/bundles.yaml":              "proc:\n  skills: [proc]\n  agents: [rev]\n  rules: [stance]\n  sessionContext: proc/ctx.md\nplain:\n  skills: [plain]\n",
	"templates/skills/proc/SKILL.md.tmpl": "---\nname: proc\ndescription: Use for proc in {{.Name}}.\n---\n\nrun {{.TargetRoot}}/skills/proc/go.sh\n",
	"templates/skills/proc/ctx.md.tmpl":   "<proc>{{.Name}}</proc>\n",
	"templates/skills/proc/go.sh.tmpl":    "#!/bin/sh\necho {{.Name}}\n",
	"templates/skills/plain/SKILL.md":     skillMD("plain"),
	"templates/agents/rev.md.tmpl":        "---\nname: rev\ndescription: Reviewer of {{.Name}}.\ntools: Read\n---\n\nReview.\n",
	"templates/rules/stance.md.tmpl":      "---\ndescription: Stance of {{.Name}}.\nalwaysApply: true\n---\n\nStance.\n",
}

func projManifest(extra string) string {
	return "defaults:\n  workspace: {{WS}}\n  harnesses: [claude, codex, cursor]\nprojects:\n  p:\n    repoRoot: p\n" + extra
}

func rewriteManifest(t *testing.T, e *Engine, ws, home, body string) {
	t.Helper()
	body = strings.ReplaceAll(strings.ReplaceAll(body, "{{WS}}", ws), "{{HOME}}", home)
	writeTree(t, e.ConfigDir, map[string]string{"manifest.yaml": body})
}

func TestProjectInstallIsIdempotentAndForceRewrites(t *testing.T) {
	e, ws, _ := newConfig(t, projManifest("    bundles: [proc]\n"), projFiles)
	repo := filepath.Join(ws, "p")

	c := statuses(installOnly(t, e, InstallOptions{Name: "p"}), "p")
	// skill (3 files) x claude+codex, agent x (claude md, codex md+toml), cursor rule, settings.json
	if c["install"] != 3*2+3+1+1 || len(c) != 1 {
		t.Fatalf("first install: %v", c)
	}
	c = statuses(installOnly(t, e, InstallOptions{Name: "p"}), "p")
	if c["skip"] != 10 || len(c) != 1 {
		t.Fatalf("second install must skip everything but the hook (no change): %v", c)
	}
	c = statuses(installOnly(t, e, InstallOptions{Name: "p", Force: true}), "p")
	if c["skip"] != 10 { // force rewrites but still reports sha-equal as skip
		t.Fatalf("force: %v", c)
	}

	// the track file and the hook
	track := readTrackFile(t, filepath.Join(repo, ".claude", "ao-manifest.json"))
	if len(track) != 10 || strings.Contains(strings.Join(track, ","), "settings.json") {
		t.Fatalf("track file: %v", track)
	}
	if !strings.Contains(readFile(t, filepath.Join(repo, ".claude", "settings.json")), `skills/proc/ctx.md`) {
		t.Error("session hook not written")
	}

	// an edited rendered file is an update, then back to skip
	f := filepath.Join(repo, ".agents", "skills", "proc", "SKILL.md")
	if err := os.WriteFile(f, []byte("drifted"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = statuses(installOnly(t, e, InstallOptions{Name: "p"}), "p")
	if c["update"] != 1 || c["skip"] != 9 {
		t.Fatalf("drift: %v", c)
	}
}

// A file that starts with a shebang is written executable, others are not.
func TestShebangFilesAreWrittenExecutable(t *testing.T) {
	e, ws, _ := newConfig(t, projManifest("    bundles: [proc]\n"), projFiles)
	installOnly(t, e, InstallOptions{Name: "p"})
	repo := filepath.Join(ws, "p")
	for rel, wantExec := range map[string]bool{
		".claude/skills/proc/go.sh":    true,
		".agents/skills/proc/go.sh":    true,
		".claude/skills/proc/SKILL.md": false,
	} {
		fi, err := os.Stat(filepath.Join(repo, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Mode()&0o111 != 0; got != wantExec {
			t.Errorf("%s: executable = %v, want %v", rel, got, wantExec)
		}
	}
}

// Moving a bundle to disabled: prunes its files and its hook; foreign files
// next to them stay.
func TestDisablingABundlePrunesItsArtifactsAndHook(t *testing.T) {
	e, ws, _ := newConfig(t, projManifest("    bundles: [proc, plain]\n"), projFiles)
	repo := filepath.Join(ws, "p")
	foreign := filepath.Join(repo, ".claude", "skills", "mine", "SKILL.md")
	writeTree(t, filepath.Dir(foreign), map[string]string{"SKILL.md": "hand-written"})
	writeTree(t, repo, map[string]string{".claude/settings.json": `{"permissions":{"allow":["Bash(ls)"]}}`})

	installOnly(t, e, InstallOptions{Name: "p"})
	if !strings.Contains(readFile(t, filepath.Join(repo, ".claude", "settings.json")), "permissions") {
		t.Fatal("hook merge must preserve other keys")
	}

	rewriteManifest(t, e, ws, "", projManifest("    bundles: [proc, plain]\n    disabled: [proc]\n"))
	res := installOnly(t, e, InstallOptions{Name: "p"})
	c := statuses(res, "p")
	if c["remove"] != 10 || c["update"] != 1 || c["skip"] != 2 { // proc: claude 3 + agent, codex 3 + agent md + toml, rule; settings updated; plain stays
		t.Fatalf("disable: %v", c)
	}
	for _, gone := range []string{".claude/skills/proc", ".claude/agents/rev.md", ".agents/skills/proc", ".codex/agents/rev.toml", ".cursor/rules/stance.mdc"} {
		if fileExists(filepath.Join(repo, gone)) {
			t.Errorf("%s should be pruned", gone)
		}
	}
	for _, kept := range []string{foreign, filepath.Join(repo, ".claude", "skills", "plain", "SKILL.md")} {
		if !fileExists(kept) {
			t.Errorf("%s must survive", kept)
		}
	}
	if s := readFile(t, filepath.Join(repo, ".claude", "settings.json")); strings.Contains(s, "SessionStart") || !strings.Contains(s, "permissions") {
		t.Errorf("hook must go, the rest must stay:\n%s", s)
	}
}

// --target never prunes another harness's files; neither does it touch the hook
// when no harness that reads it is named.
func TestPartialTargetInstallKeepsOtherHarnesses(t *testing.T) {
	e, ws, _ := newConfig(t, projManifest("    bundles: [proc, plain]\n"), projFiles)
	repo := filepath.Join(ws, "p")
	installOnly(t, e, InstallOptions{Name: "p"})

	// drop `plain` from the project, then install only codex
	rewriteManifest(t, e, ws, "", projManifest("    bundles: [proc]\n"))
	res := installOnly(t, e, InstallOptions{Name: "p", Target: "codex"})
	var removed []string
	for _, f := range res[0].Files {
		if f.Status == "remove" {
			removed = append(removed, strings.TrimPrefix(f.Path, repo+"/"))
		}
	}
	if strings.Join(removed, ",") != ".agents/skills/plain/SKILL.md" {
		t.Fatalf("--target codex may only prune codex files: %v", removed)
	}
	if !fileExists(filepath.Join(repo, ".claude", "skills", "plain", "SKILL.md")) {
		t.Fatal("the claude copy must survive a codex-only install")
	}
	if track := strings.Join(readTrackFile(t, filepath.Join(repo, ".claude", "ao-manifest.json")), ","); !strings.Contains(track, ".claude/skills/plain/SKILL.md") || strings.Contains(track, ".agents/skills/plain") {
		t.Fatalf("track file must keep the other harness's paths: %s", track)
	}
	// a full install then catches up
	c := statuses(installOnly(t, e, InstallOptions{Name: "p"}), "p")
	if c["remove"] != 1 {
		t.Fatalf("full install: %v", c)
	}

	// --what is partial too: no pruning, no hook toggling
	rewriteManifest(t, e, ws, "", projManifest("    bundles: [proc]\n    disabled: [proc]\n"))
	res = installOnly(t, e, InstallOptions{Name: "p", What: []string{"proc"}})
	if c := statuses(res, "p"); c["remove"] != 0 || len(res[0].Files) != 0 {
		t.Fatalf("--what must not prune or install a disabled bundle: %v %+v", c, res)
	}
	if !strings.Contains(readFile(t, filepath.Join(repo, ".claude", "settings.json")), "SessionStart") {
		t.Fatal("--what must not toggle the hook")
	}
}

func TestSessionHookNeedsAHarnessThatReadsIt(t *testing.T) {
	e, ws, _ := newConfig(t, projManifest("    bundles: [proc]\n    harnesses: [codex]\n"), projFiles)
	installOnly(t, e, InstallOptions{Name: "p"})
	if fileExists(filepath.Join(ws, "p", ".claude", "settings.json")) {
		t.Fatal("a codex-only project has no .claude/settings.json hook")
	}
	installOnly(t, e, InstallOptions{Name: "p", Target: "cursor"})
	if !fileExists(filepath.Join(ws, "p", ".claude", "settings.json")) {
		t.Fatal("cursor reads .claude/settings.json: the hook must be written for --target cursor")
	}
}

// projects/<name>/ stays: copied as-is, Codex agents become TOML only.
func TestVerbatimProjectSourcesKeepTheirV02Paths(t *testing.T) {
	e, ws, _ := newConfig(t, projManifest(""), map[string]string{
		"projects/p/skills/own/SKILL.md":  "---\nname: own\ndescription: Use for own.\n---\n\nkeeps {{ braces }} and a .tmpl-looking name\n",
		"projects/p/skills/own/x.md.tmpl": "not rendered {{.Nope}}\n",
		"projects/p/agents/solo.md":       "---\nname: solo\ndescription: Solo agent.\ntools: Read\n---\n\nBody.\n",
		"projects/p/agents/notes.txt":     "ignored",
	})
	got := renderEntry(t, e, "p")
	want := []string{
		".agents/skills/own/SKILL.md", ".agents/skills/own/x.md.tmpl",
		".claude/agents/solo.md", ".claude/skills/own/SKILL.md", ".claude/skills/own/x.md.tmpl",
		".codex/agents/solo.toml",
	}
	if strings.Join(paths(got), ",") != strings.Join(want, ",") {
		t.Fatalf("got %v\nwant %v", paths(got), want)
	}
	if !strings.Contains(got[".claude/skills/own/SKILL.md"], "{{ braces }}") || got[".claude/skills/own/x.md.tmpl"] != "not rendered {{.Nope}}\n" {
		t.Error("verbatim sources must not go through the template pass")
	}
	if !strings.Contains(got[".codex/agents/solo.toml"], `name = "solo"`) {
		t.Errorf("toml: %s", got[".codex/agents/solo.toml"])
	}
	_ = ws
}

// Two sources producing one path with different bytes is an error, not a coin flip.
func TestConflictingSourcesAreAnError(t *testing.T) {
	files := map[string]string{
		"templates/skills/s/SKILL.md":  skillMD("s"),
		"projects/p/skills/s/SKILL.md": skillMD("s") + "\nlocal edit\n",
	}
	e, _, _ := newConfig(t, projManifest("    skills: [s]\n    harnesses: [claude]\n"), files)
	if _, err := renderEntryErr(t, e, "p"); err == nil || !strings.Contains(err.Error(), "produced twice with different content") {
		t.Fatalf("got %v", err)
	}
	// the same bytes from both is fine
	files["projects/p/skills/s/SKILL.md"] = skillMD("s")
	e, _, _ = newConfig(t, projManifest("    skills: [s]\n    harnesses: [claude]\n"), files)
	if got := renderEntry(t, e, "p"); len(got) != 1 {
		t.Fatalf("got %v", paths(got))
	}
}

// A project installed by v0.2 (same paths, same track file) is all-skip after
// its config is migrated: nothing is rewritten and nothing is pruned.
func TestProjectMigratesFromV02State(t *testing.T) {
	e, ws, _ := newConfig(t, projManifest("    skills: [plain]\n    harnesses: [claude, codex]\n"), map[string]string{
		"templates/skills/plain/SKILL.md": skillMD("plain"),
		"projects/p/skills/own/SKILL.md":  skillMD("own"),
		"projects/p/agents/solo.md":       "---\nname: solo\ndescription: Solo agent.\ntools: Read\n---\n\nBody.\n",
	})
	repo := filepath.Join(ws, "p")
	toml, _ := codexAgent([]byte("---\nname: solo\ndescription: Solo agent.\ntools: Read\n---\n\nBody.\n"))
	writeTree(t, repo, map[string]string{
		".claude/skills/plain/SKILL.md": skillMD("plain"),
		".claude/skills/own/SKILL.md":   skillMD("own"),
		".claude/agents/solo.md":        "---\nname: solo\ndescription: Solo agent.\ntools: Read\n---\n\nBody.\n",
		".agents/skills/plain/SKILL.md": skillMD("plain"),
		".agents/skills/own/SKILL.md":   skillMD("own"),
		".codex/agents/solo.toml":       string(toml),
		".claude/ao-manifest.json": "[\n  \".agents/skills/own/SKILL.md\",\n  \".agents/skills/plain/SKILL.md\",\n  \".claude/agents/solo.md\",\n" +
			"  \".claude/skills/own/SKILL.md\",\n  \".claude/skills/plain/SKILL.md\",\n  \".codex/agents/solo.toml\"\n]\n",
	})
	c := statuses(installOnly(t, e, InstallOptions{All: true, DryRun: true}), "p")
	if c["skip"] != 6 || len(c) != 1 {
		t.Fatalf("a migrated project must be all-skip: %v", c)
	}
}

func TestRemovedV02FieldsAreRejectedWithAMigrationHint(t *testing.T) {
	for _, c := range []struct{ yaml, want string }{
		{"projects:\n  p:\n    repoRoot: /r\n    tickets: {space: X}\n", `projects.p: "tickets" was removed in trafaret 0.3 — move it under vars:`},
		{"projects:\n  p:\n    repoRoot: /r\n    gates: {packages: []}\n", `"gates" was removed`},
		{"projects:\n  p:\n    repoRoot: /r\n    targets: [claude]\n", `"targets" was removed in trafaret 0.3 — renamed to harnesses:`},
		{"defaults:\n  targets: [claude]\n", `defaults: "targets" was removed`},
		{"defaults:\n  tickets: {extraFields: x}\n", `defaults: "tickets" was removed`},
		{"homes:\n  h:\n    skillsDir: ~/.claude/skills\n", `homes.h: "skillsDir" was removed in trafaret 0.3 — use harness: and dir:`},
	} {
		p := filepath.Join(t.TempDir(), "m.yaml")
		if err := os.WriteFile(p, []byte(c.yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := loadManifest(p, "dev"); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: want %q, got %v", c.yaml, c.want, err)
		}
	}
	// vars may legitimately hold keys of the same names
	p := filepath.Join(t.TempDir(), "m.yaml")
	if err := os.WriteFile(p, []byte("projects:\n  p:\n    repoRoot: /r\n    vars: {tickets: {space: X}, gates: [], targets: x}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(p, "dev"); err != nil {
		t.Fatalf("those names are fine under vars: %v", err)
	}
}

func TestInstallNeedsANameOrAllAndKnowsNoGhosts(t *testing.T) {
	e, _, _ := newConfig(t, projManifest("    skills: [plain]\n"), map[string]string{"templates/skills/plain/SKILL.md": skillMD("plain")})
	if _, err := e.Install(InstallOptions{}); err == nil || !strings.Contains(err.Error(), "name a project or home, or pass --all") {
		t.Errorf("no name: %v", err)
	}
	if _, err := e.Install(InstallOptions{Name: "ghost"}); err == nil || !strings.Contains(err.Error(), "no such project or home: ghost") {
		t.Errorf("ghost: %v", err)
	}
	if _, err := e.Install(InstallOptions{Name: "p", Target: "vim"}); err == nil || !strings.Contains(err.Error(), "invalid --target") {
		t.Errorf("bad target: %v", err)
	}
}

func TestValidateReportsProjectProblems(t *testing.T) {
	manifest := `defaults:
  workspace: {{WS}}
projects:
  norepo:
    bundles: [plain]
    harnesses: [claude]
  noharness:
    repoRoot: a
    bundles: [plain]
  badharness:
    repoRoot: b
    bundles: [plain]
    harnesses: [vim]
  nobundle:
    repoRoot: c
    bundles: [ghost]
    harnesses: [claude]
  badoff:
    repoRoot: d
    bundles: [plain]
    disabled: [other]
    harnesses: [claude]
  nothing:
    repoRoot: e
    harnesses: [claude]
  missingskill:
    repoRoot: f
    skills: [ghost]
    harnesses: [claude]
  typo:
    repoRoot: g
    skills: [typo]
    harnesses: [claude]
  twocontexts:
    repoRoot: h
    bundles: [proc, other]
    harnesses: [claude]
homes:
  twocontexts:
    harness: claude
    dir: {{HOME}}/x
    skills: []
`
	files := map[string]string{
		"templates/bundles.yaml":              "plain:\n  skills: [plain]\nother:\n  skills: [plain]\n  sessionContext: plain/c.md\nproc:\n  skills: [plain]\n  sessionContext: plain/d.md\n",
		"templates/skills/plain/SKILL.md":     skillMD("plain"),
		"templates/skills/typo/SKILL.md.tmpl": "---\nname: typo\ndescription: d\n---\n{{.Vars.nope}}\n",
	}
	e, _, _ := newConfig(t, manifest, files)
	m, err := e.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	rep := e.Validate(m)
	var lines []string
	for _, p := range rep.Problems {
		lines = append(lines, p.Where+": "+p.Msg)
	}
	all := strings.Join(lines, "\n")
	for _, want := range []string{
		"norepo: missing repoRoot",
		"noharness: no harnesses",
		`badharness: unknown harness "vim"`,
		`nobundle: bundle "ghost" is not in templates/bundles.yaml`,
		`badoff: disabled bundle "other" is not in bundles`,
		"nothing: no bundles, skills, agents or rules and no projects/nothing sources",
		`missingskill: no skill "ghost"`,
		`typo: render error:`,
		"both set sessionContext",
		"twocontexts: name collides with a project",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("validate lacks %q in:\n%s", want, all)
		}
	}
	if rep.Projects != 9 || rep.Homes != 1 {
		t.Errorf("counts: %d projects, %d homes", rep.Projects, rep.Homes)
	}
}

// The demo config is the project's showcase: it must validate clean and diff
// in sync with nothing written, and its bundle must be what the docs say.
func TestDemoValidates(t *testing.T) {
	e, err := New(Options{Config: demoDir, Version: "v0.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := e.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if rep := e.Validate(m); len(rep.Problems) != 0 || rep.Projects != 2 || rep.Homes != 2 {
		t.Fatalf("demo must validate clean: %+v", rep)
	}
	if b := e.Bundles["ship"]; strings.Join(b.Requires, ",") != "company,lang,reviewers,branch,gates,tracker.kind" || b.SessionContext != "ship/session-context.md" {
		t.Errorf("ship bundle: %+v", b)
	}
	// the demo carries the engine constraint of this release
	if old, err := New(Options{Config: demoDir, Version: "v0.2.0"}); err != nil {
		t.Fatal(err)
	} else if _, err := old.LoadManifest(); err == nil || !strings.Contains(err.Error(), ">=0.3.0") {
		t.Errorf("a 0.2 binary must refuse the demo, got %v", err)
	}
}
