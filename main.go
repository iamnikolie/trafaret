// Command trafaret renders a shared agent/skill/rule library into projects.
//
// It reads manifest.yaml from the config directory, resolves one Project per entry,
// renders the Go templates (embedded in the binary, optionally overlaid by
// <config>/templates) with that project's tokens, and writes the concrete artifacts
// into <repoRoot>/.claude/, .cursor/, and .agents/.codex when targeted. Writes are
// sha256-idempotent. Homes (per-user harness dirs) receive global/skills/ and
// agents/<role>/skills/ from the config directory verbatim.
//
//	trafaret install <project> [--all] [--target claude|cursor|codex|all] [--what bundle,...] [--force] [--dry-run] [--json]
//	trafaret list [--json]
//	trafaret validate
//	trafaret diff <project>            # dry-run that prints per-file status
//
// Config directory: --config DIR, else $TRAFARET_CONFIG, else the nearest parent
// of the working directory that holds a manifest.yaml (see config.go).
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// --- manifest types -------------------------------------------------------

type Manifest struct {
	// Engine is an optional version constraint on the trafaret binary, e.g.
	// ">=0.2.0" or "0.2.1". Empty = no check. See checkEngine.
	Engine   string             `yaml:"engine"`
	Defaults Defaults           `yaml:"defaults"`
	Projects map[string]Project `yaml:"projects"`
	Homes    map[string]Home    `yaml:"homes"`
}

// Home is a per-user harness config directory (~/.claude, ~/.codex, ~/.cursor, …)
// that receives the global skills: global/skills/ — skill wrappers for the CLIs
// you use everywhere — and agents/<role>/skills/ — the skills a role agent (a
// long-lived named session such as a designer) brings to every repo. Both apply
// everywhere, not to one project.
type Home struct {
	Name      string   `yaml:"-"`         // filled from map key
	SkillsDir string   `yaml:"skillsDir"` // ~ is expanded
	Skills    []string `yaml:"skills"`    // subset of the global skills; empty = all of them
	Skip      []string `yaml:"skip"`      // global skills this home does not take (harness-specific ones)
}

type Defaults struct {
	Workspace string   `yaml:"workspace"`
	Targets   []string `yaml:"targets"`
	// Prose fragments every project inherits unless it sets its own.
	Tickets struct {
		ExtraFields string `yaml:"extraFields"` // -> tickets.extraFields
	} `yaml:"tickets"`
	Tools struct {
		MemoryName string `yaml:"memoryName"` // -> tools.memoryName
	} `yaml:"tools"`
}

type Project struct {
	Name       string   `yaml:"-"`        // filled from map key
	TargetRoot string   `yaml:"-"`        // set per render target: ".claude" or ".cursor"
	RepoRoot   string   `yaml:"repoRoot"` // resolved to absolute in place
	Bundles    []string `yaml:"bundles"`
	Disabled   []string `yaml:"disabled"` // bundles declared but switched off; install prunes their artifacts
	Targets    []string `yaml:"targets"`  // optional per-project override of defaults.targets
	Tickets    Tickets  `yaml:"tickets"`
	VCS        VCS      `yaml:"vcs"`
	Gates      Gates    `yaml:"gates"`
	Docs       Docs     `yaml:"docs"`
	Stack      Stack    `yaml:"stack"`
	Tools      Tools    `yaml:"tools"`
	Run        Run      `yaml:"run"`
}

// Run holds this project's default answers to the questions a run-style skill
// asks at start (review profile, work order). They are defaults, not a lock: the
// operator may override any of them interactively, and the skill can log the
// resolved plan either way.
type Run struct {
	Profile     string `yaml:"profile"`     // careful | balanced | fast
	Order       string `yaml:"order"`       // priority | cheap-first
	MaxRounds   int    `yaml:"maxRounds"`   // 0 = take the profile's budget
	Overlap     *bool  `yaml:"overlap"`     // nil = take the profile's setting
	Interactive *bool  `yaml:"interactive"` // nil = true; false pins the defaults and asks nothing
	// computed — what the skill actually branches on
	ReviewDepth string `yaml:"-"` // per-item | split
	OverlapOn   bool   `yaml:"-"`
	AskAtStart  bool   `yaml:"-"`
}

// runProfiles expands a profile name into the review depth and round budget it
// implies. The names are the operator-facing vocabulary; the expansion is what
// the skill actually branches on.
var runProfiles = map[string]struct {
	ReviewDepth string
	MaxRounds   int
	Overlap     bool
}{
	"careful":  {ReviewDepth: "per-item", MaxRounds: 3, Overlap: false},
	"balanced": {ReviewDepth: "per-item", MaxRounds: 3, Overlap: true},
	"fast":     {ReviewDepth: "split", MaxRounds: 2, Overlap: true},
}

type Tickets struct {
	CLI       string    `yaml:"cli"`
	Config    string    `yaml:"config"`
	Space     string    `yaml:"space"`
	Host      string    `yaml:"host"`
	Boards    Boards    `yaml:"boards"`
	States    States    `yaml:"states"`
	Priority  []string  `yaml:"priority"`
	RefScheme RefScheme `yaml:"refScheme"`
	DependsOn string    `yaml:"dependsOn"`
	// Work mode: every project's tasks and bugs live in one shared database
	// (db, e.g. "Development/Work"), told apart by its Project and Type fields.
	// Space is then the shared database's space (field prefix).
	DB      string `yaml:"db"`
	Project string `yaml:"project"`
	// ExtraFields is free text appended to the Work-mode list of "other fields
	// agents use" in a ticket-protocol skill — workspace-specific fields beyond Parent,
	// Blocked by and MR, e.g. "`Dev/Seat` (the role holding it)". Empty = none.
	ExtraFields string `yaml:"extraFields"`
	// computed
	TasksDB       string `yaml:"-"`
	BugsDB        string `yaml:"-"`
	PriorityChain string `yaml:"-"`
	ConfigDir     string `yaml:"-"`
	Work          bool   `yaml:"-"`
	ProjectFilter string `yaml:"-"` // ` --filter "<Space>/Project=<P>"` (leading space) or ""
	TaskFilter    string `yaml:"-"` // ProjectFilter + Type=Task
	BugFilter     string `yaml:"-"` // ProjectFilter + Type=Bug
	TaskSet       string `yaml:"-"` // ` "<Space>/Project=<P>" "<Space>/Type=Task"` for create, or ""
	BugSet        string `yaml:"-"`
	ProjectWhere  string `yaml:"-"` // FQL `"q/where": […],` line for raw queries, or ""
	ProjectParams string `yaml:"-"` // ` --params '{"$project":"<P>"}'`, or ""
}

type Boards struct {
	Tasks string `yaml:"tasks"`
	Bugs  string `yaml:"bugs"`
}

type States struct {
	Backlog    string `yaml:"backlog"`
	Todo       string `yaml:"todo"`
	InProgress string `yaml:"inProgress"`
	InReview   string `yaml:"inReview"`
	Done       string `yaml:"done"`
}

type RefScheme struct {
	Task string `yaml:"task"`
	Bug  string `yaml:"bug"`
}

type VCS struct {
	DefaultBranch    string `yaml:"defaultBranch"`
	Worktree         bool   `yaml:"worktree"`
	Push             bool   `yaml:"push"`
	CommitConvention string `yaml:"commitConvention"`
	GitlabPath       string `yaml:"gitlabPath"`
}

type Gates struct {
	Packages     []Package `yaml:"packages"`
	LintBaseline string    `yaml:"lintBaseline"`
}

type Package struct {
	Name string `yaml:"name"`
	Dir  string `yaml:"dir"`
	Cmd  string `yaml:"cmd"`
}

type Docs struct {
	Map               string   `yaml:"map"`
	ContextDir        string   `yaml:"contextDir"`
	SourceDir         string   `yaml:"sourceDir"`
	Architect         string   `yaml:"architect"`
	SpecDir           string   `yaml:"specDir"`
	PlanDir           string   `yaml:"planDir"`
	SyncCmd           string   `yaml:"syncCmd"`
	CheckCmd          string   `yaml:"checkCmd"`
	CommitPaths       []string `yaml:"commitPaths"` // paths a docs skill stages on commit; defaults to the dir of Map
	MandatorySections []string `yaml:"mandatorySections"`
}

type Stack struct {
	EnvPrefix       string   `yaml:"envPrefix"`
	ConventionsDocs []string `yaml:"conventionsDocs"`
	SubScopeAgents  []string `yaml:"subScopeAgents"`
}

type Tools struct {
	Memory string `yaml:"memory"`
	// MemoryName is what prose calls the memory system behind the Memory CLI
	// ("the lm <name> CLI"). Defaults to "memory".
	MemoryName string `yaml:"memoryName"`
	E2E        E2E    `yaml:"e2e"`
	DB         DB     `yaml:"db"`
}

// DB is read-only database access for the db bundle: the svidoq profile and
// environment that reach the project's database, and the schema that is its own.
type DB struct {
	CLI    string `yaml:"cli"`    // svidoq
	Config string `yaml:"config"` // svidoq profile (~/.svidoq/<config>)
	Env    string `yaml:"env"`    // environment inside the profile
	Schema string `yaml:"schema"` // the project's own database; empty = the repo spans every schema (infra)
	Tunnel string `yaml:"tunnel"` // idempotent command that makes the server reachable
	// computed
	Cmd string `yaml:"-"` // "<cli> --config <config> --env <env>" — the prefix of every call
}

type E2E struct {
	RunCmd   string `yaml:"runCmd"`
	Report   string `yaml:"report"`
	FileBugs string `yaml:"fileBugs"`
}

// --- bundle registry ------------------------------------------------------
// A bundle maps a name to the templates it installs. The registry is data, not
// code: templates/bundles.yaml in the template set (the embedded examples, or a
// config's templates/ overlay, whose bundles.yaml shadows the embedded one).

const bundlesFile = "bundles.yaml"

type bundle struct {
	Skills      []string `yaml:"skills"`
	Agents      []string `yaml:"agents"`
	CursorRules []string `yaml:"cursorRules"`
	// Requires names the manifest blocks the bundle's templates read, so
	// validate checks them: "tickets" (the ticket protocol + run block) and/or
	// "db" (tools.db). A bundle that requires nothing renders from the bare
	// project fields.
	Requires []string `yaml:"requires"`
	// SessionContext, when set, is a file under .claude/skills/ (e.g.
	// "my-skill/session-context.md") that a SessionStart hook in the project's
	// .claude/settings.json prints at every session start. At most one active
	// bundle of a project may set it.
	SessionContext string `yaml:"sessionContext"`
}

func (b bundle) requires(block string) bool { return contains(b.Requires, block) }

// loadBundles reads the bundle registry from the template set.
func loadBundles(fsys fs.FS) (map[string]bundle, error) {
	b, err := fs.ReadFile(fsys, bundlesFile)
	if err != nil {
		return nil, fmt.Errorf("templates/%s: %w", bundlesFile, err)
	}
	var out map[string]bundle
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("templates/%s: %w", bundlesFile, err)
	}
	for name, bd := range out {
		for _, r := range bd.Requires {
			if r != "tickets" && r != "db" {
				return nil, fmt.Errorf("templates/%s: bundle %q requires unknown block %q (want tickets|db)", bundlesFile, name, r)
			}
		}
		// the path lands inside a double-quoted shell command: keep it plain
		if sc := bd.SessionContext; sc != "" && (path.IsAbs(sc) || strings.Contains(sc, "..") || strings.ContainsAny(sc, "\"\\$`")) {
			return nil, fmt.Errorf("templates/%s: bundle %q sessionContext %q must be a relative path under .claude/skills/", bundlesFile, name, bd.SessionContext)
		}
	}
	return out, nil
}

// sessionContext returns the session-context file of the active bundles ("" for
// none) or an error when more than one bundle sets one.
func sessionContext(reg map[string]bundle, active []string) (string, error) {
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

// --- load + resolve -------------------------------------------------------

func loadManifest(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := checkEngine(m.Engine, version); err != nil {
		return nil, err
	}
	for name, p := range m.Projects {
		p.Name = name
		resolve(&p, m.Defaults)
		m.Projects[name] = p
	}
	for name, h := range m.Homes {
		h.Name = name
		h.SkillsDir = expandHome(h.SkillsDir)
		m.Homes[name] = h
	}
	return &m, nil
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func resolve(p *Project, d Defaults) {
	if p.RepoRoot != "" && !filepath.IsAbs(p.RepoRoot) {
		p.RepoRoot = filepath.Join(d.Workspace, p.RepoRoot)
	}
	if len(p.Targets) == 0 {
		p.Targets = d.Targets
	}
	t := &p.Tickets
	if t.ExtraFields == "" {
		t.ExtraFields = d.Tickets.ExtraFields
	}
	if p.Tools.MemoryName == "" {
		p.Tools.MemoryName = d.Tools.MemoryName
	}
	if p.Tools.MemoryName == "" {
		p.Tools.MemoryName = "memory"
	}
	t.TasksDB = t.Space + "/" + t.Boards.Tasks
	t.BugsDB = t.Space + "/" + t.Boards.Bugs
	if t.DB != "" {
		t.Work = true
		t.TasksDB, t.BugsDB = t.DB, t.DB
		f := func(field, v string) string { return fmt.Sprintf(` --filter "%s/%s=%s"`, t.Space, field, v) }
		set := func(field, v string) string { return fmt.Sprintf(` "%s/%s=%s"`, t.Space, field, v) }
		t.ProjectFilter = f("Project", t.Project)
		t.TaskFilter = t.ProjectFilter + f("Type", "Task")
		t.BugFilter = t.ProjectFilter + f("Type", "Bug")
		t.TaskSet = set("Project", t.Project) + set("Type", "Task")
		t.BugSet = set("Project", t.Project) + set("Type", "Bug")
		t.ProjectWhere = fmt.Sprintf(`"q/where": ["=", ["%s/Project", "enum/name"], "$project"],`, t.Space)
		t.ProjectParams = fmt.Sprintf(` --params '{"$project":"%s"}'`, t.Project)
	}
	t.PriorityChain = strings.Join(t.Priority, " → ")
	if t.Config != "" {
		t.ConfigDir = "~/.fibery/" + t.Config
	}
	// a docs skill stages these on commit; default to the directory holding the Map.
	if len(p.Docs.CommitPaths) == 0 && p.Docs.Map != "" {
		p.Docs.CommitPaths = []string{filepath.Dir(p.Docs.Map)}
	}
	resolveRun(&p.Run)
	if db := &p.Tools.DB; db.CLI != "" {
		db.Cmd = db.CLI + " --config " + db.Config
		if db.Env != "" {
			db.Cmd += " --env " + db.Env
		}
	}
}

// resolveRun fills the run block from its profile so the rendered skill always
// states concrete defaults. An unknown or empty profile falls back to
// "balanced" — validate() is what reports the typo; render must not produce a
// skill with an empty review depth.
func resolveRun(r *Run) {
	if r.Profile == "" {
		r.Profile = "balanced"
	}
	prof, ok := runProfiles[r.Profile]
	if !ok {
		prof = runProfiles["balanced"]
	}
	r.ReviewDepth = prof.ReviewDepth
	if r.MaxRounds == 0 {
		r.MaxRounds = prof.MaxRounds
	}
	r.OverlapOn = prof.Overlap
	if r.Overlap != nil {
		r.OverlapOn = *r.Overlap
	}
	r.AskAtStart = r.Interactive == nil || *r.Interactive
	if r.Order == "" {
		r.Order = "priority"
	}
}

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

// --- install --------------------------------------------------------------

type fileStatus struct {
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
func install(p Project, eng *Engine, activeBundles, targets []string, force, dryRun, prune, partialTargets bool) ([]fileStatus, error) {
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
			res = append(res, fileStatus{filepath.Join(p.RepoRoot, settingsFile), st})
		}
	}
	return res, nil
}

// writeFiles writes every rendered file whose content changed (or all of them
// with force) and returns the per-file status plus the set of paths relative
// to root, which is what the track file records.
func writeFiles(files []outFile, root string, force, dryRun bool) ([]fileStatus, map[string]bool, error) {
	var res []fileStatus
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
		res = append(res, fileStatus{f.path, status})
	}
	return res, newSet, nil
}

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

func installHome(h Home, globalDir string, force, dryRun bool) ([]fileStatus, error) {
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
func pruneHome(dir string, newSet map[string]bool, dryRun bool) ([]fileStatus, error) {
	trackPath := filepath.Join(dir, homeTrackFile)
	var old []string
	if b, err := os.ReadFile(trackPath); err == nil {
		if err := json.Unmarshal(b, &old); err != nil {
			return nil, fmt.Errorf("%s: corrupt track file: %w", trackPath, err)
		}
	}
	var res []fileStatus
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
		res = append(res, fileStatus{abs, "remove"})
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

const settingsFile = ".claude/settings.json"

// hookPrefix identifies the one settings entry trafaret owns: a command that
// prints a file under the project's .claude/skills/.
const hookPrefix = `cat "$CLAUDE_PROJECT_DIR/.claude/skills/`

func hookCommand(ctx string) string { return hookPrefix + ctx + `"` }

// ensureSessionHook additively merges, updates or removes trafaret's SessionStart
// hook in the project's .claude/settings.json, preserving every other key. ctx is
// the session-context file under .claude/skills/ ("" removes the hook). Returns
// the resulting fileStatus ("install"/"update"/"remove") or "" when nothing changed.
func ensureSessionHook(repoRoot, ctx string, dryRun bool) (string, error) {
	enable := ctx != ""
	path := filepath.Join(repoRoot, filepath.FromSlash(settingsFile))
	settings := map[string]any{}
	existed := false
	if b, err := os.ReadFile(path); err == nil {
		existed = true
		if err := json.Unmarshal(b, &settings); err != nil {
			return "", fmt.Errorf("%s: not valid JSON, refusing to manage the session hook: %w", path, err)
		}
	}

	hooks, _ := settings["hooks"].(map[string]any)
	starts, _ := hooks["SessionStart"].([]any)
	idx, current := -1, ""
	for i, g := range starts {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if inner, ok := gm["hooks"].([]any); ok {
			for _, h := range inner {
				if hm, ok := h.(map[string]any); ok {
					if cmd, _ := hm["command"].(string); strings.HasPrefix(cmd, hookPrefix) {
						idx, current = i, cmd
					}
				}
			}
		}
	}

	switch {
	case enable && current == hookCommand(ctx), !enable && idx < 0:
		return "", nil // already in the desired state
	case enable:
		entry := map[string]any{
			"matcher": "startup|clear|compact",
			"hooks": []any{map[string]any{
				"type":    "command",
				"command": hookCommand(ctx),
			}},
		}
		if idx >= 0 {
			starts[idx] = entry // another bundle's context took over
		} else {
			starts = append(starts, entry)
		}
		if hooks == nil {
			hooks = map[string]any{}
		}
		hooks["SessionStart"] = starts
		settings["hooks"] = hooks
		status := "update"
		if !existed {
			status = "install"
		}
		if !dryRun {
			if err := writeSettings(path, settings); err != nil {
				return "", err
			}
		}
		return status, nil
	default: // disable
		starts = append(starts[:idx], starts[idx+1:]...)
		if len(starts) == 0 {
			delete(hooks, "SessionStart")
		} else {
			hooks["SessionStart"] = starts
		}
		if len(hooks) == 0 {
			delete(settings, "hooks")
		} else {
			settings["hooks"] = hooks
		}
		if !dryRun {
			if len(settings) == 0 {
				if err := os.Remove(path); err != nil {
					return "", err
				}
				return "remove", nil
			}
			if err := writeSettings(path, settings); err != nil {
				return "", err
			}
		} else if len(settings) == 0 {
			return "remove", nil
		}
		return "update", nil
	}
}

func writeSettings(path string, settings map[string]any) error {
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// pruneStale deletes previously-recorded artifacts that the current render no
// longer produces, then rewrites the track file with the current set.
func pruneStale(repoRoot string, newSet map[string]bool, targets []string, preserveOtherTargets, dryRun bool) ([]fileStatus, error) {
	trackPath := filepath.Join(repoRoot, filepath.FromSlash(trackFile))
	var old []string
	if b, err := os.ReadFile(trackPath); err == nil {
		if err := json.Unmarshal(b, &old); err != nil {
			return nil, fmt.Errorf("%s: corrupt track file: %w", trackPath, err)
		}
	}
	if preserveOtherTargets {
		for _, r := range old {
			r = filepath.ToSlash(filepath.Clean(r))
			if !pathBelongsToTargets(r, targets) {
				newSet[r] = true
			}
		}
	}
	var res []fileStatus
	for _, r := range old {
		r = filepath.ToSlash(filepath.Clean(r))
		if newSet[r] || strings.Contains(r, "..") ||
			!(strings.HasPrefix(r, ".claude/") || strings.HasPrefix(r, ".cursor/") ||
				strings.HasPrefix(r, ".agents/") || strings.HasPrefix(r, ".codex/")) {
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
				base := filepath.Base(d)
				if base == ".claude" || base == ".cursor" || base == ".agents" || base == ".codex" || d == repoRoot {
					break
				}
				if os.Remove(d) != nil { // non-empty or protected — stop
					break
				}
			}
		}
		res = append(res, fileStatus{abs, "remove"})
	}
	if !dryRun {
		cur := make([]string, 0, len(newSet))
		for r := range newSet {
			cur = append(cur, r)
		}
		sort.Strings(cur)
		b, _ := json.MarshalIndent(cur, "", "  ")
		if err := os.MkdirAll(filepath.Dir(trackPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(trackPath, append(b, '\n'), 0o644); err != nil {
			return nil, err
		}
	}
	return res, nil
}

func pathBelongsToTargets(path string, targets []string) bool {
	switch {
	case strings.HasPrefix(path, ".agents/"), strings.HasPrefix(path, ".codex/"):
		return contains(targets, "codex")
	case strings.HasPrefix(path, ".cursor/"):
		return contains(targets, "cursor")
	case strings.HasPrefix(path, ".claude/"):
		return contains(targets, "claude") || contains(targets, "cursor")
	default:
		return false
	}
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h)
}

// --- commands -------------------------------------------------------------

func main() {
	cmd, args := splitCommand(os.Args[1:])
	if cmd == "" {
		usage()
		os.Exit(2)
	}

	switch cmd {
	case "-h", "--help", "help":
		usage()
		return
	case "version", "-V", "--version":
		fmt.Println(version)
		return
	case "install", "diff", "list", "validate", "serve", "root":
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		os.Exit(2)
	}

	// shared flag parsing (very small, hand-rolled)
	opt := parseFlags(args)
	eng, err := newEngine(opt)
	check(err)

	switch cmd {
	case "install":
		cmdInstall(eng, opt, false)
	case "diff":
		opt.dryRun = true
		cmdInstall(eng, opt, true)
	case "list":
		cmdList(eng, opt)
	case "validate":
		cmdValidate(eng)
	case "serve":
		cmdServe(eng, opt)
	case "root":
		fmt.Println(eng.ConfigDir)
	}
}

// valueFlags are the flags that take a separate value argument, so the command
// scan can step over them: `trafaret --config DIR diff x` and `trafaret diff x
// --config DIR` are the same invocation.
var valueFlags = map[string]bool{
	"--config": true, "--manifest": true, "--templates": true,
	"--target": true, "--what": true, "--addr": true,
}

// splitCommand returns the command (the first non-flag argument, or -h/--help/
// -V/--version) and the remaining arguments in order.
func splitCommand(args []string) (string, []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h", a == "--help", a == "-V", a == "--version", !strings.HasPrefix(a, "-"):
			rest := append(append([]string{}, args[:i]...), args[i+1:]...)
			return a, rest
		case valueFlags[a]:
			i++
		}
	}
	return "", args
}

func cmdInstall(eng *Engine, opt options, alwaysDry bool) {
	m, err := loadManifest(eng.Manifest)
	check(err)

	var names, homes []string
	if opt.all {
		for n := range m.Projects {
			names = append(names, n)
		}
		sort.Strings(names)
		for n := range m.Homes {
			homes = append(homes, n)
		}
		sort.Strings(homes)
	} else if opt.positional != "" {
		if _, ok := m.Homes[opt.positional]; ok {
			homes = []string{opt.positional}
		} else {
			names = []string{opt.positional}
		}
	} else {
		fatal("install: name a project or home, or pass --all")
	}

	type projectResult struct {
		Project string       `json:"project"`
		Files   []fileStatus `json:"files"`
	}
	var all []projectResult

	// homes take only --force/--dry-run: --target/--what are per-project knobs.
	globalDir := eng.GlobalDir()
	for _, n := range homes {
		res, err := installHome(m.Homes[n], globalDir, opt.force, opt.dryRun || alwaysDry)
		check(err)
		all = append(all, projectResult{"home:" + n, res})
	}

	for _, n := range names {
		p, ok := m.Projects[n]
		if !ok {
			fatal("no such project or home: " + n)
		}
		active := subtract(p.Bundles, p.Disabled)
		prune := true
		if len(opt.what) > 0 {
			active = intersect(active, opt.what)
			prune = false // partial install must not prune other bundles' artifacts
		}
		targets := p.Targets
		if opt.target != "" {
			targets = targetList(opt.target)
		}
		res, err := install(p, eng, active, targets, opt.force, opt.dryRun || alwaysDry, prune, opt.target != "")
		check(err)
		all = append(all, projectResult{n, res})
	}

	if opt.json {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(all)
		return
	}
	for _, pr := range all {
		var ins, upd, skip, rm int
		for _, f := range pr.Files {
			switch f.Status {
			case "install":
				ins++
			case "update":
				upd++
			case "skip":
				skip++
			case "remove":
				rm++
			}
		}
		verb := "installed"
		if opt.dryRun || alwaysDry {
			verb = "would change"
		}
		fmt.Printf("%s: %d install, %d update, %d skip, %d remove\n", pr.Project, ins, upd, skip, rm)
		for _, f := range pr.Files {
			if f.Status == "skip" && !opt.verbose {
				continue
			}
			fmt.Printf("  %-7s %s\n", f.Status, rel(f.Path))
		}
		_ = verb
	}
}

func cmdList(eng *Engine, opt options) {
	m, err := loadManifest(eng.Manifest)
	check(err)
	if opt.json {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(m.Projects)
		return
	}
	var names []string
	for n := range m.Projects {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Printf("%-16s %-10s %-8s %s\n", "PROJECT", "TARGETS", "TICKETS", "BUNDLES")
	for _, n := range names {
		p := m.Projects[n]
		fmt.Printf("%-16s %-10s %-8s %s\n", n, strings.Join(p.Targets, "+"), p.Tickets.Config, strings.ReplaceAll(bundlesDisplay(p), ", ", ","))
	}
	if len(m.Homes) == 0 {
		return
	}
	var homes []string
	for n := range m.Homes {
		homes = append(homes, n)
	}
	sort.Strings(homes)
	fmt.Printf("\n%-16s %-30s %s\n", "HOME", "SKILLS DIR", "SKILLS")
	for _, n := range homes {
		h := m.Homes[n]
		skills := "all"
		if len(h.Skills) > 0 {
			skills = strings.Join(h.Skills, ",")
		}
		if len(h.Skip) > 0 {
			skills += " (skip " + strings.Join(h.Skip, ",") + ")"
		}
		fmt.Printf("%-16s %-38s %s\n", n, h.SkillsDir, skills)
	}
}

func cmdValidate(eng *Engine) {
	m, err := loadManifest(eng.Manifest)
	check(err)
	problems := 0
	report := func(proj, msg string) {
		problems++
		fmt.Printf("  %-16s %s\n", proj, msg)
	}
	var names []string
	for n := range m.Projects {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := m.Projects[n]
		// required fields
		if p.RepoRoot == "" {
			report(n, "missing repoRoot")
		}
		for _, dn := range p.Disabled {
			if _, ok := eng.Bundles[dn]; !ok {
				report(n, fmt.Sprintf("disabled lists unknown bundle %q", dn))
			}
			if !contains(p.Bundles, dn) {
				report(n, fmt.Sprintf("disabled bundle %q is not in bundles", dn))
			}
		}
		needs := func(block string) []string {
			var by []string
			for _, bn := range p.Bundles {
				if eng.Bundles[bn].requires(block) {
					by = append(by, bn)
				}
			}
			return by
		}
		for _, bn := range p.Bundles {
			if _, ok := eng.Bundles[bn]; !ok {
				report(n, fmt.Sprintf("bundle %q is not in templates/%s", bn, bundlesFile))
			}
		}
		if by := needs("db"); len(by) > 0 && (p.Tools.DB.CLI == "" || p.Tools.DB.Config == "" || p.Tools.DB.Tunnel == "") {
			report(n, fmt.Sprintf("bundle %s needs tools.db.cli, tools.db.config and tools.db.tunnel", strings.Join(by, ", ")))
		}
		if _, err := sessionContext(eng.Bundles, subtract(p.Bundles, p.Disabled)); err != nil {
			report(n, err.Error())
		}
		// only bundles whose templates read the ticket protocol need it, so a repo
		// without a ticket board can still carry e.g. a read-only database skill.
		if len(needs("tickets")) > 0 {
			if p.Tickets.Config == "" {
				report(n, "missing tickets.config")
			}
			if p.Tickets.Space == "" {
				report(n, "missing tickets.space")
			}
			if (p.Tickets.DB == "") != (p.Tickets.Project == "") {
				report(n, "tickets.db and tickets.project go together (Work mode)")
			}
			if p.Tickets.DB != "" && !strings.HasPrefix(p.Tickets.DB, p.Tickets.Space+"/") {
				report(n, fmt.Sprintf("tickets.db %q must be in tickets.space %q", p.Tickets.DB, p.Tickets.Space))
			}
			if _, ok := runProfiles[p.Run.Profile]; !ok {
				report(n, fmt.Sprintf("run.profile %q is not one of careful|balanced|fast", p.Run.Profile))
			}
			if p.Run.Order != "priority" && p.Run.Order != "cheap-first" {
				report(n, fmt.Sprintf("run.order %q is not one of priority|cheap-first", p.Run.Order))
			}
			if p.Run.MaxRounds < 1 || p.Run.MaxRounds > 5 {
				report(n, fmt.Sprintf("run.maxRounds %d is outside 1..5", p.Run.MaxRounds))
			}
		} else if len(p.Bundles) == 0 {
			if _, err := os.Stat(filepath.Join(eng.ProjectsDir(), n)); err != nil {
				report(n, "no bundles and no projects/"+n+" sources — nothing to install")
			}
		}
		// render every active template to catch token/parse errors
		if _, err := render(p, eng, p.Bundles, p.Targets); err != nil {
			report(n, "render error: "+err.Error())
		}
	}
	var homeNames []string
	for n := range m.Homes {
		homeNames = append(homeNames, n)
	}
	sort.Strings(homeNames)
	globalDir := eng.GlobalDir()
	for _, n := range homeNames {
		if _, ok := m.Projects[n]; ok {
			report("home:"+n, "name collides with a project — `trafaret install "+n+"` would be ambiguous")
		}
		if _, err := renderHome(m.Homes[n], globalDir); err != nil {
			report("home:"+n, "render error: "+err.Error())
		}
	}
	if problems == 0 {
		fmt.Printf("ok: %d project(s), %d home(s) valid\n", len(names), len(homeNames))
		return
	}
	fmt.Printf("%d problem(s)\n", problems)
	os.Exit(1)
}

// --- tiny flag parser -----------------------------------------------------

type options struct {
	positional string
	all        bool
	force      bool
	dryRun     bool
	json       bool
	verbose    bool
	target     string   // claude | cursor | codex | both | all
	what       []string // bundle filter
	config     string   // config directory (holds manifest.yaml)
	manifest   string   // explicit manifest path; its directory becomes the config dir
	templates  string   // templates overlay directory
	addr       string   // serve: listen address
}

func parseFlags(args []string) options {
	var o options
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--all":
			o.all = true
		case a == "--force":
			o.force = true
		case a == "--dry-run":
			o.dryRun = true
		case a == "--json":
			o.json = true
		case a == "--verbose", a == "-v":
			o.verbose = true
		case a == "--target":
			i++
			o.target = arg(args, i)
		case strings.HasPrefix(a, "--target="):
			o.target = strings.TrimPrefix(a, "--target=")
		case a == "--what":
			i++
			o.what = splitCSV(arg(args, i))
		case strings.HasPrefix(a, "--what="):
			o.what = splitCSV(strings.TrimPrefix(a, "--what="))
		case a == "--config":
			i++
			o.config = arg(args, i)
		case strings.HasPrefix(a, "--config="):
			o.config = strings.TrimPrefix(a, "--config=")
		case a == "--manifest":
			i++
			o.manifest = arg(args, i)
		case strings.HasPrefix(a, "--manifest="):
			o.manifest = strings.TrimPrefix(a, "--manifest=")
		case a == "--templates":
			i++
			o.templates = arg(args, i)
		case strings.HasPrefix(a, "--templates="):
			o.templates = strings.TrimPrefix(a, "--templates=")
		case a == "--addr":
			i++
			o.addr = arg(args, i)
		case strings.HasPrefix(a, "--addr="):
			o.addr = strings.TrimPrefix(a, "--addr=")
		case strings.HasPrefix(a, "-"):
			fatal("unknown flag: " + a)
		default:
			o.positional = a
		}
	}
	return o
}

// --- helpers --------------------------------------------------------------

func arg(args []string, i int) string {
	if i >= len(args) {
		fatal("missing value for flag")
	}
	return args[i]
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func targetList(t string) []string {
	switch t {
	case "both":
		return []string{"claude", "cursor"}
	case "all":
		return []string{"claude", "cursor", "codex"}
	case "claude":
		return []string{"claude"}
	case "cursor":
		return []string{"cursor"}
	case "codex":
		return []string{"codex"}
	default:
		fatal("invalid --target (claude|cursor|codex|both|all): " + t)
		return nil
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// bundlesDisplay renders a project's bundle list with disabled ones marked.
func bundlesDisplay(p Project) string {
	disabled := map[string]bool{}
	for _, d := range p.Disabled {
		disabled[d] = true
	}
	var out []string
	for _, b := range p.Bundles {
		if disabled[b] {
			b += "(off)"
		}
		out = append(out, b)
	}
	return strings.Join(out, ", ")
}

// subtract returns a minus the members of b, preserving order.
func subtract(a, b []string) []string {
	drop := map[string]bool{}
	for _, x := range b {
		drop[x] = true
	}
	var out []string
	for _, x := range a {
		if !drop[x] {
			out = append(out, x)
		}
	}
	return out
}

func intersect(a, b []string) []string {
	var out []string
	for _, x := range a {
		if contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

func rel(p string) string {
	if wd, err := os.Getwd(); err == nil {
		if r, err := filepath.Rel(wd, p); err == nil && !strings.HasPrefix(r, "..") {
			return r
		}
	}
	return p
}

func check(err error) {
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "trafaret: "+msg)
	os.Exit(1)
}

func usage() {
	fmt.Print(`trafaret — render a shared agent/skill/rule library into projects

Usage:
  trafaret install <project> [flags]   render + write Claude, Cursor, and Codex artifacts
                                       (also prunes artifacts of bundles moved to disabled: — tracked
                                       via .claude/ao-manifest.json in the target repo)
  trafaret install --all [flags]       install every project and every home in the manifest
  trafaret install <home>              copy global/skills/ + agents/*/skills/ into one home's skills dir
                                       (homes: in manifest.yaml — e.g. ~/.claude, ~/.codex, ~/.cursor)
  trafaret diff <project|home>         dry-run: print per-file status (install/update/skip/remove)
  trafaret list [--json]               list projects in the manifest
  trafaret validate                    check the manifest + render every template
  trafaret root                        print the config directory (the one holding manifest.yaml)
  trafaret serve [--addr host:port]    local live manual: docs site rendered from the
                                       config directory on every refresh (default 127.0.0.1:4477)
  trafaret version                     print the binary version

Flags:
  --config DIR                   config directory: holds manifest.yaml, global/, agents/, projects/
                                 (default: $TRAFARET_CONFIG, else the nearest parent of the working
                                 directory that has a manifest.yaml)
  --manifest PATH                manifest file; its directory becomes the config directory
  --templates PATH               templates overlay directory (default: <config>/templates, optional).
                                 A file there replaces the built-in template with the same path
  --target claude|cursor|codex|both|all
                                 override the manifest targets
  --what bundle[,bundle]         install only these bundles (subset of the project's; skips pruning)
  --force                        rewrite even unchanged (sha256-equal) files
  --dry-run                      render but do not write
  --verbose                      include skipped files in output
  --json                         machine-readable output
`)
}
