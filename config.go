// Config resolution, the embedded template set with its on-disk overlay, and the
// manifest's engine-version gate. Everything the binary needs from "its own side"
// (templates) is compiled in; everything that belongs to the user (manifest.yaml,
// global/, agents/, projects/, optional templates/ overlay) is read from the
// config directory.
package main

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// version is stamped at build time: go build -ldflags "-X main.version=v1.2.3".
// An unstamped build is "dev", which satisfies every manifest `engine:` constraint.
var version = "dev"

const (
	manifestName = "manifest.yaml"
	envConfig    = "TRAFARET_CONFIG"
)

//go:embed templates
var embeddedFS embed.FS

// embeddedTemplates returns the templates compiled into the binary, rooted so
// that paths read "skills/<name>/SKILL.md.tmpl", "agents/<name>.md.tmpl",
// "cursor/rules/<name>.mdc.tmpl".
func embeddedTemplates() fs.FS {
	sub, err := fs.Sub(embeddedFS, "templates")
	if err != nil {
		panic(err) // the embed directive guarantees the directory
	}
	return sub
}

// --- engine ---------------------------------------------------------------

// Engine is the resolved context of one invocation: where the user's config
// lives and which template set renders.
type Engine struct {
	ConfigDir  string            // directory holding the manifest; global/, agents/, projects/ live under it
	Manifest   string            // absolute path of the manifest file
	Templates  fs.FS             // embedded templates, with OverlayDir (if any) layered on top
	OverlayDir string            // on-disk templates overlay in effect; "" when there is none
	Bundles    map[string]bundle // the registry from Templates' bundles.yaml
}

// GlobalDir is <config>/global (agents/ is its sibling).
func (e *Engine) GlobalDir() string { return filepath.Join(e.ConfigDir, "global") }

// ProjectsDir is <config>/projects — per-project verbatim sources.
func (e *Engine) ProjectsDir() string { return filepath.Join(e.ConfigDir, "projects") }

// newEngine resolves the config directory from the flags, the environment and
// the working directory, then builds the template filesystem.
func newEngine(o options) (*Engine, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dir, manifest, err := resolveConfig(o.config, o.manifest, os.Getenv(envConfig), cwd)
	if err != nil {
		return nil, err
	}
	e := &Engine{ConfigDir: dir, Manifest: manifest, Templates: embeddedTemplates()}

	overlay, explicit := o.templates, o.templates != ""
	if explicit {
		if overlay, err = absPath(overlay); err != nil {
			return nil, err
		}
	} else {
		overlay = filepath.Join(dir, "templates")
	}
	if fi, err := os.Stat(overlay); err == nil && fi.IsDir() {
		e.OverlayDir = overlay
		e.Templates = layerFS(os.DirFS(overlay), e.Templates)
	} else if explicit {
		return nil, fmt.Errorf("--templates %s: not a directory", overlay)
	}
	if e.Bundles, err = loadBundles(e.Templates); err != nil {
		return nil, err
	}
	return e, nil
}

// resolveConfig finds the config directory and the manifest file in it.
// Precedence: --manifest PATH (config dir = its directory), --config DIR,
// $TRAFARET_CONFIG, then the nearest ancestor of cwd holding a manifest.yaml.
func resolveConfig(flagConfig, flagManifest, envValue, cwd string) (dir, manifest string, err error) {
	switch {
	case flagManifest != "":
		m, err := absPath(flagManifest)
		if err != nil {
			return "", "", err
		}
		if !isFile(m) {
			return "", "", fmt.Errorf("--manifest %s: no such file", m)
		}
		return filepath.Dir(m), m, nil
	case flagConfig != "":
		return configFromDir(flagConfig, "--config")
	case envValue != "":
		return configFromDir(envValue, "$"+envConfig)
	}
	for d := cwd; ; {
		if m := filepath.Join(d, manifestName); isFile(m) {
			return d, m, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", "", fmt.Errorf("no %s found in %s or any parent directory; pass --config DIR or set $%s", manifestName, cwd, envConfig)
}

func configFromDir(d, origin string) (string, string, error) {
	abs, err := absPath(d)
	if err != nil {
		return "", "", err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return "", "", fmt.Errorf("%s %s: not a directory", origin, abs)
	}
	m := filepath.Join(abs, manifestName)
	if !isFile(m) {
		return "", "", fmt.Errorf("%s %s: no %s in it", origin, abs, manifestName)
	}
	return abs, m, nil
}

func absPath(p string) (string, error) {
	p = expandHome(p)
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	return filepath.Abs(p)
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// --- templates overlay ----------------------------------------------------

// layeredFS serves top over base: a file present in top shadows the file with
// the same path in base, directories are merged, and files only in top are
// added. When top and base disagree on a path's kind (file vs directory), top wins.
type layeredFS struct{ top, base fs.FS }

func layerFS(top, base fs.FS) fs.FS { return layeredFS{top: top, base: base} }

var (
	_ fs.ReadFileFS = layeredFS{}
	_ fs.ReadDirFS  = layeredFS{}
	_ fs.StatFS     = layeredFS{}
)

func (l layeredFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	if fi, err := fs.Stat(l.top, name); err == nil {
		return fi, nil
	}
	return fs.Stat(l.base, name)
}

func (l layeredFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}
	if fi, err := fs.Stat(l.top, name); err == nil {
		if fi.IsDir() {
			return nil, &fs.PathError{Op: "readfile", Path: name, Err: errors.New("is a directory")}
		}
		return fs.ReadFile(l.top, name)
	}
	return fs.ReadFile(l.base, name)
}

func (l layeredFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	topInfo, topErr := fs.Stat(l.top, name)
	if topErr == nil && !topInfo.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a directory")}
	}
	baseInfo, baseErr := fs.Stat(l.base, name)
	baseIsDir := baseErr == nil && baseInfo.IsDir()
	if topErr != nil && !baseIsDir {
		if baseErr == nil {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: errors.New("not a directory")}
		}
		return nil, baseErr
	}
	merged := map[string]fs.DirEntry{}
	if baseIsDir {
		entries, err := fs.ReadDir(l.base, name)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			merged[e.Name()] = e
		}
	}
	if topErr == nil {
		entries, err := fs.ReadDir(l.top, name)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			merged[e.Name()] = e
		}
	}
	out := make([]fs.DirEntry, 0, len(merged))
	for _, e := range merged {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

func (l layeredFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	fi, err := l.Stat(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if !fi.IsDir() {
		if _, err := fs.Stat(l.top, name); err == nil {
			return l.top.Open(name)
		}
		return l.base.Open(name)
	}
	entries, err := l.ReadDir(name)
	if err != nil {
		return nil, err
	}
	return &mergedDir{info: fi, entries: entries}, nil
}

// mergedDir is the fs.File of a directory in a layeredFS.
type mergedDir struct {
	info    fs.FileInfo
	entries []fs.DirEntry
	off     int
}

func (d *mergedDir) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *mergedDir) Close() error               { return nil }
func (d *mergedDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.Name(), Err: errors.New("is a directory")}
}
func (d *mergedDir) ReadDir(n int) ([]fs.DirEntry, error) {
	rest := d.entries[d.off:]
	if n <= 0 {
		d.off = len(d.entries)
		return rest, nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	if n > len(rest) {
		n = len(rest)
	}
	d.off += n
	return rest[:n], nil
}

// --- engine version gate ----------------------------------------------------

// checkEngine enforces the manifest's optional `engine:` field against the
// binary's version. An empty constraint means no check; an unstamped ("dev") or
// otherwise non-numeric binary version always passes. Accepted forms: an exact
// version ("0.2.1", "v0.2.1") or one or more comma-separated comparisons
// (">=0.2.0", ">0.2", "<1.0.0", "<=0.9", "=0.2.1"; "X.Y" means X.Y.0). A
// pre-release or build suffix on either side ("-rc1", "+meta") is ignored.
// A constraint this binary cannot parse is an error: a typo must not silently
// disable the gate.
func checkEngine(constraint, ver string) error {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return nil
	}
	reqs, err := parseConstraint(constraint)
	if err != nil {
		return fmt.Errorf("manifest engine %q: %w", constraint, err)
	}
	have, ok := parseVersion(ver)
	if !ok {
		return nil // dev / unstamped build
	}
	for _, r := range reqs {
		if !r.holds(have) {
			return fmt.Errorf("manifest requires trafaret %s, this binary is %s — upgrade trafaret (or relax engine: in %s)", constraint, ver, manifestName)
		}
	}
	return nil
}

type semver [3]int

func (a semver) cmp(b semver) int {
	for i := range a {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}

type requirement struct {
	op  string // >=, >, <=, <, =
	ver semver
}

func (r requirement) holds(have semver) bool {
	c := have.cmp(r.ver)
	switch r.op {
	case ">=":
		return c >= 0
	case ">":
		return c > 0
	case "<=":
		return c <= 0
	case "<":
		return c < 0
	default:
		return c == 0
	}
}

func parseConstraint(s string) ([]requirement, error) {
	var out []requirement
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		op := "="
		for _, cand := range []string{">=", "<=", "==", ">", "<", "="} {
			if strings.HasPrefix(part, cand) {
				op, part = cand, strings.TrimSpace(strings.TrimPrefix(part, cand))
				break
			}
		}
		if op == "==" {
			op = "="
		}
		v, ok := parseVersion(part)
		if !ok {
			return nil, fmt.Errorf("%q is not a version like X.Y.Z (operators: >= > <= < =)", part)
		}
		out = append(out, requirement{op: op, ver: v})
	}
	return out, nil
}

// parseVersion reads "X.Y.Z" or "X.Y" (optionally "v"-prefixed, optionally with
// a -prerelease / +build suffix). Anything else — "dev", a bare commit hash —
// is not a version.
func parseVersion(s string) (semver, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return semver{}, false
	}
	var v semver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}, false
		}
		v[i] = n
	}
	return v, true
}
