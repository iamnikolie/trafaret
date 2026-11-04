package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configDirCmd = &cobra.Command{
	Use:   "root",
	Short: "Print the config directory (the one holding manifest.yaml)",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(stdout, eng.ConfigDir)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configDirCmd)
}
