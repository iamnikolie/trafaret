package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func jsonIndent(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	return append(b, '\n'), err
}

func readTrack(trackPath string) ([]string, error) {
	var old []string
	if b, err := os.ReadFile(trackPath); err == nil {
		if err := json.Unmarshal(b, &old); err != nil {
			return nil, fmt.Errorf("%s: corrupt track file: %w", trackPath, err)
		}
	}
	return old, nil
}

// pruneStale deletes previously-recorded artifacts that the current render no
// longer produces, then rewrites the track file with the current set. It only
// ever deletes paths trafaret itself recorded, and only under a harness
// directory.
func pruneStale(repoRoot string, newSet map[string]bool, targets []string, preserveOtherTargets, dryRun bool) ([]FileStatus, error) {
	trackPath := filepath.Join(repoRoot, filepath.FromSlash(trackFile))
	old, err := readTrack(trackPath)
	if err != nil {
		return nil, err
	}
	if preserveOtherTargets {
		for _, r := range old {
			r = filepath.ToSlash(filepath.Clean(r))
			if !pathBelongsToTargets(r, targets) {
				newSet[r] = true
			}
		}
	}
	roots := projectManagedRoots()
	var res []FileStatus
	for _, r := range old {
		r = filepath.ToSlash(filepath.Clean(r))
		top, _, _ := strings.Cut(r, "/")
		if newSet[r] || strings.Contains(r, "..") || !contains(roots, top) {
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
				if contains(roots, filepath.Base(d)) || d == repoRoot {
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
		if err := writeTrack(trackPath, newSet); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// pruneHome does the same for one directory of a home (skills/, agents/,
// rules/): it deletes files recorded on the last install that the current render
// no longer produces, then rewrites that directory's track file. A directory
// that never held anything gets no track file.
func pruneHome(dir string, newSet map[string]bool, dryRun bool) ([]FileStatus, error) {
	trackPath := filepath.Join(dir, homeTrackFile)
	old, err := readTrack(trackPath)
	if err != nil {
		return nil, err
	}
	var res []FileStatus
	for _, r := range old {
		r = filepath.ToSlash(filepath.Clean(r))
		if newSet[r] || strings.Contains(r, "..") || filepath.IsAbs(r) {
			continue
		}
		abs := filepath.Join(dir, filepath.FromSlash(r))
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		if !dryRun {
			if err := os.Remove(abs); err != nil {
				return nil, err
			}
			for d := filepath.Dir(abs); d != dir && strings.HasPrefix(d, dir); d = filepath.Dir(d) {
				if os.Remove(d) != nil {
					break
				}
			}
		}
		res = append(res, FileStatus{abs, "remove"})
	}
	if !dryRun && (len(newSet) > 0 || old != nil || fileExists(trackPath)) {
		if err := writeTrack(trackPath, newSet); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
