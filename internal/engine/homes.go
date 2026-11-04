package engine

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// --- homes ----------------------------------------------------------------

// homeTrackFile sits in the home's skills dir and records what trafaret wrote there,
// so a skill removed from global/skills (or from a home's skills: list) is
// pruned — and nothing trafaret did not write (the harness's own skills, a synced/
// dir) is ever touched.
const homeTrackFile = ".ao-manifest.json"

// globalSkills lists the global skills: every dir under global/skills plus
// every dir under agents/<role>/skills (agents/ sits next to global/). The
// result maps skill name → source dir; names must be unique across all
// sources, since they land side by side in one home.
func globalSkills(globalDir string) (map[string]string, error) {
	out := map[string]string{}
	add := func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if prev, dup := out[e.Name()]; dup {
				return fmt.Errorf("global skill %q defined twice: %s and %s", e.Name(), prev, filepath.Join(dir, e.Name()))
			}
			out[e.Name()] = filepath.Join(dir, e.Name())
		}
		return nil
	}
	if err := add(filepath.Join(globalDir, "skills")); err != nil {
		return nil, err
	}
	agentsDir := filepath.Join(filepath.Dir(globalDir), "agents")
	roles, err := os.ReadDir(agentsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, r := range roles {
		if !r.IsDir() || strings.HasPrefix(r.Name(), ".") {
			continue
		}
		if err := add(filepath.Join(agentsDir, r.Name(), "skills")); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// skillNames returns the sorted keys of a globalSkills map.
func skillNames(m map[string]string) []string {
	var out []string
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// renderHome copies each global skill dir (global/skills/<name>/** and
// agents/<role>/skills/<name>/**) verbatim into the home's skills dir. No
// template pass: global skills carry no project tokens.
func renderHome(h Home, globalDir string) ([]outFile, error) {
	if h.SkillsDir == "" || !filepath.IsAbs(h.SkillsDir) {
		return nil, fmt.Errorf("home %q: skillsDir must be an absolute or ~/ path, got %q", h.Name, h.SkillsDir)
	}
	sources, err := globalSkills(globalDir)
	if err != nil {
		return nil, err
	}
	all := skillNames(sources)
	skills := all
	if len(h.Skills) > 0 {
		for _, s := range h.Skills {
			if !contains(all, s) {
				return nil, fmt.Errorf("home %q: no global skill %q in global/skills or agents/*/skills", h.Name, s)
			}
		}
		skills = h.Skills
	}
	if len(h.Skip) > 0 {
		for _, s := range h.Skip {
			if !contains(all, s) {
				return nil, fmt.Errorf("home %q: skip names no global skill %q in global/skills or agents/*/skills", h.Name, s)
			}
		}
		var kept []string
		for _, s := range skills {
			if !contains(h.Skip, s) {
				kept = append(kept, s)
			}
		}
		skills = kept
	}
	var out []outFile
	for _, s := range skills {
		root := sources[s]
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			out = append(out, outFile{filepath.Join(h.SkillsDir, s, rel), b})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("global skill %q: %w", s, err)
		}
	}
	for _, f := range out {
		if err := lintFrontmatter(f.path, f.bytes); err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out, nil
}

func installHome(h Home, globalDir string, force, dryRun bool) ([]FileStatus, error) {
	files, err := renderHome(h, globalDir)
	if err != nil {
		return nil, err
	}
	res, newSet, err := writeFiles(files, h.SkillsDir, force, dryRun)
	if err != nil {
		return nil, err
	}
	removed, err := pruneHome(h.SkillsDir, newSet, dryRun)
	if err != nil {
		return nil, err
	}
	return append(res, removed...), nil
}

// pruneHome deletes files recorded on the last install that the current render
// no longer produces, then rewrites the track file.
func pruneHome(dir string, newSet map[string]bool, dryRun bool) ([]FileStatus, error) {
	trackPath := filepath.Join(dir, homeTrackFile)
	var old []string
	if b, err := os.ReadFile(trackPath); err == nil {
		if err := json.Unmarshal(b, &old); err != nil {
			return nil, fmt.Errorf("%s: corrupt track file: %w", trackPath, err)
		}
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
	if !dryRun {
		cur := make([]string, 0, len(newSet))
		for r := range newSet {
			cur = append(cur, r)
		}
		sort.Strings(cur)
		b, _ := json.MarshalIndent(cur, "", "  ")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(trackPath, append(b, '\n'), 0o644); err != nil {
			return nil, err
		}
	}
	return res, nil
}
