package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is stamped at build time by the Makefile and by GoReleaser:
//
//	-ldflags "-X github.com/iamnikolie/trafaret/cmd.version=v1.2.3"
//
// An unstamped build is "dev", which satisfies every manifest `engine:`
// constraint. It is also the version the engine gate checks manifests against.
var version = "dev"

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
