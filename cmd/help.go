package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// usageText is the one help screen: `trafaret`, `trafaret help`, `-h`/`--help`
// and `<command> --help` all print it. It is a hand-written overview rather
// than cobra's generated listing, so it also explains the flags that belong to
// no single command.
const usageText = `trafaret — write a skill, agent or rule once; render it into every harness

Usage:
  trafaret install <project> [flags]   render + write the project's skills, agents and rules for each
                                       of its harnesses (also prunes artifacts of bundles moved to
                                       disabled: — tracked via .claude/ao-manifest.json in the repo)
  trafaret install --all [flags]       install every project and every home in the manifest
  trafaret install <home>              render + write into one home's harness dir (homes: in
                                       manifest.yaml — e.g. ~/.claude, ~/.codex, ~/.cursor)
  trafaret diff <project|home>         dry-run: print per-file status (install/update/skip/remove)
  trafaret list [--json]               list projects and homes in the manifest
  trafaret validate                    check the manifest + render every template
  trafaret root                        print the config directory (the one holding manifest.yaml)
  trafaret serve [--addr host:port]    local live manual: docs site rendered from the
                                       config directory on every refresh (default 127.0.0.1:4477)
  trafaret skill                       print the agent-facing reference for this CLI
  trafaret version                     print the binary version

Flags:
  --config DIR                   config directory: holds manifest.yaml, templates/, global/, agents/,
                                 projects/ (default: $TRAFARET_CONFIG, else the nearest parent of the
                                 working directory that has a manifest.yaml)
  --manifest PATH                manifest file; its directory becomes the config directory
  --templates PATH               templates overlay directory (default: <config>/templates, optional).
                                 A file there replaces the built-in template with the same path
  --target claude|cursor|codex|both|all
                                 override the harnesses of a project; a home is installed only when
                                 its harness is among them
  --what bundle[,bundle]         install only these bundles (subset of the entry's; skips pruning)
  --force                        rewrite even unchanged (sha256-equal) files
  --dry-run                      render but do not write
  --verbose                      include skipped files in output
  --json                         machine-readable output
`

// usage prints the help screen to stdout.
func usage() { fmt.Fprint(stdout, usageText) }

// helpCmd replaces cobra's generated help command: `trafaret help <anything>`
// prints the same screen as `trafaret help`.
var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Print usage",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		usage()
	},
}

func init() {
	rootCmd.SetHelpCommand(helpCmd)
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) { usage() })
}
