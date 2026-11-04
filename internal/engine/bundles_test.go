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
		"one:\n  skills: [a, b]\n  agents: [c]\n  cursorRules: [d]\n  requires: [tickets, db]\n  sessionContext: a/ctx.md\n")}})
	if err != nil {
		t.Fatal(err)
	}
	b := reg["one"]
	if strings.Join(b.Skills, ",") != "a,b" || strings.Join(b.Agents, ",") != "c" || strings.Join(b.CursorRules, ",") != "d" {
		t.Fatalf("lists not read: %#v", b)
	}
	if !b.requires("tickets") || !b.requires("db") || b.SessionContext != "a/ctx.md" {
		t.Fatalf("requires/sessionContext not read: %#v", b)
	}
}

func TestLoadBundlesRejectsBadEntries(t *testing.T) {
	cases := map[string]string{
		"missing file":           "",
		"unknown requires block": "one:\n  requires: [tickets, run]\n",
		"absolute context":       "one:\n  sessionContext: /etc/passwd\n",
		"escaping context":       "one:\n  sessionContext: ../x.md\n",
		"shell-unsafe context":   "one:\n  sessionContext: a/$(x).md\n",
		"not yaml":               "one: [\n",
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
}

func TestOverlayBundlesShadowEmbedded(t *testing.T) {
	cfg := writeManifest(t, filepath.Join(evalDir(t, t.TempDir()), "cfg"))
	dir := filepath.Join(cfg, "templates")
	if err := os.MkdirAll(filepath.Join(dir, "skills", "own"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundles.yaml"), []byte("own:\n  skills: [own]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "own", "SKILL.md.tmpl"), []byte("---\nname: own\ndescription: own skill of {{.Name}}\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := New(Options{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := e.Bundles["example"]; ok {
		t.Fatal("an overlay bundles.yaml must shadow the embedded registry, not merge with it")
	}
	files, err := render(testProject(t), e, []string{"own"}, []string{"claude"})
	if err != nil || len(files) != 1 {
		t.Fatalf("overlay bundle did not render: %v %d", err, len(files))
	}
	if _, err := render(testProject(t), e, []string{"example"}, []string{"claude"}); err == nil {
		t.Fatal("a bundle absent from the overlay registry must be unknown")
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

// A project whose settings carry the hook of a retired bundle (the 0.1 engine
// always wrote the dev-process one) gets it replaced or removed, and every other
// key survives.
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

	st, err := ensureSessionHook(repo, "example-tickets/session-context.md", false)
	if err != nil || st != "update" {
		t.Fatalf("migrate: %q %v", st, err)
	}
	got := commands(read())
	if len(got) != 2 || got[0] != "echo own" || got[1] != hookCommand("example-tickets/session-context.md") {
		t.Fatalf("legacy entry not replaced in place: %q", got)
	}
	if st, _ := ensureSessionHook(repo, "example-tickets/session-context.md", false); st != "" {
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
