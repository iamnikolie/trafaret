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
