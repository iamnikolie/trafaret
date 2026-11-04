package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// --- render ---------------------------------------------------------------

type outFile struct {
	path  string // absolute destination
	bytes []byte
}

// render produces every artifact for a project given the active bundles and targets.
//
// Claude skills and agents render into .claude/ — Cursor reads that tree natively
// via its cross-agent compatibility (.claude/skills, .claude/agents per the
// Cursor docs), and a parallel .cursor/ copy would double-load every skill
// (Cursor does not dedupe same-named skills). Codex gets skills in .agents/skills
// and native TOML custom agents in .codex/agents.
func render(p Project, eng *Engine, activeBundles []string, targets []string) ([]outFile, error) {
	wantClaude := contains(targets, "claude")
	wantCursor := contains(targets, "cursor")
	wantCodex := contains(targets, "codex")
	wantClaudeTree := wantClaude || wantCursor // the shared .claude tree serves both harnesses

	skillSet := map[string]bool{}
	agentSet := map[string]bool{}
	ruleSet := map[string]bool{}
	for _, bn := range activeBundles {
		b, ok := eng.Bundles[bn]
		if !ok {
			return nil, fmt.Errorf("unknown bundle %q (not in templates/%s)", bn, bundlesFile)
		}
		for _, s := range b.Skills {
			skillSet[s] = true
		}
		for _, a := range b.Agents {
			agentSet[a] = true
		}
		for _, r := range b.CursorRules {
			ruleSet[r] = true
		}
	}

	var out []outFile

	// skills: render every file under templates/skills/<name>/ (.tmpl rendered, others rendered passthrough)
	// {{.TargetRoot}} resolves to the tree the file lands in (always .claude here).
	if wantClaudeTree {
		p.TargetRoot = ".claude"
		for s := range skillSet {
			srcDir := path.Join("skills", s)
			entries, err := fs.ReadDir(eng.Templates, srcDir)
			if err != nil {
				return nil, fmt.Errorf("skill %q: %w", s, err)
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				dstName := strings.TrimSuffix(e.Name(), ".tmpl")
				rendered, err := renderFile(eng.Templates, path.Join(srcDir, e.Name()), p)
				if err != nil {
					return nil, err
				}
				out = append(out, outFile{filepath.Join(p.RepoRoot, ".claude", "skills", s, dstName), rendered})
			}
		}

		// agents
		for a := range agentSet {
			src := path.Join("agents", a+".md.tmpl")
			rendered, err := renderFile(eng.Templates, src, p)
			if err != nil {
				return nil, err
			}
			out = append(out, outFile{filepath.Join(p.RepoRoot, ".claude", "agents", a+".md"), rendered})
		}
	}

	if wantCodex {
		p.TargetRoot = ".agents"
		for s := range skillSet {
			srcDir := path.Join("skills", s)
			entries, err := fs.ReadDir(eng.Templates, srcDir)
			if err != nil {
				return nil, fmt.Errorf("skill %q: %w", s, err)
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				dstName := strings.TrimSuffix(e.Name(), ".tmpl")
				rendered, err := renderFile(eng.Templates, path.Join(srcDir, e.Name()), p)
				if err != nil {
					return nil, err
				}
				out = append(out, outFile{filepath.Join(p.RepoRoot, ".agents", "skills", s, dstName), rendered})
			}
		}
		for a := range agentSet {
			rendered, err := renderFile(eng.Templates, path.Join("agents", a+".md.tmpl"), p)
			if err != nil {
				return nil, err
			}
			codex, err := codexAgent(rendered)
			if err != nil {
				return nil, fmt.Errorf("codex agent %q: %w", a, err)
			}
			// Keep the rendered Markdown contract available to orchestration skills.
			// Codex loads the TOML file; skills read .agents/agents/<name>.md when
			// they need the full contract as orientation material.
			out = append(out, outFile{filepath.Join(p.RepoRoot, ".agents", "agents", a+".md"), rendered})
			out = append(out, outFile{filepath.Join(p.RepoRoot, ".codex", "agents", a+".toml"), codex})
		}
	}

	// cursor rules (cursor target only)
	if wantCursor {
		for r := range ruleSet {
			src := path.Join("cursor", "rules", r+".mdc.tmpl")
			p.TargetRoot = ".cursor"
			rendered, err := renderFile(eng.Templates, src, p)
			if err != nil {
				return nil, err
			}
			out = append(out, outFile{filepath.Join(p.RepoRoot, ".cursor", "rules", r+".mdc"), rendered})
		}
	}

	// per-project verbatim sources: projects/<name>/{skills,agents}/** — copied as-is
	// (no template pass: hand-written skills may legitimately contain "{{").
	if wantClaudeTree || wantCodex {
		projSrc := filepath.Join(eng.ProjectsDir(), p.Name)
		for _, kind := range []string{"skills", "agents"} {
			root := filepath.Join(projSrc, kind)
			if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
				continue
			}
			walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
				if wantClaudeTree {
					out = append(out, outFile{filepath.Join(p.RepoRoot, ".claude", kind, rel), b})
				}
				if wantCodex && kind == "skills" {
					out = append(out, outFile{filepath.Join(p.RepoRoot, ".agents", kind, rel), b})
				}
				if wantCodex && kind == "agents" && filepath.Ext(rel) == ".md" {
					codex, err := codexAgent(b)
					if err != nil {
						return fmt.Errorf("codex project agent %q: %w", rel, err)
					}
					out = append(out, outFile{filepath.Join(p.RepoRoot, ".codex", "agents", strings.TrimSuffix(rel, ".md")+".toml"), codex})
				}
				return nil
			})
			if walkErr != nil {
				return nil, fmt.Errorf("project sources %s/%s: %w", p.Name, kind, walkErr)
			}
		}
	}

	// frontmatter lint: a rendered artifact whose YAML header silently truncates
	// (unquoted " #") or fails to parse must never reach a repo.
	for _, f := range out {
		if err := lintFrontmatter(f.path, f.bytes); err != nil {
			return nil, err
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out, nil
}

// templateFuncs are the helpers templates may call.
var templateFuncs = template.FuncMap{
	// dash joins words with hyphens: "LINC memory" -> "LINC-memory".
	"dash": func(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), " ", "-") },
}

// renderFile executes the template at src (a slash path inside the templates
// filesystem) against the project.
func renderFile(tmpl fs.FS, src string, p Project) ([]byte, error) {
	b, err := fs.ReadFile(tmpl, src)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", src, err)
	}
	t, err := template.New(path.Base(src)).Option("missingkey=error").Funcs(templateFuncs).Parse(string(b))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", src, err)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, p); err != nil {
		return nil, fmt.Errorf("render %s: %w", src, err)
	}
	return []byte(sb.String()), nil
}
