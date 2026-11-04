package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iamnikolie/trafaret/internal/engine"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install [project|home]",
	Short: "Render and write Claude, Codex and Cursor artifacts",
	Long: `Renders a project's skills, agents and rules (its own lists plus its active
bundles) into the repo, pruning artifacts of bundles moved to disabled:. A home
name does the same into that home's harness directory (skills/, agents/, rules/).
--all installs every project and every home in the manifest.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(args, false)
	},
}

var diffCmd = &cobra.Command{
	Use:   "diff <project|home>",
	Short: "Dry-run: print per-file status (install/update/skip/remove)",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInstall(args, true)
	},
}

// runInstall is install and diff: diff is install that never writes.
func runInstall(args []string, alwaysDry bool) error {
	dry := dryRunFlag || alwaysDry

	all, err := eng.Install(engine.InstallOptions{
		All:    allFlag,
		Name:   lastArg(args),
		Force:  forceFlag,
		DryRun: dry,
		Target: targetFlag,
		What:   splitCSV(whatFlag),
	})
	if err != nil {
		return err
	}

	if jsonFlag {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")

		_ = enc.Encode(all)

		return nil
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

		fmt.Fprintf(stdout, "%s: %d install, %d update, %d skip, %d remove\n", pr.Project, ins, upd, skip, rm)

		for _, f := range pr.Files {
			if f.Status == "skip" && !verboseFlag {
				continue
			}

			fmt.Fprintf(stdout, "  %-7s %s\n", f.Status, rel(f.Path))
		}
	}

	return nil
}

// rel shows a path relative to the working directory when it lies below it.
func rel(p string) string {
	if wd, err := os.Getwd(); err == nil {
		if r, err := filepath.Rel(wd, p); err == nil && !strings.HasPrefix(r, "..") {
			return r
		}
	}

	return p
}

func init() {
	rootCmd.AddCommand(installCmd, diffCmd)
}
