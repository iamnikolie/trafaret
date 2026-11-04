// Package engine is the trafaret core: the manifest model and its resolution, the
// bundle registry, template rendering, install and prune, homes, the managed
// session hook, and config-directory resolution with the embedded example
// templates layered under the user's on-disk overlay.
//
// Everything the binary needs from "its own side" (the example templates) is
// compiled in; everything that belongs to the user (manifest.yaml, global/,
// agents/, projects/, optional templates/ overlay) is read from the config
// directory.
package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/iamnikolie/trafaret/examples"
)

const (
	manifestName = "manifest.yaml"
	envConfig    = "TRAFARET_CONFIG"
)

// embeddedTemplates returns the example templates compiled into the binary,
// rooted so that paths read "skills/<name>/SKILL.md.tmpl", "agents/<name>.md.tmpl",
// "cursor/rules/<name>.mdc.tmpl".
func embeddedTemplates() fs.FS { return examples.FS }

// --- engine ---------------------------------------------------------------

// Engine is the resolved context of one invocation: where the user's config
// lives and which template set renders.
type Engine struct {
	ConfigDir  string            // directory holding the manifest; global/, agents/, projects/ live under it
	Manifest   string            // absolute path of the manifest file
	Templates  fs.FS             // embedded templates, with OverlayDir (if any) layered on top
	OverlayDir string            // on-disk templates overlay in effect; "" when there is none
	Bundles    map[string]Bundle // the registry from Templates' bundles.yaml
	Version    string            // the binary's version, checked against the manifest's engine: constraint
}

// GlobalDir is <config>/global (agents/ is its sibling).
func (e *Engine) GlobalDir() string { return filepath.Join(e.ConfigDir, "global") }

// ProjectsDir is <config>/projects — per-project verbatim sources.
func (e *Engine) ProjectsDir() string { return filepath.Join(e.ConfigDir, "projects") }

// Options are the command-line inputs that decide where an Engine reads from.
type Options struct {
	Config    string // --config: config directory (holds manifest.yaml)
	Manifest  string // --manifest: explicit manifest path; its directory becomes the config dir
	Templates string // --templates: templates overlay directory
	Version   string // the binary's version; empty means an unstamped "dev" build
}

// LoadManifest reads the manifest of this engine's config directory, enforcing
// its `engine:` constraint against the binary's version.
func (e *Engine) LoadManifest() (*Manifest, error) { return loadManifest(e.Manifest, e.Version) }

// New resolves the config directory from the flags, the environment and the
// working directory, then builds the template filesystem.
func New(o Options) (*Engine, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dir, manifest, err := resolveConfig(o.Config, o.Manifest, os.Getenv(envConfig), cwd)
	if err != nil {
		return nil, err
	}
	e := &Engine{ConfigDir: dir, Manifest: manifest, Templates: embeddedTemplates(), Version: o.Version}
	if e.Version == "" {
		e.Version = "dev"
	}

	overlay, explicit := o.Templates, o.Templates != ""
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
