package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is stamped at build time by the Makefile and by GoReleaser:
//
//	-ldflags "-X github.com/iamnikolie/trafaret/cmd.version=v1.2.3"
//
// An unstamped build is "dev", which satisfies every manifest `engine:`
// constraint. It is also the version the engine gate checks manifests against.
var version = "dev"

// A binary built without ldflags still knows its module version: `go install
// github.com/iamnikolie/trafaret@vX.Y.Z` records vX.Y.Z, and a build inside a git
// checkout records a pseudo-version (v0.3.1-0.<date>-<rev>[+dirty]). Use it
// instead of "dev" so the printed version and the engine gate are real. Only a
// build with no module or VCS information stays "dev".
func init() {
	if version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && moduleVersion(info.Main.Version) != "" {
		version = moduleVersion(info.Main.Version)
	}
}

func moduleVersion(v string) string {
	if v == "" || v == "(devel)" {
		return ""
	}
	return v
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the binary version",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Version is data, so it goes to stdout: `v=$(trafaret version)` must work.
		fmt.Fprintln(stdout, version)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
