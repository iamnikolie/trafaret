package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPartialTargetPrunePreservesOtherHarnesses(t *testing.T) {
	repo := t.TempDir()
	old := []string{
		".claude/skills/example/SKILL.md",
		".cursor/rules/example.mdc",
		".agents/skills/old/SKILL.md",
	}
	for _, rel := range old {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := json.Marshal(old)
	track := filepath.Join(repo, filepath.FromSlash(trackFile))
	if err := os.MkdirAll(filepath.Dir(track), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(track, b, 0o644); err != nil {
		t.Fatal(err)
	}

	newSet := map[string]bool{".agents/skills/new/SKILL.md": true}
	removed, err := pruneStale(repo, newSet, []string{"codex"}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || !strings.HasSuffix(filepath.ToSlash(removed[0].Path), ".agents/skills/old/SKILL.md") {
		t.Fatalf("unexpected removals: %#v", removed)
	}
	for _, rel := range old[:2] {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
			t.Errorf("other target artifact %s was removed: %v", rel, err)
		}
	}
}

// Prune deletes only recorded paths under a harness directory: a hand-edited
// track file naming something else is ignored, not obeyed.
func TestPruneIgnoresTrackedPathsOutsideHarnessDirs(t *testing.T) {
	repo := t.TempDir()
	writeTree(t, repo, map[string]string{"README.md": "mine", "src/main.go": "mine", ".claude/skills/x/SKILL.md": "x"})
	writeTree(t, repo, map[string]string{".claude/ao-manifest.json": `["README.md", "src/main.go", "../outside", ".claude/../README.md", ".claude/skills/x/SKILL.md"]`})
	removed, err := pruneStale(repo, map[string]bool{}, []string{"claude"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || !strings.HasSuffix(filepath.ToSlash(removed[0].Path), ".claude/skills/x/SKILL.md") {
		t.Fatalf("only the harness-dir path may go: %#v", removed)
	}
	for _, keep := range []string{"README.md", "src/main.go"} {
		if !fileExists(filepath.Join(repo, keep)) {
			t.Errorf("%s must survive", keep)
		}
	}
	if fileExists(filepath.Join(repo, ".claude", "skills")) {
		t.Error("the emptied directories under the harness root should be removed")
	}
	if !fileExists(filepath.Join(repo, ".claude")) {
		t.Error("the harness root itself must stay")
	}
}

func TestPruneRefusesACorruptTrackFile(t *testing.T) {
	repo := t.TempDir()
	writeTree(t, repo, map[string]string{".claude/ao-manifest.json": "{nope"})
	if _, err := pruneStale(repo, map[string]bool{}, []string{"claude"}, false, false); err == nil || !strings.Contains(err.Error(), "corrupt track file") {
		t.Fatalf("got %v", err)
	}
}
