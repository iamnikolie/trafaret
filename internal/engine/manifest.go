package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// --- load + resolve -------------------------------------------------------

// loadManifest reads and resolves the manifest at path. version is the binary's
// version, checked against the manifest's optional `engine:` constraint.
func loadManifest(path, version string) (*Manifest, error) {
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
