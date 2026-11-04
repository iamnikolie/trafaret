package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadBundlesReadsTheRegistry(t *testing.T) {
	reg, err := loadBundles(fstest.MapFS{"bundles.yaml": {Data: []byte(
		"one:\n  skills: [a, b]\n  agents: [c]\n  rules: [d]\n  requires: [branch, tracker.kind]\n  sessionContext: a/ctx.md\n")}})
	if err != nil {
		t.Fatal(err)
	}
	b := reg["one"]
	if strings.Join(b.Skills, ",") != "a,b" || strings.Join(b.Agents, ",") != "c" || strings.Join(b.Rules, ",") != "d" {
		t.Fatalf("lists not read: %#v", b)
	}
	if strings.Join(b.Requires, ",") != "branch,tracker.kind" || b.SessionContext != "a/ctx.md" {
		t.Fatalf("requires/sessionContext not read: %#v", b)
	}
}

func TestLoadBundlesRejectsBadEntries(t *testing.T) {
	cases := map[string]string{
		"missing file":         "",
		"requires not a path":  "one:\n  requires: [\"a b\"]\n",
		"requires empty seg":   "one:\n  requires: [\"a..b\"]\n",
		"requires blank":       "one:\n  requires: [\"\"]\n",
		"absolute context":     "one:\n  sessionContext: /etc/passwd\n",
		"escaping context":     "one:\n  sessionContext: ../x.md\n",
		"shell-unsafe context": "one:\n  sessionContext: a/$(x).md\n",
		"unknown field":        "one:\n  skils: [a]\n",
		"not yaml":             "one: [\n",
	}
	for name, body := range cases {
		fsys := fstest.MapFS{}
		if body != "" {
			fsys["bundles.yaml"] = &fstest.MapFile{Data: []byte(body)}
		}
		if _, err := loadBundles(fsys); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	// the 0.2 field name gets a pointer to its replacement
	_, err := loadBundles(fstest.MapFS{"bundles.yaml": {Data: []byte("one:\n  cursorRules: [r]\n")}})
	if err == nil || !strings.Contains(err.Error(), "renamed to rules") {
		t.Errorf("cursorRules: %v", err)
	}
	// a registry with no entries is fine
	if reg, err := loadBundles(fstest.MapFS{"bundles.yaml": {Data: []byte("# nothing yet\n")}}); err != nil || len(reg) != 0 {
		t.Errorf("empty registry: %v %v", reg, err)
	}
}

func TestOverlayBundlesShadowEmbedded(t *testing.T) {
	manifest := "defaults:\n  workspace: {{WS}}\n  harnesses: [claude]\nprojects:\n  p:\n    repoRoot: p\n    bundles: [own]\n"
	e, _, _ := newConfig(t, manifest, map[string]string{
		"templates/bundles.yaml":             "own:\n  skills: [own]\n",
		"templates/skills/own/SKILL.md.tmpl": "---\nname: own\ndescription: own skill of {{.Name}}\n---\nbody\n",
	})
	if _, ok := e.Bundles["ship"]; ok {
		t.Fatal("an overlay bundles.yaml must shadow the embedded registry, not merge with it")
	}
	if got := renderEntry(t, e, "p"); len(got) != 1 {
		t.Fatalf("overlay bundle did not render: %v", paths(got))
	}
	// the embedded bundle is unknown here
	manifest = strings.Replace(manifest, "[own]", "[ship]", 1)
	e, _, _ = newConfig(t, manifest, map[string]string{"templates/bundles.yaml": "own:\n  skills: [own]\n"})
	if _, err := renderEntryErr(t, e, "p"); err == nil || !strings.Contains(err.Error(), `unknown bundle "ship"`) {
		t.Fatalf("a bundle absent from the overlay registry must be unknown, got %v", err)
	}
}

func TestSessionContextAtMostOne(t *testing.T) {
	reg := map[string]Bundle{
		"a": {SessionContext: "a/ctx.md"},
		"b": {SessionContext: "b/ctx.md"},
		"c": {},
	}
	if got, err := sessionContext(reg, []string{"c", "a"}); err != nil || got != "a/ctx.md" {
		t.Fatalf("got %q %v", got, err)
	}
	if got, err := sessionContext(reg, []string{"c"}); err != nil || got != "" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := sessionContext(reg, []string{"a", "b"}); err == nil {
		t.Fatal("two bundles with sessionContext must be an error")
	}
}

// bundle requires are var paths, validated against the merged vars of every
// project and home that carries the bundle.
func TestRequiresAreValidatedAsVarPaths(t *testing.T) {
	manifest := `vars: {branch: main}
defaults:
  workspace: {{WS}}
  harnesses: [claude]
  vars: {tracker: {kind: github}}
projects:
  good:
    repoRoot: good
    bundles: [needs]
    vars: {gates: [], tracker: {repo: a/b}}
  bad:
    repoRoot: bad
    bundles: [needs]
    vars: {gates: []}
  blankok:
    repoRoot: blankok
    bundles: [needs]
    vars: {gates: [], tracker: {kind: ""}}
  off:
    repoRoot: off
    bundles: [needs]
    disabled: [needs]
homes:
  h:
    harness: claude
    dir: {{HOME}}/h
    skills: []
    bundles: [needs]
    vars: {gates: x}
`
	e, _, _ := newConfig(t, manifest, map[string]string{
		"templates/bundles.yaml":      "needs:\n  skills: [t]\n  requires: [branch, gates, tracker.kind]\n",
		"templates/skills/t/SKILL.md": skillMD("t"),
	})
	m, err := e.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	rep := e.Validate(m)
	got := map[string][]string{}
	for _, p := range rep.Problems {
		got[p.Where] = append(got[p.Where], p.Msg)
	}
	// good and the home (which inherits tracker.kind) pass; blankok passes because
	// an existing "" counts; bad lacks nothing either: tracker.kind comes from defaults
	if len(got["good"])+len(got["blankok"])+len(got["home:h"]) != 0 {
		t.Fatalf("unexpected problems: %v", got)
	}
	// "off" declares the bundle but disables it: gates is unset, so it is reported (declared bundles stay sound)
	if len(got["off"]) != 1 || !strings.Contains(got["off"][0], `requires var "gates"`) {
		t.Fatalf("off: %v", got["off"])
	}
	if len(got["bad"]) != 0 {
		t.Fatalf("bad inherits branch and tracker.kind from the layers: %v", got["bad"])
	}

	// now really remove tracker.kind for one project (null deletes the inherited key)
	manifest = strings.ReplaceAll(strings.ReplaceAll(manifest, "{{WS}}", e.ConfigDir+"/../ws"), "{{HOME}}", e.ConfigDir+"/../home")
	manifest = strings.Replace(manifest, "vars: {gates: []}\n  blankok", "vars: {gates: [], tracker: {kind: null}}\n  blankok", 1)
	writeTree(t, e.ConfigDir, map[string]string{"manifest.yaml": manifest})
	m, _ = e.LoadManifest()
	rep = e.Validate(m)
	var badMsgs []string
	for _, p := range rep.Problems {
		if p.Where == "bad" {
			badMsgs = append(badMsgs, p.Msg)
		}
	}
	if len(badMsgs) != 1 || !strings.Contains(badMsgs[0], `bundle needs requires var "tracker.kind", which is not set`) {
		t.Fatalf("bad: %v", badMsgs)
	}
}

// A project whose settings carry the hook of a retired bundle gets it replaced
// or removed, and every other key survives.
func TestSessionHookMigratesAndRemovesOwnedEntry(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, filepath.FromSlash(settingsFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"permissions":{"allow":["Bash(ls)"]},"hooks":{"SessionStart":[` +
		`{"matcher":"startup","hooks":[{"type":"command","command":"echo own"}]},` +
		`{"matcher":"startup|clear|compact","hooks":[{"type":"command","command":"cat \"$CLAUDE_PROJECT_DIR/.claude/skills/dev-process/session-context.md\""}]}]}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	read := func() map[string]any {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	commands := func(m map[string]any) []string {
		var out []string
		hooks, _ := m["hooks"].(map[string]any)
		starts, _ := hooks["SessionStart"].([]any)
		for _, g := range starts {
			for _, h := range g.(map[string]any)["hooks"].([]any) {
				out = append(out, h.(map[string]any)["command"].(string))
			}
		}
		return out
	}

	st, err := ensureSessionHook(repo, "ship/session-context.md", false)
	if err != nil || st != "update" {
		t.Fatalf("migrate: %q %v", st, err)
	}
	got := commands(read())
	if len(got) != 2 || got[0] != "echo own" || got[1] != hookCommand("ship/session-context.md") {
		t.Fatalf("legacy entry not replaced in place: %q", got)
	}
	if st, _ := ensureSessionHook(repo, "ship/session-context.md", false); st != "" {
		t.Fatalf("second run must be a no-op, got %q", st)
	}

	if st, err := ensureSessionHook(repo, "", false); err != nil || st == "" {
		t.Fatalf("remove: %q %v", st, err)
	}
	m := read()
	if got := commands(m); len(got) != 1 || got[0] != "echo own" {
		t.Fatalf("only the owned entry may go: %q", got)
	}
	if _, ok := m["permissions"]; !ok {
		t.Fatal("unrelated keys must survive")
	}
}

// A settings file that held nothing but the hook goes away with it; a corrupt
// one is left alone with an error.
func TestSessionHookRemovesAnOtherwiseEmptyFileAndRefusesCorruptOnes(t *testing.T) {
	repo := t.TempDir()
	if st, err := ensureSessionHook(repo, "ship/session-context.md", false); err != nil || st != "install" {
		t.Fatalf("install: %q %v", st, err)
	}
	path := filepath.Join(repo, filepath.FromSlash(settingsFile))
	if st, err := ensureSessionHook(repo, "", true); err != nil || st != "remove" || !fileExists(path) {
		t.Fatalf("dry-run remove: %q %v", st, err)
	}
	if st, err := ensureSessionHook(repo, "", false); err != nil || st != "remove" || fileExists(path) {
		t.Fatalf("remove: %q %v", st, err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureSessionHook(repo, "ship/session-context.md", false); err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("corrupt settings: %v", err)
	}
	if readFile(t, path) != "{not json" {
		t.Fatal("a corrupt settings file must not be touched")
	}
}
