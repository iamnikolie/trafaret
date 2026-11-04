package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// lintFrontmatter validates the YAML frontmatter of rendered .md/.mdc artifacts:
// it must parse, SKILL.md must carry non-empty name+description, and no unquoted
// plain scalar may contain " #" (YAML starts a comment there and silently drops
// the rest of the value — a proven footgun).
func lintFrontmatter(path string, b []byte) error {
	if ext := filepath.Ext(path); ext != ".md" && ext != ".mdc" {
		return nil
	}
	s := string(b)
	if !strings.HasPrefix(s, "---\n") {
		return nil
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return fmt.Errorf("%s: unterminated frontmatter", path)
	}
	fm := s[4 : 4+end]
	var m map[string]any
	if err := yaml.Unmarshal([]byte(fm), &m); err != nil {
		return fmt.Errorf("%s: frontmatter is not valid YAML: %w", path, err)
	}
	if filepath.Base(path) == "SKILL.md" {
		for _, k := range []string{"name", "description"} {
			if v, _ := m[k].(string); strings.TrimSpace(v) == "" {
				return fmt.Errorf("%s: frontmatter missing non-empty %q", path, k)
			}
		}
	}
	for _, line := range strings.Split(fm, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if !ok || k == "" || strings.Contains(k, " ") {
			continue
		}
		switch {
		case v == "", v[0] == '"', v[0] == '\'', v[0] == '|', v[0] == '>':
			continue
		}
		if strings.Contains(v, " #") {
			return fmt.Errorf("%s: frontmatter %q contains ' #' in an unquoted value — YAML truncates it there; quote the value or drop the '#'", path, k)
		}
	}
	return nil
}
