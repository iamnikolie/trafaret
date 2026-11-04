package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// cli runs one command line through the real command tree and returns the exit
// code with what went to each stream. Flag values are reset first: the command
// tree is package state, and pflag keeps values between runs.
func cli(t *testing.T, args ...string) (code int, out, errOut string) {
	t.Helper()
	resetFlags(rootCmd)
	eng = nil

	var o, e bytes.Buffer
	oldOut, oldErr := stdout, stderr
	stdout, stderr = &o, &e
	t.Cleanup(func() { stdout, stderr = oldOut, oldErr })

	code = run(args)
	return code, o.String(), e.String()
}

func resetFlags(c *cobra.Command) {
	reset := func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	}
	c.Flags().VisitAll(reset)
	c.PersistentFlags().VisitAll(reset)
	for _, sub := range c.Commands() {
		resetFlags(sub)
	}
}

// writeConfig creates a config directory with a manifest holding one project that
// installs the embedded read-only database example into its own repo directory.
func writeConfig(t *testing.T) (cfg, repo string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg, repo = filepath.Join(root, "cfg"), filepath.Join(root, "repo")
	for _, d := range []string{cfg, repo} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `defaults:
  targets: [claude]
projects:
  demo:
    repoRoot: ` + repo + `
    bundles: [example-db]
    tools:
      db: { cli: dbq, config: acme, env: dev, schema: demo, tunnel: "true" }
`
	if err := os.WriteFile(filepath.Join(cfg, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg, repo
}

// chdirEmpty moves into a directory with no manifest anywhere above it and
// clears $TRAFARET_CONFIG, so only flags can name a config.
func chdirEmpty(t *testing.T) {
	t.Helper()
	t.Setenv("TRAFARET_CONFIG", "")
	t.Chdir(t.TempDir())
}

// Flags are persistent on the root, so they may come before or after the
// command: `trafaret --config DIR diff x` and `trafaret diff x --config DIR`
// are the same invocation.
func TestFlagsAcceptedBeforeAndAfterCommand(t *testing.T) {
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
		{[]string{"--version"}, "trafaret", "--version"},
		{[]string{"-h"}, "trafaret", "-h"},
		{[]string{}, "trafaret", ""},
	}
	for _, c := range cases {
		cmd, rest, err := rootCmd.Find(c.in)
		if err != nil {
			t.Errorf("Find(%v): %v", c.in, err)
			continue
		}
		if cmd.Name() != c.cmd || strings.Join(rest, " ") != c.restJoin {
			t.Errorf("Find(%v) = %q, %v; want %q, %q", c.in, cmd.Name(), rest, c.cmd, c.restJoin)
		}
	}

	resetFlags(rootCmd)
	cmd, rest, err := rootCmd.Find([]string{"--config", "/c", "myproj", "--dry-run"})
	if err != nil || cmd != rootCmd {
		t.Fatalf("Find: %v %v", cmd.Name(), err)
	}
	// no command named myproj: the root keeps every argument for its usage screen
	if err := cmd.ParseFlags(rest); err != nil {
		t.Fatal(err)
	}
	if configFlag != "/c" || lastArg(cmd.Flags().Args()) != "myproj" || !dryRunFlag {
		t.Errorf("flags: config=%q args=%v dryRun=%v", configFlag, cmd.Flags().Args(), dryRunFlag)
	}
	resetFlags(rootCmd)
}

func TestFlagsBeforeAndAfterGiveTheSameOutput(t *testing.T) {
	cfg, _ := writeConfig(t)
	chdirEmpty(t)

	for _, tc := range []struct{ before, after []string }{
		{[]string{"--config", cfg, "validate"}, []string{"validate", "--config", cfg}},
		{[]string{"--config=" + cfg, "list"}, []string{"list", "--config=" + cfg}},
		{[]string{"--config", cfg, "root"}, []string{"root", "--config", cfg}},
		{[]string{"--config", cfg, "diff", "demo"}, []string{"diff", "demo", "--config", cfg}},
		{[]string{"--manifest", filepath.Join(cfg, "manifest.yaml"), "list", "--json"}, []string{"list", "--json", "--manifest", filepath.Join(cfg, "manifest.yaml")}},
	} {
		c1, o1, e1 := cli(t, tc.before...)
		c2, o2, e2 := cli(t, tc.after...)
		if c1 != 0 || c1 != c2 || o1 != o2 || e1 != e2 {
			t.Errorf("%v vs %v: exit %d/%d\nstdout:\n%s\n---\n%s\nstderr: %q / %q", tc.before, tc.after, c1, c2, o1, o2, e1, e2)
		}
		if o1 == "" {
			t.Errorf("%v printed nothing", tc.before)
		}
	}
}

// Version is data, so it goes to stdout: `v=$(trafaret version)` must work.
func TestVersionGoesToStdout(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v1.2.3"
	chdirEmpty(t)

	for _, args := range [][]string{{"version"}, {"--version"}, {"-V"}, {"version", "extra"}} {
		code, out, errOut := cli(t, args...)
		if code != 0 || out != "v1.2.3\n" || errOut != "" {
			t.Errorf("%v: exit %d stdout %q stderr %q", args, code, out, errOut)
		}
	}
}

func TestEngineExempt(t *testing.T) {
	// Reachable with no config directory.
	for _, name := range []string{"trafaret", "version", "skill", "help"} {
		if !engineExempt(name) {
			t.Errorf("%s must run without a config directory", name)
		}
	}
	// Everything that reads a manifest must not be exempt.
	for _, name := range []string{"install", "diff", "list", "validate", "root", "serve"} {
		if engineExempt(name) {
			t.Errorf("%s must require a config directory", name)
		}
	}
	// ...and every registered command is classified one way or the other.
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" {
			continue
		}
		switch c.Name() {
		case "version", "skill", "install", "diff", "list", "validate", "root", "serve":
		default:
			t.Errorf("command %q is not covered by the exemption test", c.Name())
		}
	}
}

func TestVersionSkillAndHelpNeedNoManifest(t *testing.T) {
	chdirEmpty(t)

	for _, args := range [][]string{{"version"}, {"skill"}, {"help"}, {"help", "anything"}, {"-h"}, {"--help"}, {"install", "--help"}} {
		code, out, errOut := cli(t, args...)
		if code != 0 || out == "" || errOut != "" {
			t.Errorf("%v: exit %d, stdout %d bytes, stderr %q", args, code, len(out), errOut)
		}
	}
}

// Every way to ask for help prints the one usage screen, on stdout.
func TestHelpFormsPrintTheUsageScreen(t *testing.T) {
	chdirEmpty(t)

	for _, args := range [][]string{{"help"}, {"help", "foo"}, {"help", "install"}, {"-h"}, {"--help"}, {"diff", "-h"}} {
		code, out, errOut := cli(t, args...)
		if code != 0 || out != usageText || errOut != "" {
			t.Errorf("%v: exit %d, stderr %q, stdout is usage: %v", args, code, errOut, out == usageText)
		}
	}
}

func TestUsageNamesEveryCommandAndFlag(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" {
			continue
		}
		if !strings.Contains(usageText, "trafaret "+c.Name()) {
			t.Errorf("usage does not list the %q command", c.Name())
		}
	}
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "version" || f.Name == "addr" {
			return // -V/--version is an alias of the version command; --addr is shown with serve
		}
		if !strings.Contains(usageText, "--"+f.Name) {
			t.Errorf("usage does not list --%s", f.Name)
		}
	})
	if !strings.Contains(usageText, "--addr") {
		t.Error("usage does not list --addr")
	}
}

func TestNoArgumentsPrintsUsageAndExits2(t *testing.T) {
	chdirEmpty(t)

	code, out, errOut := cli(t)
	if code != 2 || out != usageText || errOut != "" {
		t.Errorf("exit %d, stderr %q, stdout is usage: %v", code, errOut, out == usageText)
	}
	// a flag alone is still "no command", and must not look for a config
	code, out, errOut = cli(t, "--config", "/nonexistent")
	if code != 2 || out != usageText || errOut != "" {
		t.Errorf("flag only: exit %d, stderr %q, stdout is usage: %v", code, errOut, out == usageText)
	}
}

func TestUnknownCommandExits2WithUsage(t *testing.T) {
	chdirEmpty(t)

	code, out, errOut := cli(t, "bogus")
	if code != 2 || errOut != "unknown command \"bogus\"\n" || out != usageText {
		t.Errorf("exit %d, stderr %q, stdout is usage: %v", code, errOut, out == usageText)
	}
}

func TestFlagErrorsKeepTheirWording(t *testing.T) {
	chdirEmpty(t)

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"validate", "--bogus"}, "trafaret: unknown flag: --bogus\n"},
		{[]string{"validate", "-x"}, "trafaret: unknown flag: -x\n"},
		{[]string{"validate", "--config"}, "trafaret: missing value for flag\n"},
		{[]string{"serve", "--addr"}, "trafaret: missing value for flag\n"},
	} {
		code, out, errOut := cli(t, tc.args...)
		if code != 1 || out != "" || errOut != tc.want {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want stderr %q", tc.args, code, out, errOut, tc.want)
		}
	}
}

func TestMissingManifestIsAnErrorOnStderr(t *testing.T) {
	chdirEmpty(t)

	for _, name := range []string{"validate", "list", "root", "install", "diff", "serve"} {
		code, out, errOut := cli(t, name)
		if code != 1 || out != "" || !strings.HasPrefix(errOut, "trafaret: no manifest.yaml found in ") ||
			!strings.HasSuffix(errOut, "pass --config DIR or set $TRAFARET_CONFIG\n") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, out, errOut)
		}
	}
	code, _, errOut := cli(t, "--config", "/nonexistent", "validate")
	if code != 1 || errOut != "trafaret: --config /nonexistent: not a directory\n" {
		t.Errorf("bad --config: exit %d, stderr %q", code, errOut)
	}
}

func TestInstallNeedsANameOrAll(t *testing.T) {
	cfg, _ := writeConfig(t)

	for _, name := range []string{"install", "diff"} {
		code, out, errOut := cli(t, name, "--config", cfg)
		if code != 1 || out != "" || errOut != "trafaret: install: name a project or home, or pass --all\n" {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, out, errOut)
		}
	}
	code, _, errOut := cli(t, "install", "nope", "--config", cfg)
	if code != 1 || errOut != "trafaret: no such project or home: nope\n" {
		t.Errorf("unknown project: exit %d, stderr %q", code, errOut)
	}
	code, _, errOut = cli(t, "install", "demo", "--target", "bogus", "--config", cfg)
	if code != 1 || errOut != "trafaret: invalid --target (claude|cursor|codex|both|all): bogus\n" {
		t.Errorf("bad target: exit %d, stderr %q", code, errOut)
	}
}

func TestInstallThenDiffIsInSync(t *testing.T) {
	cfg, repo := writeConfig(t)

	code, out, errOut := cli(t, "validate", "--config", cfg)
	if code != 0 || out != "ok: 1 project(s), 0 home(s) valid\n" || errOut != "" {
		t.Fatalf("validate: exit %d, stdout %q, stderr %q", code, out, errOut)
	}

	code, out, _ = cli(t, "diff", "demo", "--config", cfg)
	if code != 0 || !strings.HasPrefix(out, "demo: 1 install, 0 update, 0 skip, 0 remove\n") {
		t.Fatalf("diff before install: exit %d, stdout %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("diff must not write, stat err = %v", err)
	}

	code, out, _ = cli(t, "install", "demo", "--dry-run", "--json", "--config", cfg)
	if code != 0 || !strings.Contains(out, `"project": "demo"`) || !strings.Contains(out, `"status": "install"`) {
		t.Fatalf("dry-run json: exit %d, stdout %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("--dry-run must not write, stat err = %v", err)
	}

	code, out, _ = cli(t, "install", "demo", "--config", cfg)
	if code != 0 || !strings.HasPrefix(out, "demo: 1 install, 0 update, 0 skip, 0 remove\n") {
		t.Fatalf("install: exit %d, stdout %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude", "skills", "example-db", "SKILL.md")); err != nil {
		t.Fatalf("skill not written: %v", err)
	}

	code, out, _ = cli(t, "diff", "demo", "--config", cfg)
	if code != 0 || out != "demo: 0 install, 0 update, 1 skip, 0 remove\n" {
		t.Fatalf("diff after install: exit %d, stdout %q", code, out)
	}
	code, out, _ = cli(t, "diff", "demo", "--verbose", "--config", cfg)
	if code != 0 || !strings.Contains(out, "  skip    ") {
		t.Fatalf("--verbose must list skipped files: %q", out)
	}
}

func TestValidateReportsProblemsOnStdoutAndExits1(t *testing.T) {
	cfg, _ := writeConfig(t)
	bad := "projects:\n  broken:\n    bundles: [no-such-bundle]\n"
	if err := os.WriteFile(filepath.Join(cfg, "manifest.yaml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, errOut := cli(t, "validate", "--config", cfg)
	if code != 1 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"  broken           missing repoRoot\n", `bundle "no-such-bundle" is not in templates/bundles.yaml`, "problem(s)\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("validate output lacks %q:\n%s", want, out)
		}
	}
}

func TestEngineGateReadsTheStampedVersion(t *testing.T) {
	cfg, _ := writeConfig(t)
	manifest, err := os.ReadFile(filepath.Join(cfg, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "manifest.yaml"), append([]byte("engine: \">=0.2.0\"\n"), manifest...), 0o644); err != nil {
		t.Fatal(err)
	}
	old := version
	t.Cleanup(func() { version = old })

	version = "v0.1.0"
	code, _, errOut := cli(t, "validate", "--config", cfg)
	if code != 1 || !strings.Contains(errOut, "manifest requires trafaret >=0.2.0, this binary is v0.1.0") {
		t.Errorf("an old binary must be refused: exit %d, stderr %q", code, errOut)
	}
	version = "v0.2.0"
	if code, _, errOut := cli(t, "validate", "--config", cfg); code != 0 {
		t.Errorf("a satisfying binary must pass: exit %d, stderr %q", code, errOut)
	}
	version = "dev"
	if code, _, errOut := cli(t, "validate", "--config", cfg); code != 0 {
		t.Errorf("an unstamped build always passes: exit %d, stderr %q", code, errOut)
	}
}

func TestListShowsProjectsAndJSON(t *testing.T) {
	cfg, _ := writeConfig(t)

	code, out, _ := cli(t, "list", "--config", cfg)
	if code != 0 || !strings.HasPrefix(out, "PROJECT          TARGETS    TICKETS  BUNDLES\n") ||
		!strings.Contains(out, "demo             claude") || !strings.Contains(out, "example-db") {
		t.Errorf("list: exit %d, stdout:\n%s", code, out)
	}
	code, out, _ = cli(t, "list", "--json", "--config", cfg)
	if code != 0 || !strings.Contains(out, `"demo": {`) || !strings.Contains(out, `"Bundles": [`) {
		t.Errorf("list --json: exit %d, stdout:\n%s", code, out)
	}
}

func TestRootPrintsTheConfigDirectory(t *testing.T) {
	cfg, _ := writeConfig(t)

	code, out, errOut := cli(t, "root", "--config", cfg)
	if code != 0 || out != cfg+"\n" || errOut != "" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

// The skill reference is embedded and must keep up with the CLI surface.
func TestSkillPrintsTheEmbeddedReference(t *testing.T) {
	chdirEmpty(t)

	code, out, errOut := cli(t, "skill")
	if code != 0 || errOut != "" || out != skillDoc || !strings.HasPrefix(out, "# trafaret") {
		t.Fatalf("exit %d, stderr %q, matches embedded doc: %v", code, errOut, out == skillDoc)
	}
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" {
			continue
		}
		if !strings.Contains(skillDoc, "`"+c.Name()) {
			t.Errorf("skill.md does not document the %q command", c.Name())
		}
	}
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if !strings.Contains(skillDoc, "`--"+f.Name) {
			t.Errorf("skill.md does not document --%s", f.Name)
		}
	})
}
