package engine

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// --- bundle registry ------------------------------------------------------
// A bundle maps a name to the skills, agents and rules it installs. The
// registry is data, not code: templates/bundles.yaml in the template set (the
// embedded examples, or a config's templates/ overlay, whose bundles.yaml
// shadows the embedded one).

const bundlesFile = "bundles.yaml"

// Bundle is one entry of the registry.
type Bundle struct {
	Skills []string `yaml:"skills"`
	Agents []string `yaml:"agents"`
	Rules  []string `yaml:"rules"`
	// Requires lists the var paths (dotted, e.g. "tracker.kind") the bundle's
	// templates read; validate checks that each exists in the merged vars of every
	// project or home that carries the bundle.
	Requires []string `yaml:"requires"`
	// SessionContext, when set, is a file under .claude/skills/ (e.g.
	// "my-skill/session-context.md") that a SessionStart hook in the project's
	// .claude/settings.json prints at every session start. At most one active
	// bundle of a project may set it.
	SessionContext string `yaml:"sessionContext"`
}

// loadBundles reads the bundle registry from the template set.
func loadBundles(fsys fs.FS) (map[string]Bundle, error) {
	b, err := fs.ReadFile(fsys, bundlesFile)
	if err != nil {
		return nil, fmt.Errorf("templates/%s: %w", bundlesFile, err)
	}
	var out map[string]Bundle
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&out); err != nil && err != io.EOF {
		msg := err.Error()
		if strings.Contains(msg, "cursorRules") {
			msg += " (cursorRules was renamed to rules in trafaret 0.3; rules live in templates/rules/)"
		}
		return nil, fmt.Errorf("templates/%s: %s", bundlesFile, msg)
	}
	for name, bd := range out {
		for _, r := range bd.Requires {
			if !validVarPath(r) {
				return nil, fmt.Errorf("templates/%s: bundle %q requires %q, which is not a var path like \"tracker.kind\"", bundlesFile, name, r)
			}
		}
		// the path lands inside a double-quoted shell command: keep it plain
		if sc := bd.SessionContext; sc != "" && (path.IsAbs(sc) || strings.Contains(sc, "..") || strings.ContainsAny(sc, "\"\\$`")) {
			return nil, fmt.Errorf("templates/%s: bundle %q sessionContext %q must be a relative path under .claude/skills/", bundlesFile, name, bd.SessionContext)
		}
	}
	return out, nil
}

func validVarPath(p string) bool {
	if p == "" {
		return false
	}
	for _, seg := range strings.Split(p, ".") {
		if seg == "" || strings.ContainsAny(seg, " \t\r\n") {
			return false
		}
	}
	return true
}

// sessionContext returns the session-context file of the active bundles ("" for
// none) or an error when more than one bundle sets one.
func sessionContext(reg map[string]Bundle, active []string) (string, error) {
	var got, from string
	for _, bn := range active {
		sc := reg[bn].SessionContext
		if sc == "" {
			continue
		}
		if got != "" && sc != got {
			return "", fmt.Errorf("bundles %q and %q both set sessionContext; at most one may", from, bn)
		}
		got, from = sc, bn
	}
	return got, nil
}
