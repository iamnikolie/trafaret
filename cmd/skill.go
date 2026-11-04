package cmd

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed skill.md
var skillDoc string

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print the agent reference for this CLI",
	Long:  "Prints the full trafaret reference — commands, flags, the config directory, the manifest and vars, templates, harness adapters and bundles.yaml — for use as agent context.",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprint(stdout, skillDoc)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(skillCmd)
}
