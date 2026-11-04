package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestEnforcesEngine(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "m.yaml")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	if _, err := loadManifest(write("engine: \">=0.3.0\"\nprojects: {}\n"), "0.2.0"); err == nil || !strings.Contains(err.Error(), ">=0.3.0") {
		t.Fatalf("a too-old binary must be refused, got %v", err)
	}
	if _, err := loadManifest(write("engine: \">=0.3.0\"\nprojects: {}\n"), "0.3.0"); err != nil {
		t.Fatalf("a satisfying binary must load: %v", err)
	}
	if _, err := loadManifest(write("projects: {}\n"), "0.1.0"); err != nil {
		t.Fatalf("no engine field: no check: %v", err)
	}
	if _, err := loadManifest(write("someFutureField: 1\nprojects: {}\n"), "0.1.0"); err != nil {
		t.Fatalf("unknown fields must be ignored: %v", err)
	}
	if _, err := loadManifest(write("engine: \">=99.0.0\"\nprojects: {}\n"), "dev"); err != nil {
		t.Fatalf("dev builds always pass: %v", err)
	}
	// an unquoted YAML float ("engine: 0.3") is still a usable constraint
	if _, err := loadManifest(write("engine: 0.3\nprojects: {}\n"), "0.4.0"); err == nil {
		t.Fatal("engine: 0.3 is an exact-version constraint; 0.4.0 does not satisfy it")
	}
}

func TestLoadManifestResolvesPathsAndHarnesses(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	p := filepath.Join(t.TempDir(), "m.yaml")
	body := `defaults:
  workspace: /work
  harnesses: [claude, codex]
projects:
  rel: {repoRoot: rel}
  abs: {repoRoot: /elsewhere/abs, harnesses: [cursor]}
  tilde: {repoRoot: ~/code/tilde}
homes:
  h: {harness: codex, dir: ~/.codex}
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := loadManifest(p, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if m.Projects["rel"].RepoRoot != "/work/rel" || m.Projects["abs"].RepoRoot != "/elsewhere/abs" || m.Projects["tilde"].RepoRoot != "/home/someone/code/tilde" {
		t.Errorf("repoRoots: %+v", m.Projects)
	}
	if strings.Join(m.Projects["rel"].Harnesses, ",") != "claude,codex" || strings.Join(m.Projects["abs"].Harnesses, ",") != "cursor" {
		t.Errorf("harnesses: %v / %v", m.Projects["rel"].Harnesses, m.Projects["abs"].Harnesses)
	}
	if h := m.Homes["h"]; h.Dir != "/home/someone/.codex" || h.Name != "h" || h.Harness != "codex" {
		t.Errorf("home: %+v", h)
	}
	if m.Projects["rel"].Name != "rel" {
		t.Error("project name not filled from its key")
	}
}
