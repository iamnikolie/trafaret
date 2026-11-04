package engine

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// codexAgent converts the rendered Claude Markdown agent format into Codex's
// project-scoped custom-agent TOML. The body remains the single instruction source.
func codexAgent(b []byte) ([]byte, error) {
	s := string(b)
	if !strings.HasPrefix(s, "---\n") {
		return nil, fmt.Errorf("missing YAML frontmatter")
	}
	parts := strings.SplitN(strings.TrimPrefix(s, "---\n"), "\n---\n", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("unterminated YAML frontmatter")
	}
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Tools       string `yaml:"tools"`
		Model       string `yaml:"model"`
	}
	if err := yaml.Unmarshal([]byte(parts[0]), &meta); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	if meta.Name == "" || meta.Description == "" {
		return nil, fmt.Errorf("name and description are required")
	}
	var out strings.Builder
	fmt.Fprintf(&out, "name = %s\ndescription = %s\n", strconv.Quote(meta.Name), strconv.Quote(meta.Description))
	if meta.Model == "sonnet" {
		out.WriteString("model = \"gpt-5.6-terra\"\nmodel_reasoning_effort = \"medium\"\n")
	}
	if !strings.Contains(meta.Tools, "Edit") && !strings.Contains(meta.Tools, "Write") {
		out.WriteString("sandbox_mode = \"read-only\"\n")
	}
	fmt.Fprintf(&out, "developer_instructions = %s\n", strconv.Quote(strings.TrimSpace(parts[1])))
	return []byte(out.String()), nil
}
