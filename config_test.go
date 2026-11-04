package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// --- config directory resolution -------------------------------------------

// writeManifest creates dir/manifest.yaml and returns dir.
func writeManifest(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), []byte("projects: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func evalDir(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResolveConfigFlagWinsOverEnvAndCwd(t *testing.T) {
	root := evalDir(t, t.TempDir())
	flagDir := writeManifest(t, filepath.Join(root, "flag"))
	envDir := writeManifest(t, filepath.Join(root, "env"))
	cwdDir := writeManifest(t, filepath.Join(root, "cwd"))

	dir, manifest, err := resolveConfig(flagDir, "", envDir, cwdDir)
	if err != nil {
		t.Fatal(err)
	}
	if dir != flagDir || manifest != filepath.Join(flagDir, manifestName) {
		t.Fatalf("--config must win: got %s, %s", dir, manifest)
	}
}

func TestResolveConfigEnvWinsOverCwd(t *testing.T) {
	root := evalDir(t, t.TempDir())
	envDir := writeManifest(t, filepath.Join(root, "env"))
	cwdDir := writeManifest(t, filepath.Join(root, "cwd"))

	dir, _, err := resolveConfig("", "", envDir, cwdDir)
	if err != nil {
		t.Fatal(err)
	}
	if dir != envDir {
		t.Fatalf("$TRAFARET_CONFIG must win over cwd: got %s want %s", dir, envDir)
	}
}

func TestResolveConfigWalksUpFromCwd(t *testing.T) {
	root := evalDir(t, t.TempDir())
	cfg := writeManifest(t, filepath.Join(root, "cfg"))
	deep := filepath.Join(cfg, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, manifest, err := resolveConfig("", "", "", deep)
	if err != nil {
		t.Fatal(err)
	}
	if dir != cfg || manifest != filepath.Join(cfg, manifestName) {
		t.Fatalf("walk-up: got %s, %s", dir, manifest)
	}
}

func TestResolveConfigNearestAncestorWins(t *testing.T) {
	root := evalDir(t, t.TempDir())
	outer := writeManifest(t, filepath.Join(root, "outer"))
	inner := writeManifest(t, filepath.Join(outer, "inner"))
	dir, _, err := resolveConfig("", "", "", filepath.Join(inner, "x"))
	if err != nil {
		t.Fatal(err)
	}
	if dir != inner {
		t.Fatalf("nearest ancestor must win: got %s want %s", dir, inner)
	}
}

func TestResolveConfigNotFound(t *testing.T) {
	// A fresh temp dir has no manifest.yaml on its way up to the filesystem root
	// unless the host has one at "/" or the temp parent — skip then.
	root := evalDir(t, t.TempDir())
	for d := root; ; d = filepath.Dir(d) {
		if isFile(filepath.Join(d, manifestName)) {
			t.Skipf("host has a %s at %s", manifestName, d)
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	_, _, err := resolveConfig("", "", "", root)
	if err == nil || !strings.Contains(err.Error(), "no "+manifestName+" found") || !strings.Contains(err.Error(), "--config") {
		t.Fatalf("want a clear not-found error naming --config, got %v", err)
	}
}

func TestResolveConfigFlagWithoutManifestIsAnError(t *testing.T) {
	empty := evalDir(t, t.TempDir())
	_, _, err := resolveConfig(empty, "", "", empty)
	if err == nil || !strings.Contains(err.Error(), "--config") || !strings.Contains(err.Error(), manifestName) {
		t.Fatalf("want error about missing manifest under --config, got %v", err)
	}
	_, _, err = resolveConfig("", "", empty, empty)
	if err == nil || !strings.Contains(err.Error(), envConfig) {
		t.Fatalf("want error naming $%s, got %v", envConfig, err)
	}
	_, _, err = resolveConfig(filepath.Join(empty, "nope"), "", "", empty)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("want not-a-directory error, got %v", err)
	}
}

func TestResolveConfigManifestFlagSetsConfigDirToItsDir(t *testing.T) {
	root := evalDir(t, t.TempDir())
	dir := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := filepath.Join(dir, "custom-name.yaml")
	if err := os.WriteFile(m, []byte("projects: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := writeManifest(t, filepath.Join(root, "other"))

	gotDir, gotManifest, err := resolveConfig(other, m, other, other)
	if err != nil {
		t.Fatal(err)
	}
	if gotDir != dir || gotManifest != m {
		t.Fatalf("--manifest: config dir must be the manifest's dir; got %s, %s", gotDir, gotManifest)
	}
	if _, _, err := resolveConfig("", filepath.Join(dir, "missing.yaml"), "", root); err == nil {
		t.Fatal("a missing --manifest file must be an error")
	}
}

func TestResolveConfigRelativeAndHomePaths(t *testing.T) {
	root := evalDir(t, t.TempDir())
	cfg := writeManifest(t, filepath.Join(root, "rel"))
	t.Chdir(root)
	dir, _, err := resolveConfig("rel", "", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if evalDir(t, dir) != cfg {
		t.Fatalf("relative --config: got %s want %s", dir, cfg)
	}

	t.Setenv("HOME", root)
	dir, _, err = resolveConfig("~/rel", "", "", root)
	if err != nil {
		t.Fatal(err)
	}
	if evalDir(t, dir) != cfg {
		t.Fatalf("~ in --config: got %s want %s", dir, cfg)
	}
}

func TestNewEngineUsesFlagsEnvAndWalkUp(t *testing.T) {
	root := evalDir(t, t.TempDir())
	cfg := writeManifest(t, filepath.Join(root, "cfg"))
	t.Chdir(root)

	e, err := newEngine(options{config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if e.ConfigDir != cfg || e.GlobalDir() != filepath.Join(cfg, "global") || e.ProjectsDir() != filepath.Join(cfg, "projects") {
		t.Fatalf("engine paths wrong: %+v", e)
	}
	if e.OverlayDir != "" {
		t.Fatalf("no <config>/templates exists, overlay must be empty, got %q", e.OverlayDir)
	}

	t.Setenv(envConfig, cfg)
	e, err = newEngine(options{})
	if err != nil || e.ConfigDir != cfg {
		t.Fatalf("env: %+v %v", e, err)
	}

	t.Setenv(envConfig, "")
	t.Chdir(filepath.Join(cfg))
	e, err = newEngine(options{})
	if err != nil || evalDir(t, e.ConfigDir) != cfg {
		t.Fatalf("walk-up: %+v %v", e, err)
	}
}

// --- templates overlay -----------------------------------------------------

func TestLayeredFSIsAValidFS(t *testing.T) {
	base := fstest.MapFS{
		"skills/alpha/SKILL.md.tmpl": {Data: []byte("base alpha")},
		"skills/alpha/extra.md":      {Data: []byte("base extra")},
		"skills/beta/SKILL.md.tmpl":  {Data: []byte("base beta")},
		"agents/x.md.tmpl":           {Data: []byte("base x")},
	}
	top := fstest.MapFS{
		"skills/alpha/SKILL.md.tmpl": {Data: []byte("top alpha")},
		"skills/gamma/SKILL.md.tmpl": {Data: []byte("top gamma")},
		"cursor/rules/r.mdc.tmpl":    {Data: []byte("top rule")},
	}
	l := layerFS(top, base)

	if err := fstest.TestFS(l,
		"skills/alpha/SKILL.md.tmpl", "skills/alpha/extra.md", "skills/beta/SKILL.md.tmpl",
		"skills/gamma/SKILL.md.tmpl", "agents/x.md.tmpl", "cursor/rules/r.mdc.tmpl"); err != nil {
		t.Fatal(err)
	}

	read := func(name string) string {
		b, err := fs.ReadFile(l, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(b)
	}
	if got := read("skills/alpha/SKILL.md.tmpl"); got != "top alpha" {
		t.Errorf("overlay file must win, got %q", got)
	}
	if got := read("skills/alpha/extra.md"); got != "base extra" {
		t.Errorf("untouched base sibling must remain, got %q", got)
	}
	if got := read("skills/gamma/SKILL.md.tmpl"); got != "top gamma" {
		t.Errorf("overlay-only file must be added, got %q", got)
	}
	if got := read("skills/beta/SKILL.md.tmpl"); got != "base beta" {
		t.Errorf("base-only file must remain, got %q", got)
	}

	entries, err := fs.ReadDir(l, "skills")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "alpha,beta,gamma" {
		t.Errorf("merged directory listing = %v", names)
	}
}

func TestOverlayReplacesAndAddsInRender(t *testing.T) {
	cfg := writeManifest(t, filepath.Join(evalDir(t, t.TempDir()), "cfg"))
	overlay := filepath.Join(cfg, "templates")
	write := func(rel, body string) {
		path := filepath.Join(overlay, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// replaces a built-in skill file, adds a sibling file next to it
	write("skills/example-tickets/SKILL.md.tmpl", "---\nname: example-tickets\ndescription: overlaid for {{.Name}}\n---\nOVERLAY BODY {{.Tickets.Space}}\n")
	write("skills/example-tickets/notes.md.tmpl", "notes for {{.Name}}\n")

	e, err := newEngine(options{config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if e.OverlayDir != overlay {
		t.Fatalf("overlay not discovered: %q", e.OverlayDir)
	}

	p := testProject(t)
	files, err := render(p, e, []string{"example"}, []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range files {
		if strings.Contains(filepath.ToSlash(f.path), "/skills/example-tickets/") {
			got[filepath.Base(f.path)] = string(f.bytes)
		}
	}
	if !strings.Contains(got["SKILL.md"], "OVERLAY BODY Acme") || !strings.Contains(got["SKILL.md"], "overlaid for demo") {
		t.Errorf("overlay SKILL.md did not win:\n%s", got["SKILL.md"])
	}
	if got["notes.md"] != "notes for demo\n" {
		t.Errorf("overlay-only file not rendered: %q", got["notes.md"])
	}

	// the built-in set alone renders the original
	files, err = render(p, testEngine(t), []string{"example"}, []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if filepath.Base(f.path) == "SKILL.md" && strings.Contains(string(f.bytes), "OVERLAY BODY") {
			t.Fatal("embedded render must not see the overlay")
		}
	}
}

func TestTemplatesFlagOverridesOverlayDir(t *testing.T) {
	cfg := writeManifest(t, filepath.Join(evalDir(t, t.TempDir()), "cfg"))
	if err := os.MkdirAll(filepath.Join(cfg, "templates", "skills", "example-db"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "templates", "skills", "example-db", "SKILL.md.tmpl"), []byte("FROM CONFIG OVERLAY"), 0o644); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(filepath.Join(custom, "skills", "example-db"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(custom, "skills", "example-db", "SKILL.md.tmpl"), []byte("FROM FLAG OVERLAY"), 0o644); err != nil {
		t.Fatal(err)
	}

	e, err := newEngine(options{config: cfg, templates: custom})
	if err != nil {
		t.Fatal(err)
	}
	b, err := fs.ReadFile(e.Templates, "skills/example-db/SKILL.md.tmpl")
	if err != nil || string(b) != "FROM FLAG OVERLAY" {
		t.Fatalf("--templates must replace <config>/templates: %q %v", b, err)
	}

	// an empty --templates directory disables the config's overlay entirely
	empty := t.TempDir()
	e, err = newEngine(options{config: cfg, templates: empty})
	if err != nil {
		t.Fatal(err)
	}
	b, err = fs.ReadFile(e.Templates, "skills/example-db/SKILL.md.tmpl")
	if err != nil || strings.Contains(string(b), "OVERLAY") {
		t.Fatalf("empty --templates must leave only the embedded file: %q %v", b, err)
	}

	if _, err := newEngine(options{config: cfg, templates: filepath.Join(empty, "missing")}); err == nil || !strings.Contains(err.Error(), "--templates") {
		t.Fatalf("a missing --templates dir must be an error, got %v", err)
	}
}

func TestEmbeddedTemplatesAreComplete(t *testing.T) {
	tmpl := embeddedTemplates()
	reg, err := loadBundles(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg) == 0 {
		t.Fatal("embedded bundles.yaml lists no bundles")
	}
	for _, b := range reg {
		for _, s := range b.Skills {
			if _, err := fs.Stat(tmpl, "skills/"+s+"/SKILL.md.tmpl"); err != nil {
				t.Errorf("bundle skill %q missing from the embedded templates: %v", s, err)
			}
		}
		for _, a := range b.Agents {
			if _, err := fs.Stat(tmpl, "agents/"+a+".md.tmpl"); err != nil {
				t.Errorf("bundle agent %q missing from the embedded templates: %v", a, err)
			}
		}
		for _, r := range b.CursorRules {
			if _, err := fs.Stat(tmpl, "cursor/rules/"+r+".mdc.tmpl"); err != nil {
				t.Errorf("bundle rule %q missing from the embedded templates: %v", r, err)
			}
		}
	}
}

func TestEveryBundleRendersFromEmbeddedTemplates(t *testing.T) {
	p := testProject(t)
	eng := testEngine(t)
	var all []string
	for name := range eng.Bundles {
		all = append(all, name)
	}
	files, err := render(p, eng, all, []string{"claude", "cursor", "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("nothing rendered")
	}
}

// --- engine version gate ---------------------------------------------------

func TestCheckEngine(t *testing.T) {
	cases := []struct {
		constraint, ver string
		ok              bool
	}{
		{"", "0.1.0", true},         // absent field: no check
		{"  ", "0.1.0", true},       // blank: no check
		{">=0.2.0", "dev", true},    // dev always passes
		{">=99.0.0", "dev", true},   // ... whatever the constraint
		{">=0.2.0", "0.2.0", true},  // boundary
		{">=0.2.0", "0.1.9", false}, // too old
		{">=0.2.0", "v0.3.1", true}, // v prefix on the binary
		{">=v0.2.0", "0.3.1", true}, // v prefix on the constraint
		{">=0.2", "0.2.0", true},    // X.Y means X.Y.0
		{">0.2.0", "0.2.0", false},  // strict
		{">0.2.0", "0.2.1", true},
		{"<1.0.0", "0.9.9", true},
		{"<1.0.0", "1.0.0", false},
		{"<=1.0.0", "1.0.0", true},
		{"0.2.1", "0.2.1", true}, // exact
		{"0.2.1", "0.2.2", false},
		{"=0.2.1", "0.2.1", true},
		{"==0.2.1", "0.2.0", false},
		{">=0.2.0, <1.0.0", "0.5.0", true}, // range
		{">=0.2.0, <1.0.0", "1.2.0", false},
		{">= 0.2.0", "0.2.0", true},    // space after the operator
		{">=0.2.0", "0.3.0-rc1", true}, // pre-release suffix ignored
		{">=0.2.0", "0.3.0-4-gabc123-dirty", true},
		{">=0.2.0", "abc1234", true}, // bare commit hash: not a version, treated as dev
		{">=1.0.0", "0.9.0+meta", false},
	}
	for _, c := range cases {
		err := checkEngine(c.constraint, c.ver)
		if c.ok && err != nil {
			t.Errorf("checkEngine(%q, %q) = %v, want pass", c.constraint, c.ver, err)
		}
		if !c.ok && err == nil {
			t.Errorf("checkEngine(%q, %q) passed, want an error", c.constraint, c.ver)
		}
	}
}

func TestCheckEngineErrorIsClear(t *testing.T) {
	err := checkEngine(">=2.0.0", "1.4.0")
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{">=2.0.0", "1.4.0", "trafaret", manifestName} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestCheckEngineRejectsMalformedConstraint(t *testing.T) {
	for _, c := range []string{"banana", ">=", "^1.2.3", ">=1.x", "1.2.3.4", ">=1,"} {
		if err := checkEngine(c, "1.0.0"); err == nil {
			t.Errorf("malformed constraint %q must be an error", c)
		}
		// ... even for a dev build: a typo must not silently disable the gate
		if err := checkEngine(c, "dev"); err == nil {
			t.Errorf("malformed constraint %q must be an error on dev builds too", c)
		}
	}
}

func TestLoadManifestEnforcesEngine(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "m.yaml")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	old := version
	defer func() { version = old }()

	version = "0.1.0"
	if _, err := loadManifest(write("engine: \">=0.2.0\"\nprojects: {}\n")); err == nil || !strings.Contains(err.Error(), ">=0.2.0") {
		t.Fatalf("a too-old binary must be refused, got %v", err)
	}
	version = "0.2.0"
	if _, err := loadManifest(write("engine: \">=0.2.0\"\nprojects: {}\n")); err != nil {
		t.Fatalf("a satisfying binary must load: %v", err)
	}
	version = "0.1.0"
	if _, err := loadManifest(write("projects: {}\n")); err != nil {
		t.Fatalf("no engine field: no check: %v", err)
	}
	if _, err := loadManifest(write("someFutureField: 1\nprojects: {}\n")); err != nil {
		t.Fatalf("unknown fields must be ignored: %v", err)
	}
	version = "dev"
	if _, err := loadManifest(write("engine: \">=99.0.0\"\nprojects: {}\n")); err != nil {
		t.Fatalf("dev builds always pass: %v", err)
	}
	// an unquoted YAML float ("engine: 0.2") is still a usable constraint
	version = "0.3.0"
	if _, err := loadManifest(write("engine: 0.2\nprojects: {}\n")); err == nil {
		t.Fatal("engine: 0.2 is an exact-version constraint; 0.3.0 does not satisfy it")
	}
}

// --- command line ----------------------------------------------------------

func TestSplitCommandAcceptsFlagsBeforeAndAfter(t *testing.T) {
	cases := []struct {
		in       []string
		cmd      string
		restJoin string
	}{
		{[]string{"diff", "x"}, "diff", "x"},
		{[]string{"--config", "/c", "diff", "x"}, "diff", "--config /c x"},
		{[]string{"diff", "x", "--config", "/c"}, "diff", "x --config /c"},
		{[]string{"--config=/c", "install", "--all", "--target", "claude"}, "install", "--config=/c --all --target claude"},
		{[]string{"--templates", "/t", "--manifest", "/m", "list"}, "list", "--templates /t --manifest /m"},
		{[]string{"--version"}, "--version", ""},
		{[]string{"-h"}, "-h", ""},
		{[]string{}, "", ""},
	}
	for _, c := range cases {
		cmd, rest := splitCommand(c.in)
		if cmd != c.cmd || strings.Join(rest, " ") != c.restJoin {
			t.Errorf("splitCommand(%v) = %q, %v; want %q, %q", c.in, cmd, rest, c.cmd, c.restJoin)
		}
	}
	o := parseFlags([]string{"--config", "/c", "myproj", "--dry-run"})
	if o.config != "/c" || o.positional != "myproj" || !o.dryRun {
		t.Errorf("parseFlags: %+v", o)
	}
}

// --- tokens that replaced private prose ------------------------------------

func TestMemoryNameAndExtraFieldsTokens(t *testing.T) {
	renderOne := func(p Project, skill, file string) string {
		t.Helper()
		files, err := render(p, testEngine(t), []string{"example"}, []string{"claude"})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(filepath.ToSlash(f.path), skill+"/"+file) {
				return string(f.bytes)
			}
		}
		t.Fatalf("%s/%s not rendered", skill, file)
		return ""
	}

	// default: neutral wording, nothing deployment-specific
	p := testProject(t)
	impl := renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(impl, "`mem` memory CLI") || !strings.Contains(impl, "learnings back to memory.") {
		t.Errorf("default memory wording not rendered:\n%s", grepLines(impl, "memory"))
	}

	// configured through defaults, inherited by the project
	p = testProject(t)
	p.Tools.MemoryName = ""
	p.Tickets.DB, p.Tickets.Project = "Acme/Work", "Widgets"
	p.Tickets.ExtraFields = ""
	resolve(&p, Defaults{
		Tickets: struct {
			ExtraFields string `yaml:"extraFields"`
		}{ExtraFields: "`Acme/Seat` (who holds it)"},
		Tools: struct {
			MemoryName string `yaml:"memoryName"`
		}{MemoryName: "team memory"},
	})
	impl = renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(impl, "`mem` team-memory CLI") || !strings.Contains(impl, "learnings back to team memory.") {
		t.Errorf("memoryName default not applied:\n%s", grepLines(impl, "memory"))
	}
	wt := renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(wt, "(merge request URL), `Acme/Seat` (who holds it).\n") {
		t.Errorf("extraFields default not applied:\n%s", grepLines(wt, "merge request"))
	}

	// a project's own value wins over the default
	p.Tools.MemoryName = "own memory"
	p.Tickets.ExtraFields = "`Acme/Own` (own field)"
	resolve(&p, Defaults{})
	wt = renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(wt, "(merge request URL), `Acme/Own` (own field).\n") {
		t.Errorf("project extraFields must win:\n%s", grepLines(wt, "merge request"))
	}

	// no extra fields at all: the sentence still closes cleanly
	p.Tickets.ExtraFields = ""
	resolve(&p, Defaults{})
	wt = renderOne(p, "example-tickets", "SKILL.md")
	if !strings.Contains(wt, "(merge request URL).\n") {
		t.Errorf("empty extraFields must collapse:\n%s", grepLines(wt, "merge request"))
	}
}

func grepLines(s, sub string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// --- nothing private ships in the engine -----------------------------------

// The repository is public: no deployment-specific name, host, path or address
// may appear in the sources, templates or docs. Values a template needs come from
// manifest tokens. (Patterns are assembled from parts so this file does not trip
// its own check.)
func TestNoPrivateStringsInSources(t *testing.T) {
	forbidden := []string{
		"lang" + "gerone", "ma" + "ko", "/Users" + "/", "imn" + "dev", "mb" + "dev",
		"fibery" + ".io", "@" + "gmail", "iamnikolie" + ".info", "imn-" + "ao", "lg-" + "ao",
	}
	allowed := map[string][]string{
		// the module path and README install line name the publisher
		"go.mod":    {"iam" + "nikolie"},
		"README.md": {"iam" + "nikolie"},
	}
	self := "config_test.go"
	var scan func(dir string)
	scan = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			switch {
			case e.IsDir():
				if e.Name() == ".git" || e.Name() == "site-assets" {
					continue
				}
				scan(path)
			// .goreleaser.yaml publishes to the owner's Homebrew tap and names the
			// formula commit author by design; it is release plumbing, not engine.
			case e.Name() == self, e.Name() == "trafaret", e.Name() == ".goreleaser.yaml", strings.HasSuffix(e.Name(), ".sum"):
			default:
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				low := strings.ToLower(string(b))
				for _, f := range forbidden {
					if strings.Contains(low, f) {
						t.Errorf("%s contains private string %q", path, f)
					}
				}
				if strings.Contains(low, "iam"+"nikolie") && len(allowed[e.Name()]) == 0 {
					t.Errorf("%s names the publisher; only the module path and README may", path)
				}
			}
		}
	}
	scan(".")
}
