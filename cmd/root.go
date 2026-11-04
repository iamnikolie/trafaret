// Package cmd implements the trafaret command line.
//
//	trafaret install <project|home> [--all] [--target claude|cursor|codex|both|all] [--what bundle,...] [--force] [--dry-run] [--json]
//	trafaret list [--json]
//	trafaret validate
//	trafaret diff <project|home>       # dry-run that prints per-file status
//	trafaret root                      # print the config directory
//	trafaret serve [--addr host:port]  # local live manual
//	trafaret skill | version | help
//
// Config directory: --config DIR, else $TRAFARET_CONFIG, else the nearest parent
// of the working directory that holds a manifest.yaml.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/iamnikolie/trafaret/internal/engine"
	"github.com/spf13/cobra"
)

// Flags shared by every command. They are persistent on the root so they work
// before and after the command name: `trafaret --config DIR diff x` and
// `trafaret diff x --config DIR` are the same invocation.
var (
	allFlag       bool
	forceFlag     bool
	dryRunFlag    bool
	jsonFlag      bool
	verboseFlag   bool
	versionFlag   bool
	targetFlag    string
	whatFlag      string
	configFlag    string
	manifestFlag  string
	templatesFlag string
	addrFlag      string

	// eng is the engine of this invocation, built before any command that
	// reads a manifest runs.
	eng *engine.Engine
)

// stdout and stderr are indirected so tests can capture them.
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// exitError makes Execute exit with a code and print nothing further: the
// command already said what it had to say (a usage screen, a validate report).
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

var rootCmd = &cobra.Command{
	Use:   "trafaret",
	Short: "Write a skill, agent or rule once; render it into every harness",
	Long: `trafaret — write a skill, agent or rule once; render it into every harness.

One library of templates plus free vars in a manifest.yaml, rendered into
Claude Code, Codex and Cursor — for per-repo projects and for user-level homes
(~/.claude, ~/.codex, ~/.cursor) through one pipeline. Writes are
sha256-idempotent.

Run 'trafaret skill' for the full agent reference.`,
	// Anything that is not a known command lands in RunE, which reports it the
	// way the CLI always has (usage + exit 2) instead of cobra's wording.
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag {
			fmt.Fprintln(stdout, version)

			return exitError{0}
		}

		if engineExempt(cmd.Name()) {
			return nil
		}

		var err error

		eng, err = engine.New(engine.Options{
			Config:    configFlag,
			Manifest:  manifestFlag,
			Templates: templatesFlag,
			Version:   version,
		})

		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		}

		usage()

		return exitError{2}
	},
}

// engineExempt lists the commands that run without a config directory: the
// root itself (which only prints usage), `version`, `skill` and `help`.
func engineExempt(name string) bool {
	switch name {
	case "trafaret", "version", "skill", "help":
		return true
	}

	return false
}

// flagError rewords pflag's parse errors to what the CLI has always printed.
func flagError(cmd *cobra.Command, err error) error {
	msg := err.Error()

	switch {
	case strings.HasPrefix(msg, "flag needs an argument"):
		return errors.New("missing value for flag")
	case strings.HasPrefix(msg, "unknown shorthand flag:"):
		// "unknown shorthand flag: 'x' in -x"
		if i := strings.LastIndex(msg, " in "); i >= 0 {
			return errors.New("unknown flag: " + msg[i+len(" in "):])
		}
	}

	return err
}

// run executes the command line and returns the process exit code.
func run(args []string) int {
	rootCmd.SetArgs(args)

	err := rootCmd.ExecuteContext(context.Background())
	if err == nil {
		return 0
	}

	var ee exitError
	if errors.As(err, &ee) {
		return ee.code
	}

	fmt.Fprintln(stderr, "trafaret: "+err.Error())

	return 1
}

// Execute runs the root command and exits.
func Execute() {
	os.Exit(run(os.Args[1:]))
}

// splitCSV splits a comma-separated flag value, dropping blanks.
func splitCSV(s string) []string {
	var out []string

	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}

	return out
}

// lastArg is the positional argument: when several are given the last wins.
func lastArg(args []string) string {
	if len(args) == 0 {
		return ""
	}

	return args[len(args)-1]
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetFlagErrorFunc(flagError)

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&configFlag, "config", "", "config directory: holds manifest.yaml, templates/, global/, agents/, projects/; or $TRAFARET_CONFIG")
	pf.StringVar(&manifestFlag, "manifest", "", "manifest file; its directory becomes the config directory")
	pf.StringVar(&templatesFlag, "templates", "", "templates overlay directory (default: <config>/templates)")
	pf.StringVar(&targetFlag, "target", "", "override the harnesses: claude|cursor|codex|both|all (homes: only those of that harness)")
	pf.StringVar(&whatFlag, "what", "", "install only these bundles (comma-separated; skips pruning)")
	pf.StringVar(&addrFlag, "addr", "", "serve: listen address (default 127.0.0.1:4477)")
	pf.BoolVar(&allFlag, "all", false, "install every project and every home")
	pf.BoolVar(&forceFlag, "force", false, "rewrite even unchanged (sha256-equal) files")
	pf.BoolVar(&dryRunFlag, "dry-run", false, "render but do not write")
	pf.BoolVar(&jsonFlag, "json", false, "machine-readable output")
	pf.BoolVarP(&verboseFlag, "verbose", "v", false, "include skipped files in output")
	pf.BoolVarP(&versionFlag, "version", "V", false, "print the binary version")
}
