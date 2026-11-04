package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// pruneStale deletes previously-recorded artifacts that the current render no
// longer produces, then rewrites the track file with the current set.
func pruneStale(repoRoot string, newSet map[string]bool, targets []string, preserveOtherTargets, dryRun bool) ([]FileStatus, error) {
	trackPath := filepath.Join(repoRoot, filepath.FromSlash(trackFile))
	var old []string
	if b, err := os.ReadFile(trackPath); err == nil {
		if err := json.Unmarshal(b, &old); err != nil {
			return nil, fmt.Errorf("%s: corrupt track file: %w", trackPath, err)
		}
	}
	if preserveOtherTargets {
		for _, r := range old {
			r = filepath.ToSlash(filepath.Clean(r))
			if !pathBelongsToTargets(r, targets) {
				newSet[r] = true
			}
		}
	}
	var res []FileStatus
	for _, r := range old {
		r = filepath.ToSlash(filepath.Clean(r))
		if newSet[r] || strings.Contains(r, "..") ||
			!(strings.HasPrefix(r, ".claude/") || strings.HasPrefix(r, ".cursor/") ||
				strings.HasPrefix(r, ".agents/") || strings.HasPrefix(r, ".codex/")) {
			continue
		}
		abs := filepath.Join(repoRoot, filepath.FromSlash(r))
		if _, err := os.Stat(abs); err != nil {
			continue // already gone
		}
		if !dryRun {
			if err := os.Remove(abs); err != nil {
				return nil, err
			}
			// drop now-empty parent dirs, but never managed harness roots
			for d := filepath.Dir(abs); ; d = filepath.Dir(d) {
				base := filepath.Base(d)
				if base == ".claude" || base == ".cursor" || base == ".agents" || base == ".codex" || d == repoRoot {
					break
				}
				if os.Remove(d) != nil { // non-empty or protected — stop
					break
				}
			}
		}
		res = append(res, FileStatus{abs, "remove"})
	}
	if !dryRun {
		cur := make([]string, 0, len(newSet))
		for r := range newSet {
			cur = append(cur, r)
		}
		sort.Strings(cur)
		b, _ := json.MarshalIndent(cur, "", "  ")
		if err := os.MkdirAll(filepath.Dir(trackPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(trackPath, append(b, '\n'), 0o644); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func pathBelongsToTargets(path string, targets []string) bool {
	switch {
	case strings.HasPrefix(path, ".agents/"), strings.HasPrefix(path, ".codex/"):
		return contains(targets, "codex")
	case strings.HasPrefix(path, ".cursor/"):
		return contains(targets, "cursor")
	case strings.HasPrefix(path, ".claude/"):
		return contains(targets, "claude") || contains(targets, "cursor")
	default:
		return false
	}
}
