package engine

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// --- install --------------------------------------------------------------

// FileStatus is the outcome for one file of an install.
type FileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"` // install | update | skip | remove
}

// trackFile is the per-repo record of every artifact trafaret wrote on the last
// install. It is what makes disabling a bundle real: files recorded here but
// absent from the current render are pruned. The "ao-" prefix is the engine's
// historical name, kept so repos installed by earlier builds keep pruning.
const trackFile = ".claude/ao-manifest.json"

// install renders the active bundles and writes the artifacts. When prune is
// true (full install, no --what filter) it also deletes artifacts recorded in
// the repo's track file that the current render no longer produces — this is
// how a bundle moved to `disabled:` actually leaves the repo. Only paths trafaret
// itself recorded, under .claude/ or .cursor/, are ever deleted.
func install(p Project, eng *Engine, activeBundles, targets []string, force, dryRun, prune, partialTargets bool) ([]FileStatus, error) {
	files, err := render(p, eng, activeBundles, targets)
	if err != nil {
		return nil, err
	}
	res, newSet, err := writeFiles(files, p.RepoRoot, force, dryRun)
	if err != nil {
		return nil, err
	}

	if prune {
		removed, err := pruneStale(p.RepoRoot, newSet, targets, partialTargets, dryRun)
		if err != nil {
			return nil, err
		}
		res = append(res, removed...)
	}

	// A bundle with sessionContext ships a per-project SessionStart hook: trafaret
	// owns exactly one entry in .claude/settings.json and adds/updates/removes it to
	// match the bundle state. Managed only on full installs — a --what subset must
	// not toggle it.
	if prune {
		ctx, err := sessionContext(eng.Bundles, activeBundles)
		if err != nil {
			return nil, err
		}
		if !contains(targets, "claude") && !contains(targets, "cursor") {
			ctx = ""
		}
		st, err := ensureSessionHook(p.RepoRoot, ctx, dryRun)
		if err != nil {
			return nil, err
		}
		if st != "" {
			res = append(res, FileStatus{filepath.Join(p.RepoRoot, settingsFile), st})
		}
	}
	return res, nil
}

// writeFiles writes every rendered file whose content changed (or all of them
// with force) and returns the per-file status plus the set of paths relative
// to root, which is what the track file records.
func writeFiles(files []outFile, root string, force, dryRun bool) ([]FileStatus, map[string]bool, error) {
	var res []FileStatus
	newSet := map[string]bool{}
	for _, f := range files {
		if r, err := filepath.Rel(root, f.path); err == nil {
			newSet[filepath.ToSlash(r)] = true
		}
		status := "install"
		if existing, err := os.ReadFile(f.path); err == nil {
			if sha(existing) == sha(f.bytes) {
				status = "skip"
			} else {
				status = "update"
			}
		}
		if !dryRun && (status != "skip" || force) {
			if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
				return nil, nil, err
			}
			if err := os.WriteFile(f.path, f.bytes, 0o644); err != nil {
				return nil, nil, err
			}
		}
		res = append(res, FileStatus{f.path, status})
	}
	return res, newSet, nil
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h)
}
