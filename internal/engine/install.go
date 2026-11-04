package engine

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// homeTrackFile sits in each directory of a home that trafaret writes into
// (skills/, agents/, rules/) and records what it wrote there, so a skill removed
// from the sources is pruned — and nothing trafaret did not write (the
// harness's own skills, a synced/ dir) is ever touched.
const homeTrackFile = ".ao-manifest.json"

// projectItems resolves what a project installs: its own lists (unless direct
// is false, as for --what) plus the contents of the given bundles.
func (e *Engine) projectItems(ns *Namespace, p Project, active []string, direct bool) (itemSet, error) {
	var d itemSet
	if direct {
		d = itemSet{skills: p.Skills, agents: p.Agents, rules: p.Rules}
	}
	it, err := collectItems(e.Bundles, d, active)
	if err != nil {
		return it, err
	}
	return it, ns.check(it)
}

// homeItems resolves what a home installs. An omitted skills: list means every
// global skill (<config>/global/skills and <config>/agents/*/skills).
func (e *Engine) homeItems(ns *Namespace, h Home, active []string, direct bool) (itemSet, error) {
	var d itemSet
	if direct {
		d = itemSet{skills: h.Skills, agents: h.Agents, rules: h.Rules}
		if h.Skills == nil {
			d.skills = ns.globalSkillNames()
		}
	}
	it, err := collectItems(e.Bundles, d, active)
	if err != nil {
		return it, err
	}
	if err := ns.check(it); err != nil {
		return it, err
	}
	all := append(append(ns.SkillNames(), ns.AgentNames()...), ns.RuleNames()...)
	for _, s := range h.Skip {
		if !contains(all, s) {
			return it, fmt.Errorf("skip names no skill, agent or rule %q", s)
		}
	}
	it.skills, it.agents, it.rules = subtract(it.skills, h.Skip), subtract(it.agents, h.Skip), subtract(it.rules, h.Skip)
	return it, nil
}

func (e *Engine) projectEntity(p Project, harnesses []string) entity {
	return entity{scope: projectScope, name: p.Name, base: p.RepoRoot, harnesses: harnesses, vars: p.Vars,
		verbatimDir: filepath.Join(e.ProjectsDir(), p.Name)}
}

func homeEntity(h Home) entity {
	return entity{scope: homeScope, name: h.Name, base: h.Dir, harnesses: []string{h.Harness}, vars: h.Vars}
}

// installProject renders the project and writes the artifacts. When prune is
// true (full install, no --what filter) it also deletes artifacts recorded in
// the repo's track file that the current render no longer produces — this is
// how a bundle moved to `disabled:` actually leaves the repo. Only paths trafaret
// itself recorded, under the harness directories, are ever deleted.
func (e *Engine) installProject(ns *Namespace, p Project, active, harnesses []string, direct, force, dryRun, prune, partialTargets bool) ([]FileStatus, error) {
	if p.RepoRoot == "" {
		return nil, errors.New("missing repoRoot")
	}
	it, err := e.projectItems(ns, p, active, direct)
	if err != nil {
		return nil, err
	}
	files, err := ns.render(e.projectEntity(p, harnesses), it)
	if err != nil {
		return nil, err
	}
	res, newSet, err := writeFiles(files, p.RepoRoot, force, dryRun)
	if err != nil {
		return nil, err
	}

	if prune {
		removed, err := pruneStale(p.RepoRoot, newSet, harnesses, partialTargets, dryRun)
		if err != nil {
			return nil, err
		}
		res = append(res, removed...)
	}

	// A bundle with sessionContext ships a per-project SessionStart hook: trafaret
	// owns exactly one entry in .claude/settings.json and adds/updates/removes it to
	// match the bundle state. Managed only on full installs — a --what subset must
	// not toggle it — and a --target install that names no harness reading that file
	// (codex alone) leaves it as it is: the hook belongs to another harness.
	if prune && (!partialTargets || hasHook(harnesses)) {
		ctx, err := sessionContext(e.Bundles, active)
		if err != nil {
			return nil, err
		}
		if !hasHook(harnesses) {
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

// installHome renders the home and writes the artifacts, one track file per
// directory of the home it writes into.
func (e *Engine) installHome(ns *Namespace, h Home, active []string, direct, force, dryRun, prune bool) ([]FileStatus, error) {
	if h.Dir == "" || !filepath.IsAbs(h.Dir) {
		return nil, fmt.Errorf("dir must be an absolute or ~/ path, got %q", h.Dir)
	}
	it, err := e.homeItems(ns, h, active, direct)
	if err != nil {
		return nil, err
	}
	files, err := ns.render(homeEntity(h), it)
	if err != nil {
		return nil, err
	}
	zones := map[string][]outFile{}
	for _, f := range files {
		r, _ := filepath.Rel(h.Dir, f.path)
		zone, _, ok := strings.Cut(filepath.ToSlash(r), "/")
		if !ok {
			return nil, fmt.Errorf("%s lands outside a skills/agents/rules directory of the home", f.path)
		}
		zones[zone] = append(zones[zone], f)
	}
	var res []FileStatus
	for _, zone := range homeZones(h.Harness) {
		zoneDir := filepath.Join(h.Dir, zone)
		w, newSet, err := writeFiles(zones[zone], zoneDir, force, dryRun)
		if err != nil {
			return nil, err
		}
		res = append(res, w...)
		if prune {
			removed, err := pruneHome(zoneDir, newSet, dryRun)
			if err != nil {
				return nil, err
			}
			res = append(res, removed...)
		}
	}
	return res, nil
}

// homeZones are the directories of a home the harness writes into, in order.
func homeZones(harness string) []string {
	ad, ok := findAdapter(harness)
	if !ok {
		return nil
	}
	var out []string
	for _, pls := range [][]placement{ad.home.skills, ad.home.agents, ad.home.rules} {
		for _, pl := range pls {
			if z, _, _ := strings.Cut(pl.path, "/"); !contains(out, z) {
				out = append(out, z)
			}
		}
	}
	return out
}

// writeFiles writes every rendered file whose content changed (or all of them
// with force) and returns the per-file status plus the set of paths relative
// to root, which is what the track file records. A file that starts with a
// shebang is written executable.
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
			mode := os.FileMode(0o644)
			if bytes.HasPrefix(f.bytes, []byte("#!")) {
				mode = 0o755
			}
			if err := os.WriteFile(f.path, f.bytes, mode); err != nil {
				return nil, nil, err
			}
			if mode != 0o644 {
				if err := os.Chmod(f.path, mode); err != nil {
					return nil, nil, err
				}
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

// writeTrack records the set of paths (relative to the track file's tree) on disk.
func writeTrack(trackPath string, set map[string]bool) error {
	cur := make([]string, 0, len(set))
	for r := range set {
		cur = append(cur, r)
	}
	sort.Strings(cur)
	b, _ := jsonIndent(cur)
	if err := os.MkdirAll(filepath.Dir(trackPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(trackPath, b, 0o644)
}
